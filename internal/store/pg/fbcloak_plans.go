//go:build !sqliteonly

package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/channels/fbcloak"
)

// PGFBCloakPlanStore is the PostgreSQL implementation of fbcloak.PlanStore.
type PGFBCloakPlanStore struct {
	db *sql.DB
}

func NewPGFBCloakPlanStore(db *sql.DB) *PGFBCloakPlanStore {
	return &PGFBCloakPlanStore{db: db}
}

// Compile-time guard.
var _ fbcloak.PlanStore = (*PGFBCloakPlanStore)(nil)

const planSelectColumns = `id, tenant_id, credential_id, psid, conversation_id, recipient_name,
		status, scheduled_at, message_draft, reason, skip_reason,
		generated_by_model, generated_at, summary_version,
		sent_at, send_log_id, created_at, updated_at`

func (s *PGFBCloakPlanStore) Create(ctx context.Context, tenantID uuid.UUID, in fbcloak.PlanInput) (fbcloak.Plan, error) {
	if tenantID == uuid.Nil {
		return fbcloak.Plan{}, fbcloak.ErrInvalidTenant
	}
	if in.CredentialID == uuid.Nil {
		return fbcloak.Plan{}, errors.New("credentialID required")
	}
	if in.PSID == "" {
		return fbcloak.Plan{}, errors.New("psid required")
	}
	if strings.TrimSpace(in.MessageDraft) == "" {
		return fbcloak.Plan{}, errors.New("messageDraft required")
	}
	if in.ScheduledAt.IsZero() {
		return fbcloak.Plan{}, errors.New("scheduledAt required")
	}

	id := uuid.New()
	now := time.Now().UTC()
	const q = `
		INSERT INTO fbcloak_engagement_plans
		  (id, tenant_id, credential_id, psid, conversation_id, recipient_name,
		   status, scheduled_at, message_draft, reason,
		   generated_by_model, generated_at, summary_version,
		   created_at, updated_at)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),
		        'pending',$7,$8,$9,$10,$11,$12,$13,$13)
		RETURNING ` + planSelectColumns

	row := s.db.QueryRowContext(ctx, q,
		id, tenantID, in.CredentialID, in.PSID, in.ConversationID, in.RecipientName,
		in.ScheduledAt.UTC(), in.MessageDraft, in.Reason,
		in.GeneratedByModel, now, in.SummaryVersion, now,
	)
	plan, err := s.scanPlan(row)
	if err != nil {
		if isUniqueViolation(err) {
			return fbcloak.Plan{}, fbcloak.ErrActiveConflict
		}
		return fbcloak.Plan{}, fmt.Errorf("insert plan: %w", err)
	}
	return plan, nil
}

// CreateSkipped inserts a row directly in status='skipped' so the audit
// trail can record an LLM "no-send" decision without a transient pending
// state that the Executor could pick up.
func (s *PGFBCloakPlanStore) CreateSkipped(ctx context.Context, tenantID uuid.UUID, in fbcloak.PlanInput, skipReason string) (fbcloak.Plan, error) {
	if tenantID == uuid.Nil {
		return fbcloak.Plan{}, fbcloak.ErrInvalidTenant
	}
	if in.CredentialID == uuid.Nil {
		return fbcloak.Plan{}, errors.New("credentialID required")
	}
	if in.PSID == "" {
		return fbcloak.Plan{}, errors.New("psid required")
	}
	if skipReason == "" {
		return fbcloak.Plan{}, errors.New("skipReason required")
	}

	id := uuid.New()
	now := time.Now().UTC()
	scheduledAt := in.ScheduledAt
	if scheduledAt.IsZero() {
		scheduledAt = now // dummy; status='skipped' is terminal so it never fires
	}
	const q = `
		INSERT INTO fbcloak_engagement_plans
		  (id, tenant_id, credential_id, psid, conversation_id, recipient_name,
		   status, scheduled_at, message_draft, reason, skip_reason,
		   generated_by_model, generated_at, summary_version,
		   created_at, updated_at)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),
		        'skipped',$7,$8,$9,$10,$11,$12,$13,$14,$14)
		RETURNING ` + planSelectColumns

	row := s.db.QueryRowContext(ctx, q,
		id, tenantID, in.CredentialID, in.PSID, in.ConversationID, in.RecipientName,
		scheduledAt.UTC(), "", in.Reason, skipReason,
		in.GeneratedByModel, now, in.SummaryVersion, now,
	)
	plan, err := s.scanPlan(row)
	if err != nil {
		// Skipped status is NOT in the active partial unique index, so duplicate
		// (credential, psid) inserts are allowed (audit trail can grow).
		return fbcloak.Plan{}, fmt.Errorf("insert skipped plan: %w", err)
	}
	return plan, nil
}

func (s *PGFBCloakPlanStore) Get(ctx context.Context, tenantID, id uuid.UUID) (fbcloak.Plan, error) {
	const q = `SELECT ` + planSelectColumns + ` FROM fbcloak_engagement_plans WHERE tenant_id = $1 AND id = $2`
	return s.scanPlan(s.db.QueryRowContext(ctx, q, tenantID, id))
}

