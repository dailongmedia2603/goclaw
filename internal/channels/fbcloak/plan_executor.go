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

	"github.com/google/uuid"
)

// PlanExecutor polls due plans every TickInterval and dispatches each via
// SendExecutor (the same one Phase 2's JobRunner uses). Status transitions:
//
//	pending → sent  (success)
//	pending → pending (transient send failure — next tick retries)
//	pending unchanged on policy skip — Plan stays pending until Replan flow
//	  picks up the customer-replied event, OR scheduled_at slips past TTL.
//
// Sem + CredLock fields share concurrency state with JobRunner so neither
// feature can flood the chrome sidecar or run two browser sessions for the
// same credential simultaneously.
type PlanExecutor struct {
	Plans          PlanStore
	Credentials    CredentialStore
	SessionFactory SessionFactory
	Send           *SendExecutor

	Logger        *slog.Logger
	TickInterval  time.Duration
	MaxConcurrent int

	Killswitch *atomic.Bool
	Sem        chan struct{} // shared with JobRunner; if nil, executor allocates own
	CredLock   *sync.Map     // shared with JobRunner; if nil, executor allocates own

	Events EventPublisher

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
	clock   func() time.Time
}

const DefaultPlanExecutorTick = time.Hour

func (e *PlanExecutor) Start(ctx context.Context) error {
	if e.Plans == nil || e.Credentials == nil || e.SessionFactory == nil || e.Send == nil {
		return errors.New("plan_executor: missing required deps")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		return nil
	}
	if e.TickInterval == 0 {
		e.TickInterval = DefaultPlanExecutorTick
	}
	if e.Killswitch == nil {
		e.Killswitch = &atomic.Bool{}
	}
	if e.Logger == nil {
		e.Logger = slog.Default()
	}
	if e.clock == nil {
		e.clock = func() time.Time { return time.Now().UTC() }
	}
	if e.Sem == nil {
		if e.MaxConcurrent <= 0 {
			e.MaxConcurrent = 5
		}
		e.Sem = make(chan struct{}, e.MaxConcurrent)
	}
	if e.CredLock == nil {
		e.CredLock = &sync.Map{}
	}

	e.stopCh = make(chan struct{})
	e.running = true
	go e.loop(ctx)
	e.Logger.Info("fbcloak.plan_exec.started",
		"tick", e.TickInterval, "max_concurrent", cap(e.Sem),
	)
	return nil
}

func (e *PlanExecutor) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running {
		return
	}
	close(e.stopCh)
	e.running = false
}

func (e *PlanExecutor) loop(ctx context.Context) {
	t := time.NewTicker(e.TickInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-t.C:
			if e.Killswitch.Load() {
				continue
			}
			e.tick(ctx)
		}
	}
}

func (e *PlanExecutor) tick(ctx context.Context) {
	plans, err := e.Plans.DuePlans(ctx, e.clock(), 50)
	if err != nil {
		e.Logger.Warn("fbcloak.plan_exec.due_failed", "err", err)
		return
	}
	for _, p := range plans {
		if e.Killswitch.Load() {
			return
		}
		select {
		case e.Sem <- struct{}{}:
			go func(plan Plan) {
				defer func() { <-e.Sem }()
				if err := e.executeOne(ctx, plan); err != nil {
					e.Logger.Warn("fbcloak.plan_exec.failed",
						"plan", plan.ID, "psid", plan.PSID, "err", err)
				}
			}(p)
		default:
			// Semaphore full — defer to next tick.
			e.Logger.Debug("fbcloak.plan_exec.deferred", "plan", p.ID)
		}
	}
}

