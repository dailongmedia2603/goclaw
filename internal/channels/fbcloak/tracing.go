//go:build !sqliteonly

package fbcloak

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/internal/tracing"
)

// Tracing in fbcloak follows the codebase's emit-based pattern (see
// internal/hooks/tracing.go) — there is no OTel build tag. We push spans
// into the in-process collector via CollectorFromContext; export to OTLP is
// optional and configured at process startup, not per-span. When no
// collector is attached (tests, tenants without tracing), all helpers are
// silent no-ops.
//
// Two-phase pattern: long operations like a job run can EmitJobStart()
// (status=running) and update with EmitJobFinish() once done. Short send
// spans use the simpler EmitSendSpan() that emits a completed span at the
// end.

// EmitJobStart enqueues a "running" span for a job run. Returns the
// generated span ID so the caller can pair it with EmitJobFinish.
// Returns uuid.Nil when no collector is attached — callers should treat a
// nil return as "tracing disabled" and skip the matching finish call.
func EmitJobStart(ctx context.Context, jobID, credentialID, fanpageID string, startedAt time.Time) uuid.UUID {
	col := tracing.CollectorFromContext(ctx)
	if col == nil {
		return uuid.Nil
	}
	spanID := uuid.New()
	meta, _ := json.Marshal(map[string]string{
		"job_id":        jobID,
		"credential_id": credentialID,
		"fanpage_id":    fanpageID,
	})
	span := store.SpanData{
		ID:        spanID,
		TraceID:   ensureTraceID(ctx),
		SpanType:  store.SpanTypeEvent,
		Name:      "fbcloak.job.run",
		StartTime: startedAt,
		Status:    store.SpanStatusRunning,
		Metadata:  meta,
		TenantID:  store.TenantIDFromContext(ctx),
		TeamID:    tracing.TraceTeamIDPtrFromContext(ctx),
		CreatedAt: startedAt,
	}
	if parent := tracing.ParentSpanIDFromContext(ctx); parent != uuid.Nil {
		p := parent
		span.ParentSpanID = &p
	}
	col.EmitSpan(span)
	return spanID
}

// EmitJobFinish updates a span previously created by EmitJobStart with the
// final status, duration, and per-run counters. No-op if spanID is nil
// (tracing was disabled) or no collector is attached.
func EmitJobFinish(ctx context.Context, spanID uuid.UUID, startedAt time.Time, status JobStatus, sent, failed, skipped int, errMsg string) {
	if spanID == uuid.Nil {
		return
	}
	col := tracing.CollectorFromContext(ctx)
	if col == nil {
		return
	}
	end := time.Now().UTC()
	durationMS := max(int(end.Sub(startedAt)/time.Millisecond), 0)
	spanStatus := store.SpanStatusCompleted
	if status == JobStatusFailed || status == JobStatusKilled {
		spanStatus = store.SpanStatusError
	}
	meta, _ := json.Marshal(map[string]any{
		"job_status": string(status),
		"sent":       sent,
		"failed":     failed,
		"skipped":    skipped,
	})
	col.EmitSpanUpdate(spanID, ensureTraceID(ctx), map[string]any{
		"end_time":    end,
		"duration_ms": durationMS,
		"status":      spanStatus,
		"error":       errMsg,
		"metadata":    meta,
	})
}

// EmitSendSpan records a single send attempt as a completed span. Used for
// short operations where the two-phase pattern adds no value. errMsg is
// empty on success.
func EmitSendSpan(ctx context.Context, sendLogID, jobID, conversationID string, startedAt time.Time, status SendStatus, errMsg string) {
	col := tracing.CollectorFromContext(ctx)
	if col == nil {
		return
	}
	end := time.Now().UTC()
	durationMS := max(int(end.Sub(startedAt)/time.Millisecond), 0)
	spanStatus := store.SpanStatusCompleted
	if status == SendStatusFailed {
		spanStatus = store.SpanStatusError
	}
	meta, _ := json.Marshal(map[string]string{
		"send_log_id":     sendLogID,
		"job_id":          jobID,
		"conversation_id": conversationID,
		"send_status":     string(status),
	})
	span := store.SpanData{
		ID:         uuid.New(),
		TraceID:    ensureTraceID(ctx),
		SpanType:   store.SpanTypeEvent,
		Name:       "fbcloak.send",
		StartTime:  startedAt,
		EndTime:    &end,
		DurationMS: durationMS,
		Status:     spanStatus,
		Error:      errMsg,
		Metadata:   meta,
		TenantID:   store.TenantIDFromContext(ctx),
		TeamID:     tracing.TraceTeamIDPtrFromContext(ctx),
		CreatedAt:  end,
	}
	if parent := tracing.ParentSpanIDFromContext(ctx); parent != uuid.Nil {
		p := parent
		span.ParentSpanID = &p
	}
	col.EmitSpan(span)
}

// ensureTraceID returns the trace ID from context, or generates a new one
// when fbcloak is the trace root (no upstream caller attached an ID). The
// generated ID is NOT propagated back into ctx — callers wanting that must
// wrap with tracing.WithTraceID themselves. We keep that boundary explicit
// to avoid spans from sibling job runs colliding under one synthetic trace.
func ensureTraceID(ctx context.Context) uuid.UUID {
	if id := tracing.TraceIDFromContext(ctx); id != uuid.Nil {
		return id
	}
	return uuid.New()
}
