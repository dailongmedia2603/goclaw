package fbbackfill

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// e2eFactory wires a real BackfillClient against a test server URL.
// Exercises the actual HTTP-level code path from phase 1.
type e2eFactory struct{ baseURL string }

func (f e2eFactory) Build(token, pageID string) GraphAPIBackfill {
	return NewBackfillClient(token, pageID,
		WithBaseURL(f.baseURL),
		WithSleep(func(time.Duration) {}),
		WithMaxRetries(2),
	)
}

// TestE2E_FullPipeline drives:
//   - real BackfillClient against httptest.Server
//   - real JobRunner
//   - real summarizerImpl (short path only, no LLM call)
//   - fakeEpisodicStore + fakeInstanceStore
//
// Verifies an EpisodicSummary lands with SourceID matching the PSID
// convention, which is the proof-of-work that the whole feature
// delivers value.
func TestE2E_FullPipeline(t *testing.T) {
	// Mock Graph API. Returns one page of 2 conversations, each with 2 messages.
	var convoCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		q := r.URL.Query()
		if q.Get("access_token") == "" {
			t.Errorf("access_token missing in request to %s", path)
		}
		switch {
		case strings.HasSuffix(path, "/PAGE1/conversations"):
			convoCalls++
			_, _ = w.Write([]byte(`{
				"data": [
					{"id":"t_1","updated_time":"2024-06-15T08:30:45+0000","message_count":2,
					 "participants":{"data":[
					   {"id":"PSID_ALICE","name":"Alice"},
					   {"id":"PAGE1","name":"My Page"}
					 ]}},
					{"id":"t_2","updated_time":"2024-06-14T09:00:00+0000","message_count":2,
					 "participants":{"data":[
					   {"id":"PSID_BOB","name":"Bob"},
					   {"id":"PAGE1","name":"My Page"}
					 ]}}
				],
				"paging":{"cursors":{"after":""}}
			}`))
		case strings.HasSuffix(path, "/t_1/messages"):
			_, _ = w.Write([]byte(`{
				"data":[
					{"id":"m_1","message":"Hi, I need help with my order #42",
					 "from":{"id":"PSID_ALICE","name":"Alice"},
					 "created_time":"2024-06-15T08:30:00+0000"},
					{"id":"m_2","message":"Order #42 is on its way — tracking: XYZ",
					 "from":{"id":"PAGE1","name":"My Page"},
					 "created_time":"2024-06-15T08:31:00+0000"}
				],
				"paging":{"cursors":{"after":""}}
			}`))
		case strings.HasSuffix(path, "/t_2/messages"):
			_, _ = w.Write([]byte(`{
				"data":[
					{"id":"m_3","message":"Can I get a refund?",
					 "from":{"id":"PSID_BOB","name":"Bob"},
					 "created_time":"2024-06-14T09:00:00+0000"}
				],
				"paging":{"cursors":{"after":""}}
			}`))
		default:
			t.Errorf("unexpected path: %s", path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// Instance + stores.
	inst := newFakeInstanceStore()
	instID := uuid.New()
	tenantID := uuid.New()
	agentID := uuid.New()
	creds, _ := json.Marshal(facebookCreds{PageAccessToken: "PAT_LIVE"})
	inst.seed(&store.ChannelInstanceData{
		BaseModel:   store.BaseModel{ID: instID},
		TenantID:    tenantID,
		AgentID:     agentID,
		Name:        "fb-e2e",
		ChannelType: "facebook",
		Credentials: creds,
		Config:      json.RawMessage(`{"page_id":"PAGE1","features":{"messenger_auto_reply":true}}`),
	})

	ep := newFakeEpisodicStore()
	ss := NewStateStore(inst)
	summ := NewSummarizer(ep, NewNoopLLMResolver(), SummarizerConfig{ShortPathThreshold: 20})

	emitter := &fakeEmitter{}
	runner := NewJobRunner(RunnerDeps{
		StateStore:        ss,
		Instances:         inst,
		ClientFactory:     e2eFactory{baseURL: srv.URL},
		Summarizer:        summ,
		Emitter:           emitter,
		MaxConcurrentJobs: 1,
	})

	if err := runner.Start(context.Background(), instID, StartOpts{TriggeredBy: "manual"}); err != nil {
		t.Fatal(err)
	}
	runner.Wait(instID)

	st, _ := ss.Get(context.Background(), instID)
	if st.Status != StatusCompleted {
		t.Fatalf("status=%s want completed (err=%s)", st.Status, st.LastError)
	}
	if st.ConversationsDone != 2 {
		t.Errorf("ConversationsDone=%d, want 2", st.ConversationsDone)
	}
	if st.MessagesIngested != 3 {
		t.Errorf("MessagesIngested=%d, want 3", st.MessagesIngested)
	}
	if st.EpisodicsCreated != 2 {
		t.Errorf("EpisodicsCreated=%d, want 2", st.EpisodicsCreated)
	}

	// The key invariant: episodic entries are keyed by PSID to match the
	// webhook runtime's session-key convention, so the agent's memory
	// search hits them when the customer next messages.
	gotSources := make(map[string]bool)
	for _, r := range ep.rows {
		gotSources[r.SourceID] = true
		if r.UserID != "PSID_ALICE" && r.UserID != "PSID_BOB" {
			t.Errorf("UserID=%s not a PSID", r.UserID)
		}
		if r.SessionKey != r.UserID {
			t.Errorf("SessionKey must equal UserID (PSID), got %s/%s", r.SessionKey, r.UserID)
		}
		if r.L0Abstract == "" {
			t.Errorf("L0Abstract empty")
		}
	}
	if !gotSources[SourceIDFor("PAGE1", "PSID_ALICE")] {
		t.Errorf("Alice's SourceID missing: got %v", gotSources)
	}
	if !gotSources[SourceIDFor("PAGE1", "PSID_BOB")] {
		t.Errorf("Bob's SourceID missing: got %v", gotSources)
	}

	// Idempotency: re-run should skip both (SkipExisting=true default).
	if err := runner.Start(context.Background(), instID, StartOpts{SkipExisting: true, TriggeredBy: "retry"}); err != nil {
		t.Fatal(err)
	}
	runner.Wait(instID)
	if len(ep.rows) != 2 {
		t.Errorf("re-run should not duplicate; rows=%d", len(ep.rows))
	}
	st2, _ := ss.Get(context.Background(), instID)
	if st2.ConversationsSkipped == 0 {
		t.Errorf("re-run should have skipped >0, got Skipped=%d", st2.ConversationsSkipped)
	}

	// Emitter should have at least started + completed events.
	events := emitter.Events()
	if len(events) == 0 {
		t.Errorf("no events emitted")
	}
	sawStarted := false
	sawCompleted := false
	for _, e := range events {
		if e == "started" {
			sawStarted = true
		}
		if e == "completed" {
			sawCompleted = true
		}
	}
	if !sawStarted || !sawCompleted {
		t.Errorf("missing lifecycle events: %v", events)
	}

	if convoCalls < 1 {
		t.Errorf("expected at least 1 conversations call, got %d", convoCalls)
	}
}

// TestE2E_RateLimitPausesJob drives the runner against a mock that
// returns a saturated BUC response, proving the paused-with-auto-resume
// flow lands correctly.
func TestE2E_RateLimitPausesJob(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Business-Use-Case-Usage",
			`{"PAGE1":[{"type":"pages","call_count":100,"total_cputime":100,"total_time":100,"estimated_time_to_regain_access":15}]}`)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":80001,"message":"BUC exhausted"}}`))
	}))
	defer srv.Close()

	inst := newFakeInstanceStore()
	instID := uuid.New()
	creds, _ := json.Marshal(facebookCreds{PageAccessToken: "PAT"})
	inst.seed(&store.ChannelInstanceData{
		BaseModel:   store.BaseModel{ID: instID},
		TenantID:    uuid.New(),
		AgentID:     uuid.New(),
		Name:        "fb-rate",
		ChannelType: "facebook",
		Credentials: creds,
		Config:      json.RawMessage(`{"page_id":"PAGE1"}`),
	})
	ss := NewStateStore(inst)
	runner := NewJobRunner(RunnerDeps{
		StateStore:    ss,
		Instances:     inst,
		ClientFactory: e2eFactory{baseURL: srv.URL},
		Summarizer:    NewSummarizer(newFakeEpisodicStore(), NewNoopLLMResolver(), SummarizerConfig{}),
		Emitter:       &fakeEmitter{},
	})

	if err := runner.Start(context.Background(), instID, StartOpts{}); err != nil {
		t.Fatal(err)
	}
	// Wait for paused.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		st, _ := ss.Get(context.Background(), instID)
		if st != nil && st.Status == StatusPaused {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	st, _ := ss.Get(context.Background(), instID)
	t.Fatalf("job did not transition to paused; got %+v", st)
}

