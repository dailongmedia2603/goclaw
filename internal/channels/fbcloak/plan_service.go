//go:build !sqliteonly

package fbcloak

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// PlanStats are aggregate counts surfaced to UI dashboard.
type PlanStats struct {
	Pending      int `json:"pending"`
	Sent         int `json:"sent"`
	ReplanNeeded int `json:"replanNeeded"`
	Skipped      int `json:"skipped"`
	Cancelled    int `json:"cancelled"`
	Superseded   int `json:"superseded"`
	Total        int `json:"total"`
}

// CreatePlan persists a new engagement plan. Caller MUST hold the
// per-credential lock to avoid Active-conflict races; under the lock,
// at most one Generator/Replan worker creates plans for the credential.
func (s *Service) CreatePlan(ctx context.Context, tenantID uuid.UUID, in PlanInput) (Plan, error) {
	if err := s.guard(); err != nil {
		return Plan{}, err
	}
	if s.deps.PlanStore == nil {
		return Plan{}, ErrFeatureDisabled
	}
	if tenantID == uuid.Nil {
		return Plan{}, ErrInvalidTenant
	}
	// 90-day TTL: scheduled_at beyond is rejected here AND cleaned up by Phase 5.4 worker.
	if in.ScheduledAt.After(time.Now().Add(90 * 24 * time.Hour)) {
		return Plan{}, ErrScheduleTooFar
	}
	plan, err := s.deps.PlanStore.Create(ctx, tenantID, in)
	if err != nil {
		return Plan{}, err
	}
	if s.deps.Logger != nil {
		s.deps.Logger.Info("fbcloak.plan.created",
			"tenant", tenantID, "credential", in.CredentialID,
			"psid", in.PSID, "scheduled_at", in.ScheduledAt,
		)
	}
	return plan, nil
}

func (s *Service) GetPlan(ctx context.Context, tenantID, id uuid.UUID) (Plan, error) {
	if err := s.guard(); err != nil {
		return Plan{}, err
	}
	if s.deps.PlanStore == nil {
		return Plan{}, ErrFeatureDisabled
	}
	return s.deps.PlanStore.Get(ctx, tenantID, id)
}

func (s *Service) ListPlans(ctx context.Context, tenantID uuid.UUID, f PlanFilter) ([]Plan, int, error) {
	if err := s.guard(); err != nil {
		return nil, 0, err
	}
	if s.deps.PlanStore == nil {
		return nil, 0, ErrFeatureDisabled
	}
	return s.deps.PlanStore.List(ctx, tenantID, f)
}

func (s *Service) CancelPlan(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := s.guard(); err != nil {
		return err
	}
	if s.deps.PlanStore == nil {
		return ErrFeatureDisabled
	}
	return s.deps.PlanStore.MarkCancelled(ctx, tenantID, id)
}

func (s *Service) PlanStats(ctx context.Context, tenantID uuid.UUID) (PlanStats, error) {
	if err := s.guard(); err != nil {
		return PlanStats{}, err
	}
	if s.deps.PlanStore == nil {
		return PlanStats{}, ErrFeatureDisabled
	}
	counts, err := s.deps.PlanStore.CountByStatus(ctx, tenantID)
	if err != nil {
		return PlanStats{}, err
	}
	stats := PlanStats{
		Pending:      counts[PlanStatusPending],
		Sent:         counts[PlanStatusSent],
		ReplanNeeded: counts[PlanStatusReplanNeeded],
		Skipped:      counts[PlanStatusSkipped],
		Cancelled:    counts[PlanStatusCancelled],
		Superseded:   counts[PlanStatusSuperseded],
	}
	stats.Total = stats.Pending + stats.Sent + stats.ReplanNeeded +
		stats.Skipped + stats.Cancelled + stats.Superseded
	return stats, nil
}
