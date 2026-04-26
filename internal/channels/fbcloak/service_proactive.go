//go:build !sqliteonly

package fbcloak

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SendProactiveOpts describes one ad-hoc send. Used by the dual-mode
// router (channels.FBProactiveRouter) when delta>7d. Unlike job runs,
// this path bypasses cron + dailyCap heuristics — but still respects
// killswitch, edition, killcheckpoint, and disclaimer ack.
type SendProactiveOpts struct {
	FanpageID     string
	RecipientPSID string
	Message       string
	// LastInboundAt is supplied by the caller (router resolved it
	// already) so the verify step can reject obviously stale data.
	LastInboundAt time.Time
	// DryRun routes through the executor's dry-run path (no real send).
	// Plan principle: "dry-run mặc định" — RPC layer should default true
	// unless the operator explicitly opts in to live.
	DryRun bool
}

// SendProactive performs one re-engagement send outside the job runner.
// Returns the SendLog row's ID so callers can pull the screenshot via
// fbcloak.log.screenshot. The credential is looked up by fanpageID; if
// none exists for the tenant the call fails with ErrCredentialNotFound.
//
// Disclaimer: enforced when DisclaimerStore is wired. Tests can wire a
// nil store to bypass.
func (s *Service) SendProactive(ctx context.Context, tenantID uuid.UUID, opts SendProactiveOpts) (uuid.UUID, error) {
	if err := s.guard(); err != nil {
		return uuid.Nil, err
	}
	if s.deps.JobStore == nil || s.deps.JobRunner == nil {
		return uuid.Nil, errors.New("fbcloak: scheduler not configured")
	}
	if tenantID == uuid.Nil {
		return uuid.Nil, errors.New("tenantID is required")
	}
	if opts.FanpageID == "" || opts.RecipientPSID == "" || opts.Message == "" {
		return uuid.Nil, errors.New("fanpageID, recipientPSID, and message are required")
	}
	// Disclaimer gate.
	if s.deps.Disclaimer != nil {
		ack, err := s.deps.Disclaimer.GetAtVersion(ctx, tenantID, CurrentDisclaimerVersion)
		if err != nil {
			return uuid.Nil, fmt.Errorf("disclaimer check: %w", err)
		}
		if ack == nil {
			return uuid.Nil, ErrDisclaimerRequired
		}
	}
	// Resolve credential.
	cred, err := s.deps.CredentialStore.GetByFanpage(ctx, tenantID, opts.FanpageID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("get credential: %w", err)
	}
	if cred.Status != StatusActive {
		return uuid.Nil, fmt.Errorf("credential status=%s, abort", cred.Status)
	}

	// Build a synthetic Job so the runner's RunOnce path stays the only
	// place that knows the executor wiring. DryRun is plumbed from opts —
	// caller decides (RPC layer defaults to true). DailyCap=1 keeps the
	// executor's per-recipient cooldown policy active.
	syntheticJob := Job{
		ID:                 uuid.New(),
		TenantID:           tenantID,
		CredentialID:       cred.ID,
		Name:               "proactive-" + opts.RecipientPSID,
		TargetMinIdle:      0,                 // bypass — caller chose this path
		TargetMaxIdle:      WindowSafeMaxIdle, // hard upper bound, see below
		DailyCap:           1,
		WorkingHours:       WorkingHours{Start: "00:00", End: "23:59", TZ: "UTC"},
		CronExpr:           "@manual",
		Enabled:            true,
		DryRun:             opts.DryRun,
		UseScannerFallback: false,
	}
	target := Target{
		RecipientPSID:  opts.RecipientPSID,
		ConversationID: "", // resolver fallback may locate; otherwise direct PSID URL
		LastMessageAt:  opts.LastInboundAt,
		Source:         "router",
	}
	// Open session, build executor, send.
	page, closer, err := s.openSessionForCred(ctx, syntheticJob.ID, cred)
	if err != nil {
		return uuid.Nil, err
	}
	defer closer()
	exec := &SendExecutor{
		Policy:       NewPolicyForSend(s.deps.JobStore),
		Verify:       VerifyLastMessage,
		VerifyConfig: VerifyConfig{Tolerance: 2 * 24 * time.Hour, MinIdle: 0, Now: time.Now},
		Log:          s.deps.JobStore,
		Events:       s.deps.Events,
	}
	req := SendRequest{
		Job:        syntheticJob,
		Credential: cred,
		Target:     target,
		Message:    opts.Message,
		DryRun:     opts.DryRun,
	}
	log, err := exec.Execute(ctx, page, req)
	if err != nil {
		return log.ID, err
	}
	if log.Status != SendStatusSent {
		// Skipped/failed: caller wants the actual error, not a generic OK.
		reason := ""
		if log.SkipReason != nil {
			reason = *log.SkipReason
		} else if log.Error != nil {
			reason = *log.Error
		}
		return log.ID, fmt.Errorf("send not delivered: status=%s reason=%s", log.Status, reason)
	}
	return log.ID, nil
}

// WindowSafeMaxIdle is a safety cap used by SendProactive's synthetic
// Job — the router has already validated the window, but we keep an
// upper bound so any policy-min-idle check inside the executor can't
// surprise-skip with a tiny default.
const WindowSafeMaxIdle = 365 * 24 * time.Hour

// openSessionForCred wraps the SessionFactory call. Extracted so
// SendProactive doesn't depend on JobRunnerImpl's internal struct.
func (s *Service) openSessionForCred(ctx context.Context, jobID uuid.UUID, cred Credential) (PageSession, func(), error) {
	// SessionFactory lives on JobRunnerImpl; assert and reuse.
	if jr, ok := s.deps.JobRunner.(*JobRunnerImpl); ok && jr.SessionFactory != nil {
		return jr.SessionFactory.Open(ctx, jobID, cred)
	}
	return nil, nil, errors.New("fbcloak: SessionFactory not wired on JobRunner")
}

// NewPolicyForSend builds a default policy for ad-hoc sends. Mirrors what
// the runner uses except dailyCap is ignored (synthetic job has cap=1).
func NewPolicyForSend(store JobStore) *Policy {
	return NewPolicy(DefaultPolicyConfig(), store)
}
