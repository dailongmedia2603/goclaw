// Package eventbus provides typed domain events for the v3 consolidation pipeline.
// Separate from internal/bus (retained for channel message routing).
//
// V3 design: Phase 1C — foundation interface.
package eventbus

import "time"

// EventType identifies the event category.
type EventType string

const (
	EventSessionCompleted EventType = "session.completed"
	EventEpisodicCreated  EventType = "episodic.created"
	EventEntityUpserted EventType = "entity.upserted"
	EventRunCompleted   EventType = "run.completed"
	EventToolExecuted     EventType = "tool.executed"

	// Context pruning observability (Phase 05)
	EventContextPruned EventType = "context.pruned"

	// Vault events (v3 enrichment pipeline)
	EventVaultDocUpserted EventType = "vault.doc_upserted"

	// Delegation events (v3 orchestration)
	EventDelegateSent      EventType = "delegate.sent"
	EventDelegateCompleted EventType = "delegate.completed"
	EventDelegateFailed    EventType = "delegate.failed"

	// FBCloak browser-automation re-engagement (Standard edition).
	EventFBCloakJobStarted   EventType = "fbcloak.job_started"
	EventFBCloakJobCompleted EventType = "fbcloak.job_completed"
	EventFBCloakSent         EventType = "fbcloak.sent"
	EventFBCloakBlocked      EventType = "fbcloak.blocked"
	EventFBCloakCheckpoint   EventType = "fbcloak.checkpoint"
)

// DomainEvent is a typed event with metadata for the consolidation pipeline.
//
// Identity invariant: TenantID and AgentID are string fields for legacy wire
// compatibility, but consumers parse them as UUIDs before touching the DB.
// Publishers MUST supply valid UUID strings — never agent_key or tenant_slug.
// The publish-time observer in validate_agent_id.go warns on drift.
// See docs/agent-identity-conventions.md.
type DomainEvent struct {
	ID        string // UUID v7 for ordering
	Type      EventType
	SourceID  string // dedup key (e.g. session key, run ID)
	TenantID  string // MUST be a valid UUID string — never tenant_slug
	AgentID   string // MUST be a valid UUID string — never agent_key
	UserID    string
	Timestamp time.Time
	Payload   any // typed per EventType (see payload structs below)
}

// --- Typed payloads, one per EventType ---

// SessionCompletedPayload is emitted after session end or compaction.
type SessionCompletedPayload struct {
	SessionKey      string
	MessageCount    int
	TokensUsed      int
	Summary         string // compaction summary if available
	CompactionCount int    // tracks how many times compaction ran
}

// EpisodicCreatedPayload is emitted after episodic summary is stored.
type EpisodicCreatedPayload struct {
	EpisodicID  string
	SessionKey  string
	Summary     string
	KeyEntities []string
}

// EntityUpsertedPayload is emitted after KG entity upsert.
type EntityUpsertedPayload struct {
	EntityIDs []string
}

// RunCompletedPayload is emitted after pipeline run finishes.
type RunCompletedPayload struct {
	RunID      string
	Iterations int
	TokensUsed int
	ToolCalls  int
	LoopKilled bool
}

// ToolExecutedPayload is emitted per tool call for metrics.
type ToolExecutedPayload struct {
	ToolName string
	Duration time.Duration
	Success  bool
	ReadOnly bool
}

// DelegateSentPayload is emitted when a delegation is dispatched.
type DelegateSentPayload struct {
	DelegationID string
	FromAgent    string
	ToAgent      string
	Task         string
	Mode         string // "async" or "sync"
}

// DelegateCompletedPayload is emitted when a delegatee finishes.
type DelegateCompletedPayload struct {
	DelegationID string
	FromAgent    string
	ToAgent      string
	Content      string
	MediaCount   int // number of media files produced by delegatee
}

// DelegateFailedPayload is emitted when a delegation fails.
type DelegateFailedPayload struct {
	DelegationID string
	FromAgent    string
	ToAgent      string
	Error        string
}

// ContextPrunedPayload is emitted when pruning mutates context messages.
// Payload intentionally excludes raw message content (counts + tokens only).
type ContextPrunedPayload struct {
	SessionKey     string
	TokensBefore   int
	TokensAfter    int
	Budget         int
	ResultsTrimmed int    // soft-trimmed count
	ResultsCleared int    // hard-cleared count
	Compacted      bool
	Trigger        string // "soft" | "hard" | "compact"
}

// FBCloakJobStartedPayload — emitted when a job starts a run cycle.
type FBCloakJobStartedPayload struct {
	JobID         string
	CredentialID  string
	FanpageID     string
	Conversations int // resolver-supplied target count for this run
	DryRun        bool
}

// FBCloakJobCompletedPayload — emitted at the end of every run regardless
// of success. Subscribers gate alerting on Status (e.g. "fail" pages admin).
type FBCloakJobCompletedPayload struct {
	JobID    string
	Sent     int
	Skipped  int
	Failed   int
	Status   string // mirrors fbcloak.JobStatus
	Duration time.Duration
}

// FBCloakSentPayload — emitted per successful send (sent only — dry_run
// does NOT emit). Useful for external broadcast/audit pipelines.
type FBCloakSentPayload struct {
	JobID           string
	SendLogID       string
	ConversationID  string
	RecipientPSID   string
	LastInboundAt   time.Time
}

// FBCloakBlockedPayload — emitted when a send is skipped due to policy
// (cooldown, daily cap, opt-out keyword) rather than a hard error.
type FBCloakBlockedPayload struct {
	JobID         string
	RecipientPSID string
	Reason        string
}

// FBCloakCheckpointPayload — emitted when the checkpoint detector trips
// during a run. Subscribers should alert ops; the credential is
// automatically marked status=checkpoint and the job aborted upstream.
type FBCloakCheckpointPayload struct {
	CredentialID   string
	JobID          string
	Kind           string // "security" | "captcha" | "login" | "suspended"
	ScreenshotPath string // empty if capture failed
}

// VaultDocUpsertedPayload is emitted after a vault document is registered/updated.
type VaultDocUpsertedPayload struct {
	DocID       string // vault_documents.id (UUID)
	TenantID    string // tenant context (per-item for batch safety)
	AgentID     string // agent that wrote the file
	Path        string // workspace-relative file path
	ContentHash string // SHA-256 of content at write time
	Workspace   string // absolute workspace path for file reading
}
