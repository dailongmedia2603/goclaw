package fbbackfill

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// noopSleep discards backoff waits so tests run instantly. Tests that
// verify backoff duration check the sleep args via a channel or counter.
func noopSleep(_ time.Duration) {}

// fixedClock returns a deterministic time for log/metric assertions.
func fixedClock() time.Time { return time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC) }

func newTestClient(server *httptest.Server, pageID string) *BackfillClient {
	return NewBackfillClient("test-token", pageID,
		WithBaseURL(server.URL),
		WithSleep(noopSleep),
		WithClock(fixedClock),
		WithMaxRetries(3),
	)
}

// ---------- ListConversations ----------

func TestListConversations_SinglePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v25.0/PAGE1/conversations") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("platform"); got != "MESSENGER" {
			t.Errorf("platform=%q, want MESSENGER", got)
		}
		if got := r.URL.Query().Get("access_token"); got != "test-token" {
			t.Errorf("access_token missing or wrong")
		}
		_, _ = w.Write([]byte(`{
			"data": [
				{"id":"t_1","updated_time":"2024-06-15T08:30:45+0000","message_count":3,
				 "participants":{"data":[{"id":"PSID1","name":"Alice"},{"id":"PAGE1","name":"My Page"}]}},
				{"id":"t_2","updated_time":"2024-06-14T09:00:00+0000","message_count":5,
				 "participants":{"data":[{"id":"PSID2","name":"Bob"},{"id":"PAGE1","name":"My Page"}]}}
			],
			"paging":{"cursors":{"before":"X","after":""}}
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "PAGE1")
	page, err := c.ListConversations(context.Background(), "")
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(page.Data) != 2 {
		t.Fatalf("got %d conversations, want 2", len(page.Data))
	}
	if page.Data[0].ID != "t_1" || page.Data[0].MessageCount != 3 {
		t.Errorf("conv[0]: %+v", page.Data[0])
	}
	if page.Data[0].UpdatedTime.Year() != 2024 {
		t.Errorf("UpdatedTime not parsed: %v", page.Data[0].UpdatedTime)
	}
	if page.Next != "" {
		t.Errorf("Next should be empty at end of list, got %q", page.Next)
	}
}

func TestListConversations_Pagination(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		after := r.URL.Query().Get("after")
		switch n {
		case 1:
			if after != "" {
				t.Errorf("first call should have no after cursor, got %q", after)
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"t_1"},{"id":"t_2"}],"paging":{"cursors":{"after":"CURSOR_A"}}}`))
		case 2:
			if after != "CURSOR_A" {
				t.Errorf("second call after=%q, want CURSOR_A", after)
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"t_3"}],"paging":{"cursors":{"after":""}}}`))
		default:
			t.Errorf("unexpected extra call %d", n)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv, "PAGE1")
	page1, err := c.ListConversations(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if page1.Next != "CURSOR_A" {
		t.Errorf("page1.Next=%q, want CURSOR_A", page1.Next)
	}
	page2, err := c.ListConversations(context.Background(), page1.Next)
	if err != nil {
		t.Fatal(err)
	}
	if page2.Next != "" {
		t.Errorf("page2.Next should be empty, got %q", page2.Next)
	}
	total := len(page1.Data) + len(page2.Data)
	if total != 3 {
		t.Errorf("total conversations = %d, want 3", total)
	}
	if atomic.LoadInt32(&callCount) != 2 {
		t.Errorf("call count = %d, want 2", callCount)
	}
}

// ---------- ListMessages ----------

func TestListMessages_WithAttachments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"data": [
				{"id":"m_1","message":"Hi","created_time":"2024-06-15T08:30:00+0000",
				 "from":{"id":"PSID1","name":"Alice"}},
				{"id":"m_2","message":"","created_time":"2024-06-15T08:31:00+0000",
				 "from":{"id":"PSID1","name":"Alice"},
				 "attachments":{"data":[{"mime_type":"image/jpeg","name":"photo.jpg","size":1024}]}}
			],
			"paging":{"cursors":{"after":""}}
		}`))
	}))
	defer srv.Close()
	c := newTestClient(srv, "PAGE1")
	page, err := c.ListMessages(context.Background(), "t_1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Data) != 2 {
		t.Fatalf("got %d messages, want 2", len(page.Data))
	}
	if page.Data[1].Attachments.Data[0].MimeType != "image/jpeg" {
		t.Errorf("attachment mime_type not parsed: %+v", page.Data[1].Attachments)
	}
	if page.Data[0].CreatedTime.Minute() != 30 {
		t.Errorf("created_time not parsed: %v", page.Data[0].CreatedTime)
	}
}

