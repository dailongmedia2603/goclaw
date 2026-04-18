// Package facebookmessenger provides a GoClaw channel for Facebook Messenger personal
// chat automation, powered by an external mautrix-meta sidecar (AGPL-3.0, separate process).
//
// IMPORTANT: this package must NOT import mautrix/meta as a Go library —
// doing so would pull AGPL-3.0 into the GoClaw binary. All communication with
// the sidecar is via HTTP (localhost or cluster-internal network).
package facebookmessenger

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/channels"
)

// Compile-time interface assertions. Catch upstream interface changes at build time.
var (
	_ channels.Channel        = (*Channel)(nil)
	_ channels.WebhookChannel = (*Channel)(nil)
)

const (
	// webhookBodyLimit caps inbound webhook body size to mitigate resource exhaustion.
	webhookBodyLimit = 4 << 20 // 4 MiB

	// healthCheckInterval is how often we ping the sidecar /healthz when running.
	healthCheckInterval = 30 * time.Second

	// healthCheckTimeout caps a single /healthz ping so it can't block the loop.
	healthCheckTimeout = 10 * time.Second

	apiVersionHeader = "X-Fbm-Api-Version"
	apiVersion       = "v1"
	signatureHeader  = "X-Fbm-Signature"
)

// Channel implements channels.Channel and channels.WebhookChannel for personal
// Facebook Messenger chat (not Fanpage — that's the `facebook` channel).
type Channel struct {
	*channels.BaseChannel

	creds *Credentials
	cfg   *Config

	mu       sync.Mutex
	started  bool
	stopCh   chan struct{}
	client   *sidecarClient
	healthOK atomic.Bool

	// rateLimiter is populated by Phase 5. Phase 2 leaves it nil (Send has no
	// limiter) — left as a hook point so Phase 5 can wire without touching Send.
	rateLimiter outboundGate
}

// outboundGate is the narrow interface the Channel uses to rate-limit outbound sends.
// Phase 2 defaults to nopGate (always allow). Phase 5 swaps in OutboundRateLimiter.
type outboundGate interface {
	Wait(ctx context.Context) error
}

type nopGate struct{}

func (nopGate) Wait(_ context.Context) error { return nil }

// New constructs a Channel instance. Does not open connections or start goroutines.
// The outbound rate limiter is auto-initialized from cfg.RateLimitPerMin.
func New(name string, creds *Credentials, cfg *Config, msgBus *bus.MessageBus) *Channel {
	c := &Channel{
		BaseChannel: channels.NewBaseChannel(name, msgBus, nil),
		creds:       creds,
		cfg:         cfg,
		stopCh:      make(chan struct{}),
	}
	if cfg != nil && cfg.RateLimitPerMin > 0 {
		c.rateLimiter = NewOutboundRateLimiter(cfg.RateLimitPerMin)
	} else {
		c.rateLimiter = nopGate{}
	}
	c.SetType(channels.TypeFacebookPersonal)
	return c
}

// SetRateLimiter injects a rate limiter for outbound Send calls.
// Called by Phase 5 hardening path. Safe to call before or after Start.
func (c *Channel) SetRateLimiter(rl outboundGate) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if rl == nil {
		c.rateLimiter = nopGate{}
		return
	}
	c.rateLimiter = rl
}

// Start opens the sidecar connection and kicks off a health monitor goroutine.
// The initial health check failure does NOT fail Start — we want the gateway
// to stay up even if the sidecar is temporarily unreachable; the health loop
// will retry and flip the health state accordingly.
func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return nil
	}
	c.client = newSidecarClient(c.creds.SidecarURL, c.creds.AuthToken)
	c.started = true
	c.stopCh = make(chan struct{}) // fresh channel per Start so Stop/Start cycles are supported
	c.mu.Unlock()

	// Initial health probe with bounded timeout (doesn't block startup).
	probeCtx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()
	if err := c.client.Health(probeCtx); err != nil {
		c.healthOK.Store(false)
		slog.Warn("facebook_personal.health.initial_failed",
			"channel", c.Name(), "err", err)
		c.MarkDegraded("Sidecar unreachable", err.Error(), channels.ChannelFailureKindNetwork, true)
	} else {
		c.healthOK.Store(true)
		c.MarkHealthy("Connected to sidecar")
	}

	go c.healthLoop(ctx)
	return nil
}

// Stop signals goroutines to exit. Idempotent.
func (c *Channel) Stop(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.started {
		return nil
	}
	c.started = false
	select {
	case <-c.stopCh:
		// already closed
	default:
		close(c.stopCh)
	}
	return nil
}

// IsRunning reports whether Start has been called and Stop hasn't.
func (c *Channel) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.started
}

