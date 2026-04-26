//go:build !sqliteonly

package fbcloak

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakePage implements PageSession for executor tests.
type fakePage struct {
	navURL          string
	navErr          error
	inspector       *stubInspector
	typeReadback    string
	typeErr         error
	clickErr        error
	waitErr         error
	preShot         string
	postShot        string
	calls           []string
}

func (p *fakePage) NavigateThread(_ context.Context, _ string, _ string, _ string) (string, error) {
	p.calls = append(p.calls, "navigate")
	return p.navURL, p.navErr
}
func (p *fakePage) Inspector() ThreadInspector { return p.inspector }
func (p *fakePage) TypeMessage(_ context.Context, msg string) (string, error) {
	p.calls = append(p.calls, "type:"+msg)
	if p.typeReadback == "" {
		return msg, p.typeErr
	}
	return p.typeReadback, p.typeErr
}
func (p *fakePage) ClickSend(_ context.Context) error {
	p.calls = append(p.calls, "click")
	return p.clickErr
}
func (p *fakePage) WaitSendConfirmed(_ context.Context, _ time.Duration) error {
	p.calls = append(p.calls, "wait")
	return p.waitErr
}
func (p *fakePage) ScreenshotPre(_ context.Context) (string, error)  { return p.preShot, nil }
func (p *fakePage) ScreenshotPost(_ context.Context) (string, error) { return p.postShot, nil }

// fakeJobStore is a minimal JobStore for tests that only exercise log paths.
type fakeJobStore struct {
	mu      sync.Mutex
	logs    []SendLog
	count   int
	lastSendAt *time.Time
}

func (f *fakeJobStore) CreateJob(context.Context, Job) (Job, error)            { return Job{}, nil }
func (f *fakeJobStore) GetJob(context.Context, uuid.UUID, uuid.UUID) (Job, error) { return Job{}, nil }
func (f *fakeJobStore) ListJobs(context.Context, uuid.UUID) ([]Job, error)     { return nil, nil }
func (f *fakeJobStore) UpdateJob(context.Context, uuid.UUID, Job) error         { return nil }
func (f *fakeJobStore) SetJobEnabled(context.Context, uuid.UUID, uuid.UUID, bool) error {
	return nil
}
func (f *fakeJobStore) SetJobDryRun(context.Context, uuid.UUID, uuid.UUID, bool) error {
	return nil
}
func (f *fakeJobStore) UpdateJobRunResult(context.Context, uuid.UUID, uuid.UUID, JobStatus, time.Time) error {
	return nil
}
func (f *fakeJobStore) DeleteJob(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (f *fakeJobStore) DueJobs(context.Context, time.Time, int) ([]Job, error) {
	return nil, nil
}
func (f *fakeJobStore) LogSend(_ context.Context, l SendLog) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logs = append(f.logs, l)
	return nil
}
func (f *fakeJobStore) ListSendLog(context.Context, uuid.UUID, *uuid.UUID, int) ([]SendLog, error) {
	return f.logs, nil
}
func (f *fakeJobStore) ListSendLogFiltered(context.Context, uuid.UUID, SendLogFilter) ([]SendLog, error) {
	return f.logs, nil
}
func (f *fakeJobStore) GetSendLog(context.Context, uuid.UUID, uuid.UUID) (SendLog, error) {
	return SendLog{}, nil
}
func (f *fakeJobStore) CountTodaySends(context.Context, uuid.UUID, string, time.Time) (int, error) {
	return f.count, nil
}
func (f *fakeJobStore) LastSendTo(context.Context, uuid.UUID, string) (*time.Time, error) {
	return f.lastSendAt, nil
}

func newExecutor(t *testing.T) (*SendExecutor, *fakeJobStore) {
	t.Helper()
	store := &fakeJobStore{}
	cfg := DefaultPolicyConfig()
	pol := NewPolicy(cfg, store)
	return &SendExecutor{
		Policy:       pol,
		Verify:       VerifyLastMessage,
		VerifyConfig: VerifyConfig{Tolerance: 2 * 24 * time.Hour, MinIdle: 7 * 24 * time.Hour, Now: time.Now},
		Log:          store,
	}, store
}

