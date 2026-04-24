package fbbackfill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// ---------- Fakes ----------

type fakeGraphClient struct {
	// Convos is the list returned across ListConversations pagination.
	// Split into ConvosPages (per-page slices) for multi-page tests.
	ConvosPages [][]Conversation
	convosCall  int

	// Messages[convoID] = slice of messages for that conversation
	Messages map[string][]Message

	// Error injection
	ErrConversations error
	ErrMessages      error
	ErrAfterCallN    int // if >0, return ErrConversations after the Nth call
	buc              *bucTracker
}

func newFakeGraphClient() *fakeGraphClient {
	return &fakeGraphClient{
		Messages: make(map[string][]Message),
		buc:      &bucTracker{},
	}
}

func (f *fakeGraphClient) ListConversations(_ context.Context, cursor string) (*ListConversationsPage, error) {
	f.convosCall++
	if f.ErrAfterCallN > 0 && f.convosCall >= f.ErrAfterCallN && f.ErrConversations != nil {
		return nil, f.ErrConversations
	}
	if f.ErrConversations != nil && f.ErrAfterCallN == 0 {
		return nil, f.ErrConversations
	}
	if len(f.ConvosPages) == 0 {
		return &ListConversationsPage{}, nil
	}
	// Cursor encodes the page index as "page-N".
	idx := 0
	if cursor != "" {
		_, _ = fmt.Sscanf(cursor, "page-%d", &idx)
	}
	if idx >= len(f.ConvosPages) {
		return &ListConversationsPage{}, nil
	}
	next := ""
	if idx+1 < len(f.ConvosPages) {
		next = fmt.Sprintf("page-%d", idx+1)
	}
	return &ListConversationsPage{Data: f.ConvosPages[idx], Next: next}, nil
}

func (f *fakeGraphClient) ListMessages(_ context.Context, convoID, _ string) (*ListMessagesPage, error) {
	if f.ErrMessages != nil {
		return nil, f.ErrMessages
	}
	return &ListMessagesPage{Data: f.Messages[convoID], Next: ""}, nil
}

func (f *fakeGraphClient) BUCTracker() *bucTracker { return f.buc }

type fakeClientFactory struct{ client GraphAPIBackfill }

func (f fakeClientFactory) Build(_, _ string) GraphAPIBackfill { return f.client }

// fakeSummarizer records calls for test assertion.
type fakeSummarizer struct {
	mu           sync.Mutex
	calls        []SummarizeInput
	existing     map[string]bool // sourceID → already summarized
	summarizeErr error
}

func newFakeSummarizer() *fakeSummarizer { return &fakeSummarizer{existing: make(map[string]bool)} }

func (s *fakeSummarizer) AlreadySummarized(_ context.Context, _ uuid.UUID, _, sourceID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.existing[sourceID], nil
}

func (s *fakeSummarizer) Summarize(_ context.Context, input SummarizeInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.summarizeErr != nil {
		return s.summarizeErr
	}
	s.calls = append(s.calls, input)
	s.existing[input.SourceID] = true
	return nil
}

func (s *fakeSummarizer) Calls() []SummarizeInput {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]SummarizeInput, len(s.calls))
	copy(out, s.calls)
	return out
}

// fakeEmitter records emitted events.
type fakeEmitter struct {
	mu     sync.Mutex
	events []string
}

func (e *fakeEmitter) push(s string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, s)
}
func (e *fakeEmitter) EmitStarted(_, _ uuid.UUID)                   { e.push("started") }
func (e *fakeEmitter) EmitProgress(_, _ uuid.UUID, _ *BackfillState) { e.push("progress") }
func (e *fakeEmitter) EmitPaused(_, _ uuid.UUID, reason string)     { e.push("paused:" + reason) }
func (e *fakeEmitter) EmitResumed(_, _ uuid.UUID)                   { e.push("resumed") }
func (e *fakeEmitter) EmitCompleted(_, _ uuid.UUID, _ *BackfillState) { e.push("completed") }
func (e *fakeEmitter) EmitFailed(_, _ uuid.UUID, _ string)          { e.push("failed") }
func (e *fakeEmitter) Events() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, len(e.events))
	copy(out, e.events)
	return out
}