// Send posts the outbound message to the sidecar. Respects rate limiter (Phase 5).
func (c *Channel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	c.mu.Lock()
	client := c.client
	limiter := c.rateLimiter
	started := c.started
	c.mu.Unlock()

	if !started || client == nil {
		return ErrNotStarted
	}

	if err := limiter.Wait(ctx); err != nil {
		IncRateLimited()
		return err
	}

	media := make([]mediaUpload, 0, len(msg.Media))
	for _, m := range msg.Media {
		if m.URL == "" {
			continue
		}
		media = append(media, mediaUpload{
			URL:         m.URL,
			ContentType: m.ContentType,
			Caption:     m.Caption,
		})
	}

	req := sendRequest{
		ChatID:  msg.ChatID,
		Content: msg.Content,
		Media:   media,
	}
	if replyTo := msg.Metadata["fbm_reply_to"]; replyTo != "" {
		req.ReplyTo = replyTo
	}

	_, err := client.Send(ctx, req)
	if err == nil {
		IncOutbound()
	}
	return err
}

// WebhookHandler returns the path + handler to mount on the gateway HTTP mux.
func (c *Channel) WebhookHandler() (string, http.Handler) {
	return c.webhookPath(), http.HandlerFunc(c.handleWebhook)
}

// webhookPath is deterministic per instance — sidecar can be configured once.
func (c *Channel) webhookPath() string {
	return "/channels/facebook_personal/" + c.Name() + "/webhook"
}

// IsHealthy reflects the last /healthz ping result. Used by status reporter.
func (c *Channel) IsHealthy() bool {
	return c.healthOK.Load()
}

// --- Internal ---

func (c *Channel) healthLoop(ctx context.Context) {
	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
			err := c.client.Health(pingCtx)
			cancel()
			if err != nil {
				if c.healthOK.Swap(false) {
					slog.Warn("facebook_personal.health.disconnected",
						"channel", c.Name(), "err", err)
					c.MarkDegraded("Sidecar unreachable", err.Error(), channels.ChannelFailureKindNetwork, true)
				}
			} else {
				if !c.healthOK.Swap(true) {
					slog.Info("facebook_personal.health.reconnected", "channel", c.Name())
				}
				c.MarkHealthy("Connected to sidecar")
			}
		}
	}
}

// handleWebhook processes inbound events from the sidecar.
// Steps:
//  1. Method + body-size check
//  2. API version header check
//  3. HMAC signature verification
//  4. JSON parse + event mapping
//  5. IsAllowed filter
//  6. Publish to bus
func (c *Channel) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Body limit: MaxBytesReader caps read and sets an error on overflow.
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, webhookBodyLimit))
	if err != nil {
		slog.Warn("facebook_personal.webhook.body_read_failed",
			"channel", c.Name(), "err", err)
		http.Error(w, "body read failed", http.StatusBadRequest)
		return
	}

	// Enforce API version. Mismatch = reject so we don't silently misinterpret.
	if v := r.Header.Get(apiVersionHeader); v != "" && v != apiVersion {
		slog.Warn("facebook_personal.webhook.api_version_mismatch",
			"channel", c.Name(), "got", v, "want", apiVersion)
		http.Error(w, "unsupported api version", http.StatusBadRequest)
		return
	}

	// HMAC signature.
	if err := VerifyWebhookSignature(body, r.Header.Get(signatureHeader), c.creds.WebhookSecret, nil); err != nil {
		IncSignatureFail()
		slog.Warn("security.facebook_personal.webhook.signature_failed",
			"channel", c.Name(), "err", err)
		http.Error(w, "signature invalid", http.StatusUnauthorized)
		return
	}

	var event sidecarInboundEvent
	if err := json.Unmarshal(body, &event); err != nil {
		slog.Warn("facebook_personal.webhook.bad_json",
			"channel", c.Name(), "err", err)
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	msg, err := mapEventToInbound(event, c.TenantID(), c.AgentID(), c.Name())
	if err != nil {
		// Non-message events are acknowledged with 202 so sidecar doesn't retry.
		if errors.Is(err, ErrNotAMessage) {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		slog.Warn("facebook_personal.webhook.event_invalid",
			"channel", c.Name(), "err", err, "event_type", event.EventType)
		http.Error(w, "event invalid", http.StatusBadRequest)
		return
	}

	// DM / group policy (pairing / allowlist / disabled / open).
	// For pairing, this creates a pairing record visible in the admin UI and
	// sends a short prompt to the sender so they know the request is pending.
	if event.IsGroup {
		if !c.checkGroupPolicy(r.Context(), msg.SenderID, msg.ChatID) {
			w.WriteHeader(http.StatusAccepted)
			return
		}
	} else {
		if !c.checkDMPolicy(r.Context(), msg.SenderID, msg.ChatID) {
			w.WriteHeader(http.StatusAccepted)
			return
		}
	}

	// Sender allowlist (BaseChannel) check.
	if !c.IsAllowed(msg.SenderID) {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	c.publishInbound(msg)
	IncInbound()
	w.WriteHeader(http.StatusNoContent)
}

// publishInbound pushes to the bus. Indirected through a method so tests can
// swap the bus without reaching into the struct.
func (c *Channel) publishInbound(msg bus.InboundMessage) {
	if b := c.Bus(); b != nil {
		b.PublishInbound(msg)
	}
}