func TestExecutor_DryRun_StopsBeforeType(t *testing.T) {
	exec, store := newExecutor(t)
	page := &fakePage{
		navURL:    "https://business.facebook.com/latest/inbox?asset_id=1&active_chat_thread_id=t_x",
		inspector: &stubInspector{ax: "Jane • 30 ngày"},
	}
	now := time.Now()
	req := SendRequest{
		Job:        Job{ID: uuid.New(), TenantID: uuid.New(), DailyCap: 30},
		Credential: Credential{ID: uuid.New(), FanpageID: "1"},
		Target:     Target{RecipientPSID: "P", LastMessageAt: now.Add(-30 * 24 * time.Hour)},
		Message:    "Chào anh ạ",
		DryRun:     true,
	}
	log, err := exec.Execute(t.Context(), page, req)
	if err != nil {
		t.Fatal(err)
	}
	if log.Status != SendStatusDryRun {
		t.Errorf("status: got %s, want dry_run", log.Status)
	}
	for _, c := range page.calls {
		if c == "click" || c == "type:Chào anh ạ" {
			t.Errorf("dry-run should not call %s", c)
		}
	}
	if len(store.logs) != 1 {
		t.Errorf("expected 1 log row, got %d", len(store.logs))
	}
}

func TestExecutor_LiveSend_HappyPath(t *testing.T) {
	exec, store := newExecutor(t)
	page := &fakePage{
		navURL:    "https://business.facebook.com/latest/inbox?asset_id=1&active_chat_thread_id=t_x",
		inspector: &stubInspector{ax: "Jane • 30 ngày"},
	}
	now := time.Now()
	req := SendRequest{
		Job:        Job{ID: uuid.New(), TenantID: uuid.New(), DailyCap: 30},
		Credential: Credential{ID: uuid.New(), FanpageID: "1"},
		Target:     Target{RecipientPSID: "P", LastMessageAt: now.Add(-30 * 24 * time.Hour)},
		Message:    "Chào anh ạ",
		DryRun:     false,
	}
	log, err := exec.Execute(t.Context(), page, req)
	if err != nil {
		t.Fatal(err)
	}
	if log.Status != SendStatusSent {
		t.Errorf("status: got %s, want sent", log.Status)
	}
	expectedSequence := []string{"navigate", "type:Chào anh ạ", "click", "wait"}
	if len(page.calls) != len(expectedSequence) {
		t.Fatalf("call seq length: got %v, want %v", page.calls, expectedSequence)
	}
	for i, c := range expectedSequence {
		if page.calls[i] != c {
			t.Errorf("seq[%d]: got %s, want %s", i, page.calls[i], c)
		}
	}
	if len(store.logs) != 1 {
		t.Errorf("expected 1 log row, got %d", len(store.logs))
	}
}

func TestExecutor_VerifyMismatch_SkipsSend(t *testing.T) {
	exec, store := newExecutor(t)
	page := &fakePage{
		navURL: "https://business.facebook.com/latest/inbox?asset_id=1&active_chat_thread_id=t_x",
		// DB says 30d ago; UI shows 1d ago → customer replied → skip.
		inspector: &stubInspector{ax: "Jane • 1 ngày"},
	}
	now := time.Now()
	req := SendRequest{
		Job:        Job{ID: uuid.New(), TenantID: uuid.New(), DailyCap: 30},
		Credential: Credential{ID: uuid.New(), FanpageID: "1"},
		Target:     Target{RecipientPSID: "P", LastMessageAt: now.Add(-30 * 24 * time.Hour)},
		Message:    "Chào anh",
		DryRun:     false,
	}
	exec.VerifyConfig = VerifyConfig{Tolerance: 2 * 24 * time.Hour, MinIdle: 7 * 24 * time.Hour, Now: func() time.Time { return now }}
	log, err := exec.Execute(t.Context(), page, req)
	if err != nil {
		t.Fatal(err)
	}
	if log.Status != SendStatusSkipped {
		t.Errorf("status: got %s, want skipped", log.Status)
	}
	if log.SkipReason == nil || *log.SkipReason != "customer_replied_recently" {
		t.Errorf("skip reason: got %v", log.SkipReason)
	}
	for _, c := range page.calls {
		if c == "click" {
			t.Error("verify-skip should not click send")
		}
	}
	if len(store.logs) != 1 {
		t.Errorf("expected 1 log row, got %d", len(store.logs))
	}
}

