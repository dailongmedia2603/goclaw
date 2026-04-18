package facebookmessenger

import (
	"strconv"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
)

// sidecarInboundEvent is the JSON shape sidecar shim POSTs to our webhook endpoint.
// Stable across sidecar versions; breaking changes require the sidecar to bump
// X-Fbm-Api-Version header (enforced by verifyWebhookRequest).
type sidecarInboundEvent struct {
	APIVersion string         `json:"api_version,omitempty"` // "v1" — informational; version is primarily on header
	EventType  string         `json:"event_type"`            // "message" | "reaction" | "typing" | "read_receipt" | ...
	MessageID  string         `json:"message_id"`
	ThreadID   string         `json:"thread_id"`    // FB thread/chat ID
	IsGroup    bool           `json:"is_group"`
	SenderID   string         `json:"sender_id"`    // FB user ID
	SenderName string         `json:"sender_name,omitempty"`
	Content    string         `json:"content,omitempty"`
	Media      []sidecarMedia `json:"media,omitempty"`
	ReplyTo    string         `json:"reply_to,omitempty"`
	Timestamp  int64          `json:"timestamp"` // unix seconds
}

type sidecarMedia struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type,omitempty"`
	Filename    string `json:"filename,omitempty"`
	SizeBytes   int64  `json:"size_bytes,omitempty"`
}

// mapEventToInbound converts a sidecar event into a GoClaw bus.InboundMessage.
// Returns ErrNotAMessage for non-message events (typing, reactions, read receipts)
// so the webhook handler can 202 them and move on. Validation of required fields
// happens first; malformed events return ErrMissingThread / ErrMissingSender.
func mapEventToInbound(
	e sidecarInboundEvent,
	tenantID uuid.UUID,
	agentID string,
	channelName string,
) (bus.InboundMessage, error) {
	if e.EventType != "message" {
		return bus.InboundMessage{}, ErrNotAMessage
	}
	if e.ThreadID == "" {
		return bus.InboundMessage{}, ErrMissingThread
	}
	if e.SenderID == "" {
		return bus.InboundMessage{}, ErrMissingSender
	}

	peerKind := "direct"
	if e.IsGroup {
		peerKind = "group"
	}

	media := make([]bus.MediaFile, 0, len(e.Media))
	for _, m := range e.Media {
		if m.URL == "" {
			continue
		}
		media = append(media, bus.MediaFile{
			Path:     m.URL, // Phase 2 keeps sidecar-served URL; Phase 5 adds download+cache.
			MimeType: m.ContentType,
			Filename: m.Filename,
		})
	}

	meta := map[string]string{
		"fbm_message_id": e.MessageID,
	}
	if e.SenderName != "" {
		meta["fbm_sender_name"] = e.SenderName
	}
	if e.ReplyTo != "" {
		meta["fbm_reply_to"] = e.ReplyTo
	}
	if e.Timestamp > 0 {
		meta["fbm_timestamp"] = strconv.FormatInt(e.Timestamp, 10)
	}

	return bus.InboundMessage{
		Channel:  channelName,
		SenderID: e.SenderID,
		ChatID:   e.ThreadID,
		Content:  e.Content,
		Media:    media,
		PeerKind: peerKind,
		TenantID: tenantID,
		AgentID:  agentID,
		Metadata: meta,
	}, nil
}
