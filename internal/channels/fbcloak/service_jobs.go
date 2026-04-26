//go:build !sqliteonly

package fbcloak

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/adhocore/gronx"
	"github.com/google/uuid"
)

// CreateJobInput is the user-supplied data for fbcloak.jobs.create.
type CreateJobInput struct {
	CredentialID       uuid.UUID    `json:"credentialId"`
	Name               string       `json:"name"`
	TemplateID         *uuid.UUID   `json:"templateId,omitempty"`
	TargetMinIdleSec   int          `json:"targetMinIdleSec"`
	TargetMaxIdleSec   int          `json:"targetMaxIdleSec"`
	DailyCap           int          `json:"dailyCap"`
	WorkingHours       WorkingHours `json:"workingHours"`
	CronExpr           string       `json:"cronExpr"`
	UseScannerFallback bool         `json:"useScannerFallback"`
}

// SetJobStore wires the JobStore (Phase 2). Phase 1 left the field nil.
func (s *Service) SetJobStore(js JobStore) { s.deps.JobStore = js }

// SetJobRunner wires the runner so RunJobNow can drive it.
func (s *Service) SetJobRunner(r JobRunner) { s.deps.JobRunner = r }

// JobRunner is the runtime contract the Service uses for RunJobNow. The real
// implementation lives in job_runner.go; tests can substitute a fake.
type JobRunner interface {
	RunOnce(ctx context.Context, j Job) (JobStatus, error)
}

// CreateJob validates + persists a new job.
func (s *Service) CreateJob(ctx context.Context, tenantID uuid.UUID, in CreateJobInput) (Job, error) {
	if err := s.guard(); err != nil {
		return Job{}, err
	}
	if s.deps.JobStore == nil {
		return Job{}, errors.New("fbcloak: JobStore not configured")
	}
	if tenantID == uuid.Nil {
		return Job{}, errors.New("tenant_id is required")
	}
	if in.CredentialID == uuid.Nil {
		return Job{}, errors.New("credentialId is required")
	}
	if in.Name == "" {
		return Job{}, errors.New("name is required")
	}
	if in.CronExpr == "" {
		return Job{}, errors.New("cronExpr is required")
	}
	if !gronx.New().IsValid(in.CronExpr) {
		return Job{}, fmt.Errorf("invalid cronExpr: %s", in.CronExpr)
	}
	if err := AssertCapValid(in.DailyCap, 50); err != nil {
		return Job{}, err
	}
	// Verify credential belongs to tenant (defence-in-depth; FK already
	// enforces existence, but we want a friendly error message).
	if _, err := s.deps.CredentialStore.Get(ctx, tenantID, in.CredentialID); err != nil {
		return Job{}, err
	}
	minIdle := time.Duration(in.TargetMinIdleSec) * time.Second
	maxIdle := time.Duration(in.TargetMaxIdleSec) * time.Second
	if minIdle <= 0 {
		minIdle = 7 * 24 * time.Hour
	}
	if maxIdle <= minIdle {
		maxIdle = minIdle + 30*24*time.Hour
	}
	wh := in.WorkingHours
	if wh == (WorkingHours{}) {
		wh = WorkingHours{Start: "08:00", End: "21:00", TZ: "Asia/Ho_Chi_Minh"}
	}

	j := Job{
		TenantID:           tenantID,
		CredentialID:       in.CredentialID,
		Name:               in.Name,
		TemplateID:         in.TemplateID,
		TargetMinIdle:      minIdle,
		TargetMaxIdle:      maxIdle,
		DailyCap:           in.DailyCap,
		WorkingHours:       wh,
		CronExpr:           in.CronExpr,
		Enabled:            false, // operator must enable explicitly after disclaimer ack (Phase 4)
		DryRun:             true,  // safe default
		UseScannerFallback: in.UseScannerFallback,
	}
	created, err := s.deps.JobStore.CreateJob(ctx, j)
	if err != nil {
		return Job{}, err
	}
	s.deps.Logger.Info("fbcloak.job.created", "tenant", tenantID, "job", created.ID)
	return created, nil
}

// ListJobs returns all jobs for the tenant.
func (s *Service) ListJobs(ctx context.Context, tenantID uuid.UUID) ([]Job, error) {
	if err := s.guard(); err != nil {
		return nil, err
	}
	if s.deps.JobStore == nil {
		return nil, errors.New("fbcloak: JobStore not configured")
	}
	return s.deps.JobStore.ListJobs(ctx, tenantID)
}

// ToggleJob enables/disables a job. Phase 4 gates `enable=true` behind a
// per-tenant ack of CurrentDisclaimerVersion. Disabling is always allowed.
// When DisclaimerStore is unwired (dev / tests), the gate is skipped.
func (s *Service) ToggleJob(ctx context.Context, tenantID, id uuid.UUID, enabled bool) error {
	if err := s.guard(); err != nil {
		return err
	}
	if s.deps.JobStore == nil {
		return errors.New("fbcloak: JobStore not configured")
	}
	if enabled && s.deps.Disclaimer != nil {
		ack, err := s.deps.Disclaimer.GetAtVersion(ctx, tenantID, CurrentDisclaimerVersion)
		if err != nil {
			return fmt.Errorf("disclaimer check: %w", err)
		}
		if ack == nil {
			return ErrDisclaimerRequired
		}
	}
	return s.deps.JobStore.SetJobEnabled(ctx, tenantID, id, enabled)
}