// ---------- Test harness ----------

type testHarness struct {
	t          *testing.T
	instances  *fakeInstanceStore
	stateStore *StateStore
	graph      *fakeGraphClient
	summarizer *fakeSummarizer
	emitter    *fakeEmitter
	runner     *JobRunner
	instanceID uuid.UUID
	tenantID   uuid.UUID
	agentID    uuid.UUID
}

func newTestHarness(t *testing.T) *testHarness {
	t.Helper()
	inst := newFakeInstanceStore()
	instID := uuid.New()
	tenantID := uuid.New()
	agentID := uuid.New()
	creds, _ := json.Marshal(facebookCreds{PageAccessToken: "PAT123"})
	inst.seed(&store.ChannelInstanceData{
		BaseModel:   store.BaseModel{ID: instID},
		TenantID:    tenantID,
		AgentID:     agentID,
		Name:        "fb-harness",
		ChannelType: "facebook",
		Credentials: creds,
		Config:      json.RawMessage(`{"page_id":"PAGE1","features":{"messenger_auto_reply":true}}`),
	})
	ss := NewStateStore(inst)
	graph := newFakeGraphClient()
	summ := newFakeSummarizer()
	em := &fakeEmitter{}
	r := NewJobRunner(RunnerDeps{
		StateStore:        ss,
		Instances:         inst,
		ClientFactory:     fakeClientFactory{client: graph},
		Summarizer:        summ,
		Emitter:           em,
		MaxConcurrentJobs: 2,
	})
	return &testHarness{
		t:          t,
		instances:  inst,
		stateStore: ss,
		graph:      graph,
		summarizer: summ,
		emitter:    em,
		runner:     r,
		instanceID: instID,
		tenantID:   tenantID,
		agentID:    agentID,
	}
}

// waitForStatus polls state until it matches target or timeout.
func (h *testHarness) waitForStatus(target JobStatus, timeout time.Duration) *BackfillState {
	h.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		st, err := h.stateStore.Get(context.Background(), h.instanceID)
		if err == nil && st != nil && st.Status == target {
			return st
		}
		time.Sleep(5 * time.Millisecond)
	}
	st, _ := h.stateStore.Get(context.Background(), h.instanceID)
	h.t.Fatalf("waitForStatus(%s) timed out after %v; current=%+v", target, timeout, st)
	return nil
}

// ---------- Tests ----------

func TestJobRunner_StartMissingCreds(t *testing.T) {
	h := newTestHarness(t)
	// Overwrite with empty creds.
	inst, _ := h.instances.Get(context.Background(), h.instanceID)
	inst.Credentials = nil
	h.instances.seed(inst)
	err := h.runner.Start(context.Background(), h.instanceID, StartOpts{})
	if !errors.Is(err, ErrMissingAccessToken) {
		t.Fatalf("expected ErrMissingAccessToken, got %v", err)
	}
}

func TestJobRunner_StartMissingPageID(t *testing.T) {
	h := newTestHarness(t)
	inst, _ := h.instances.Get(context.Background(), h.instanceID)
	inst.Config = json.RawMessage(`{}`)
	h.instances.seed(inst)
	err := h.runner.Start(context.Background(), h.instanceID, StartOpts{})
	if !errors.Is(err, ErrMissingPageID) {
		t.Fatalf("expected ErrMissingPageID, got %v", err)
	}
}

func TestJobRunner_NotFacebookType(t *testing.T) {
	h := newTestHarness(t)
	inst, _ := h.instances.Get(context.Background(), h.instanceID)
	inst.ChannelType = "telegram"
	h.instances.seed(inst)
	err := h.runner.Start(context.Background(), h.instanceID, StartOpts{})
	if err == nil || !contains(err.Error(), "not facebook") {
		t.Fatalf("expected type-mismatch error, got %v", err)
	}
}