func TestExecutor_PolicyContentBlocked(t *testing.T) {
	exec, store := newExecutor(t)
	page := &fakePage{}
	req := SendRequest{
		Job:        Job{ID: uuid.New(), TenantID: uuid.New(), DailyCap: 30},
		Credential: Credential{ID: uuid.New(), FanpageID: "1"},
		Target:     Target{RecipientPSID: "P"},
		Message:    "Sale 50% off!",
	}
	log, err := exec.Execute(t.Context(), page, req)
	if err != nil {
		t.Fatal(err)
	}
	if log.Status != SendStatusSkipped {
		t.Errorf("expected skipped, got %s", log.Status)
	}
	if log.SkipReason == nil || *log.SkipReason != "content_blocked" {
		t.Errorf("expected content_blocked, got %v", log.SkipReason)
	}
	if len(page.calls) != 0 {
		t.Errorf("policy block should not navigate (calls=%v)", page.calls)
	}
	if len(store.logs) != 1 {
		t.Errorf("expected 1 log row, got %d", len(store.logs))
	}
}

func TestExecutor_RedirectInterstitial_Fails(t *testing.T) {
	exec, _ := newExecutor(t)
	page := &fakePage{navURL: "https://www.facebook.com/login/?next=..."}
	req := SendRequest{
		Job:        Job{ID: uuid.New(), TenantID: uuid.New(), DailyCap: 30},
		Credential: Credential{ID: uuid.New()},
		Target:     Target{RecipientPSID: "P", LastMessageAt: time.Now().Add(-30 * 24 * time.Hour)},
		Message:    "ok",
	}
	log, err := exec.Execute(t.Context(), page, req)
	if err == nil {
		t.Error("expected error on interstitial")
	}
	if log.Status != SendStatusFailed {
		t.Errorf("status: got %s, want failed", log.Status)
	}
}

func TestExecutor_NavigateError(t *testing.T) {
	exec, _ := newExecutor(t)
	page := &fakePage{navErr: errors.New("conn reset")}
	req := SendRequest{
		Job:        Job{ID: uuid.New(), TenantID: uuid.New(), DailyCap: 30},
		Credential: Credential{ID: uuid.New()},
		Target:     Target{RecipientPSID: "P", LastMessageAt: time.Now().Add(-30 * 24 * time.Hour)},
		Message:    "ok",
	}
	log, err := exec.Execute(t.Context(), page, req)
	if err == nil {
		t.Error("expected nav error to propagate")
	}
	if log.Status != SendStatusFailed {
		t.Errorf("status: got %s, want failed", log.Status)
	}
}

func TestExecutor_NilPolicy(t *testing.T) {
	exec := &SendExecutor{Log: &fakeJobStore{}}
	_, err := exec.Execute(t.Context(), &fakePage{}, SendRequest{})
	if err == nil {
		t.Error("expected error for nil policy")
	}
}

// Compile-time guard that our fake stays in sync with the interface.
var _ PageSession = (*fakePage)(nil)
var _ JobStore = (*fakeJobStore)(nil)

func TestExecutor_TypeReadbackMismatch(t *testing.T) {
	exec, _ := newExecutor(t)
	page := &fakePage{
		navURL:       "https://business.facebook.com/latest/inbox?asset_id=1&active_chat_thread_id=t_x",
		inspector:    &stubInspector{ax: "Jane • 30 ngày"},
		typeReadback: "wrong text",
	}
	now := time.Now()
	req := SendRequest{
		Job:        Job{ID: uuid.New(), TenantID: uuid.New(), DailyCap: 30},
		Credential: Credential{ID: uuid.New(), FanpageID: "1"},
		Target:     Target{RecipientPSID: "P", LastMessageAt: now.Add(-30 * 24 * time.Hour)},
		Message:    "Chào anh ạ",
		DryRun:     false,
	}
	exec.VerifyConfig = VerifyConfig{Tolerance: 2 * 24 * time.Hour, MinIdle: 7 * 24 * time.Hour, Now: func() time.Time { return now }}
	log, err := exec.Execute(t.Context(), page, req)
	if err == nil || !strings.Contains(err.Error(), "readback mismatch") {
		t.Fatalf("expected readback mismatch error, got: %v", err)
	}
	if log.Status != SendStatusFailed {
		t.Errorf("status: got %s, want failed", log.Status)
	}
}
