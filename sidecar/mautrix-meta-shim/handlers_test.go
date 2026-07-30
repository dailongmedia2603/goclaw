package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- requireAuth tests ---

func TestRequireAuth_MissingHeader(t *testing.T) {
	h := requireAuth("secret-token", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

func TestRequireAuth_ValidHeader(t *testing.T) {
	h := requireAuth("secret-token", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// --- webhookForwarder tests ---

type captured struct {
	body      []byte
	signature string
	apiVer    string
	count     int
}

func TestWebhookForwarder_PostsSignedBody(t *testing.T) {
	cap := &captured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cap.count++
		cap.body, _ = io.ReadAll(r.Body)
		cap.signature = r.Header.Get("X-Fbm-Signature")
		cap.apiVer = r.Header.Get("X-Fbm-Api-Version")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	f := newWebhookForwarder(srv.URL, "secret123")
	err := f.Post(testCtx(t), SidecarEvent{
		EventType: "message",
		ThreadID:  "t1",
		SenderID:  "s1",
		Content:   "hi",
		Timestamp: time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if cap.count != 1 {
		t.Errorf("count=%d want 1", cap.count)
	}
	if cap.apiVer != "v1" {
		t.Errorf("api version header wrong: %q", cap.apiVer)
	}
	// Verify signature is well-formed (starts with t=).
	if !strings.HasPrefix(cap.signature, "t=") {
		t.Errorf("signature header shape wrong: %q", cap.signature)
	}
	// Body must contain sent fields.
	var out map[string]any
	if err := json.Unmarshal(cap.body, &out); err != nil {
		t.Fatalf("body unmarshal: %v", err)
	}
	if out["event_type"] != "message" || out["thread_id"] != "t1" {
		t.Errorf("payload wrong: %v", out)
	}
	// API version should be populated (we set it in Post).
	if out["api_version"] != "v1" {
		t.Errorf("api_version field missing: %v", out)
	}
}

func TestWebhookForwarder_4xxNoRetry(t *testing.T) {
	cap := &captured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		cap.count++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"rejected"}`))
	}))
	defer srv.Close()

	f := newWebhookForwarder(srv.URL, "s")
	err := f.Post(testCtx(t), SidecarEvent{EventType: "message", ThreadID: "t"})
	if err == nil {
		t.Fatal("expected error on 4xx")
	}
	if cap.count != 1 {
		t.Errorf("4xx should not retry — count=%d", cap.count)
	}
}

// --- translateTimelineEvent tests ---

func TestTranslate_SkipsOwnSends(t *testing.T) {
	ev := map[string]any{
		"type":   "m.room.message",
		"sender": "@admin:fbm.local",
		"content": map[string]any{"body": "echo"},
	}
	_, ok := translateTimelineEvent(ev, "room", "thread1")
	if ok {
		t.Error("admin echoes should be skipped")
	}
}

func TestTranslate_SkipsBotNotices(t *testing.T) {
	ev := map[string]any{
		"type":   "m.room.message",
		"sender": "@metabot:fbm.local",
		"content": map[string]any{"body": "system"},
	}
	_, ok := translateTimelineEvent(ev, "room", "thread1")
	if ok {
		t.Error("bot notices should be skipped")
	}
}

func TestTranslate_ExtractsGhostUserID(t *testing.T) {
	ev := map[string]any{
		"type":             "m.room.message",
		"sender":           "@meta_100012345:fbm.local",
		"event_id":         "$evt1",
		"origin_server_ts": float64(1_700_000_000_000),
		"content":          map[string]any{"body": "hello"},
	}
	out, ok := translateTimelineEvent(ev, "room-1", "thread-42")
	if !ok {
		t.Fatal("expected translate to succeed")
	}
	if out.SenderID != "100012345" {
		t.Errorf("ghost user ID not extracted: %q", out.SenderID)
	}
	if out.Content != "hello" {
		t.Errorf("content: %q", out.Content)
	}
	if out.ThreadID != "thread-42" {
		t.Errorf("thread ID: %q", out.ThreadID)
	}
	if out.Timestamp != 1_700_000_000 {
		t.Errorf("timestamp: %d", out.Timestamp)
	}
}

func TestTranslate_IgnoresNonMessageType(t *testing.T) {
	for _, typ := range []string{"m.room.member", "m.room.topic", ""} {
		ev := map[string]any{"type": typ, "sender": "@meta_1:x"}
		_, ok := translateTimelineEvent(ev, "r", "t")
		if ok {
			t.Errorf("type %q should be ignored", typ)
		}
	}
}

func TestExtractThreadIDFromState(t *testing.T) {
	events := []map[string]any{
		{"type": "m.room.member", "content": map[string]any{}},
		{
			"type": "m.bridge",
			"content": map[string]any{
				"channel": map[string]any{"id": "thread-abc123"},
			},
		},
	}
	if got := func() string { id, _ := extractRoomInfoFromState(events); return id }(); got != "thread-abc123" {
		t.Errorf("got=%q want=thread-abc123", got)
	}
}

func TestExtractThreadIDFromState_Missing(t *testing.T) {
	if got := func() string { id, _ := extractRoomInfoFromState(nil); return id }(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// --- Helpers ---

func testCtx(_ *testing.T) (ctx testContext) { //nolint:unused
	ctx.deadline = time.Now().Add(5 * time.Second)
	return
}

type testContext struct {
	deadline time.Time
}

func (c testContext) Deadline() (time.Time, bool) { return c.deadline, true }
func (c testContext) Done() <-chan struct{}       { return nil }
func (c testContext) Err() error                  { return nil }
func (c testContext) Value(_ any) any             { return nil }