func TestJobRunner_FullRun_SinglePage(t *testing.T) {
	h := newTestHarness(t)
	h.graph.ConvosPages = [][]Conversation{
		{
			{ID: "t_1", Participants: participants("PSID1", "PAGE1")},
			{ID: "t_2", Participants: participants("PSID2", "PAGE1")},
		},
	}
	h.graph.Messages["t_1"] = []Message{
		{ID: "m_1", Message: "Hi", From: ConversationParticipant{ID: "PSID1"},
			CreatedTime: parseTime("2024-06-01T10:00:00Z")},
	}
	h.graph.Messages["t_2"] = []Message{
		{ID: "m_2", Message: "Hello", From: ConversationParticipant{ID: "PSID2"},
			CreatedTime: parseTime("2024-06-02T10:00:00Z")},
	}

	if err := h.runner.Start(context.Background(), h.instanceID, StartOpts{TriggeredBy: "manual"}); err != nil {
		t.Fatal(err)
	}
	st := h.waitForStatus(StatusCompleted, 2*time.Second)
	if st.ConversationsDone != 2 {
		t.Errorf("ConversationsDone=%d, want 2", st.ConversationsDone)
	}
	if st.MessagesIngested != 2 {
		t.Errorf("MessagesIngested=%d, want 2", st.MessagesIngested)
	}
	if st.EpisodicsCreated != 2 {
		t.Errorf("EpisodicsCreated=%d, want 2", st.EpisodicsCreated)
	}
	if len(h.summarizer.Calls()) != 2 {
		t.Errorf("Summarize calls=%d, want 2", len(h.summarizer.Calls()))
	}
	got := map[string]bool{}
	for _, c := range h.summarizer.Calls() {
		got[c.PSID] = true
		if c.PageID != "PAGE1" {
			t.Errorf("PageID=%s, want PAGE1", c.PageID)
		}
	}
	if !got["PSID1"] || !got["PSID2"] {
		t.Errorf("expected both PSIDs summarized, got %v", got)
	}
}

func TestJobRunner_FullRun_Pagination(t *testing.T) {
	h := newTestHarness(t)
	h.graph.ConvosPages = [][]Conversation{
		{{ID: "t_1", Participants: participants("PSID1", "PAGE1")}},
		{{ID: "t_2", Participants: participants("PSID2", "PAGE1")}},
		{{ID: "t_3", Participants: participants("PSID3", "PAGE1")}},
	}
	for i, convoID := range []string{"t_1", "t_2", "t_3"} {
		h.graph.Messages[convoID] = []Message{
			{ID: fmt.Sprintf("m_%d", i), Message: "Hi", From: ConversationParticipant{ID: fmt.Sprintf("PSID%d", i+1)}},
		}
	}
	if err := h.runner.Start(context.Background(), h.instanceID, StartOpts{}); err != nil {
		t.Fatal(err)
	}
	st := h.waitForStatus(StatusCompleted, 2*time.Second)
	if st.ConversationsDone != 3 {
		t.Errorf("want 3 convos done, got %d", st.ConversationsDone)
	}
}

func TestJobRunner_SkipExisting(t *testing.T) {
	h := newTestHarness(t)
	h.graph.ConvosPages = [][]Conversation{
		{
			{ID: "t_1", Participants: participants("PSID1", "PAGE1")},
			{ID: "t_2", Participants: participants("PSID2", "PAGE1")},
		},
	}
	h.graph.Messages["t_1"] = []Message{{ID: "m_1", From: ConversationParticipant{ID: "PSID1"}}}
	h.graph.Messages["t_2"] = []Message{{ID: "m_2", From: ConversationParticipant{ID: "PSID2"}}}
	// Pre-mark PSID1 as existing.
	h.summarizer.existing[SourceIDFor("PAGE1", "PSID1")] = true

	if err := h.runner.Start(context.Background(), h.instanceID, StartOpts{SkipExisting: true}); err != nil {
		t.Fatal(err)
	}
	st := h.waitForStatus(StatusCompleted, 2*time.Second)
	if st.ConversationsSkipped != 1 {
		t.Errorf("Skipped=%d, want 1", st.ConversationsSkipped)
	}
	// Only PSID2 should have been summarized.
	calls := h.summarizer.Calls()
	if len(calls) != 1 || calls[0].PSID != "PSID2" {
		t.Errorf("expected only PSID2 summarized, got %+v", calls)
	}
}

