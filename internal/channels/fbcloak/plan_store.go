//go:build !sqliteonly

package fbcloak

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// PlanStore is the persistence interface for engagement plans.
// PG impl in internal/store/pg/fbcloak_plans.go.
type PlanStore interface {
	Create(ctx context.Context, tenantID uuid.UUID, in PlanInput) (Plan, error)
	Get(ctx context.Context, tenantID, id uuid.UUID) (Plan, error)
	GetActiveForRecipient(ctx context.Context, tenantID, credentialID uuid.UUID, psid string) (Plan, error)
	List(ctx context.Context, tenantID uuid.UUID, f PlanFilter) ([]Plan, int, error)

	// DuePlans returns up to `limit` plans with scheduled_at <= now AND status='pending'.
	// Cross-tenant — used by Executor cron tick.
	DuePlans(ctx context.Context, now time.Time, limit int) ([]Plan, error)

	// ReplanNeeded returns plans with status='replan_needed' AND updated_at <= now-delay.
	// Cross-tenant.
	ReplanNeeded(ctx context.Context, now time.Time, delay time.Duration, limit int) ([]Plan, error)

	MarkSent(ctx context.Context, tenantID, id, sendLogID uuid.UUID) error
	MarkSuperseded(ctx context.Context, tenantID, id uuid.UUID) error
	MarkCancelled(ctx context.Context, tenantID, id uuid.UUID) error
	MarkSkipped(ctx context.Context, tenantID, id uuid.UUID, skipReason string) error

	// CreateSkipped inserts a row already in status='skipped' so the audit
	// trail records "Generator visited but said no" without ever passing
	// through 'pending'. Necessary because a transient failure between the
	// pending-insert and MarkSkipped could otherwise leave the placeholder
	// message ("(skipped by orchestrator)") visible to the Executor.
	CreateSkipped(ctx context.Context, tenantID uuid.UUID, in PlanInput, skipReason string) (Plan, error)

	// MarkReplanNeeded flips status to 'replan_needed' for ALL active plans
	// matching (credential_id, psid). Used by Invalidator. Cross-tenant: caller
	// passes credential_id (which is tenant-bound via fbcloak_credentials FK).
	MarkReplanNeeded(ctx context.Context, credentialID uuid.UUID, psid string) (int, error)

	// AutoCancelExpired flips status='cancelled' for pending plans where
	// scheduled_at > now+ttl. Periodic cleanup.
	AutoCancelExpired(ctx context.Context, now time.Time, ttl time.Duration) (int, error)

	// CountByStatus returns counts grouped by status for a tenant. UI dashboard.
	CountByStatus(ctx context.Context, tenantID uuid.UUID) (map[PlanStatus]int, error)
}
