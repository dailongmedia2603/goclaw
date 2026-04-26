//go:build !sqliteonly

package fbcloak

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// JobStore is the persistence contract for fbcloak_jobs and fbcloak_send_log.
// All methods are tenant-scoped; cross-tenant access yields ErrJobNotFound or
// is silently filtered (List).
type JobStore interface {
	// Job CRUD
	CreateJob(ctx context.Context, j Job) (Job, error)
	GetJob(ctx context.Context, tenantID, id uuid.UUID) (Job, error)
	ListJobs(ctx context.Context, tenantID uuid.UUID) ([]Job, error)
	UpdateJob(ctx context.Context, tenantID uuid.UUID, j Job) error
	SetJobEnabled(ctx context.Context, tenantID, id uuid.UUID, enabled bool) error
	SetJobDryRun(ctx context.Context, tenantID, id uuid.UUID, dryRun bool) error
	UpdateJobRunResult(ctx context.Context, tenantID, id uuid.UUID, status JobStatus, nextRun time.Time) error
	DeleteJob(ctx context.Context, tenantID, id uuid.UUID) error

	// Cross-tenant scan for the scheduler (uses indexed where enabled=true).
	DueJobs(ctx context.Context, now time.Time, limit int) ([]Job, error)

	// Send log
	LogSend(ctx context.Context, l SendLog) error
	ListSendLog(ctx context.Context, tenantID uuid.UUID, jobID *uuid.UUID, limit int) ([]SendLog, error)
	ListSendLogFiltered(ctx context.Context, tenantID uuid.UUID, opts SendLogFilter) ([]SendLog, error)
	GetSendLog(ctx context.Context, tenantID, sendLogID uuid.UUID) (SendLog, error)
	CountTodaySends(ctx context.Context, credentialID uuid.UUID, fanpageID string, since time.Time) (int, error)
	LastSendTo(ctx context.Context, credentialID uuid.UUID, recipientPSID string) (*time.Time, error)
}