// TestE2E_AuthExpiredFails proves the auth-error path.
func TestE2E_AuthExpiredFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid OAuth access token","code":190,"type":"OAuthException"}}`))
	}))
	defer srv.Close()

	inst := newFakeInstanceStore()
	instID := uuid.New()
	creds, _ := json.Marshal(facebookCreds{PageAccessToken: "EXPIRED"})
	inst.seed(&store.ChannelInstanceData{
		BaseModel:   store.BaseModel{ID: instID},
		TenantID:    uuid.New(),
		AgentID:     uuid.New(),
		Name:        "fb-expired",
		ChannelType: "facebook",
		Credentials: creds,
		Config:      json.RawMessage(`{"page_id":"PAGE1"}`),
	})
	ss := NewStateStore(inst)
	runner := NewJobRunner(RunnerDeps{
		StateStore:    ss,
		Instances:     inst,
		ClientFactory: e2eFactory{baseURL: srv.URL},
		Summarizer:    NewSummarizer(newFakeEpisodicStore(), NewNoopLLMResolver(), SummarizerConfig{}),
		Emitter:       &fakeEmitter{},
	})
	if err := runner.Start(context.Background(), instID, StartOpts{}); err != nil {
		t.Fatal(err)
	}
	runner.Wait(instID)
	st, _ := ss.Get(context.Background(), instID)
	if st.Status != StatusFailed {
		t.Fatalf("want failed, got %s", st.Status)
	}
	if !strings.Contains(st.LastError, "expired") {
		t.Errorf("LastError should mention 'expired', got %q", st.LastError)
	}
}

// Use fmt to avoid unused import warnings when the file is imported.
var _ = fmt.Sprint
