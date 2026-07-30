//go:build !sqliteonly

package fbcloak

import (
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/eventbus"
)

// EventPublisher decouples fbcloak from the concrete bus implementation —
// tests and lite builds can wire a no-op. Matches the eventbus.Publish
// signature minus the receiver so wiring stays tidy.
type EventPublisher interface {
	Publish(e eventbus.DomainEvent)
}

// noopPublisher is the default when no bus is wired. All Publish calls
// are silently dropped — fbcloak must NEVER block on event delivery.
type noopPublisher struct{}

func (noopPublisher) Publish(eventbus.DomainEvent) {}

// NoopPublisher returns a publisher that drops every event. Wired by
// default when Service.deps.Events is nil.
func NoopPublisher() EventPublisher { return noopPublisher{} }

// publishJobStarted emits eventbus.EventFBCloakJobStarted. SourceID is the
// job UUID (dedupes if a tick re-fires while a worker is still running).
func publishJobStarted(p EventPublisher, tenantID uuid.UUID, j Job, conversations int) {
	if p == nil {
		return
	}
	p.Publish(eventbus.DomainEvent{
		ID:        uuid.NewString(),
		Type:      eventbus.EventFBCloakJobStarted,
		SourceID:  j.ID.String(),
		TenantID:  tenantID.String(),
		Timestamp: time.Now().UTC(),
		Payload: eventbus.FBCloakJobStartedPayload{
			JobID:         j.ID.String(),
			CredentialID:  j.CredentialID.String(),
			FanpageID:     "", // filled by caller via Job → Credential lookup if needed
			Conversations: conversations,
			DryRun:        j.DryRun,
		},
	})
}

// publishJobCompleted emits eventbus.EventFBCloakJobCompleted. SourceID is
// `<jobID>:<startTime>` so multiple runs of the same job are deduped
// independently inside the bus dedup TTL.
func publishJobCompleted(p EventPublisher, tenantID uuid.UUID, j Job, status JobStatus, sent, skipped, failed int, dur time.Duration, startedAt time.Time) {
	if p == nil {
		return
	}
	p.Publish(eventbus.DomainEvent{
		ID:        uuid.NewString(),
		Type:      eventbus.EventFBCloakJobCompleted,
		SourceID:  j.ID.String() + ":" + startedAt.Format(time.RFC3339Nano),
		TenantID:  tenantID.String(),
		Timestamp: time.Now().UTC(),
		Payload: eventbus.FBCloakJobCompletedPayload{
			JobID:    j.ID.String(),
			Sent:     sent,
			Skipped:  skipped,
			Failed:   failed,
			Status:   string(status),
			Duration: dur,
		},
	})
}

// publishSent emits eventbus.EventFBCloakSent for one successful send.
// Use SendLog.ID as SourceID — each send is a unique event by row UUID.
func publishSent(p EventPublisher, tenantID uuid.UUID, jobID uuid.UUID, log SendLog) {
	if p == nil {
		return
	}
	psid := ""
	if log.RecipientPSID != nil {
		psid = *log.RecipientPSID
	}
	var lastInbound time.Time
	if log.LastInboundAt != nil {
		lastInbound = *log.LastInboundAt
	}
	p.Publish(eventbus.DomainEvent{
		ID:        uuid.NewString(),
		Type:      eventbus.EventFBCloakSent,
		SourceID:  log.ID.String(),
		TenantID:  tenantID.String(),
		Timestamp: time.Now().UTC(),
		Payload: eventbus.FBCloakSentPayload{
			JobID:          jobID.String(),
			SendLogID:      log.ID.String(),
			ConversationID: log.ConversationID,
			RecipientPSID:  psid,
			LastInboundAt:  lastInbound,
		},
	})
}

// publishBlocked emits eventbus.EventFBCloakBlocked when policy skips a
// send. Reason mirrors fbcloak.SkipReason text.
func publishBlocked(p EventPublisher, tenantID uuid.UUID, jobID uuid.UUID, recipientPSID, reason, sendLogID string) {
	if p == nil {
		return
	}
	p.Publish(eventbus.DomainEvent{
		ID:        uuid.NewString(),
		Type:      eventbus.EventFBCloakBlocked,
		SourceID:  sendLogID,
		TenantID:  tenantID.String(),
		Timestamp: time.Now().UTC(),
		Payload: eventbus.FBCloakBlockedPayload{
			JobID:         jobID.String(),
			RecipientPSID: recipientPSID,
			Reason:        reason,
		},
	})
}

// publishCheckpoint emits eventbus.EventFBCloakCheckpoint when the detector
// trips. Caller passes the screenshot path of the evidence capture (may be
// empty if the capture failed — alerts still fire).
func publishCheckpoint(p EventPublisher, tenantID uuid.UUID, credentialID, jobID uuid.UUID, kind, screenshotPath string) {
	if p == nil {
		return
	}
	p.Publish(eventbus.DomainEvent{
		ID:        uuid.NewString(),
		Type:      eventbus.EventFBCloakCheckpoint,
		SourceID:  credentialID.String() + ":" + kind,
		TenantID:  tenantID.String(),
		Timestamp: time.Now().UTC(),
		Payload: eventbus.FBCloakCheckpointPayload{
			CredentialID:   credentialID.String(),
			JobID:          jobID.String(),
			Kind:           kind,
			ScreenshotPath: screenshotPath,
		},
	})
}