func TestJobRunner_MaxConversationsCap(t *testing.T) {
	h := newTestHarness(t)
	h.graph.ConvosPages = [][]Conversation{
		{
			{ID: "t_1", Participants: participants("PSID1", "PAGE1")},
			{ID: "t_2", Participants: participants("PSID2", "PAGE1")},
			{ID: "t_3", Participants: participants("PSID3", "PAGE1")},
		},
	}
	for _, cid := range []string{"t_1", "t_2", "t_3"} {
		h.graph.Messages[cid] = []Message{{ID: "m", From: ConversationParticipant{ID: "PSID" + cid}}}
	}
	if err := h.runner.Start(context.Background(), h.instanceID, StartOpts{MaxConversations: 2}); err != nil {
		t.Fatal(err)
	}
	st := h.waitForStatus(StatusCompleted, 2*time.Second)
	if st.ConversationsDone != 2 {
		t.Errorf("want 2 (capped), got %d", st.ConversationsDone)
	}
	if !contains(st.LastError, "cap") {
		t.Errorf("expected cap message, got %q", st.LastError)
	}
}

func TestJobRunner_AuthExpiredFails(t *testing.T) {
	h := newTestHarness(t)
	h.graph.ErrConversations = fmt.Errorf("%w: code=190", ErrAuthExpired)
	if err := h.runner.Start(context.Background(), h.instanceID, StartOpts{}); err != nil {
		t.Fatal(err)
	}
	st := h.waitForStatus(StatusFailed, 2*time.Second)
	if !contains(st.LastError, "expired") {
		t.Errorf("expected expired msg, got %q", st.LastError)
	}
	if !containsStr(h.emitter.Events(), "failed") {
		t.Errorf("failed event not emitted")
	}
}

func TestJobRunner_RateLimitPauses(t *testing.T) {
	h := newTestHarness(t)
	h.graph.ErrConversations = fmt.Errorf("%w: saturated", ErrRateLimit)
	if err := h.runner.Start(context.Background(), h.instanceID, StartOpts{}); err != nil {
		t.Fatal(err)
	}
	st := h.waitForStatus(StatusPaused, 2*time.Second)
	if !contains(st.LastError, "rate limit") {
		t.Errorf("expected rate limit msg, got %q", st.LastError)
	}
	if !containsStr(h.emitter.Events(), "paused:rate_limit") {
		t.Errorf("paused:rate_limit event not emitted, got %v", h.emitter.Events())
	}
}

func TestJobRunner_Cancel(t *testing.T) {
	h := newTestHarness(t)
	// Give the job something to loop on.
	convos := make([]Conversation, 20)
	for i := range convos {
		convos[i] = Conversation{ID: fmt.Sprintf("t_%d", i), Participants: participants(fmt.Sprintf("PSID%d", i), "PAGE1")}
		h.graph.Messages[convos[i].ID] = []Message{{ID: "m", From: ConversationParticipant{ID: fmt.Sprintf("PSID%d", i)}}}
	}
	h.graph.ConvosPages = [][]Conversation{convos}
	if err := h.runner.Start(context.Background(), h.instanceID, StartOpts{}); err != nil {
		t.Fatal(err)
	}
	// Cancel almost immediately.
	if err := h.runner.Cancel(context.Background(), h.instanceID); err != nil {
		t.Fatal(err)
	}
	h.runner.Wait(h.instanceID)
	st, _ := h.stateStore.Get(context.Background(), h.instanceID)
	if st.Status != StatusCancelled && st.Status != StatusCompleted {
		// Cancel race: if the job finished before cancel landed, completed is acceptable.
		t.Errorf("status=%v, want cancelled or completed", st.Status)
	}
}

