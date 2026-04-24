package fbbackfill

import "github.com/google/uuid"

// EventEmitter publishes backfill lifecycle events to WS-connected UIs.
// Concrete implementation in phase 5; the job runner depends only on the
// interface so it can be tested with a fake.
type EventEmitter interface {
	EmitStarted(tenantID, instanceID uuid.UUID)
	EmitProgress(tenantID, instanceID uuid.UUID, st *BackfillState)
	EmitPaused(tenantID, instanceID uuid.UUID, reason string)
	EmitResumed(tenantID, instanceID uuid.UUID)
	EmitCompleted(tenantID, instanceID uuid.UUID, st *BackfillState)
	EmitFailed(tenantID, instanceID uuid.UUID, err string)
}

// noopEmitter discards all events. Used when no broadcaster is wired
// (e.g., tests that do not care about events, or Deps.Broadcaster=nil).
type noopEmitter struct{}

func (noopEmitter) EmitStarted(uuid.UUID, uuid.UUID)                       {}
func (noopEmitter) EmitProgress(uuid.UUID, uuid.UUID, *BackfillState)      {}
func (noopEmitter) EmitPaused(uuid.UUID, uuid.UUID, string)                {}
func (noopEmitter) EmitResumed(uuid.UUID, uuid.UUID)                       {}
func (noopEmitter) EmitCompleted(uuid.UUID, uuid.UUID, *BackfillState)     {}
func (noopEmitter) EmitFailed(uuid.UUID, uuid.UUID, string)                {}
