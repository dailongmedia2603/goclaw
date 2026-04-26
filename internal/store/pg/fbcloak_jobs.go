//go:build !sqliteonly

package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/channels/fbcloak"
)

// PGFBCloakJobStore implements fbcloak.JobStore against PostgreSQL.
type PGFBCloakJobStore struct {
	db *sql.DB
}

// NewPGFBCloakJobStore wires the store to a *sql.DB. The store does not own
// the connection lifecycle.
func NewPGFBCloakJobStore(db *sql.DB) *PGFBCloakJobStore {
	return &PGFBCloakJobStore{db: db}
}

// Compile-time check.
var _ fbcloak.JobStore = (*PGFBCloakJobStore)(nil)

// --- Job CRUD ---

func (s *PGFBCloakJobStore) CreateJob(ctx context.Context, j fbcloak.Job) (fbcloak.Job, error) {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	now := time.Now().UTC()
	if j.CreatedAt.IsZero() {
		j.CreatedAt = now
	}
	j.UpdatedAt = now
	if j.DailyCap == 0 {
		j.DailyCap = 30
	}
	if j.WorkingHours == (fbcloak.WorkingHours{}) {
		j.WorkingHours = fbcloak.WorkingHours{Start: "08:00", End: "21:00", TZ: "Asia/Ho_Chi_Minh"}
	}
	whJSON, err := json.Marshal(j.WorkingHours)
	if err != nil {
		return fbcloak.Job{}, fmt.Errorf("marshal working_hours: %w", err)
	}
	const q = `
		INSERT INTO fbcloak_jobs (
			id, tenant_id, credential_id, name, template_id,
			target_min_idle, target_max_idle, daily_cap, working_hours,
			cron_expr, enabled, dry_run, use_scanner_fallback,
			next_run_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5, make_interval(secs => $6), make_interval(secs => $7),
		         $8, $9::jsonb, $10, $11, $12, $13, $14, $15, $16)
	`
	_, err = s.db.ExecContext(ctx, q,
		j.ID, j.TenantID, j.CredentialID, j.Name, j.TemplateID,
		j.TargetMinIdle.Seconds(), j.TargetMaxIdle.Seconds(),
		j.DailyCap, string(whJSON), j.CronExpr,
		j.Enabled, j.DryRun, j.UseScannerFallback,
		j.NextRunAt, j.CreatedAt, j.UpdatedAt,
	)
	if err != nil {
		return fbcloak.Job{}, fmt.Errorf("insert job: %w", err)
	}
	return j, nil
}

func (s *PGFBCloakJobStore) GetJob(ctx context.Context, tenantID, id uuid.UUID) (fbcloak.Job, error) {
	const q = jobSelect + ` WHERE tenant_id = $1 AND id = $2`
	row := s.db.QueryRowContext(ctx, q, tenantID, id)
	j, err := scanJobRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fbcloak.Job{}, fbcloak.ErrJobNotFound
		}
		return fbcloak.Job{}, err
	}
	return j, nil
}

