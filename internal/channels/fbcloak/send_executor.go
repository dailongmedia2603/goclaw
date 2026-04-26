//go:build !sqliteonly

package fbcloak

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PageSession is the browser surface area the executor needs. send_executor
// keeps it small + interface-shaped so unit tests can drive a fake without
// running Chrome. The production wire-up (rod_page_session.go in Phase 3)
// implements this against a *rod.Page.
type PageSession interface {
	NavigateThread(ctx context.Context, fanpageID, conversationID, recipientPSID string) (finalURL string, err error)
	Inspector() ThreadInspector
	TypeMessage(ctx context.Context, message string) (readback string, err error)
	ClickSend(ctx context.Context) error
	WaitSendConfirmed(ctx context.Context, timeout time.Duration) error
	ScreenshotPre(ctx context.Context) (path string, err error)  // no-op (return "") when disabled
	ScreenshotPost(ctx context.Context) (path string, err error) // no-op (return "") when disabled
}

// ExecutorClock lets tests freeze time without touching the real clock.
type ExecutorClock func() time.Time

// SendExecutor performs one re-engagement send: navigate → verify → render-side
// readback → click → log. Dry-run short-circuits before any input action.
type SendExecutor struct {
	Policy       *Policy
	Verify       func(ctx context.Context, ins ThreadInspector, expected Target, cfg VerifyConfig) (VerifyVerdict, error)
	VerifyConfig VerifyConfig
	Humanizer    *Humanizer
	Log          JobStore // we only use Log methods; share interface
	Now          ExecutorClock
	// Events is optional; nil → events silently dropped. Production wires
	// the same publisher used by JobRunnerImpl so subscribers see a
	// consistent stream of fbcloak.* events.
	Events EventPublisher
}

// SendRequest bundles a single send attempt.
type SendRequest struct {
	Job        Job
	Credential Credential
	Target     Target
	Message    string
	DryRun     bool
}

// Execute runs a single attempt end-to-end. Returns the SendLog row that was
// (or would have been, in dry-run) persisted, plus an error for unexpected
// failures (DB / browser hard errors). Policy skips and verify mismatches are
// NOT errors — they yield SendStatusSkipped with a SkipReason.
//
// Phase-3 instrumentation: every persisted log path goes through
// recordSendOutcome which increments the matching SendStatus counter and
// emits a span. Successful sends additionally publish EventFBCloakSent;
// policy skips publish EventFBCloakBlocked.
func (e *SendExecutor) Execute(ctx context.Context, page PageSession, req SendRequest) (SendLog, error) {
	if e.Policy == nil {
		return SendLog{}, errors.New("executor: nil Policy")
	}
	if e.Verify == nil {
		e.Verify = VerifyLastMessage // default
	}
	now := time.Now
	if e.Now != nil {
		now = e.Now
	}
	IncSendsAttempted()
	sendStarted := now()

	base := SendLog{
		ID:             uuid.New(),
		TenantID:       req.Job.TenantID,
		JobID:          req.Job.ID,
		CredentialID:   req.Credential.ID,
		FanpageID:      req.Credential.FanpageID,
		ConversationID: req.Target.ConversationID,
		MessageText:    req.Message,
		SentAt:         now(),
	}
	if req.Target.RecipientPSID != "" {
		psid := req.Target.RecipientPSID
		base.RecipientPSID = &psid
	}
	if req.Target.RecipientName != "" {
		name := req.Target.RecipientName
		base.RecipientName = &name
	}
	if !req.Target.LastMessageAt.IsZero() {
		ts := req.Target.LastMessageAt
		base.LastInboundAt = &ts
	}

	// Policy: cheap rules first.
	if reason, err := e.Policy.AllowSend(ctx, req.Job, req.Credential.ID, req.Target, req.Message); err != nil {
		return e.fail(ctx, sendStarted, base, fmt.Errorf("policy: %w", err))
	} else if reason != "" {
		base.Status = SendStatusSkipped
		s := string(reason)
		base.SkipReason = &s
		return e.skip(ctx, sendStarted, base, s)
	}

	// Navigate (always, even dry-run — we want to confirm the URL pattern works
	// before logging "would send").
	finalURL, err := page.NavigateThread(ctx, req.Credential.FanpageID, req.Target.ConversationID, req.Target.RecipientPSID)
	if err != nil {
		return e.fail(ctx, sendStarted, base, fmt.Errorf("navigate: %w", err))
	}
	if isInterstitialURL(finalURL) {
		return e.fail(ctx, sendStarted, base, fmt.Errorf("redirect to %s", finalURL))
	}

	// Verify-last-message.
	v, err := e.Verify(ctx, page.Inspector(), req.Target, e.VerifyConfig)
	if err != nil {
		return e.fail(ctx, sendStarted, base, fmt.Errorf("verify: %w", err))
	}
	if !v.OK {
		base.Status = SendStatusSkipped
		s := v.Mismatch
		base.SkipReason = &s
		return e.skip(ctx, sendStarted, base, s)
	}

	// Pre-send screenshot (best-effort — Phase 3c wires real impl).
	if shot, err := page.ScreenshotPre(ctx); err == nil && shot != "" {
		base.ScreenshotPre = &shot
	} else if err != nil {
		IncScreenshotError()
	}

	if req.DryRun {
		base.Status = SendStatusDryRun
		return e.complete(ctx, sendStarted, base)
	}

	// Type with humanized timing (executor picks delays via humanizer; the
	// actual key-by-key emission is page.TypeMessage's responsibility).
	readback, err := page.TypeMessage(ctx, req.Message)
	if err != nil {
		return e.fail(ctx, sendStarted, base, fmt.Errorf("type: %w", err))
	}
	if !strings.Contains(readback, req.Message) && readback != req.Message {
		// Conservative compare: textbox should at least contain the message.
		return e.fail(ctx, sendStarted, base, fmt.Errorf("readback mismatch: got %q want %q", readback, req.Message))
	}

	if err := page.ClickSend(ctx); err != nil {
		return e.fail(ctx, sendStarted, base, fmt.Errorf("click send: %w", err))
	}
	if err := page.WaitSendConfirmed(ctx, 15*time.Second); err != nil {
		return e.fail(ctx, sendStarted, base, fmt.Errorf("confirm send: %w", err))
	}

	if shot, err := page.ScreenshotPost(ctx); err == nil && shot != "" {
		base.ScreenshotPost = &shot
	} else if err != nil {
		IncScreenshotError()
	}

	base.Status = SendStatusSent
	return e.complete(ctx, sendStarted, base)
}

