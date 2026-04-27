//go:build !sqliteonly

package fbcloak

import (
	"time"

	"github.com/google/uuid"
)

// PlanStatus is the lifecycle state of an engagement plan.
type PlanStatus string

const (
	PlanStatusPending      PlanStatus = "pending"
	PlanStatusSent         PlanStatus = "sent"
	PlanStatusSuperseded   PlanStatus = "superseded"
	PlanStatusCancelled    PlanStatus = "cancelled"
	PlanStatusReplanNeeded PlanStatus = "replan_needed"
	PlanStatusSkipped      PlanStatus = "skipped"
)

// IsTerminal returns true when the status will not transition further.
func (s PlanStatus) IsTerminal() bool {
	switch s {
	case PlanStatusSent, PlanStatusSuperseded, PlanStatusCancelled, PlanStatusSkipped:
		return true
	}
	return false
}

// IsActive returns true when the row counts toward the per-recipient unique constraint.
func (s PlanStatus) IsActive() bool {
	return s == PlanStatusPending || s == PlanStatusReplanNeeded
}

// Plan is one AI-curated engagement decision for one (credential, psid).
type Plan struct {
	ID             uuid.UUID `db:"id" json:"id"`
	TenantID       uuid.UUID `db:"tenant_id" json:"tenantId"`
	CredentialID   uuid.UUID `db:"credential_id" json:"credentialId"`
	PSID           string    `db:"psid" json:"psid"`
	ConversationID string    `db:"conversation_id" json:"conversationId,omitempty"`
	RecipientName  string    `db:"recipient_name" json:"recipientName,omitempty"`

	Status       PlanStatus `db:"status" json:"status"`
	ScheduledAt  time.Time  `db:"scheduled_at" json:"scheduledAt"`
	MessageDraft string     `db:"message_draft" json:"messageDraft"`
	Reason       string     `db:"reason" json:"reason"`
	SkipReason   string     `db:"skip_reason" json:"skipReason,omitempty"`

	GeneratedByModel string    `db:"generated_by_model" json:"generatedByModel,omitempty"`
	GeneratedAt      time.Time `db:"generated_at" json:"generatedAt"`
	SummaryVersion   int       `db:"summary_version" json:"summaryVersion"`

	SentAt    *time.Time `db:"sent_at" json:"sentAt,omitempty"`
	SendLogID *uuid.UUID `db:"send_log_id" json:"sendLogId,omitempty"`

	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}

// PlanInput is what Plan Generator computes and what callers insert.
type PlanInput struct {
	CredentialID     uuid.UUID
	PSID             string
	ConversationID   string
	RecipientName    string
	ScheduledAt      time.Time
	MessageDraft     string
	Reason           string
	GeneratedByModel string
	SummaryVersion   int
}

// PlanFilter filters list queries.
type PlanFilter struct {
	Status          []PlanStatus
	CredentialID    *uuid.UUID
	PSID            string
	ScheduledAfter  time.Time
	ScheduledBefore time.Time
	Limit           int
	Offset          int
}