func (s *PGFBCloakJobStore) ListJobs(ctx context.Context, tenantID uuid.UUID) ([]fbcloak.Job, error) {
	const q = jobSelect + ` WHERE tenant_id = $1 ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, q, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()
	var out []fbcloak.Job
	for rows.Next() {
		j, err := scanJobRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (s *PGFBCloakJobStore) UpdateJob(ctx context.Context, tenantID uuid.UUID, j fbcloak.Job) error {
	whJSON, err := json.Marshal(j.WorkingHours)
	if err != nil {
		return fmt.Errorf("marshal working_hours: %w", err)
	}
	const q = `
		UPDATE fbcloak_jobs SET
			name = $3,
			template_id = $4,
			target_min_idle = make_interval(secs => $5),
			target_max_idle = make_interval(secs => $6),
			daily_cap = $7,
			working_hours = $8::jsonb,
			cron_expr = $9,
			use_scanner_fallback = $10,
			updated_at = NOW()
		 WHERE tenant_id = $1 AND id = $2
	`
	res, err := s.db.ExecContext(ctx, q,
		tenantID, j.ID, j.Name, j.TemplateID,
		j.TargetMinIdle.Seconds(), j.TargetMaxIdle.Seconds(),
		j.DailyCap, string(whJSON), j.CronExpr, j.UseScannerFallback,
	)
	if err != nil {
		return fmt.Errorf("update job: %w", err)
	}
	return checkAffected(res, fbcloak.ErrJobNotFound)
}

func (s *PGFBCloakJobStore) SetJobEnabled(ctx context.Context, tenantID, id uuid.UUID, enabled bool) error {
	const q = `UPDATE fbcloak_jobs SET enabled = $3, updated_at = NOW() WHERE tenant_id = $1 AND id = $2`
	res, err := s.db.ExecContext(ctx, q, tenantID, id, enabled)
	if err != nil {
		return fmt.Errorf("set enabled: %w", err)
	}
	return checkAffected(res, fbcloak.ErrJobNotFound)
}

func (s *PGFBCloakJobStore) SetJobDryRun(ctx context.Context, tenantID, id uuid.UUID, dryRun bool) error {
	const q = `UPDATE fbcloak_jobs SET dry_run = $3, updated_at = NOW() WHERE tenant_id = $1 AND id = $2`
	res, err := s.db.ExecContext(ctx, q, tenantID, id, dryRun)
	if err != nil {
		return fmt.Errorf("set dry_run: %w", err)
	}
	return checkAffected(res, fbcloak.ErrJobNotFound)
}

func (s *PGFBCloakJobStore) UpdateJobRunResult(ctx context.Context, tenantID, id uuid.UUID, status fbcloak.JobStatus, nextRun time.Time) error {
	const q = `
		UPDATE fbcloak_jobs SET
			last_run_status = $3,
			last_run_at = NOW(),
			next_run_at = $4,
			updated_at = NOW()
		 WHERE tenant_id = $1 AND id = $2
	`
	res, err := s.db.ExecContext(ctx, q, tenantID, id, string(status), nextRun)
	if err != nil {
		return fmt.Errorf("update run result: %w", err)
	}
	return checkAffected(res, fbcloak.ErrJobNotFound)
}

func (s *PGFBCloakJobStore) DeleteJob(ctx context.Context, tenantID, id uuid.UUID) error {
	const q = `DELETE FROM fbcloak_jobs WHERE tenant_id = $1 AND id = $2`
	res, err := s.db.ExecContext(ctx, q, tenantID, id)
	if err != nil {
		return fmt.Errorf("delete job: %w", err)
	}
	return checkAffected(res, fbcloak.ErrJobNotFound)
}

// DueJobs returns enabled jobs whose next_run_at has passed. Cross-tenant —
// the scheduler runs them in the order returned (oldest due first).
func (s *PGFBCloakJobStore) DueJobs(ctx context.Context, now time.Time, limit int) ([]fbcloak.Job, error) {
	if limit <= 0 {
		limit = 50
	}
	const q = jobSelect + `
		 WHERE enabled = TRUE AND next_run_at IS NOT NULL AND next_run_at <= $1
		 ORDER BY next_run_at ASC
		 LIMIT $2
	`
	rows, err := s.db.QueryContext(ctx, q, now, limit)
	if err != nil {
		return nil, fmt.Errorf("due jobs: %w", err)
	}
	defer rows.Close()
	var out []fbcloak.Job
	for rows.Next() {
		j, err := scanJobRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// --- send_log ---

func (s *PGFBCloakJobStore) LogSend(ctx context.Context, l fbcloak.SendLog) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	if l.SentAt.IsZero() {
		l.SentAt = time.Now().UTC()
	}
	const q = `
		INSERT INTO fbcloak_send_log (
			id, tenant_id, job_id, credential_id, fanpage_id, conversation_id,
			recipient_psid, recipient_name, last_inbound_at, message_text,
			status, skip_reason, error, screenshot_pre, screenshot_post, sent_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	`
	_, err := s.db.ExecContext(ctx, q,
		l.ID, l.TenantID, l.JobID, l.CredentialID, l.FanpageID, l.ConversationID,
		l.RecipientPSID, l.RecipientName, l.LastInboundAt, l.MessageText,
		string(l.Status), l.SkipReason, l.Error, l.ScreenshotPre, l.ScreenshotPost, l.SentAt,
	)
	if err != nil {
		return fmt.Errorf("insert send_log: %w", err)
	}
	return nil
}

func (s *PGFBCloakJobStore) ListSendLog(ctx context.Context, tenantID uuid.UUID, jobID *uuid.UUID, limit int) ([]fbcloak.SendLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	const baseSQL = `
		SELECT id, tenant_id, job_id, credential_id, fanpage_id, conversation_id,
		       recipient_psid, recipient_name, last_inbound_at, message_text,
		       status, skip_reason, error, screenshot_pre, screenshot_post, sent_at
		  FROM fbcloak_send_log
		 WHERE tenant_id = $1
	`
	args := []any{tenantID}
	q := baseSQL
	if jobID != nil {
		q += ` AND job_id = $2 ORDER BY sent_at DESC LIMIT $3`
		args = append(args, *jobID, limit)
	} else {
		q += ` ORDER BY sent_at DESC LIMIT $2`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list send_log: %w", err)
	}
	defer rows.Close()
	var out []fbcloak.SendLog
	for rows.Next() {
		l, err := scanSendLogRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// ListSendLogFiltered is the Phase-3 query used by the SendLog UI page.
// Builds the WHERE clause from non-zero filter fields. All queries are
// scoped to tenant_id ($1) — verified at the call site as well, so a
// crafted opts struct cannot leak rows across tenants. Limits: default 100,
// max 500. Offset 0..N for pagination.
func (s *PGFBCloakJobStore) ListSendLogFiltered(ctx context.Context, tenantID uuid.UUID, opts fbcloak.SendLogFilter) ([]fbcloak.SendLog, error) {
	limit := opts.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	offset := max(opts.Offset, 0)
	q := `
		SELECT id, tenant_id, job_id, credential_id, fanpage_id, conversation_id,
		       recipient_psid, recipient_name, last_inbound_at, message_text,
		       status, skip_reason, error, screenshot_pre, screenshot_post, sent_at
		  FROM fbcloak_send_log
		 WHERE tenant_id = $1`
	args := []any{tenantID}
	idx := 2
	if opts.JobID != nil {
		q += fmt.Sprintf(" AND job_id = $%d", idx)
		args = append(args, *opts.JobID)
		idx++
	}
	if opts.Status != "" {
		q += fmt.Sprintf(" AND status = $%d", idx)
		args = append(args, string(opts.Status))
		idx++
	}
	if !opts.FromDate.IsZero() {
		q += fmt.Sprintf(" AND sent_at >= $%d", idx)
		args = append(args, opts.FromDate)
		idx++
	}
	if !opts.ToDate.IsZero() {
		q += fmt.Sprintf(" AND sent_at <= $%d", idx)
		args = append(args, opts.ToDate)
		idx++
	}
	q += fmt.Sprintf(" ORDER BY sent_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list send_log filtered: %w", err)
	}
	defer rows.Close()
	var out []fbcloak.SendLog
	for rows.Next() {
		l, err := scanSendLogRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// GetSendLog returns one row by id within the tenant scope. Cross-tenant
// access yields sql.ErrNoRows wrapped — the UI never sees raw SQL errors.
func (s *PGFBCloakJobStore) GetSendLog(ctx context.Context, tenantID, sendLogID uuid.UUID) (fbcloak.SendLog, error) {
	const q = `
		SELECT id, tenant_id, job_id, credential_id, fanpage_id, conversation_id,
		       recipient_psid, recipient_name, last_inbound_at, message_text,
		       status, skip_reason, error, screenshot_pre, screenshot_post, sent_at
		  FROM fbcloak_send_log
		 WHERE tenant_id = $1 AND id = $2
	`
	row := s.db.QueryRowContext(ctx, q, tenantID, sendLogID)
	l, err := scanSendLogRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fbcloak.SendLog{}, fbcloak.ErrSendLogNotFound
		}
		return fbcloak.SendLog{}, fmt.Errorf("get send_log: %w", err)
	}
	return l, nil
}

func (s *PGFBCloakJobStore) CountTodaySends(ctx context.Context, credentialID uuid.UUID, fanpageID string, since time.Time) (int, error) {
	const q = `
		SELECT COUNT(*) FROM fbcloak_send_log
		 WHERE credential_id = $1 AND status = 'sent' AND sent_at >= $2
		   AND ($3 = '' OR fanpage_id = $3)
	`
	var n int
	err := s.db.QueryRowContext(ctx, q, credentialID, since, fanpageID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count today: %w", err)
	}
	return n, nil
}

func (s *PGFBCloakJobStore) LastSendTo(ctx context.Context, credentialID uuid.UUID, recipientPSID string) (*time.Time, error) {
	const q = `
		SELECT MAX(sent_at) FROM fbcloak_send_log
		 WHERE credential_id = $1 AND recipient_psid = $2 AND status = 'sent'
	`
	var t sql.NullTime
	err := s.db.QueryRowContext(ctx, q, credentialID, recipientPSID).Scan(&t)
	if err != nil {
		return nil, fmt.Errorf("last send: %w", err)
	}
	if !t.Valid {
		return nil, nil
	}
	return &t.Time, nil
}

// --- helpers ---

const jobSelect = `
	SELECT id, tenant_id, credential_id, name, template_id,
	       EXTRACT(EPOCH FROM target_min_idle)::bigint AS min_idle_sec,
	       EXTRACT(EPOCH FROM target_max_idle)::bigint AS max_idle_sec,
	       daily_cap, working_hours::text, cron_expr,
	       enabled, dry_run, use_scanner_fallback,
	       next_run_at, last_run_at, last_run_status, created_at, updated_at
	  FROM fbcloak_jobs
`

func scanJobRow(scanner rowScanner) (fbcloak.Job, error) {
	var j fbcloak.Job
	var templateID uuid.NullUUID
	var minSec, maxSec int64
	var whJSON string
	var nextRun, lastRun sql.NullTime
	var lastStatus sql.NullString
	if err := scanner.Scan(
		&j.ID, &j.TenantID, &j.CredentialID, &j.Name, &templateID,
		&minSec, &maxSec, &j.DailyCap, &whJSON, &j.CronExpr,
		&j.Enabled, &j.DryRun, &j.UseScannerFallback,
		&nextRun, &lastRun, &lastStatus, &j.CreatedAt, &j.UpdatedAt,
	); err != nil {
		return fbcloak.Job{}, err
	}
	if templateID.Valid {
		j.TemplateID = &templateID.UUID
	}
	j.TargetMinIdle = time.Duration(minSec) * time.Second
	j.TargetMaxIdle = time.Duration(maxSec) * time.Second
	if err := json.Unmarshal([]byte(whJSON), &j.WorkingHours); err != nil {
		return fbcloak.Job{}, fmt.Errorf("unmarshal working_hours: %w", err)
	}
	if nextRun.Valid {
		j.NextRunAt = &nextRun.Time
	}
	if lastRun.Valid {
		j.LastRunAt = &lastRun.Time
	}
	if lastStatus.Valid {
		st := fbcloak.JobStatus(lastStatus.String)
		j.LastRunStatus = &st
	}
	return j, nil
}

func scanSendLogRow(scanner rowScanner) (fbcloak.SendLog, error) {
	var l fbcloak.SendLog
	var psid, name, skipReason, errStr, shotPre, shotPost sql.NullString
	var lastInbound sql.NullTime
	var status string
	if err := scanner.Scan(
		&l.ID, &l.TenantID, &l.JobID, &l.CredentialID, &l.FanpageID, &l.ConversationID,
		&psid, &name, &lastInbound, &l.MessageText,
		&status, &skipReason, &errStr, &shotPre, &shotPost, &l.SentAt,
	); err != nil {
		return fbcloak.SendLog{}, err
	}
	l.Status = fbcloak.SendStatus(status)
	if psid.Valid {
		l.RecipientPSID = &psid.String
	}
	if name.Valid {
		l.RecipientName = &name.String
	}
	if skipReason.Valid {
		l.SkipReason = &skipReason.String
	}
	if errStr.Valid {
		l.Error = &errStr.String
	}
	if shotPre.Valid {
		l.ScreenshotPre = &shotPre.String
	}
	if shotPost.Valid {
		l.ScreenshotPost = &shotPost.String
	}
	if lastInbound.Valid {
		l.LastInboundAt = &lastInbound.Time
	}
	return l, nil
}
