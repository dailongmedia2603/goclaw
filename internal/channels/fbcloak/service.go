//go:build !sqliteonly

package fbcloak

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/pkg/browser"
)

// Deps bundles the external collaborators a Service needs. Construct via
// NewService(deps); fields are validated at construction. JobStore and
// JobRunner are optional in Phase 1 (credential CRUD only) — set later via
// SetJobStore/SetJobRunner. Events / Screenshot are optional Phase-3
// observability hooks; nil → no-op.
type Deps struct {
	CredentialStore CredentialStore
	HealthProbe     *HealthProbe
	JobStore        JobStore  // Phase 2+
	JobRunner       JobRunner // Phase 2+
	Events          EventPublisher    // Phase 3+; nil → NoopPublisher
	Screenshot      *ScreenshotWriter // Phase 3+; nil → screenshots disabled
	Disclaimer      DisclaimerStore   // Phase 4+; nil → disclaimer ack always passes (dev mode)
	PlanStore       PlanStore         // Phase 5+; nil disables Plan-Based Brain Mode
	Logger          *slog.Logger
	DefaultUA       string
}

// Service is the package-level facade. Phase 1 only exposes credential CRUD
// + health probe; Phase 2 will add Job CRUD + scheduler; Phase 3 + 4 add
// observability and the dual-mode router.
type Service struct {
	deps       Deps
	killswitch atomic.Bool
}

