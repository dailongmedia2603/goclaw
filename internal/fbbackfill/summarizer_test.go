package fbbackfill

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nextlevelbuilder/goclaw/internal/providers"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// ---------- fakes ----------

type fakeEpisodicStore struct {
	mu   sync.Mutex
	rows map[string]*store.EpisodicSummary // keyed by ID.String()
}

func newFakeEpisodicStore() *fakeEpisodicStore {
	return &fakeEpisodicStore{rows: make(map[string]*store.EpisodicSummary)}
}

func (f *fakeEpisodicStore) Create(_ context.Context, ep *store.EpisodicSummary) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Enforce source-id uniqueness for dedup semantics the real store has.
	for _, r := range f.rows {
		if r.SourceID == ep.SourceID && r.AgentID == ep.AgentID && r.UserID == ep.UserID {
			return errors.New("fake: duplicate source_id")
		}
	}
	f.rows[ep.ID.String()] = ep
	return nil
}

func (f *fakeEpisodicStore) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.rows, id)
	return nil
}

func (f *fakeEpisodicStore) ExistsBySourceID(_ context.Context, agentID, userID, sourceID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.SourceID == sourceID && r.AgentID.String() == agentID && r.UserID == userID {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeEpisodicStore) List(_ context.Context, agentID, userID string, limit, offset int) ([]store.EpisodicSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var matches []store.EpisodicSummary
	for _, r := range f.rows {
		if r.AgentID.String() == agentID && r.UserID == userID {
			matches = append(matches, *r)
		}
	}
	start := offset
	if start > len(matches) {
		return nil, nil
	}
	end := start + limit
	if end > len(matches) {
		end = len(matches)
	}
	return matches[start:end], nil
}

// Unused methods — stubbed.
func (f *fakeEpisodicStore) Get(context.Context, string) (*store.EpisodicSummary, error) {
	return nil, errors.New("not used")
}
func (f *fakeEpisodicStore) Search(context.Context, string, string, string, store.EpisodicSearchOptions) ([]store.EpisodicSearchResult, error) {
	return nil, errors.New("not used")
}
func (f *fakeEpisodicStore) PruneExpired(context.Context) (int, error) { return 0, nil }
func (f *fakeEpisodicStore) ListUnpromoted(context.Context, string, string, int) ([]store.EpisodicSummary, error) {
	return nil, nil
}
func (f *fakeEpisodicStore) ListUnpromotedScored(context.Context, string, string, int) ([]store.EpisodicSummary, error) {
	return nil, nil
}
func (f *fakeEpisodicStore) MarkPromoted(context.Context, []string) error { return nil }
func (f *fakeEpisodicStore) CountUnpromoted(context.Context, string, string) (int, error) {
	return 0, nil
}
func (f *fakeEpisodicStore) RecordRecall(context.Context, string, float64) error { return nil }
func (f *fakeEpisodicStore) SetEmbeddingProvider(store.EmbeddingProvider)        {}
func (f *fakeEpisodicStore) Close() error                                        { return nil }

// ---- fake LLM ----

type fakeLLMClient struct {
	response string
	err      error
	called   int
}

func (f *fakeLLMClient) Chat(_ context.Context, _ providers.ChatRequest) (*providers.ChatResponse, error) {
	f.called++
	if f.err != nil {
		return nil, f.err
	}
	return &providers.ChatResponse{
		Content: f.response,
		Usage:   &providers.Usage{CompletionTokens: 42},
	}, nil
}
func (f *fakeLLMClient) DefaultModel() string { return "fake-model" }

type fakeResolver struct{ client *fakeLLMClient }

func (r fakeResolver) Resolve(_ context.Context, _ uuid.UUID) (LLMClient, string) {
	if r.client == nil {
		return nil, ""
	}
	return r.client, "fake-model"
}

// ---------- tests ----------

func TestSummarizer_EmptyMessagesSkip(t *testing.T) {
	s := NewSummarizer(newFakeEpisodicStore(), NewNoopLLMResolver(), SummarizerConfig{})
	err := s.Summarize(context.Background(), SummarizeInput{
		AgentID:  uuid.New(),
		PSID:     "PSID1",
		PageID:   "PAGE1",
		SourceID: "fb_backfill:PAGE1:PSID1",
	})
	if err != nil {
		t.Errorf("empty messages should skip silently, got %v", err)
	}
}

func TestSummarizer_ShortPath_NoLLMCall(t *testing.T) {
	ep := newFakeEpisodicStore()
	fakeLLM := &fakeLLMClient{response: `{"summary":"x","l0":"x","topics":["x"]}`}
	s := NewSummarizer(ep, fakeResolver{client: fakeLLM}, SummarizerConfig{ShortPathThreshold: 20})

	msgs := []Message{
		{ID: "m1", Message: "Hi, I want to order pizza", From: ConversationParticipant{ID: "PSID1"},
			CreatedTime: parseTime("2024-06-01T10:00:00Z")},
		{ID: "m2", Message: "Sure, which one?", From: ConversationParticipant{ID: "PAGE1"},
			CreatedTime: parseTime("2024-06-01T10:01:00Z")},
	}
	in := SummarizeInput{
		InstanceID: uuid.New(), TenantID: uuid.New(), AgentID: uuid.New(),
		PageID: "PAGE1", PSID: "PSID1",
		SourceID: "fb_backfill:PAGE1:PSID1",
		Messages: msgs,
	}
	if err := s.Summarize(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if fakeLLM.called != 0 {
		t.Errorf("short path should not call LLM, got %d calls", fakeLLM.called)
	}
	if len(ep.rows) != 1 {
		t.Fatalf("want 1 episodic row, got %d", len(ep.rows))
	}
	for _, r := range ep.rows {
		if r.SourceID != in.SourceID {
			t.Errorf("SourceID mismatch: %s", r.SourceID)
		}
		if r.SessionKey != "PSID1" {
			t.Errorf("SessionKey must be PSID, got %s", r.SessionKey)
		}
		if r.UserID != "PSID1" {
			t.Errorf("UserID must be PSID, got %s", r.UserID)
		}
		if r.TurnCount != 2 {
			t.Errorf("TurnCount=%d, want 2", r.TurnCount)
		}
		if r.L0Abstract == "" {
			t.Errorf("L0Abstract empty")
		}
	}
}

func TestSummarizer_LongPath_UsesLLM(t *testing.T) {
	ep := newFakeEpisodicStore()
	fakeLLM := &fakeLLMClient{
		response: `{"summary":"Customer wanted a refund.","l0":"Refund request","topics":["refund","order"]}`,
	}
	s := NewSummarizer(ep, fakeResolver{client: fakeLLM}, SummarizerConfig{ShortPathThreshold: 2})

	msgs := make([]Message, 10)
	for i := range msgs {
		msgs[i] = Message{
			ID: fmt.Sprintf("m%d", i),
			Message: fmt.Sprintf("message %d", i),
			From: ConversationParticipant{ID: "PSID1"},
			CreatedTime: parseTime("2024-06-01T10:00:00Z").Add(time.Duration(i) * time.Minute),
		}
	}
	in := SummarizeInput{
		AgentID: uuid.New(), TenantID: uuid.New(),
		PageID: "PAGE1", PSID: "PSID1",
		SourceID: SourceIDFor("PAGE1", "PSID1"),
		Messages: msgs,
	}
	if err := s.Summarize(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if fakeLLM.called != 1 {
		t.Errorf("LLM should be called once, got %d", fakeLLM.called)
	}
	for _, r := range ep.rows {
		if r.Summary != "Customer wanted a refund." {
			t.Errorf("summary mismatch: %s", r.Summary)
		}
		if r.L0Abstract != "Refund request" {
			t.Errorf("l0 mismatch: %s", r.L0Abstract)
		}
		if len(r.KeyTopics) != 2 {
			t.Errorf("topics count=%d, want 2", len(r.KeyTopics))
		}
	}
}

func TestSummarizer_LLMFailure_FallsBackToShortPath(t *testing.T) {
	ep := newFakeEpisodicStore()
	fakeLLM := &fakeLLMClient{err: errors.New("timeout")}
	s := NewSummarizer(ep, fakeResolver{client: fakeLLM}, SummarizerConfig{ShortPathThreshold: 2})

	msgs := make([]Message, 10)
	for i := range msgs {
		msgs[i] = Message{
			Message: "text " + fmt.Sprint(i),
			From:    ConversationParticipant{ID: "PSID1"},
			CreatedTime: parseTime("2024-06-01T10:00:00Z").Add(time.Duration(i) * time.Minute),
		}
	}
	in := SummarizeInput{
		AgentID: uuid.New(), TenantID: uuid.New(),
		PageID: "PAGE1", PSID: "PSID1",
		SourceID: SourceIDFor("PAGE1", "PSID1"),
		Messages: msgs,
	}
	if err := s.Summarize(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if fakeLLM.called != 1 {
		t.Errorf("LLM attempted once, got %d", fakeLLM.called)
	}
	if len(ep.rows) != 1 {
		t.Errorf("fallback still writes episodic, got %d rows", len(ep.rows))
	}
}

func TestSummarizer_NoLLMResolver(t *testing.T) {
	ep := newFakeEpisodicStore()
	// Long conversation + no resolver → must still produce via concat path.
	s := NewSummarizer(ep, nil, SummarizerConfig{ShortPathThreshold: 2})
	msgs := make([]Message, 5)
	for i := range msgs {
		msgs[i] = Message{Message: "x", From: ConversationParticipant{ID: "PSID1"},
			CreatedTime: parseTime("2024-06-01T10:00:00Z")}
	}
	in := SummarizeInput{
		AgentID: uuid.New(), TenantID: uuid.New(),
		PageID: "PAGE1", PSID: "PSID1",
		SourceID: SourceIDFor("PAGE1", "PSID1"),
		Messages: msgs,
	}
	if err := s.Summarize(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if len(ep.rows) != 1 {
		t.Fatalf("expected concat fallback, got %d rows", len(ep.rows))
	}
}

func TestSummarizer_Idempotent_SkipExisting(t *testing.T) {
	ep := newFakeEpisodicStore()
	s := NewSummarizer(ep, NewNoopLLMResolver(), SummarizerConfig{})
	in := SummarizeInput{
		AgentID: uuid.New(), TenantID: uuid.New(),
		PageID: "PAGE1", PSID: "PSID1",
		SourceID: SourceIDFor("PAGE1", "PSID1"),
		Messages: []Message{{Message: "hi", From: ConversationParticipant{ID: "PSID1"},
			CreatedTime: parseTime("2024-06-01T10:00:00Z")}},
	}
	// First call creates.
	if err := s.Summarize(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	// Second call must skip (same SourceID) → still 1 row.
	if err := s.Summarize(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if len(ep.rows) != 1 {
		t.Errorf("idempotency broken: rows=%d", len(ep.rows))
	}
}

func TestSummarizer_ForceRecreate(t *testing.T) {
	ep := newFakeEpisodicStore()
	s := NewSummarizer(ep, NewNoopLLMResolver(), SummarizerConfig{})
	baseIn := SummarizeInput{
		AgentID: uuid.New(), TenantID: uuid.New(),
		PageID: "PAGE1", PSID: "PSID1",
		SourceID: SourceIDFor("PAGE1", "PSID1"),
		Messages: []Message{{Message: "v1", From: ConversationParticipant{ID: "PSID1"},
			CreatedTime: parseTime("2024-06-01T10:00:00Z")}},
	}
	if err := s.Summarize(context.Background(), baseIn); err != nil {
		t.Fatal(err)
	}
	forced := baseIn
	forced.Messages = []Message{{Message: "v2-new", From: ConversationParticipant{ID: "PSID1"},
		CreatedTime: parseTime("2024-07-01T10:00:00Z")}}
	forced.ForceRecreate = true
	if err := s.Summarize(context.Background(), forced); err != nil {
		t.Fatal(err)
	}
	if len(ep.rows) != 1 {
		t.Fatalf("ForceRecreate should produce exactly 1 row, got %d", len(ep.rows))
	}
	for _, r := range ep.rows {
		if !strings.Contains(r.Summary, "v2-new") {
			t.Errorf("ForceRecreate did not replace; summary=%s", r.Summary)
		}
	}
}

func TestSummarizer_ParseLLMSummary_WithCodeFence(t *testing.T) {
	raw := "Here is the summary:\n\n```json\n{\"summary\":\"hello\",\"l0\":\"x\",\"topics\":[\"a\",\"b\"]}\n```\n"
	r, err := parseLLMSummary(raw)
	if err != nil {
		t.Fatal(err)
	}
	if r.Summary != "hello" || r.L0 != "x" {
		t.Errorf("parse: %+v", r)
	}
}

func TestSummarizer_ParseLLMSummary_MalformedRejected(t *testing.T) {
	if _, err := parseLLMSummary("not json at all"); err == nil {
		t.Errorf("expected error for non-JSON")
	}
	if _, err := parseLLMSummary(`{"l0":"x"}`); err == nil {
		t.Errorf("missing summary must error")
	}
}

func TestSummarizer_BuildTranscript_Truncates(t *testing.T) {
	msgs := make([]Message, 300)
	for i := range msgs {
		msgs[i] = Message{Message: fmt.Sprintf("msg%d", i),
			From:        ConversationParticipant{ID: "PSID1"},
			CreatedTime: parseTime("2024-06-01T10:00:00Z").Add(time.Duration(i) * time.Minute)}
	}
	out := buildTranscript(msgs, "PAGE1", 100)
	if !strings.Contains(out, "Earlier 200 messages omitted") {
		t.Errorf("expected truncation header; got head: %q", out[:120])
	}
	// Last message must be present (msg299).
	if !strings.Contains(out, "msg299") {
		t.Errorf("tail missing — last message should be kept")
	}
}

func TestExtractTopics_SmokeTest(t *testing.T) {
	msgs := []Message{
		{Message: "refund refund please refund the order", From: ConversationParticipant{ID: "PSID1"}},
		{Message: "order is broken, requesting refund", From: ConversationParticipant{ID: "PSID1"}},
	}
	got := extractTopics(msgs)
	if len(got) == 0 {
		t.Errorf("expected topics, got none")
	}
	// "refund" should surface (4 occurrences, non-stopword, len≥4)
	found := false
	for _, g := range got {
		if g == "refund" {
			found = true
		}
	}
	if !found {
		t.Errorf("topic 'refund' missing from %v", got)
	}
}
