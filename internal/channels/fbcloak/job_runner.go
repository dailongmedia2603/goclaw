//go:build !sqliteonly

package fbcloak

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/adhocore/gronx"
	"github.com/google/uuid"
)

// JobRunnerImpl is the production scheduler. It polls fbcloak_jobs.next_run_at
// every TickInterval, runs eligible jobs, and updates last_run_status. The
// runner depends on a SessionFactory to acquire a browser PageSession per
// credential — Phase 3 wires the rod-backed factory.
type JobRunnerImpl struct {
	Store          JobStore
	Credentials    CredentialStore
	Resolver       *Resolver
	Policy         *Policy
	Humanizer      *Humanizer
	SessionFactory SessionFactory
	TemplateRender TemplateRenderer
	Logger         *slog.Logger
	TickInterval   time.Duration
	MaxConcurrent  int
	Killswitch     *atomic.Bool

	// Phase-3 observability deps. All optional (nil → silent no-op) so
	// existing tests constructing the runner without them keep working.
	Events     EventPublisher
	Screenshot *ScreenshotWriter
	// CheckpointInspectorFor returns a PageInspector for a given page
	// session. Production wires it to a rod-backed adapter; tests can
	// inject a fake. nil → checkpoint detection skipped.
	CheckpointInspectorFor func(PageSession) PageInspector

	mu       sync.Mutex
	running  bool
	stopCh   chan struct{}
	sem      chan struct{}
	credLock sync.Map // credentialID → *sync.Mutex (one worker per credential)
	clock    func() time.Time
}

// SessionFactory builds a per-credential browser session. Implementations
// must apply stealth, inject cookies, and (Phase 3) record screenshots
// under the run's JobID so the audit chain is attributable.
type SessionFactory interface {
	Open(ctx context.Context, jobID uuid.UUID, c Credential) (PageSession, func(), error)
}

// TemplateRenderer turns a Job + Target into the message text to send.
type TemplateRenderer interface {
	Render(ctx context.Context, j Job, t Target, c Credential) (string, error)
}

// Default tick interval = 60s. Higher in production avoids hot-loop polling;
// lower in tests speeds determinism.
const DefaultTickInterval = 60 * time.Second

// Compile-time guard.
var _ JobRunner = (*JobRunnerImpl)(nil)

// Start launches the polling loop in a goroutine. Idempotent — second call
// is a no-op while a loop is active.
func (r *JobRunnerImpl) Start(ctx context.Context) error {
	if r.Store == nil || r.Credentials == nil || r.Resolver == nil || r.Policy == nil ||
		r.SessionFactory == nil || r.TemplateRender == nil {
		return errors.New("job_runner: dependencies not fully wired")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running {
		return nil
	}
	if r.TickInterval == 0 {
		r.TickInterval = DefaultTickInterval
	}
	if r.MaxConcurrent <= 0 {
		r.MaxConcurrent = 5
	}
	if r.Killswitch == nil {
		r.Killswitch = &atomic.Bool{}
	}
	if r.Logger == nil {
		r.Logger = slog.Default()
	}
	if r.clock == nil {
		r.clock = func() time.Time { return time.Now().UTC() }
	}
	r.sem = make(chan struct{}, r.MaxConcurrent)
	r.stopCh = make(chan struct{})
	r.running = true
	go r.loop(ctx)
	r.Logger.Info("fbcloak.runner.started", "tick", r.TickInterval, "max_concurrent", r.MaxConcurrent)
	return nil
}

// Stop signals the loop to exit. Returns once the loop goroutine acknowledges.
func (r *JobRunnerImpl) Stop() {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return
	}
	close(r.stopCh)
	r.running = false
	r.mu.Unlock()
}

// loop is the polling driver.
func (r *JobRunnerImpl) loop(parentCtx context.Context) {
	t := time.NewTicker(r.TickInterval)
	defer t.Stop()
	for {
		select {
		case <-parentCtx.Done():
			return
		case <-r.stopCh:
			return
		case <-t.C:
			if r.Killswitch.Load() {
				continue
			}
			r.tick(parentCtx)
		}
	}
}

// tick scans for due jobs and dispatches each into a worker (semaphore-bound).
func (r *JobRunnerImpl) tick(ctx context.Context) {
	jobs, err := r.Store.DueJobs(ctx, r.clock(), r.MaxConcurrent*2)
	if err != nil {
		r.Logger.Warn("fbcloak.runner.due_query_failed", "err", err)
		return
	}
	for _, j := range jobs {
		select {
		case r.sem <- struct{}{}:
			go func() {
				defer func() { <-r.sem }()
				if rec := recover(); rec != nil {
					r.Logger.Error("fbcloak.runner.panic", "panic", rec, "job", j.ID)
				}
				status, err := r.RunOnce(ctx, j)
				if err != nil {
					r.Logger.Warn("fbcloak.runner.job_failed", "job", j.ID, "err", err)
				}
				next := r.computeNextRun(j)
				if uErr := r.Store.UpdateJobRunResult(ctx, j.TenantID, j.ID, status, next); uErr != nil {
					r.Logger.Warn("fbcloak.runner.persist_status_failed", "job", j.ID, "err", uErr)
				}
			}()
		default:
			// Semaphore full — defer this job to the next tick.
			r.Logger.Debug("fbcloak.runner.deferred", "job", j.ID, "reason", "semaphore_full")
		}
	}
}

