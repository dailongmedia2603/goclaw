package fbbackfill

import (
	"time"

	"github.com/google/uuid"
)

// JobStatus is the lifecycle state of a backfill job.
type JobStatus string

const (
	StatusPending   JobStatus = "pending"
	StatusRunning   JobStatus = "running"
	StatusPaused    JobStatus = "paused"
	StatusCompleted JobStatus = "completed"
	StatusFailed    JobStatus = "failed"
	StatusCancelled JobStatus = "cancelled"
)

// IsTerminal reports whether status is a final state that will not advance
// without a new Start/Retry call.
func (s JobStatus) IsTerminal() bool {
	switch s {
	case StatusCompleted, StatusFailed, StatusCancelled:
		return true
	}
	return false
}

// BackfillStateVersion is the schema version of the JSON blob stored under
// channel_instances.config._backfill. Bump when making a breaking change
// to the struct; older readers fall back to defaults.
const BackfillStateVersion = 1

// TriggeredBy enumerates how a backfill job was initiated, recorded for
// telemetry. Values: "auto_on_create", "manual", "retry", "resume".
type TriggerSource string

// BackfillState is the full state blob persisted per channel instance.
type BackfillState struct {
	Version int       `json:"version"`
	Status  JobStatus `json:"status"`

	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	UpdatedAt  time.Time  `json:"updated_at"`

	LastError string `json:"last_error,omitempty"`

	// Progress counters. Total is best-effort — Graph API does not return
	// an upfront count, so ConversationsTotal grows as pages arrive.
	ConversationsTotal int `json:"conversations_total"`
	ConversationsDone  int `json:"conversations_done"`
	ConversationsSkipped int `json:"conversations_skipped"`
	MessagesIngested   int `json:"messages_ingested"`
	EpisodicsCreated   int `json:"episodics_created"`

	// Resume cursors.
	ConversationCursor string `json:"conversation_cursor,omitempty"`
	CurrentConvoID     string `json:"current_convo_id,omitempty"`
	MessageCursor      string `json:"message_cursor,omitempty"`

	// Config snapshot — frozen at job start so mid-run config changes do
	// not change behavior in flight.
	MaxConversations int           `json:"max_conversations"`
	SkipExisting     bool          `json:"skip_existing"`
	ForceRecreate    bool          `json:"force_recreate"`
	TriggeredBy      TriggerSource `json:"triggered_by"`
}

// NewBackfillState returns a fresh pending state ready for Start().
func NewBackfillState(opts StartOpts) *BackfillState {
	now := time.Now().UTC()
	st := &BackfillState{
		Version:          BackfillStateVersion,
		Status:           StatusPending,
		UpdatedAt:        now,
		MaxConversations: opts.MaxConversations,
		SkipExisting:     opts.SkipExisting,
		ForceRecreate:    opts.ForceRecreate,
		TriggeredBy:      opts.TriggeredBy,
	}
	if st.MaxConversations == 0 {
		st.MaxConversations = DefaultMaxConversations
	}
	return st
}

// DefaultMaxConversations caps the per-job conversation count unless the
// caller explicitly asks for more. 500 covers nearly every real-world
// Messenger inbox while bounding LLM cost.
const DefaultMaxConversations = 500

// StartOpts parameterizes a new backfill job.
type StartOpts struct {
	// MaxConversations caps how many conversations to process in this run.
	// 0 → use DefaultMaxConversations. Negative → unlimited.
	MaxConversations int

	// SkipExisting is true to skip conversations whose SourceID already has
	// an EpisodicSummary. Safe default for re-sync.
	SkipExisting bool

	// ForceRecreate is true to delete existing episodic entries for this
	// Page's backfill and re-create them. Overrides SkipExisting.
	ForceRecreate bool

	// TriggeredBy is a telemetry label. See TriggerSource.
	TriggeredBy TriggerSource
}

// InstanceWithState pairs a channel instance's identifying fields with its
// current BackfillState. Returned by StateStore.ListActive for startup
// cleanup and RPC list operations.
type InstanceWithState struct {
	InstanceID  uuid.UUID
	TenantID    uuid.UUID
	AgentID     uuid.UUID
	Name        string
	Credentials []byte // decrypted by the upstream store layer
	Config      []byte // raw config JSONB
	State       *BackfillState
}

// BackfillConfigKey is the JSON key under channel_instances.config where
// state is persisted. Leading underscore by convention signals
// "fork-private" and avoids collision with upstream config fields.
const BackfillConfigKey = "_backfill"