// AckDisclaimer records the acking user's UUID against
// CurrentDisclaimerVersion. Subsequent calls with the same version are a
// no-op (UPSERT). userID may be nil when the caller can't extract a
// user_id from the WS context — auditors can still see acked_at + tenant.
func (s *Service) AckDisclaimer(ctx context.Context, tenantID uuid.UUID, userID *uuid.UUID, version string) error {
	if err := s.guard(); err != nil {
		return err
	}
	if s.deps.Disclaimer == nil {
		return errors.New("fbcloak: DisclaimerStore not configured")
	}
	if version == "" {
		version = CurrentDisclaimerVersion
	}
	return s.deps.Disclaimer.Ack(ctx, tenantID, version, userID)
}

// DisclaimerStatus reports whether the tenant has acked the current
// version. UI uses Required=true to render the modal automatically on
// first visit and to block enable-toggle clicks. When the store is
// unwired, returns Required=false so dev/test environments aren't gated.
func (s *Service) DisclaimerStatus(ctx context.Context, tenantID uuid.UUID) (DisclaimerStatus, error) {
	if err := s.guard(); err != nil {
		return DisclaimerStatus{}, err
	}
	if s.deps.Disclaimer == nil {
		return DisclaimerStatus{CurrentVersion: CurrentDisclaimerVersion, Required: false}, nil
	}
	current, err := s.deps.Disclaimer.GetAtVersion(ctx, tenantID, CurrentDisclaimerVersion)
	if err != nil {
		return DisclaimerStatus{}, err
	}
	latest, _ := s.deps.Disclaimer.GetLatest(ctx, tenantID) // best-effort
	return DisclaimerStatus{
		CurrentVersion: CurrentDisclaimerVersion,
		Required:       current == nil,
		Latest:         latest,
	}, nil
}

// SetJobDryRun toggles the dry_run flag on a single job.
func (s *Service) SetJobDryRun(ctx context.Context, tenantID, id uuid.UUID, dryRun bool) error {
	if err := s.guard(); err != nil {
		return err
	}
	if s.deps.JobStore == nil {
		return errors.New("fbcloak: JobStore not configured")
	}
	return s.deps.JobStore.SetJobDryRun(ctx, tenantID, id, dryRun)
}

// DeleteJob removes a job + its send_log via FK cascade.
func (s *Service) DeleteJob(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := s.guard(); err != nil {
		return err
	}
	if s.deps.JobStore == nil {
		return errors.New("fbcloak: JobStore not configured")
	}
	return s.deps.JobStore.DeleteJob(ctx, tenantID, id)
}

// RunJobNow triggers a single execution outside the cron schedule. Still
// honours daily-cap, cooldown, working hours.
func (s *Service) RunJobNow(ctx context.Context, tenantID, id uuid.UUID) (JobStatus, error) {
	if err := s.guard(); err != nil {
		return "", err
	}
	if s.deps.JobStore == nil || s.deps.JobRunner == nil {
		return "", errors.New("fbcloak: scheduler not configured")
	}
	j, err := s.deps.JobStore.GetJob(ctx, tenantID, id)
	if err != nil {
		return "", err
	}
	return s.deps.JobRunner.RunOnce(ctx, j)
}

// ListSendLog returns recent send_log rows scoped to tenant. jobID may be nil.
func (s *Service) ListSendLog(ctx context.Context, tenantID uuid.UUID, jobID *uuid.UUID, limit int) ([]SendLog, error) {
	if err := s.guard(); err != nil {
		return nil, err
	}
	if s.deps.JobStore == nil {
		return nil, errors.New("fbcloak: JobStore not configured")
	}
	return s.deps.JobStore.ListSendLog(ctx, tenantID, jobID, limit)
}

// ListSendLogFiltered exposes the Phase-3 filtered query to the RPC layer.
// All filter dimensions optional; tenant_id is enforced server-side.
func (s *Service) ListSendLogFiltered(ctx context.Context, tenantID uuid.UUID, opts SendLogFilter) ([]SendLog, error) {
	if err := s.guard(); err != nil {
		return nil, err
	}
	if s.deps.JobStore == nil {
		return nil, errors.New("fbcloak: JobStore not configured")
	}
	return s.deps.JobStore.ListSendLogFiltered(ctx, tenantID, opts)
}

// GetSendLog returns a single send_log row scoped to tenant. Used by the
// screenshot-URL RPC to look up screenshot paths before signing.
func (s *Service) GetSendLog(ctx context.Context, tenantID, sendLogID uuid.UUID) (SendLog, error) {
	if err := s.guard(); err != nil {
		return SendLog{}, err
	}
	if s.deps.JobStore == nil {
		return SendLog{}, errors.New("fbcloak: JobStore not configured")
	}
	return s.deps.JobStore.GetSendLog(ctx, tenantID, sendLogID)
}