func TestListMessages_EmptyConversationID(t *testing.T) {
	c := NewBackfillClient("t", "PAGE1", WithSleep(noopSleep))
	_, err := c.ListMessages(context.Background(), "", "")
	if err == nil {
		t.Fatal("expected error on empty conversation id")
	}
}

// ---------- Error classification ----------

func TestDoRequest_AuthExpired_190(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Session expired","code":190,"type":"OAuthException"}}`))
	}))
	defer srv.Close()
	c := newTestClient(srv, "PAGE1")
	_, err := c.ListConversations(context.Background(), "")
	if !errors.Is(err, ErrAuthExpired) {
		t.Fatalf("expected ErrAuthExpired, got %v", err)
	}
}

func TestDoRequest_RateLimit_4_Retries(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"code":4,"message":"rate limited"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[],"paging":{"cursors":{"after":""}}}`))
	}))
	defer srv.Close()
	c := newTestClient(srv, "PAGE1")
	_, err := c.ListConversations(context.Background(), "")
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if atomic.LoadInt32(&calls) != 3 {
		t.Errorf("calls=%d, want 3 (2 retries + success)", calls)
	}
}

func TestDoRequest_RateLimit_Saturated_NoRetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Saturated BUC header
		w.Header().Set("X-Business-Use-Case-Usage",
			`{"PAGE1":[{"type":"pages","call_count":100,"total_cputime":100,"total_time":100,"estimated_time_to_regain_access":45}]}`)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":80001,"message":"BUC exhausted"}}`))
	}))
	defer srv.Close()
	c := newTestClient(srv, "PAGE1")
	_, err := c.ListConversations(context.Background(), "")
	if !errors.Is(err, ErrRateLimit) {
		t.Fatalf("expected ErrRateLimit, got %v", err)
	}
	if !c.bucTracker.IsSaturated() {
		t.Errorf("BUC tracker should be saturated")
	}
	if resume := c.bucTracker.ResumeAfter(); resume != 45*time.Minute {
		t.Errorf("ResumeAfter=%v, want 45m", resume)
	}
}

func TestDoRequest_BadRequest_NoRetry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":100,"message":"Invalid parameter"}}`))
	}))
	defer srv.Close()
	c := newTestClient(srv, "PAGE1")
	_, err := c.ListConversations(context.Background(), "")
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("bad request should not retry, calls=%d", calls)
	}
}