func TestJobRunner_RetryResetsCursors(t *testing.T) {
	h := newTestHarness(t)
	// Prime state as failed mid-run.
	failed := &BackfillState{
		Version: 1, Status: StatusFailed,
		ConversationCursor: "old-cursor",
		ConversationsDone:  5, MessagesIngested: 30, EpisodicsCreated: 4,
		LastError: "something broke",
	}
	if err := h.stateStore.Save(context.Background(), h.instanceID, failed); err != nil {
		t.Fatal(err)
	}
	h.graph.ConvosPages = [][]Conversation{
		{{ID: "t_1", Participants: participants("PSID1", "PAGE1")}},
	}
	h.graph.Messages["t_1"] = []Message{{ID: "m_1", From: ConversationParticipant{ID: "PSID1"}}}

	if err := h.runner.Retry(context.Background(), h.instanceID); err != nil {
		t.Fatal(err)
	}
	st := h.waitForStatus(StatusCompleted, 2*time.Second)
	if st.ConversationsDone != 1 {
		t.Errorf("Retry should reset counters; got Done=%d", st.ConversationsDone)
	}
	if st.ConversationCursor != "" {
		t.Errorf("Retry should clear cursor; got %q", st.ConversationCursor)
	}
	if st.TriggeredBy != "retry" {
		t.Errorf("TriggeredBy=%s, want retry", st.TriggeredBy)
	}
}

func TestJobRunner_DoubleStartRejected(t *testing.T) {
	h := newTestHarness(t)
	// Create a never-completing job by returning no convos after the first check.
	h.graph.ConvosPages = [][]Conversation{
		{{ID: "t_1", Participants: participants("PSID1", "PAGE1")}},
	}
	h.graph.Messages["t_1"] = []Message{{ID: "m", From: ConversationParticipant{ID: "PSID1"}}}
	// Block summarize to keep the job alive while we try a second Start.
	block := make(chan struct{})
	h.summarizer.summarizeErr = nil
	origSummarize := h.summarizer
	// We can't easily override method without wrapping — use a mutex hold.
	origSummarize.mu.Lock()
	go func() {
		// Release after we've confirmed double-start.
		<-block
		origSummarize.mu.Unlock()
	}()
	defer func() {
		close(block)
		h.runner.Wait(h.instanceID)
	}()

	if err := h.runner.Start(context.Background(), h.instanceID, StartOpts{}); err != nil {
		t.Fatal(err)
	}
	// Wait until runner records the job.
	for i := 0; i < 100; i++ {
		if h.runner.Running(h.instanceID) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	err := h.runner.Start(context.Background(), h.instanceID, StartOpts{})
	if err == nil || !contains(err.Error(), "already running") {
		t.Errorf("expected already-running error, got %v", err)
	}
}

func TestJobRunner_ResumeAfterGoroutineExit(t *testing.T) {
	h := newTestHarness(t)
	// Seed state as paused (simulating gateway restart).
	paused := NewBackfillState(StartOpts{TriggeredBy: "manual"})
	paused.Status = StatusPaused
	if err := h.stateStore.Save(context.Background(), h.instanceID, paused); err != nil {
		t.Fatal(err)
	}
	h.graph.ConvosPages = [][]Conversation{
		{{ID: "t_1", Participants: participants("PSID1", "PAGE1")}},
	}
	h.graph.Messages["t_1"] = []Message{{ID: "m", From: ConversationParticipant{ID: "PSID1"}}}
	if err := h.runner.Resume(context.Background(), h.instanceID); err != nil {
		t.Fatal(err)
	}
	st := h.waitForStatus(StatusCompleted, 2*time.Second)
	if st.ConversationsDone != 1 {
		t.Errorf("want 1 done, got %d", st.ConversationsDone)
	}
}

// ---------- helpers ----------

func participants(psid, pageID string) struct {
	Data []ConversationParticipant `json:"data"`
} {
	return struct {
		Data []ConversationParticipant `json:"data"`
	}{
		Data: []ConversationParticipant{
			{ID: psid, Name: "user-" + psid},
			{ID: pageID, Name: "My Page"},
		},
	}
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func contains(haystack string, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func containsStr(xs []string, needle string) bool {
	for _, s := range xs {
		if s == needle || indexOf(s, needle) >= 0 {
			return true
		}
	}
	return false
}

func indexOf(s, sub string) int {
	n, m := len(s), len(sub)
	if m == 0 {
		return 0
	}
	if m > n {
		return -1
	}
	for i := 0; i+m <= n; i++ {
		if s[i:i+m] == sub {
			return i
		}
	}
	return -1
}

// Keep atomic import from being removed.
var _ = atomic.Int32{}
