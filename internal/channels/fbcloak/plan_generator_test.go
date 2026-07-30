//go:build !sqliteonly

package fbcloak

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeEpisodic implements EpisodicSource for tests.
type fakeEpisodic struct {
	byFanpage map[string][]EpisodicTarget
	byPSID    map[string]EpisodicTarget
}

func (f *fakeEpisodic) ListByFanpage(_ context.Context, _ uuid.UUID, fanpageID string, _, _ time.Duration, _ int) ([]EpisodicTarget, error) {
	return f.byFanpage[fanpageID], nil
}

func (f *fakeEpisodic) GetByPSID(_ context.Context, _ uuid.UUID, _, psid string) (EpisodicTarget, error) {
	t, ok := f.byPSID[psid]
	if !ok {
		return EpisodicTarget{}, errors.New("not found")
	}
	return t, nil
}

// fakeLLM returns a canned decision per call.
type fakeLLM struct {
	out      []string // queued raw responses (text)
	err      error
	calls    int
	lastSys  string
	lastUser string
}

func (l *fakeLLM) Generate(_ context.Context, in PlanLLMInput) (PlanLLMOutput, error) {
	l.calls++
	l.lastSys = in.SystemPrompt
	l.lastUser = in.UserPrompt
	if l.err != nil {
		return PlanLLMOutput{}, l.err
	}
	if len(l.out) == 0 {
		return PlanLLMOutput{Text: `{"should_send":false,"skip_reason":"too_recent","reason":"x"}`}, nil
	}
	text := l.out[0]
	l.out = l.out[1:]
	return PlanLLMOutput{Text: text, Model: "test-model", InputTokens: 100, OutputTokens: 50}, nil
}

// fakeCredStore implements CredentialStore minimally for Generator tests.
type fakeCredStore struct {
	creds map[uuid.UUID]Credential
}

func (f *fakeCredStore) Create(_ context.Context, c Credential) (Credential, error) {
	f.creds[c.ID] = c
	return c, nil
}
func (f *fakeCredStore) Get(_ context.Context, _, id uuid.UUID) (Credential, error) {
	c, ok := f.creds[id]
	if !ok {
		return Credential{}, ErrCredentialNotFound
	}
	return c, nil
}
func (f *fakeCredStore) GetByFanpage(_ context.Context, _ uuid.UUID, _ string) (Credential, error) {
	return Credential{}, ErrCredentialNotFound
}
func (f *fakeCredStore) ListByTenant(_ context.Context, tenantID uuid.UUID) ([]Credential, error) {
	var out []Credential
	for _, c := range f.creds {
		if c.TenantID == tenantID {
			out = append(out, c)
		}
	}
	return out, nil
}
func (f *fakeCredStore) UpdateStatus(_ context.Context, _, id uuid.UUID, status CredentialStatus) error {
	c := f.creds[id]
	c.Status = status
	f.creds[id] = c
	return nil
}
func (f *fakeCredStore) UpdateCookies(_ context.Context, _, _ uuid.UUID, _ string) error  { return nil }
func (f *fakeCredStore) UpdateLastCheck(_ context.Context, _, _ uuid.UUID) error          { return nil }
func (f *fakeCredStore) Delete(_ context.Context, _, _ uuid.UUID) error                   { return nil }

// fakeActiveCredsList wraps fakeCredStore as CredentialActiveLister.
type fakeActiveCredsList struct {
	store *fakeCredStore
}

func (f *fakeActiveCredsList) ListAllActive(_ context.Context) ([]Credential, error) {
	var out []Credential
	for _, c := range f.store.creds {
		if c.Status == StatusActive {
			out = append(out, c)
		}
	}
	return out, nil
}