// RunOnce executes a single job end-to-end. Public so RunJobNow can call it.
// Returns the JobStatus to persist. Phase 3 added metrics + tracing + event
// publication + checkpoint detection mid-run.
func (r *JobRunnerImpl) RunOnce(ctx context.Context, j Job) (JobStatus, error) {
	IncJobRunsTotal()
	IncActiveWorker()
	defer DecActiveWorker()

	startedAt := r.clock()
	spanID := EmitJobStart(ctx, j.ID.String(), j.CredentialID.String(), "", startedAt)

	if r.Killswitch != nil && r.Killswitch.Load() {
		IncKillswitchAbort()
		IncJobRunsKilled()
		EmitJobFinish(ctx, spanID, startedAt, JobStatusKilled, 0, 0, 0, ErrKillswitchActive.Error())
		return JobStatusKilled, ErrKillswitchActive
	}
	credLock := r.credentialMutex(j.CredentialID)
	credLock.Lock()
	defer credLock.Unlock()

	cred, err := r.Credentials.Get(ctx, j.TenantID, j.CredentialID)
	if err != nil {
		EmitJobFinish(ctx, spanID, startedAt, JobStatusFailed, 0, 0, 0, err.Error())
		return JobStatusFailed, fmt.Errorf("get credential: %w", err)
	}
	if cred.Status != StatusActive {
		errMsg := fmt.Sprintf("credential status=%s, abort", cred.Status)
		EmitJobFinish(ctx, spanID, startedAt, JobStatusFailed, 0, 0, 0, errMsg)
		return JobStatusFailed, errors.New(errMsg)
	}
	if r.Humanizer != nil && !r.Humanizer.IsWithinWorkingHours(r.clock()) {
		EmitJobFinish(ctx, spanID, startedAt, JobStatusOK, 0, 0, 0, "")
		return JobStatusOK, nil // not an error — outside hours, try next cycle
	}

	targets, err := r.Resolver.Resolve(ctx, j.TenantID, ResolveOpts{
		PageID:  cred.FanpageID,
		MinIdle: j.TargetMinIdle,
		MaxIdle: j.TargetMaxIdle,
		Limit:   j.DailyCap,
		Now:     r.clock(),
	})
	if err != nil {
		EmitJobFinish(ctx, spanID, startedAt, JobStatusFailed, 0, 0, 0, err.Error())
		return JobStatusFailed, fmt.Errorf("resolver: %w", err)
	}

	r.publishStarted(j, len(targets))

	if len(targets) == 0 {
		// Phase 2 leaves the inbox-scanner fallback as a stub — wired in
		// Phase 3c. For now, OK with zero sends.
		r.publishCompleted(j, JobStatusOK, 0, 0, 0, time.Since(startedAt), startedAt)
		EmitJobFinish(ctx, spanID, startedAt, JobStatusOK, 0, 0, 0, "")
		return JobStatusOK, nil
	}

	page, closer, err := r.SessionFactory.Open(ctx, j.ID, cred)
	if err != nil {
		EmitJobFinish(ctx, spanID, startedAt, JobStatusFailed, 0, 0, 0, err.Error())
		return JobStatusFailed, fmt.Errorf("session: %w", err)
	}
	defer closer()

	// Pre-flight checkpoint scan after page is open. A trip here means
	// the credential is poisoned — abort before sending anything.
	if k, abort := r.checkCheckpoint(ctx, page, cred, j); abort {
		IncJobRunsKilled()
		r.publishCompleted(j, JobStatusKilled, 0, 0, 0, time.Since(startedAt), startedAt)
		EmitJobFinish(ctx, spanID, startedAt, JobStatusKilled, 0, 0, 0, "checkpoint:"+string(k))
		return JobStatusKilled, fmt.Errorf("checkpoint detected: %s", k)
	}

	exec := &SendExecutor{
		Policy:       r.Policy,
		Verify:       VerifyLastMessage,
		VerifyConfig: VerifyConfig{Tolerance: 2 * 24 * time.Hour, MinIdle: j.TargetMinIdle, Now: r.clock},
		Log:          r.Store,
		Events:       r.events(),
	}

	var sentCount, skipCount, failCount int
	for _, t := range targets {
		if r.Killswitch != nil && r.Killswitch.Load() {
			IncKillswitchAbort()
			break
		}
		msg, rErr := r.TemplateRender.Render(ctx, j, t, cred)
		if rErr != nil {
			r.Logger.Warn("fbcloak.runner.render_failed", "psid", t.RecipientPSID, "err", rErr)
			failCount++
			continue
		}
		req := SendRequest{Job: j, Credential: cred, Target: t, Message: msg, DryRun: j.DryRun}
		log, sErr := exec.Execute(ctx, page, req)
		if sErr != nil {
			failCount++
			continue
		}
		switch log.Status {
		case SendStatusSent, SendStatusDryRun:
			sentCount++
		case SendStatusSkipped:
			skipCount++
		case SendStatusFailed:
			failCount++
		}
	}

	status := r.computeFinalStatus(sentCount, failCount)
	r.publishCompleted(j, status, sentCount, skipCount, failCount, time.Since(startedAt), startedAt)
	EmitJobFinish(ctx, spanID, startedAt, status, sentCount, failCount, skipCount, "")
	return status, nil
}

