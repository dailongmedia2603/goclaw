package fbbackfill

import (
	"context"

	"github.com/google/uuid"
)

// Summarizer transforms a batch of FB messages for one PSID into an
// EpisodicSummary entry, idempotent on SourceID. The concrete
// implementation lives in summarizer_impl.go (phase 4); phase 3 only
// depends on this interface so the job runner can be tested with a fake.
type Summarizer interface {
	// AlreadySummarized reports whether an EpisodicSummary with the given
	// SourceID already exists. Used as a fast-path skip before the job
	// even pulls messages for a conversation.
	AlreadySummarized(ctx context.Context, agentID uuid.UUID, psid, sourceID string) (bool, error)

	// Summarize writes exactly one EpisodicSummary for the PSID. Idempotent:
	// if an entry with the same SourceID already exists and
	// input.ForceRecreate is false, the call is a no-op.
	Summarize(ctx context.Context, input SummarizeInput) error
}

// SummarizeInput carries everything the summarizer needs for one PSID.
// Messages should be sorted chronologically (oldest-first) by the caller
// before being handed over — the Graph API returns newest-first.
type SummarizeInput struct {
	InstanceID uuid.UUID
	TenantID   uuid.UUID
	AgentID    uuid.UUID
	PageID     string
	PSID       string
	SourceID   string // "fb_backfill:{page_id}:{psid}"

	Messages []Message

	// ForceRecreate = true deletes the existing summary (if any) and
	// re-creates. Used for explicit re-sync.
	ForceRecreate bool
}

// SourceIDFor builds the canonical SourceID for a backfilled conversation.
// Exported for use by tests and the summarizer implementation.
func SourceIDFor(pageID, psid string) string {
	return "fb_backfill:" + pageID + ":" + psid
}
