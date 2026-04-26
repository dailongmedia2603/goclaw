//go:build !sqliteonly

package cmd

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/go-rod/rod"
	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/channels/fbcloak"
	"github.com/nextlevelbuilder/goclaw/internal/channels/fbproactive"
	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/edition"
	"github.com/nextlevelbuilder/goclaw/internal/eventbus"
	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/gateway/methods"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/internal/store/pg"
	"github.com/nextlevelbuilder/goclaw/pkg/browser"
)

// wireFBCloak wires the full fbcloak feature into the gateway:
//   - Phase 1+2: credential CRUD, job CRUD, log filters, screenshot URL RPC.
//   - Phase 3: metrics, tracing, screenshot writer, killswitch hot-reload,
//     event publisher, checkpoint detector inspector.
//   - Phase 3c: rod-backed SessionFactory + JobRunner started.
//   - Phase 4: disclaimer ack, dual-mode router (FBProactiveRouter), full
//     RPC handlers for all of the above.
//
// Short-circuits when the edition has FBCloakEnabled=false (typically Lite,
// where this file is replaced by the no-op stub via build tags).
func wireFBCloak(server *gateway.Server, pgStores *store.Stores, cfg *config.Config, domainBus eventbus.DomainEventBus) {
	if !edition.Current().FBCloakEnabled {
		slog.Info("fbcloak: feature disabled by edition; skipping wire")
		return
	}
	if pgStores == nil || pgStores.DB == nil {
		slog.Warn("fbcloak: no PG database; skipping wire")
		return
	}

	encKey := os.Getenv("GOCLAW_ENCRYPTION_KEY")
	credStore := pg.NewPGFBCloakCredentialStore(pgStores.DB, encKey)
	jobStore := pg.NewPGFBCloakJobStore(pgStores.DB)
	disclaimerStore := pg.NewPGFBCloakDisclaimerStore(pgStores.DB)
	lastInboundResolver := pg.NewPGLastInboundResolver(pgStores.DB)

	// Phase 3 screenshot writer. Path defaults to {dataDir}/fbcloak/screenshots.
	retention := time.Duration(cfg.Channels.FBCloak.ScreenshotRetentionDays) * 24 * time.Hour
	shot, sErr := fbcloak.NewScreenshotWriter(cfg.DataDir, cfg.Channels.FBCloak.ScreenshotDir, retention)
	if sErr != nil {
		slog.Warn("fbcloak: screenshot writer disabled", "err", sErr)
		shot = nil
	}

	// Phase 3 event publisher. domainBus is the same DomainEventBus the
	// rest of the gateway uses; subscribers (admin Telegram alerts, etc.)
	// can listen for fbcloak.* events through one bus.
	var events fbcloak.EventPublisher
	if domainBus != nil {
		events = domainBus
	}

	// Phase 3c: per-fbcloak browser.Manager. We do NOT reuse the agent's
	// browser tool registry — fbcloak needs its own incognito-per-credential
	// lifecycle independent of the agent's shared browser pool. Connects to
	// the same Chrome sidecar (env GOCLAW_BROWSER_REMOTE_URL) when set,
	// otherwise launches a local headless Chrome.
	browserMgr := buildFBCloakBrowserManager(cfg)

	// Health probe (Phase 1) — uses the same browser manager so the cookie
	// inject + redirect check share the production stack.
	probe := &fbcloak.HealthProbe{
		NewBrowser: func(ctx context.Context, _ fbcloak.Credential) (*rod.Browser, func(), error) {
			b, err := browserMgr.NewIncognitoContext(ctx)
			if err != nil {
				return nil, func() {}, err
			}
			return b, func() { _ = b.Close() }, nil
		},
	}

	svc, err := fbcloak.NewService(fbcloak.Deps{
		CredentialStore: credStore,
		HealthProbe:     probe,
		JobStore:        jobStore,
		Disclaimer:      disclaimerStore,
		Screenshot:      shot,
		Events:          events,
		Logger:          slog.Default(),
	})
	if err != nil {
		slog.Error("fbcloak: failed to construct service; feature disabled", "err", err)
		return
	}

	// Phase 3c: SessionFactory + JobRunner.
	sessionFactory := &fbcloak.RodSessionFactory{
		BrowserMgr: browserMgr,
		Writer:     shot,
	}
	resolver := fbcloak.NewResolver(pgStores.DB)
	policy := fbcloak.NewPolicy(fbcloak.DefaultPolicyConfig(), jobStore)
	humanizer := fbcloak.NewHumanizer(time.Now().UnixNano(), fbcloak.DefaultHumanizeConfig())
	templateRenderer := &fbcloak.SimpleTemplateRenderer{}
	maxConcurrent := cfg.Channels.FBCloak.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}
	runner := &fbcloak.JobRunnerImpl{
		Store:                  jobStore,
		Credentials:            credStore,
		Resolver:               resolver,
		Policy:                 policy,
		Humanizer:              humanizer,
		SessionFactory:         sessionFactory,
		TemplateRender:         templateRenderer,
		Logger:                 slog.Default(),
		TickInterval:           60 * time.Second,
		MaxConcurrent:          maxConcurrent,
		Killswitch:             svc.KillswitchFlag(),
		Events:                 events,
		Screenshot:             shot,
		CheckpointInspectorFor: fbcloak.PageSessionToInspector,
	}
	if startErr := runner.Start(context.Background()); startErr != nil {
		slog.Warn("fbcloak: job runner start failed; RunNow disabled", "err", startErr)
	} else {
		slog.Info("fbcloak: job runner started", "tick", "60s", "max_concurrent", maxConcurrent)
	}
	svc.SetJobRunner(runner)

	// Phase 3 killswitch watcher (hot-reload of GOCLAW_FBCLOAK_KILLSWITCH).
	pollSec := cfg.Channels.FBCloak.KillswitchPollSeconds
	if pollSec == 0 {
		pollSec = 30
	}
	if pollSec > 0 {
		watcher, wErr := fbcloak.NewKillswitchWatcher(
			svc.KillswitchFlag(),
			time.Duration(pollSec)*time.Second,
			slog.Default(),
		)
		if wErr != nil {
			slog.Warn("fbcloak: killswitch watcher init failed", "err", wErr)
			if os.Getenv("GOCLAW_FBCLOAK_KILLSWITCH") == "1" {
				svc.SetKillswitch(true)
			}
		} else {
			watcher.Start(context.Background())
			slog.Info("fbcloak: killswitch watcher started", "interval_sec", pollSec)
		}
	} else if os.Getenv("GOCLAW_FBCLOAK_KILLSWITCH") == "1" {
		svc.SetKillswitch(true)
		slog.Warn("security.fbcloak.killswitch_active",
			"reason", "GOCLAW_FBCLOAK_KILLSWITCH=1 at startup",
		)
	}

	// Phase 4 dual-mode router. Graph nil → ≤7d API path returns
	// ErrGraphSenderUnconfigured (intentional — fbm.Channel.SendViaGraph
	// is a separate sub-plan). Cloak path (>7d) is fully wired.
	router := &fbproactive.FBProactiveRouter{
		Resolver: lastInboundResolver,
		Graph:    nil,
		Cloak:    fbcloakRouterAdapter{svc: svc},
	}

	// RPC registration (Phase 1+2 + Phase 4 in one pass).
	rpc := methods.NewFBCloakMethods(svc, cfg)
	rpc.Register(server.Router())
	rpc.RegisterJobs(server.Router())
	phase4 := methods.NewFBCloakPhase4Methods(svc, router, cfg)
	phase4.Register(server.Router())

	slog.Info("fbcloak: registered RPC handlers (Phase 1-4)",
		"edition", edition.Current().Name,
		"disclaimer_version", fbcloak.CurrentDisclaimerVersion,
		"runner_started", runner != nil,
	)
}