// RunDueForTenant runs Executor synchronously for any pending plans of THIS
// tenant. RPC `fbcloak.plans.run-due` calls this. Returns count attempted.
func (e *PlanExecutor) RunDueForTenant(ctx context.Context, tenantID uuid.UUID) (int, error) {
	plans, _, err := e.Plans.List(ctx, tenantID, PlanFilter{
		Status:          []PlanStatus{PlanStatusPending},
		ScheduledBefore: e.clock(),
		Limit:           50,
	})
	if err != nil {
		return 0, err
	}
	executed := 0
	for _, p := range plans {
		if e.Killswitch.Load() {
			break
		}
		if err := e.executeOne(ctx, p); err != nil {
			e.Logger.Warn("fbcloak.plan_exec.run_due_failed",
				"plan", p.ID, "err", err)
			continue
		}
		executed++
	}
	return executed, nil
}

// executeOne dispatches one plan via SendExecutor.Execute.
func (e *PlanExecutor) executeOne(ctx context.Context, plan Plan) error {
	credLock := e.credentialMutex(plan.CredentialID)
	credLock.Lock()
	defer credLock.Unlock()

	cred, err := e.Credentials.Get(ctx, plan.TenantID, plan.CredentialID)
	if err != nil {
		return fmt.Errorf("get credential: %w", err)
	}
	if cred.Status != StatusActive {
		// Skip but don't transition — wait for credential to recover.
		return fmt.Errorf("credential not active (status=%s)", cred.Status)
	}

	page, closer, err := e.SessionFactory.Open(ctx, plan.ID, cred)
	if err != nil {
		return fmt.Errorf("open session: %w", err)
	}
	defer closer()

	// Synthesize a Job for SendExecutor. ID = plan.ID gives the SendLog row
	// a JobID FK that points back to this plan for audit. DryRun = false:
	// once Generator decided to schedule, by the time we reach Executor the
	// admin had a window to Cancel — sending real is the contract.
	syntheticJob := Job{
		ID:            plan.ID,
		TenantID:      plan.TenantID,
		CredentialID:  plan.CredentialID,
		Name:          "plan-" + plan.PSID,
		DryRun:        false,
		TargetMinIdle: 0,
		TargetMaxIdle: 365 * 24 * time.Hour,
	}
	target := Target{
		RecipientPSID:  plan.PSID,
		ConversationID: plan.ConversationID,
		RecipientName:  plan.RecipientName,
		// Plan rows don't carry last_inbound_at — Replan flow handles
		// "customer replied recently" via status='replan_needed'. By the
		// time Executor sees status='pending' here, no reply has arrived
		// since plan creation, so verify_last_message can use a generous
		// anchor that never trips its tolerance window.
		LastMessageAt: plan.GeneratedAt.Add(-30 * 24 * time.Hour),
		Source:        "plan",
	}
	req := SendRequest{
		Job: syntheticJob, Credential: cred, Target: target,
		Message: plan.MessageDraft, DryRun: false,
	}

	log, err := e.Send.Execute(ctx, page, req)
	if err != nil {
		IncPlanExecError()
		return fmt.Errorf("send execute: %w", err)
	}

	switch log.Status {
	case SendStatusSent, SendStatusDryRun:
		if uErr := e.Plans.MarkSent(ctx, plan.TenantID, plan.ID, log.ID); uErr != nil {
			e.Logger.Warn("fbcloak.plan_exec.mark_sent_failed",
				"plan", plan.ID, "err", uErr)
			return uErr
		}
		IncPlanSent()
	case SendStatusSkipped:
		// Policy/verify rejected. Plan stays pending — next-tick retry only
		// helps if the skip reason is transient (cap reached, cooldown).
		// Permanent skips (opt-out keyword in customer's last message) won't
		// flip on retry; admin can Cancel in UI.
		e.Logger.Info("fbcloak.plan_exec.skipped_by_policy",
			"plan", plan.ID, "reason", deref(log.SkipReason))
		IncPlanSkippedByPolicy()
	case SendStatusFailed:
		IncPlanExecError()
	}
	return nil
}

func (e *PlanExecutor) credentialMutex(id uuid.UUID) *sync.Mutex {
	v, _ := e.CredLock.LoadOrStore(id, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
