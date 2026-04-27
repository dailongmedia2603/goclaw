//go:build !sqliteonly

package fbcloak

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakePlanStore is an in-memory PlanStore for testing service guard chain.
// Only the methods Service plan methods touch are implemented; others panic
// to surface accidental usage.
type fakePlanStore struct {
	mu     sync.Mutex
	plans  map[uuid.UUID]Plan
	counts map[PlanStatus]int
}

func newFakePlanStore() *fakePlanStore {
	return &fakePlanStore{
		plans:  make(map[uuid.UUID]Plan),
		counts: make(map[PlanStatus]int),
	}
}

func (f *fakePlanStore) Create(_ context.Context, tenantID uuid.UUID, in PlanInput) (Plan, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := Plan{
		ID: uuid.New(), TenantID: tenantID,
		CredentialID:   in.CredentialID,
		PSID:           in.PSID,
		Status:         PlanStatusPending,
		ScheduledAt:    in.ScheduledAt,
		MessageDraft:   in.MessageDraft,
		Reason:         in.Reason,
		GeneratedByModel: in.GeneratedByModel,
		SummaryVersion: in.SummaryVersion,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	f.plans[p.ID] = p
	f.counts[PlanStatusPending]++
	return p, nil
}

func (f *fakePlanStore) Get(_ context.Context, tenantID, id uuid.UUID) (Plan, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.plans[id]
	if !ok || p.TenantID != tenantID {
		return Plan{}, ErrPlanNotFound
	}
	return p, nil
}

func (f *fakePlanStore) GetActiveForRecipient(_ context.Context, _, _ uuid.UUID, _ string) (Plan, error) {
	return Plan{}, ErrPlanNotFound
}

func (f *fakePlanStore) List(_ context.Context, tenantID uuid.UUID, _ PlanFilter) ([]Plan, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Plan
	for _, p := range f.plans {
		if p.TenantID == tenantID {
			out = append(out, p)
		}
	}
	return out, len(out), nil
}

func (f *fakePlanStore) DuePlans(_ context.Context, _ time.Time, _ int) ([]Plan, error) { return nil, nil }
func (f *fakePlanStore) ReplanNeeded(_ context.Context, _ time.Time, _ time.Duration, _ int) ([]Plan, error) {
	return nil, nil
}

func (f *fakePlanStore) MarkSent(_ context.Context, _, _, _ uuid.UUID) error      { return nil }
func (f *fakePlanStore) MarkSuperseded(_ context.Context, _, _ uuid.UUID) error   { return nil }
func (f *fakePlanStore) MarkCancelled(_ context.Context, tenantID, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.plans[id]
	if !ok || p.TenantID != tenantID {
		return ErrPlanNotFound
	}
	p.Status = PlanStatusCancelled
	f.plans[id] = p
	return nil
}
func (f *fakePlanStore) MarkSkipped(_ context.Context, _, _ uuid.UUID, _ string) error { return nil }
func (f *fakePlanStore) MarkReplanNeeded(_ context.Context, _ uuid.UUID, _ string) (int, error) {
	return 0, nil
}
func (f *fakePlanStore) AutoCancelExpired(_ context.Context, _ time.Time, _ time.Duration) (int, error) {
	return 0, nil
}
func (f *fakePlanStore) CountByStatus(_ context.Context, _ uuid.UUID) (map[PlanStatus]int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[PlanStatus]int, len(f.counts))
	for k, v := range f.counts {
		out[k] = v
	}
	return out, nil
}

func newTestService(plans PlanStore) *Service {
	return &Service{
		deps: Deps{
			PlanStore: plans,
			Logger:    slog.Default(),
		},
	}
}

func TestService_CreatePlan_GuardChain(t *testing.T) {
	store := newFakePlanStore()
	svc := newTestService(store)
	ctx := context.Background()
	tenant := uuid.New()
	cred := uuid.New()
	in := PlanInput{
		CredentialID: cred, PSID: "100",
		ScheduledAt: time.Now().Add(7 * 24 * time.Hour),
		MessageDraft: "hi",
	}

	// happy path
	if _, err := svc.CreatePlan(ctx, tenant, in); err != nil {
		t.Fatalf("happy path: %v", err)
	}

	// killswitch
	svc.SetKillswitch(true)
	if _, err := svc.CreatePlan(ctx, tenant, in); !errors.Is(err, ErrKillswitchActive) {
		t.Errorf("killswitch on: got %v, want ErrKillswitchActive", err)
	}
	svc.SetKillswitch(false)

	// schedule too far
	farIn := in
	farIn.ScheduledAt = time.Now().Add(120 * 24 * time.Hour)
	if _, err := svc.CreatePlan(ctx, tenant, farIn); !errors.Is(err, ErrScheduleTooFar) {
		t.Errorf("too far: got %v, want ErrScheduleTooFar", err)
	}

	// nil tenant
	if _, err := svc.CreatePlan(ctx, uuid.Nil, in); !errors.Is(err, ErrInvalidTenant) {
		t.Errorf("nil tenant: got %v, want ErrInvalidTenant", err)
	}
}

func TestService_CreatePlan_NoStore(t *testing.T) {
	svc := newTestService(nil)
	if _, err := svc.CreatePlan(context.Background(), uuid.New(), PlanInput{
		CredentialID: uuid.New(), PSID: "1", ScheduledAt: time.Now().Add(time.Hour), MessageDraft: "x",
	}); !errors.Is(err, ErrFeatureDisabled) {
		t.Errorf("nil store: got %v, want ErrFeatureDisabled", err)
	}
}

func TestService_PlanStats_AllZeroWhenEmpty(t *testing.T) {
	store := newFakePlanStore()
	svc := newTestService(store)
	stats, err := svc.PlanStats(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("PlanStats: %v", err)
	}
	if stats.Total != 0 {
		t.Errorf("empty: Total = %d, want 0", stats.Total)
	}
}

func TestService_CancelPlan_TenantIsolation(t *testing.T) {
	store := newFakePlanStore()
	svc := newTestService(store)
	ctx := context.Background()
	tenantA := uuid.New()
	tenantB := uuid.New()
	plan, err := svc.CreatePlan(ctx, tenantA, PlanInput{
		CredentialID: uuid.New(), PSID: "1",
		ScheduledAt: time.Now().Add(time.Hour),
		MessageDraft: "x",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Tenant B cannot cancel tenant A's plan.
	if err := svc.CancelPlan(ctx, tenantB, plan.ID); !errors.Is(err, ErrPlanNotFound) {
		t.Errorf("cross-tenant cancel: got %v, want ErrPlanNotFound", err)
	}

	// Owner can cancel.
	if err := svc.CancelPlan(ctx, tenantA, plan.ID); err != nil {
		t.Errorf("owner cancel: %v", err)
	}
}