func newGeneratorTestRig(t *testing.T) (*PlanGenerator, *fakePlanStore, *fakeLLM, *fakeCredStore, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	tenantID := uuid.New()
	credID := uuid.New()
	credStore := &fakeCredStore{creds: map[uuid.UUID]Credential{}}
	cred := Credential{
		ID:          credID,
		TenantID:    tenantID,
		FanpageID:   "100",
		FanpageName: "Test Page",
		Status:      StatusActive,
	}
	credStore.creds[credID] = cred

	plans := newFakePlanStore()
	llm := &fakeLLM{}
	episodic := &fakeEpisodic{
		byFanpage: map[string][]EpisodicTarget{
			"100": {
				{
					PSID:           "200",
					ConversationID: "t_200",
					RecipientName:  "John",
					LastInboundAt:  time.Now().Add(-14 * 24 * time.Hour),
					TurnCount:      5,
					SummaryText:    "Customer asked about pricing",
					SummaryVersion: 1,
				},
			},
		},
		byPSID: map[string]EpisodicTarget{},
	}
	gen := &PlanGenerator{
		Plans:           plans,
		Credentials:     credStore,
		ActiveCredsList: &fakeActiveCredsList{store: credStore},
		Episodic:        episodic,
		LLM:             llm,
		Logger:          slog.Default(),
		Model:           "test-model",
		MaxConcurrent:   1,
		BatchSize:       10,
		MinIdle:         7 * 24 * time.Hour,
		MaxIdle:         30 * 24 * time.Hour,
		JitterRange:     0,
		Killswitch:      &atomic.Bool{},
		clock:           func() time.Time { return time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC) },
	}
	return gen, plans, llm, credStore, tenantID, credID, uuid.New()
}

func TestGenerator_RunForCredential_HappyPathSend(t *testing.T) {
	gen, plans, llm, _, tenantID, credID, _ := newGeneratorTestRig(t)
	llm.out = []string{`{"should_send":true,"send_after_days":7,"message":"hello","reason":"warm lead"}`}

	summary, err := gen.RunForCredential(context.Background(), tenantID, credID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if summary.Created != 1 {
		t.Errorf("Created = %d, want 1", summary.Created)
	}
	if len(plans.plans) != 1 {
		t.Errorf("plans count = %d, want 1", len(plans.plans))
	}
	for _, p := range plans.plans {
		if p.Status != PlanStatusPending {
			t.Errorf("status = %s, want pending", p.Status)
		}
		if p.MessageDraft != "hello" {
			t.Errorf("message = %q, want hello", p.MessageDraft)
		}
	}
}

func TestGenerator_RunForCredential_HappyPathSkip(t *testing.T) {
	gen, _, llm, _, tenantID, credID, _ := newGeneratorTestRig(t)
	llm.out = []string{`{"should_send":false,"skip_reason":"too_recent","reason":"khách vừa nhắn"}`}

	summary, err := gen.RunForCredential(context.Background(), tenantID, credID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if summary.Skipped != 1 || summary.Created != 0 {
		t.Errorf("got Created=%d Skipped=%d, want 0/1", summary.Created, summary.Skipped)
	}
}

func TestGenerator_RunForCredential_LLMError(t *testing.T) {
	gen, _, llm, _, tenantID, credID, _ := newGeneratorTestRig(t)
	llm.err = errors.New("provider down")

	summary, err := gen.RunForCredential(context.Background(), tenantID, credID)
	if err != nil {
		t.Fatalf("RunForCredential should not propagate per-target error: %v", err)
	}
	if summary.Errors != 1 {
		t.Errorf("Errors = %d, want 1", summary.Errors)
	}
}

func TestGenerator_RunForCredential_KillswitchAborts(t *testing.T) {
	gen, _, _, _, tenantID, credID, _ := newGeneratorTestRig(t)
	gen.Killswitch.Store(true)

	summary, err := gen.RunForCredential(context.Background(), tenantID, credID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if summary.Created != 0 || summary.Skipped != 0 || summary.Errors != 0 {
		t.Errorf("killswitch must abort cleanly, got %+v", summary)
	}
}

func TestGenerator_RunForCredential_CredentialNotActive(t *testing.T) {
	gen, _, _, credStore, tenantID, credID, _ := newGeneratorTestRig(t)
	c := credStore.creds[credID]
	c.Status = StatusExpired
	credStore.creds[credID] = c

	_, err := gen.RunForCredential(context.Background(), tenantID, credID)
	if err == nil {
		t.Fatal("expected error for inactive credential")
	}
}

func TestGenerator_PromptInjection(t *testing.T) {
	gen, _, llm, _, tenantID, credID, _ := newGeneratorTestRig(t)
	llm.out = []string{`{"should_send":false,"skip_reason":"x","reason":"y"}`}

	if _, err := gen.RunForCredential(context.Background(), tenantID, credID); err != nil {
		t.Fatalf("err: %v", err)
	}
	if llm.lastSys == "" {
		t.Error("system prompt not injected")
	}
	if !contains(llm.lastSys, "Test Page") {
		t.Errorf("fanpage_name not substituted: %q", llm.lastSys)
	}
	if !contains(llm.lastUser, "John") || !contains(llm.lastUser, "pricing") {
		t.Errorf("user prompt missing target data: %q", llm.lastUser)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && len(substr) > 0 && (s == substr ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr))))
}
