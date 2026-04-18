package facebookmessenger

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestMapEvent_DirectMessage(t *testing.T) {
	tenantID := uuid.New()
	e := sidecarInboundEvent{
		EventType: "message",
		MessageID: "mid.$abc",
		ThreadID:  "thread-1",
		IsGroup:   false,
		SenderID:  "fbuser-1",
		Content:   "hello",
		Timestamp: 1700000000,
	}
	msg, err := mapEventToInbound(e, tenantID, "agent-1", "fbm-alice")
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if msg.Channel != "fbm-alice" {
		t.Errorf("channel=%q", msg.Channel)
	}
	if msg.SenderID != "fbuser-1" || msg.ChatID != "thread-1" || msg.Content != "hello" {
		t.Errorf("basic fields: %+v", msg)
	}
	if msg.PeerKind != "direct" {
		t.Errorf("PeerKind=%q want=direct", msg.PeerKind)
	}
	if msg.TenantID != tenantID {
		t.Errorf("TenantID lost")
	}
	if msg.AgentID != "agent-1" {
		t.Errorf("AgentID=%q", msg.AgentID)
	}
	if msg.Metadata["fbm_message_id"] != "mid.$abc" {
		t.Errorf("message_id metadata missing: %v", msg.Metadata)
	}
	if msg.Metadata["fbm_timestamp"] != "1700000000" {
		t.Errorf("timestamp metadata: %v", msg.Metadata)
	}
}

func TestMapEvent_GroupMessage(t *testing.T) {
	e := sidecarInboundEvent{
		EventType: "message",
		ThreadID:  "group-42",
		SenderID:  "fbuser-1",
		IsGroup:   true,
	}
	msg, err := mapEventToInbound(e, uuid.Nil, "", "fbm-alice")
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if msg.PeerKind != "group" {
		t.Errorf("PeerKind=%q want=group", msg.PeerKind)
	}
}

func TestMapEvent_WithMedia(t *testing.T) {
	e := sidecarInboundEvent{
		EventType: "message",
		ThreadID:  "t1",
		SenderID:  "s1",
		Media: []sidecarMedia{
			{URL: "http://a/1.jpg", ContentType: "image/jpeg", Filename: "photo.jpg"},
			{URL: "http://a/2.mp4", ContentType: "video/mp4"},
			{URL: ""}, // empty URL should be dropped
		},
	}
	msg, err := mapEventToInbound(e, uuid.Nil, "", "fbm")
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if len(msg.Media) != 2 {
		t.Fatalf("media count=%d want=2", len(msg.Media))
	}
	if msg.Media[0].Path != "http://a/1.jpg" || msg.Media[0].MimeType != "image/jpeg" || msg.Media[0].Filename != "photo.jpg" {
		t.Errorf("media[0]: %+v", msg.Media[0])
	}
}

func TestMapEvent_NonMessageReturnsNotAMessage(t *testing.T) {
	for _, evType := range []string{"typing", "reaction", "read_receipt", ""} {
		e := sidecarInboundEvent{EventType: evType, ThreadID: "t", SenderID: "s"}
		_, err := mapEventToInbound(e, uuid.Nil, "", "c")
		if !errors.Is(err, ErrNotAMessage) {
			t.Errorf("type=%q: expected ErrNotAMessage, got %v", evType, err)
		}
	}
}

func TestMapEvent_MissingThread(t *testing.T) {
	e := sidecarInboundEvent{EventType: "message", SenderID: "s1"}
	_, err := mapEventToInbound(e, uuid.Nil, "", "c")
	if !errors.Is(err, ErrMissingThread) {
		t.Errorf("expected ErrMissingThread, got %v", err)
	}
}

func TestMapEvent_MissingSender(t *testing.T) {
	e := sidecarInboundEvent{EventType: "message", ThreadID: "t1"}
	_, err := mapEventToInbound(e, uuid.Nil, "", "c")
	if !errors.Is(err, ErrMissingSender) {
		t.Errorf("expected ErrMissingSender, got %v", err)
	}
}

func TestMapEvent_ReplyAndSenderName(t *testing.T) {
	e := sidecarInboundEvent{
		EventType:  "message",
		ThreadID:   "t1",
		SenderID:   "s1",
		SenderName: "Alice Example",
		ReplyTo:    "mid.$parent",
	}
	msg, err := mapEventToInbound(e, uuid.Nil, "", "c")
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if msg.Metadata["fbm_sender_name"] != "Alice Example" {
		t.Errorf("sender_name: %v", msg.Metadata)
	}
	if msg.Metadata["fbm_reply_to"] != "mid.$parent" {
		t.Errorf("reply_to: %v", msg.Metadata)
	}
}