func TestDoRequest_5xx_Retry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"data":[],"paging":{"cursors":{"after":""}}}`))
	}))
	defer srv.Close()
	c := newTestClient(srv, "PAGE1")
	_, err := c.ListConversations(context.Background(), "")
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if atomic.LoadInt32(&calls) != 3 {
		t.Errorf("calls=%d, want 3", calls)
	}
}

func TestDoRequest_5xx_Exhausted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := newTestClient(srv, "PAGE1")
	_, err := c.ListConversations(context.Background(), "")
	if !errors.Is(err, ErrTransient) {
		t.Fatalf("expected ErrTransient after 5xx exhausted, got %v", err)
	}
}

func TestDoRequest_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":803,"message":"conversation not found"}}`))
	}))
	defer srv.Close()
	c := newTestClient(srv, "PAGE1")
	_, err := c.ListMessages(context.Background(), "t_gone", "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ---------- BUC tracker ----------

func TestBUCTracker_ThresholdsAndPacing(t *testing.T) {
	tr := &bucTracker{}

	// Below threshold — no pause
	tr.ParseHeader(`{"P":[{"call_count":40,"total_cputime":30,"total_time":20}]}`)
	if d := tr.ShouldPauseFor(); d != 0 {
		t.Errorf("below threshold, got pause %v", d)
	}

	// 50-70 band → 2s
	tr.ParseHeader(`{"P":[{"call_count":60,"total_cputime":30,"total_time":30}]}`)
	if d := tr.ShouldPauseFor(); d != 2*time.Second {
		t.Errorf("50-70, got %v want 2s", d)
	}

	// 70-90 band → 10s
	tr.ParseHeader(`{"P":[{"call_count":80,"total_cputime":30,"total_time":30}]}`)
	if d := tr.ShouldPauseFor(); d != 10*time.Second {
		t.Errorf("70-90, got %v want 10s", d)
	}

	// 90+ band → 60s and saturated at 100
	tr.ParseHeader(`{"P":[{"call_count":95,"total_cputime":30,"total_time":30}]}`)
	if d := tr.ShouldPauseFor(); d != 60*time.Second {
		t.Errorf("90-99, got %v want 60s", d)
	}
	if tr.IsSaturated() {
		t.Errorf("95 should not be saturated")
	}

	tr.ParseHeader(`{"P":[{"call_count":100,"total_cputime":100,"total_time":100,"estimated_time_to_regain_access":30}]}`)
	if !tr.IsSaturated() {
		t.Errorf("100 should be saturated")
	}
	if tr.ResumeAfter() != 30*time.Minute {
		t.Errorf("ResumeAfter=%v, want 30m", tr.ResumeAfter())
	}
}

func TestBUCTracker_FallbackResume(t *testing.T) {
	tr := &bucTracker{}
	tr.ParseHeader(`{"P":[{"call_count":100,"total_cputime":100,"total_time":100}]}`) // no regain_access
	if tr.ResumeAfter() != time.Hour {
		t.Errorf("missing regain_access should fallback to 1h, got %v", tr.ResumeAfter())
	}
}

func TestBUCTracker_MalformedHeader(t *testing.T) {
	tr := &bucTracker{}
	tr.ParseHeader("not json")
	if d := tr.ShouldPauseFor(); d != 0 {
		t.Errorf("malformed header should be ignored, got pause %v", d)
	}
	tr.ParseHeader("")
	if d := tr.ShouldPauseFor(); d != 0 {
		t.Errorf("empty header should be ignored, got pause %v", d)
	}
}

// ---------- Helpers / parsers ----------

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"", 0},
		{"garbage", 0},
		{"-5", 0},
		{"10", 10 * time.Second},
		{"  30  ", 30 * time.Second},
		{"500", 120 * time.Second}, // capped at 120
	}
	for _, tt := range tests {
		got := parseRetryAfter(tt.in, 120)
		if got != tt.want {
			t.Errorf("parseRetryAfter(%q)=%v want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseGraphTime(t *testing.T) {
	cases := map[string]bool{
		"2024-06-15T08:30:45+0000": true,
		"2024-06-15T08:30:45Z":     true,
		"2024-06-15T08:30:45.000Z": true,
		"":                         false,
		"not-a-time":               false,
	}
	for in, shouldParse := range cases {
		got := parseGraphTime(in)
		if shouldParse && got.IsZero() {
			t.Errorf("parseGraphTime(%q) returned zero time", in)
		}
		if !shouldParse && !got.IsZero() {
			t.Errorf("parseGraphTime(%q)=%v, want zero", in, got)
		}
	}
}

// Ensure JSON round-trip for error envelope shape.
func TestGraphAPIError_Decode(t *testing.T) {
	raw := `{"error":{"message":"m","type":"OAuthException","code":190,"error_subcode":460,"fbtrace_id":"abc"}}`
	var ge graphAPIError
	if err := json.Unmarshal([]byte(raw), &ge); err != nil {
		t.Fatal(err)
	}
	if ge.Error.Code != 190 || ge.Error.ErrorSubcode != 460 {
		t.Errorf("decode: %+v", ge)
	}
}