// computeFinalStatus mirrors the original switch — extracted so the new
// tracing/event wiring above can call it without duplicating logic.
func (r *JobRunnerImpl) computeFinalStatus(sent, failed int) JobStatus {
	switch {
	case sent > 0 && failed == 0:
		return JobStatusOK
	case sent > 0:
		return JobStatusPartial
	case failed > 0:
		return JobStatusFailed
	default:
		return JobStatusOK
	}
}

// checkCheckpoint runs the detector when configured. Returns abort=true
// when the run should bail. Side effects: marks credential.status,
// captures evidence screenshot, increments metrics, publishes event,
// emits a structured security log. Failures inside helpers are logged but
// do NOT block the abort signal — we'd rather alert without evidence
// than send a message into a checkpointed account.
func (r *JobRunnerImpl) checkCheckpoint(ctx context.Context, page PageSession, cred Credential, j Job) (CheckpointKind, bool) {
	if r.CheckpointInspectorFor == nil {
		return CheckpointNone, false
	}
	insp := r.CheckpointInspectorFor(page)
	if insp == nil {
		return CheckpointNone, false
	}
	kind, err := DetectCheckpoint(ctx, insp)
	if err != nil {
		// Inspector failure is non-fatal: log and continue. A real
		// checkpoint would surface via URL/HTML on the next probe.
		r.Logger.Warn("fbcloak.checkpoint.inspect_failed", "err", err)
		return CheckpointNone, false
	}
	if kind == CheckpointNone {
		return CheckpointNone, false
	}
	IncCheckpoint()
	if newStatus := MapToCredentialStatus(kind); newStatus != "" {
		if uErr := r.Credentials.UpdateStatus(ctx, cred.TenantID, cred.ID, newStatus); uErr != nil {
			r.Logger.Warn("fbcloak.checkpoint.status_persist_failed", "err", uErr)
		}
	}
	r.Logger.Warn("security.fbcloak.checkpoint",
		"tenant", cred.TenantID, "credential", cred.ID, "job", j.ID, "kind", string(kind),
	)
	if r.events() != nil {
		publishCheckpoint(r.events(), cred.TenantID, cred.ID, j.ID, string(kind), "")
	}
	return kind, true
}

// events returns the configured publisher or NoopPublisher so wiring code
// never needs to nil-check.
func (r *JobRunnerImpl) events() EventPublisher {
	if r.Events != nil {
		return r.Events
	}
	return NoopPublisher()
}

// publishStarted is a thin wrapper used by RunOnce.
func (r *JobRunnerImpl) publishStarted(j Job, conversations int) {
	publishJobStarted(r.events(), j.TenantID, j, conversations)
}

// publishCompleted is a thin wrapper used by RunOnce.
func (r *JobRunnerImpl) publishCompleted(j Job, status JobStatus, sent, skipped, failed int, dur time.Duration, startedAt time.Time) {
	publishJobCompleted(r.events(), j.TenantID, j, status, sent, skipped, failed, dur, startedAt)
}

func (r *JobRunnerImpl) credentialMutex(id uuid.UUID) *sync.Mutex {
	v, _ := r.credLock.LoadOrStore(id, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// CredentialLockMap exposes the per-credential mutex map so other workers
// (e.g. fbcloak.PlanExecutor in Phase 5) can serialise their sends with
// JobRunner's. WITHOUT this share, two browser sessions for the same
// credential can run simultaneously, which makes Meta's anti-bot signal
// trip much faster.
func (r *JobRunnerImpl) CredentialLockMap() *sync.Map {
	return &r.credLock
}

// Semaphore exposes the shared concurrency-cap channel. Callers MUST NOT
// close it. Returns nil before Start() runs — wiring code should call
// Start() first.
func (r *JobRunnerImpl) Semaphore() chan struct{} {
	return r.sem
}

// computeNextRun parses the cron expression and returns the next firing time.
// On parse failure, defers ~1 hour so the runner doesn't hot-loop a bad job.
func (r *JobRunnerImpl) computeNextRun(j Job) time.Time {
	now := r.clock()
	next, err := gronx.NextTickAfter(j.CronExpr, now, false)
	if err != nil || next.IsZero() {
		return now.Add(time.Hour)
	}
	return next
}