func (s *PGFBCloakPlanStore) GetActiveForRecipient(ctx context.Context, tenantID, credentialID uuid.UUID, psid string) (fbcloak.Plan, error) {
	const q = `SELECT ` + planSelectColumns + ` FROM fbcloak_engagement_plans
	            WHERE tenant_id = $1 AND credential_id = $2 AND psid = $3
	              AND status IN ('pending','replan_needed') LIMIT 1`
	return s.scanPlan(s.db.QueryRowContext(ctx, q, tenantID, credentialID, psid))
}

func (s *PGFBCloakPlanStore) List(ctx context.Context, tenantID uuid.UUID, f fbcloak.PlanFilter) ([]fbcloak.Plan, int, error) {
	args := []any{tenantID}
	conds := []string{"tenant_id = $1"}
	idx := 2

	if len(f.Status) > 0 {
		placeholders := make([]string, len(f.Status))
		for i, st := range f.Status {
			placeholders[i] = fmt.Sprintf("$%d", idx)
			args = append(args, string(st))
			idx++
		}
		conds = append(conds, fmt.Sprintf("status IN (%s)", strings.Join(placeholders, ",")))
	}
	if f.CredentialID != nil {
		conds = append(conds, fmt.Sprintf("credential_id = $%d", idx))
		args = append(args, *f.CredentialID)
		idx++
	}
	if f.PSID != "" {
		conds = append(conds, fmt.Sprintf("psid = $%d", idx))
		args = append(args, f.PSID)
		idx++
	}
	if !f.ScheduledAfter.IsZero() {
		conds = append(conds, fmt.Sprintf("scheduled_at >= $%d", idx))
		args = append(args, f.ScheduledAfter.UTC())
		idx++
	}
	if !f.ScheduledBefore.IsZero() {
		conds = append(conds, fmt.Sprintf("scheduled_at <= $%d", idx))
		args = append(args, f.ScheduledBefore.UTC())
		idx++
	}

	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	whereClause := strings.Join(conds, " AND ")

	var total int
	if err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM fbcloak_engagement_plans WHERE "+whereClause, args...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count plans: %w", err)
	}

	listQ := "SELECT " + planSelectColumns + " FROM fbcloak_engagement_plans WHERE " + whereClause +
		fmt.Sprintf(" ORDER BY scheduled_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, listQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list plans: %w", err)
	}
	defer rows.Close()

	out, err := s.scanRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (s *PGFBCloakPlanStore) DuePlans(ctx context.Context, now time.Time, limit int) ([]fbcloak.Plan, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	const q = `SELECT ` + planSelectColumns + ` FROM fbcloak_engagement_plans
	            WHERE status = 'pending' AND scheduled_at <= $1
	            ORDER BY scheduled_at ASC LIMIT $2`
	rows, err := s.db.QueryContext(ctx, q, now.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("due plans: %w", err)
	}
	defer rows.Close()
	return s.scanRows(rows)
}

func (s *PGFBCloakPlanStore) ReplanNeeded(ctx context.Context, now time.Time, delay time.Duration, limit int) ([]fbcloak.Plan, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	cutoff := now.Add(-delay).UTC()
	const q = `SELECT ` + planSelectColumns + ` FROM fbcloak_engagement_plans
	            WHERE status = 'replan_needed' AND updated_at <= $1
	            ORDER BY updated_at ASC LIMIT $2`
	rows, err := s.db.QueryContext(ctx, q, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("replan needed: %w", err)
	}
	defer rows.Close()
	return s.scanRows(rows)
}

func (s *PGFBCloakPlanStore) MarkSent(ctx context.Context, tenantID, id, sendLogID uuid.UUID) error {
	const q = `UPDATE fbcloak_engagement_plans
	             SET status = 'sent', sent_at = NOW(), send_log_id = $3, updated_at = NOW()
	           WHERE tenant_id = $1 AND id = $2 AND status IN ('pending','replan_needed')`
	res, err := s.db.ExecContext(ctx, q, tenantID, id, sendLogID)
	if err != nil {
		return fmt.Errorf("mark sent: %w", err)
	}
	return planCheckAffected(res, fbcloak.ErrPlanTerminal)
}

func (s *PGFBCloakPlanStore) MarkSuperseded(ctx context.Context, tenantID, id uuid.UUID) error {
	const q = `UPDATE fbcloak_engagement_plans SET status = 'superseded', updated_at = NOW()
	           WHERE tenant_id = $1 AND id = $2 AND status IN ('pending','replan_needed')`
	res, err := s.db.ExecContext(ctx, q, tenantID, id)
	if err != nil {
		return fmt.Errorf("mark superseded: %w", err)
	}
	return planCheckAffected(res, fbcloak.ErrPlanTerminal)
}

func (s *PGFBCloakPlanStore) MarkCancelled(ctx context.Context, tenantID, id uuid.UUID) error {
	const q = `UPDATE fbcloak_engagement_plans SET status = 'cancelled', updated_at = NOW()
	           WHERE tenant_id = $1 AND id = $2`
	res, err := s.db.ExecContext(ctx, q, tenantID, id)
	if err != nil {
		return fmt.Errorf("mark cancelled: %w", err)
	}
	return planCheckAffected(res, fbcloak.ErrPlanNotFound)
}

