package fbbackfill

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// BroadcastFunc is the callback the gateway provides so fbbackfill can
// push EventFrame's out to connected WS clients. The tenantID is passed
// as a string (rather than uuid.UUID) to keep the gateway wire-up site
// free of the uuid import — a fork-safety optimisation.
//
// The implementation is expected to broadcast the event to all clients
// whose TenantID().String() equals tenantID.
type BroadcastFunc func(tenantID string, event *protocol.EventFrame)

// Event name prefix under which all backfill events are published. UI
// subscribes to this prefix to receive start/progress/pause/completed/failed.
const (
	EventPrefix          = "fb_backfill."
	EventStarted         = "fb_backfill.started"
	EventProgress        = "fb_backfill.progress"
	EventPaused          = "fb_backfill.paused"
	EventResumed         = "fb_backfill.resumed"
	EventCompleted       = "fb_backfill.completed"
	EventFailed          = "fb_backfill.failed"
)

// throttledEmitter implements EventEmitter. Progress events are throttled
// to at most one per minInterval (default 2s) to avoid flooding the WS
// stream when a job is processing a large page quickly (e.g. all convos
// hit the SkipExisting fast path).
type throttledEmitter struct {
	bcast       BroadcastFunc
	minInterval time.Duration

	mu         sync.Mutex
	lastEmitAt map[uuid.UUID]time.Time
}

// NewThrottledEmitter constructs an EventEmitter that forwards to the
// gateway broadcaster, throttling progress events.
func NewThrottledEmitter(bcast BroadcastFunc, minInterval time.Duration) EventEmitter {
	if minInterval <= 0 {
		minInterval = 2 * time.Second
	}
	return &throttledEmitter{
		bcast:       bcast,
		minInterval: minInterval,
		lastEmitAt:  make(map[uuid.UUID]time.Time),
	}
}

func (e *throttledEmitter) emit(tenantID, instanceID uuid.UUID, name string, payload map[string]any) {
	if e.bcast == nil {
		return
	}
	payload["instanceId"] = instanceID.String()
	e.bcast(tenantID.String(), protocol.NewEvent(name, payload))
}

func (e *throttledEmitter) EmitStarted(tenantID, instanceID uuid.UUID) {
	e.emit(tenantID, instanceID, EventStarted, map[string]any{})
}

func (e *throttledEmitter) EmitProgress(tenantID, instanceID uuid.UUID, st *BackfillState) {
	// Throttle: at most one progress event per minInterval per instance.
	e.mu.Lock()
	last := e.lastEmitAt[instanceID]
	now := time.Now()
	if !last.IsZero() && now.Sub(last) < e.minInterval {
		e.mu.Unlock()
		return
	}
	e.lastEmitAt[instanceID] = now
	e.mu.Unlock()
	e.emit(tenantID, instanceID, EventProgress, map[string]any{"state": st})
}

func (e *throttledEmitter) EmitPaused(tenantID, instanceID uuid.UUID, reason string) {
	e.emit(tenantID, instanceID, EventPaused, map[string]any{"reason": reason})
}

func (e *throttledEmitter) EmitResumed(tenantID, instanceID uuid.UUID) {
	e.emit(tenantID, instanceID, EventResumed, map[string]any{})
}

func (e *throttledEmitter) EmitCompleted(tenantID, instanceID uuid.UUID, st *BackfillState) {
	e.emit(tenantID, instanceID, EventCompleted, map[string]any{"state": st})
}

func (e *throttledEmitter) EmitFailed(tenantID, instanceID uuid.UUID, errMsg string) {
	e.emit(tenantID, instanceID, EventFailed, map[string]any{"error": errMsg})
}