// buildFBCloakBrowserManager constructs the dedicated browser manager.
// Honours the same env / sidecar conventions the agent's browser tool uses
// (GOCLAW_BROWSER_REMOTE_URL → ws://chrome:9222 in Docker), but a separate
// Manager instance so fbcloak's incognito contexts don't share state with
// agent sessions.
func buildFBCloakBrowserManager(cfg *config.Config) *browser.Manager {
	var opts []browser.Option
	if remote := os.Getenv("GOCLAW_BROWSER_REMOTE_URL"); remote != "" {
		opts = append(opts, browser.WithRemoteURL(remote))
	} else if _, err := os.Stat("/.dockerenv"); err == nil {
		opts = append(opts, browser.WithRemoteURL("ws://chrome:9222"))
	} else {
		opts = append(opts, browser.WithHeadless(true))
	}
	// Conservative defaults — fbcloak runs are short-lived but we don't
	// want a single stuck send to hold the action lock forever.
	opts = append(opts,
		browser.WithActionTimeout(45*time.Second),
		browser.WithIdleTimeout(10*time.Minute),
		browser.WithMaxPages(maxFBCloakConcurrentPages(cfg)),
	)
	return browser.New(opts...)
}

func maxFBCloakConcurrentPages(cfg *config.Config) int {
	if cfg.Channels.FBCloak.MaxConcurrent > 0 {
		return cfg.Channels.FBCloak.MaxConcurrent
	}
	return 5
}

// fbcloakRouterAdapter glues fbcloak.Service.SendProactive to the router's
// CloakSender contract. Resolving last_inbound_at is the router's job;
// here we just pass through with the synthetic-Job opts.
type fbcloakRouterAdapter struct {
	svc *fbcloak.Service
}

func (a fbcloakRouterAdapter) SendProactive(ctx context.Context, tenantID uuid.UUID, fanpageID, recipientPSID, message string, dryRun bool) (string, error) {
	id, err := a.svc.SendProactive(ctx, tenantID, fbcloak.SendProactiveOpts{
		FanpageID:     fanpageID,
		RecipientPSID: recipientPSID,
		Message:       message,
		DryRun:        dryRun,
	})
	if id == uuid.Nil {
		return "", err
	}
	return id.String(), err
}