func (s *PGFBCloakPlanStore) MarkSkipped(ctx context.Context, tenantID, id uuid.UUID, skipReason string) error {
	const q = `UPDATE fbcloak_engagement_plans
	             SET status = 'skipped', skip_reason = $3, updated_at = NOW()
	           WHERE tenant_id = $1 AND id = $2 AND status = 'pending'`
	res, err := s.db.ExecContext(ctx, q, tenantID, id, skipReason)
	if err != nil {
		return fmt.Errorf("mark skipped: %w", err)
	}
	return planCheckAffected(res, fbcloak.ErrPlanTerminal)
}

func (s *PGFBCloakPlanStore) MarkReplanNeeded(ctx context.Context, credentialID uuid.UUID, psid string) (int, error) {
	const q = `UPDATE fbcloak_engagement_plans
	             SET status = 'replan_needed', updated_at = NOW()
	           WHERE credential_id = $1 AND psid = $2 AND status IN ('pending','sent')`
	res, err := s.db.ExecContext(ctx, q, credentialID, psid)
	if err != nil {
		return 0, fmt.Errorf("mark replan: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (s *PGFBCloakPlanStore) AutoCancelExpired(ctx context.Context, now time.Time, ttl time.Duration) (int, error) {
	cutoff := now.Add(ttl).UTC()
	const q = `UPDATE fbcloak_engagement_plans SET status = 'cancelled', updated_at = NOW()
	           WHERE status = 'pending' AND scheduled_at > $1`
	res, err := s.db.ExecContext(ctx, q, cutoff)
	if err != nil {
		return 0, fmt.Errorf("auto-cancel: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (s *PGFBCloakPlanStore) CountByStatus(ctx context.Context, tenantID uuid.UUID) (map[fbcloak.PlanStatus]int, error) {
	const q = `SELECT status, COUNT(*) FROM fbcloak_engagement_plans
	            WHERE tenant_id = $1 GROUP BY status`
	rows, err := s.db.QueryContext(ctx, q, tenantID)
	if err != nil {
		return nil, fmt.Errorf("count by status: %w", err)
	}
	defer rows.Close()
	out := make(map[fbcloak.PlanStatus]int)
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return nil, err
		}
		out[fbcloak.PlanStatus(st)] = n
	}
	return out, rows.Err()
}

// --- helpers ---

func (s *PGFBCloakPlanStore) scanPlan(scanner planRowScanner) (fbcloak.Plan, error) {
	var p fbcloak.Plan
	var convID, recipName, skipReason, model sql.NullString
	var sentAt sql.NullTime
	var sendLogID uuid.NullUUID
	var status string
	if err := scanner.Scan(
		&p.ID, &p.TenantID, &p.CredentialID, &p.PSID, &convID, &recipName,
		&status, &p.ScheduledAt, &p.MessageDraft, &p.Reason, &skipReason,
		&model, &p.GeneratedAt, &p.SummaryVersion,
		&sentAt, &sendLogID, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fbcloak.Plan{}, fbcloak.ErrPlanNotFound
		}
		return fbcloak.Plan{}, err
	}
	p.Status = fbcloak.PlanStatus(status)
	if convID.Valid {
		p.ConversationID = convID.String
	}
	if recipName.Valid {
		p.RecipientName = recipName.String
	}
	if skipReason.Valid {
		p.SkipReason = skipReason.String
	}
	if model.Valid {
		p.GeneratedByModel = model.String
	}
	if sentAt.Valid {
		t := sentAt.Time
		p.SentAt = &t
	}
	if sendLogID.Valid {
		id := sendLogID.UUID
		p.SendLogID = &id
	}
	return p, nil
}

func (s *PGFBCloakPlanStore) scanRows(rows *sql.Rows) ([]fbcloak.Plan, error) {
	var out []fbcloak.Plan
	for rows.Next() {
		p, err := s.scanPlan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// planRowScanner is a minimal interface mirroring sql.Row + sql.Rows Scan.
// Defined here (not reused from fbcloak_credentials.go) so the credentials
// store stays compile-tag isolated.
type planRowScanner interface {
	Scan(dst ...any) error
}

// planCheckAffected is the unique-not-found discriminator: 0 affected rows
// → return notFoundErr (or ErrPlanTerminal when caller already filtered by
// status condition that suggests "terminal" rather than "missing").
func planCheckAffected(res sql.Result, notFoundErr error) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return notFoundErr
	}
	return nil
}

// isUniqueViolation detects PG error code 23505 (unique_violation). Pattern
// is driver-agnostic: pgx surfaces it inside the error string, lib/pq via
// PqError code. String contains is robust enough for our single-purpose
// check (avoids importing pgconn package directly here).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "duplicate key value") ||
		strings.Contains(s, "23505") ||
		strings.Contains(s, "unique constraint")
}