// fail records a hard error: persists the failure log, increments
// counters, emits a span, returns the original error wrapped.
func (e *SendExecutor) fail(ctx context.Context, sendStarted time.Time, base SendLog, err error) (SendLog, error) {
	base.Status = SendStatusFailed
	msg := err.Error()
	base.Error = &msg
	logErr := e.Log.LogSend(ctx, base)
	e.recordSendOutcome(ctx, base, sendStarted, msg)
	if logErr != nil {
		return base, fmt.Errorf("%w (also: persist failed: %v)", err, logErr)
	}
	return base, err
}

// skip records a policy-driven skip (cooldown, daily cap, opt-out, verify
// mismatch). Persists, emits Blocked event, increments counter, emits span.
func (e *SendExecutor) skip(ctx context.Context, sendStarted time.Time, base SendLog, reason string) (SendLog, error) {
	out, err := e.persist(ctx, base)
	if err == nil {
		psid := ""
		if base.RecipientPSID != nil {
			psid = *base.RecipientPSID
		}
		publishBlocked(e.events(), base.TenantID, base.JobID, psid, reason, base.ID.String())
	}
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	e.recordSendOutcome(ctx, out, sendStarted, errMsg)
	return out, err
}

// complete records a successful send (status=sent or status=dry_run).
// Sent additionally emits EventFBCloakSent; dry_run does not (signalling
// "would have sent" to subscribers would muddy the audit stream).
func (e *SendExecutor) complete(ctx context.Context, sendStarted time.Time, base SendLog) (SendLog, error) {
	out, err := e.persist(ctx, base)
	if err == nil && base.Status == SendStatusSent {
		publishSent(e.events(), base.TenantID, base.JobID, out)
	}
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	e.recordSendOutcome(ctx, out, sendStarted, errMsg)
	return out, err
}

// persist is the unchanged DB write — kept as a small primitive that the
// fail/skip/complete wrappers compose around.
func (e *SendExecutor) persist(ctx context.Context, l SendLog) (SendLog, error) {
	if err := e.Log.LogSend(ctx, l); err != nil {
		return l, fmt.Errorf("persist send_log: %w", err)
	}
	return l, nil
}

// recordSendOutcome handles the cross-cutting metric+tracing bookkeeping
// every status path needs. errMsg empty → success path.
func (e *SendExecutor) recordSendOutcome(ctx context.Context, out SendLog, sendStarted time.Time, errMsg string) {
	IncSendStatus(out.Status)
	EmitSendSpan(ctx, out.ID.String(), out.JobID.String(), out.ConversationID, sendStarted, out.Status, errMsg)
}

// events returns the wired publisher or NoopPublisher so callers never
// nil-check.
func (e *SendExecutor) events() EventPublisher {
	if e.Events != nil {
		return e.Events
	}
	return NoopPublisher()
}

func isInterstitialURL(u string) bool {
	return strings.Contains(u, "/login") || strings.Contains(u, "/checkpoint") || strings.Contains(u, "/two_step_verification")
}