// NewService constructs a Service. CredentialStore and Logger are required;
// HealthProbe is optional (nil disables fbcloak.test_credential).
func NewService(deps Deps) (*Service, error) {
	if deps.CredentialStore == nil {
		return nil, errors.New("fbcloak: CredentialStore is required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.DefaultUA == "" {
		deps.DefaultUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
	}
	if deps.Events == nil {
		deps.Events = NoopPublisher()
	}
	return &Service{deps: deps}, nil
}

// Events returns the wired publisher (NoopPublisher when none configured).
// Exposed so wired components like JobRunnerImpl can pull it without
// duplicating Deps state.
func (s *Service) Events() EventPublisher { return s.deps.Events }

// Screenshotter returns the configured ScreenshotWriter (nil when
// screenshots are disabled).
func (s *Service) Screenshotter() *ScreenshotWriter { return s.deps.Screenshot }

// KillswitchFlag returns a pointer to the atomic.Bool the killswitch
// watcher mutates. Lets external wiring (cmd/gateway.go) attach the
// watcher without exposing the whole Service struct.
func (s *Service) KillswitchFlag() *atomic.Bool { return &s.killswitch }

// SetKillswitch toggles a runtime kill flag. Phase 3 will additionally watch
// the GOCLAW_FBCLOAK_KILLSWITCH env var.
func (s *Service) SetKillswitch(on bool) {
	s.killswitch.Store(on)
}

// Killed reports whether the killswitch is currently engaged.
func (s *Service) Killed() bool { return s.killswitch.Load() }

// guard centralises the edition + killswitch check applied to every public
// entry. Returns nil when calls may proceed.
func (s *Service) guard() error {
	if !EditionAllowed() {
		return ErrFeatureDisabled
	}
	if s.killswitch.Load() {
		return ErrKillswitchActive
	}
	return nil
}

// AddCredential validates input, encrypts secrets via the store, and persists.
// The Credential returned has CookiesEnc/ProxyURLEnc set (encrypted at rest)
// and Cookies/ProxyURL cleared (no plaintext leak in API responses).
func (s *Service) AddCredential(ctx context.Context, tenantID uuid.UUID, in CreateCredentialInput) (Credential, error) {
	if err := s.guard(); err != nil {
		return Credential{}, err
	}
	if tenantID == uuid.Nil {
		return Credential{}, errors.New("tenant_id is required")
	}
	if strings.TrimSpace(in.FanpageID) == "" {
		return Credential{}, errors.New("fanpageId is required")
	}
	if strings.TrimSpace(in.FanpageName) == "" {
		return Credential{}, errors.New("fanpageName is required")
	}
	cookies, err := browser.UnmarshalCookies(in.Cookies)
	if err != nil {
		return Credential{}, fmt.Errorf("invalid cookies JSON: %w", err)
	}
	if err := browser.ValidateFBCookies(cookies, browser.FBRequiredCookieNames); err != nil {
		return Credential{}, err
	}
	if in.ProxyURL != "" {
		if !validProxyURL(in.ProxyURL) {
			return Credential{}, ErrInvalidProxyURL
		}
	}

	c := Credential{
		TenantID:    tenantID,
		FanpageID:   strings.TrimSpace(in.FanpageID),
		FanpageName: strings.TrimSpace(in.FanpageName),
		Cookies:     in.Cookies,
		ProxyURL:    in.ProxyURL,
		UserAgent:   firstNonEmpty(in.UserAgent, s.deps.DefaultUA),
		ViewportW:   firstNonZero(in.ViewportW, 1366),
		ViewportH:   firstNonZero(in.ViewportH, 768),
		Timezone:    firstNonEmpty(in.Timezone, "Asia/Ho_Chi_Minh"),
		Status:      StatusActive,
	}

	created, err := s.deps.CredentialStore.Create(ctx, c)
	if err != nil {
		return Credential{}, err
	}
	created = redact(created)
	s.deps.Logger.Info("fbcloak.credential.created",
		"tenant", tenantID, "credential", created.ID, "fanpage", created.FanpageID,
	)
	return created, nil
}

// ListCredentials returns all credentials for the tenant. Plaintext cookies +
// proxy are NEVER included in the result — those fields are zeroed.
func (s *Service) ListCredentials(ctx context.Context, tenantID uuid.UUID) ([]Credential, error) {
	if err := s.guard(); err != nil {
		return nil, err
	}
	if tenantID == uuid.Nil {
		return nil, errors.New("tenant_id is required")
	}
	creds, err := s.deps.CredentialStore.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for i := range creds {
		creds[i] = redact(creds[i])
	}
	return creds, nil
}

// TestCredential runs a health probe (cookie inject + /me redirect check) and
// updates the stored credential's status accordingly.
func (s *Service) TestCredential(ctx context.Context, tenantID, id uuid.UUID) (ProbeResult, error) {
	if err := s.guard(); err != nil {
		return ProbeResult{}, err
	}
	if s.deps.HealthProbe == nil {
		return ProbeResult{}, errors.New("health probe not configured")
	}
	cred, err := s.deps.CredentialStore.Get(ctx, tenantID, id)
	if err != nil {
		return ProbeResult{}, err
	}
	res, err := s.deps.HealthProbe.Run(ctx, cred)
	if err != nil {
		return ProbeResult{}, err
	}
	// Persist updated status only if it changed materially.
	if res.Status != "" && res.Status != cred.Status {
		if uErr := s.deps.CredentialStore.UpdateStatus(ctx, tenantID, id, res.Status); uErr != nil {
			s.deps.Logger.Warn("fbcloak.credential.status_persist_failed", "err", uErr)
		}
	}
	if uErr := s.deps.CredentialStore.UpdateLastCheck(ctx, tenantID, id); uErr != nil {
		s.deps.Logger.Warn("fbcloak.credential.last_check_persist_failed", "err", uErr)
	}
	return res, nil
}

// DeleteCredential removes a credential after verifying tenant scope. The
// FK cascade in the migration handles dependent jobs and send_log rows.
func (s *Service) DeleteCredential(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := s.guard(); err != nil {
		return err
	}
	if err := s.deps.CredentialStore.Delete(ctx, tenantID, id); err != nil {
		return err
	}
	s.deps.Logger.Info("fbcloak.credential.deleted", "tenant", tenantID, "credential", id)
	return nil
}

// --- helpers ---

// redact zeroes plaintext-only fields before returning a Credential to
// callers (RPC, UI, logs).
func redact(c Credential) Credential {
	c.Cookies = ""
	c.ProxyURL = ""
	return c
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstNonZero(values ...int) int {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}

// validProxyURL accepts socks5://, socks4://, http://, https:// proxy URLs.
// Empty strings are rejected here — callers must pre-check.
func validProxyURL(s string) bool {
	if s == "" {
		return false
	}
	for _, prefix := range []string{"socks5://", "socks4://", "http://", "https://"} {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

// SetPlanStore wires the plan store post-construction. Phase 5 init path
// calls this so Service plan methods see deps.PlanStore != nil.
func (s *Service) SetPlanStore(ps PlanStore) {
	s.deps.PlanStore = ps
}

// PlanStoreRef is intentionally unexported on Service; subsystems that need
// the store (Generator, Executor, Invalidator, Replan) receive it directly
// via their own wiring, not through Service.

