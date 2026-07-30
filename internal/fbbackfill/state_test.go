package fbbackfill

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// fakeInstanceStore is a minimal in-memory stand-in for
// store.ChannelInstanceStore, sufficient for exercising StateStore.
// Only the methods StateStore actually calls are implemented; the rest
// return errors so a misuse is caught during tests.
type fakeInstanceStore struct {
	mu   sync.Mutex
	rows map[uuid.UUID]*store.ChannelInstanceData
}

func newFakeInstanceStore() *fakeInstanceStore {
	return &fakeInstanceStore{rows: make(map[uuid.UUID]*store.ChannelInstanceData)}
}

func (f *fakeInstanceStore) seed(inst *store.ChannelInstanceData) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows[inst.ID] = inst
}

func (f *fakeInstanceStore) Get(_ context.Context, id uuid.UUID) (*store.ChannelInstanceData, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.rows[id]
	if !ok {
		return nil, errors.New("fake: not found")
	}
	// Return a deep copy so caller mutations do not bleed back.
	clone := *row
	clone.Config = append(json.RawMessage(nil), row.Config...)
	return &clone, nil
}

func (f *fakeInstanceStore) Update(_ context.Context, id uuid.UUID, updates map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.rows[id]
	if !ok {
		return errors.New("fake: not found")
	}
	if v, ok := updates["config"]; ok {
		switch cfg := v.(type) {
		case json.RawMessage:
			row.Config = append(json.RawMessage(nil), cfg...)
		case []byte:
			row.Config = append(json.RawMessage(nil), cfg...)
		case string:
			row.Config = json.RawMessage(cfg)
		default:
			b, err := json.Marshal(cfg)
			if err != nil {
				return err
			}
			row.Config = b
		}
	}
	return nil
}

func (f *fakeInstanceStore) ListAllInstances(_ context.Context) ([]store.ChannelInstanceData, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]store.ChannelInstanceData, 0, len(f.rows))
	for _, r := range f.rows {
		clone := *r
		clone.Config = append(json.RawMessage(nil), r.Config...)
		out = append(out, clone)
	}
	return out, nil
}

// Unused methods stubbed to satisfy the interface.
func (f *fakeInstanceStore) Create(context.Context, *store.ChannelInstanceData) error {
	return errors.New("fake: Create not used")
}
func (f *fakeInstanceStore) GetByName(context.Context, string) (*store.ChannelInstanceData, error) {
	return nil, errors.New("fake: GetByName not used")
}
func (f *fakeInstanceStore) Delete(context.Context, uuid.UUID) error {
	return errors.New("fake: Delete not used")
}
func (f *fakeInstanceStore) ListEnabled(context.Context) ([]store.ChannelInstanceData, error) {
	return nil, errors.New("fake: ListEnabled not used")
}
func (f *fakeInstanceStore) ListAll(context.Context) ([]store.ChannelInstanceData, error) {
	return nil, errors.New("fake: ListAll not used")
}
func (f *fakeInstanceStore) ListAllEnabled(context.Context) ([]store.ChannelInstanceData, error) {
	return nil, errors.New("fake: ListAllEnabled not used")
}
func (f *fakeInstanceStore) ListPaged(context.Context, store.ChannelInstanceListOpts) ([]store.ChannelInstanceData, error) {
	return nil, errors.New("fake: ListPaged not used")
}
func (f *fakeInstanceStore) CountInstances(context.Context, store.ChannelInstanceListOpts) (int, error) {
	return 0, errors.New("fake: CountInstances not used")
}

// ---------- Tests ----------

func TestBackfillState_RoundTrip(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	st := &BackfillState{
		Version:            BackfillStateVersion,
		Status:             StatusRunning,
		StartedAt:          &now,
		UpdatedAt:          now,
		ConversationsTotal: 10,
		ConversationsDone:  4,
		MessagesIngested:   57,
		EpisodicsCreated:   3,
		ConversationCursor: "CURSOR_A",
		MaxConversations:   500,
		SkipExisting:       true,
		TriggeredBy:        "manual",
	}
	blob, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	var back BackfillState
	if err := json.Unmarshal(blob, &back); err != nil {
		t.Fatal(err)
	}
	if back.Status != StatusRunning || back.ConversationsDone != 4 || back.ConversationCursor != "CURSOR_A" {
		t.Errorf("round-trip mismatch: %+v", back)
	}
}

func TestStateStore_GetNoState(t *testing.T) {
	f := newFakeInstanceStore()
	id := uuid.New()
	f.seed(&store.ChannelInstanceData{
		BaseModel:   store.BaseModel{ID: id},
		Name:        "fb-test",
		ChannelType: "facebook",
		Config:      json.RawMessage(`{"page_id":"PAGE1"}`),
	})
	s := NewStateStore(f)
	_, err := s.Get(context.Background(), id)
	if !errors.Is(err, ErrNoState) {
		t.Fatalf("expected ErrNoState, got %v", err)
	}
}

func TestStateStore_SavePreservesOtherKeys(t *testing.T) {
	f := newFakeInstanceStore()
	id := uuid.New()
	f.seed(&store.ChannelInstanceData{
		BaseModel:   store.BaseModel{ID: id},
		Name:        "fb-test",
		ChannelType: "facebook",
		Config:      json.RawMessage(`{"page_id":"PAGE1","features":{"messenger_auto_reply":true},"allow_from":["u1","u2"]}`),
	})
	s := NewStateStore(f)

	st := NewBackfillState(StartOpts{SkipExisting: true, TriggeredBy: "manual"})
	st.Status = StatusRunning
	st.ConversationsDone = 2
	if err := s.Save(context.Background(), id, st); err != nil {
		t.Fatal(err)
	}

	// Re-read raw config and confirm both _backfill AND original keys survive.
	raw, err := f.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw.Config, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["page_id"]; !ok {
		t.Errorf("page_id key was dropped")
	}
	if _, ok := m["features"]; !ok {
		t.Errorf("features key was dropped")
	}
	if _, ok := m["allow_from"]; !ok {
		t.Errorf("allow_from key was dropped")
	}
	if _, ok := m[BackfillConfigKey]; !ok {
		t.Errorf("_backfill key missing after save")
	}
}

func TestStateStore_SaveLoad(t *testing.T) {
	f := newFakeInstanceStore()
	id := uuid.New()
	f.seed(&store.ChannelInstanceData{
		BaseModel:   store.BaseModel{ID: id},
		Name:        "fb-test",
		ChannelType: "facebook",
		Config:      json.RawMessage(`{}`),
	})
	s := NewStateStore(f)

	st := NewBackfillState(StartOpts{MaxConversations: 200, TriggeredBy: "auto_on_create"})
	st.Status = StatusPending
	if err := s.Save(context.Background(), id, st); err != nil {
		t.Fatal(err)
	}

	got, err := s.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.MaxConversations != 200 || got.TriggeredBy != "auto_on_create" {
		t.Errorf("mismatch: %+v", got)
	}
	if got.UpdatedAt.IsZero() {
		t.Errorf("UpdatedAt not set by Save")
	}
}

func TestStateStore_Delete(t *testing.T) {
	f := newFakeInstanceStore()
	id := uuid.New()
	f.seed(&store.ChannelInstanceData{
		BaseModel:   store.BaseModel{ID: id},
		Name:        "fb-test",
		ChannelType: "facebook",
		Config:      json.RawMessage(`{"page_id":"PAGE1","_backfill":{"version":1,"status":"completed"}}`),
	})
	s := NewStateStore(f)
	if err := s.Delete(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	_, err := s.Get(context.Background(), id)
	if !errors.Is(err, ErrNoState) {
		t.Errorf("expected ErrNoState after Delete, got %v", err)
	}
	// page_id should survive
	raw, _ := f.Get(context.Background(), id)
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw.Config, &m)
	if _, ok := m["page_id"]; !ok {
		t.Errorf("page_id should survive Delete")
	}
}

// TestStateStore_ConcurrentSaveBlobIntegrity verifies that concurrent Save
// calls do not corrupt the JSON blob. The StateStore contract is:
//
//   - Save is serialized per-instance (via internal mutex) so the JSON
//     written to the DB is always a valid write of a well-formed state.
//   - The StateStore does NOT prevent lost updates when callers perform
//     a read-modify-write from multiple goroutines — that is the caller's
//     responsibility. In production, the job runner is single-writer per
//     instance, so RMW is inherently serialized at that level.
//
// This test therefore checks blob integrity, not update count.
func TestStateStore_ConcurrentSaveBlobIntegrity(t *testing.T) {
	f := newFakeInstanceStore()
	id := uuid.New()
	f.seed(&store.ChannelInstanceData{
		BaseModel:   store.BaseModel{ID: id},
		Name:        "fb-test",
		ChannelType: "facebook",
		Config:      json.RawMessage(`{"page_id":"PAGE1"}`),
	})
	s := NewStateStore(f)

	var wg sync.WaitGroup
	const N = 50
	for i := 0; i < N; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			st := NewBackfillState(StartOpts{TriggeredBy: "manual"})
			st.Status = StatusRunning
			st.MessagesIngested = i
			if err := s.Save(context.Background(), id, st); err != nil {
				t.Errorf("save: %v", err)
			}
		}()
	}
	wg.Wait()

	// Final config blob must still be parseable with both fb keys intact.
	raw, err := f.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw.Config, &m); err != nil {
		t.Fatalf("config blob corrupted: %v — %s", err, string(raw.Config))
	}
	if _, ok := m["page_id"]; !ok {
		t.Errorf("page_id key lost during concurrent writes")
	}
	if _, ok := m[BackfillConfigKey]; !ok {
		t.Errorf("_backfill key missing")
	}
	// _backfill blob must itself be a valid BackfillState JSON.
	var st BackfillState
	if err := json.Unmarshal(m[BackfillConfigKey], &st); err != nil {
		t.Errorf("_backfill blob is not valid BackfillState JSON: %v", err)
	}
	if st.Status != StatusRunning {
		t.Errorf("final status=%v, want running", st.Status)
	}
}

func TestStateStore_ListActive(t *testing.T) {
	f := newFakeInstanceStore()

	// facebook instance, running
	idA := uuid.New()
	f.seed(&store.ChannelInstanceData{
		BaseModel:   store.BaseModel{ID: idA},
		Name:        "fb-a",
		ChannelType: "facebook",
		Config:      json.RawMessage(`{"_backfill":{"version":1,"status":"running"}}`),
	})
	// facebook instance, completed — should be filtered out
	idB := uuid.New()
	f.seed(&store.ChannelInstanceData{
		BaseModel:   store.BaseModel{ID: idB},
		Name:        "fb-b",
		ChannelType: "facebook",
		Config:      json.RawMessage(`{"_backfill":{"version":1,"status":"completed"}}`),
	})
	// telegram instance, running — wrong channel type, filtered out
	idC := uuid.New()
	f.seed(&store.ChannelInstanceData{
		BaseModel:   store.BaseModel{ID: idC},
		Name:        "tg",
		ChannelType: "telegram",
		Config:      json.RawMessage(`{"_backfill":{"version":1,"status":"running"}}`),
	})
	// facebook instance, paused — kept
	idD := uuid.New()
	f.seed(&store.ChannelInstanceData{
		BaseModel:   store.BaseModel{ID: idD},
		Name:        "fb-d",
		ChannelType: "facebook",
		Config:      json.RawMessage(`{"_backfill":{"version":1,"status":"paused"}}`),
	})

	s := NewStateStore(f)
	active, err := s.ListActive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 2 {
		t.Fatalf("active=%d, want 2 (fb-a + fb-d)", len(active))
	}
	names := map[string]bool{}
	for _, a := range active {
		names[a.Name] = true
	}
	if !names["fb-a"] || !names["fb-d"] {
		t.Errorf("wrong actives: %+v", names)
	}
}

func TestStateStore_MarkStaleAsPaused(t *testing.T) {
	f := newFakeInstanceStore()
	idRunning := uuid.New()
	f.seed(&store.ChannelInstanceData{
		BaseModel:   store.BaseModel{ID: idRunning},
		ChannelType: "facebook",
		Config:      json.RawMessage(`{"_backfill":{"version":1,"status":"running"}}`),
	})
	idPaused := uuid.New()
	f.seed(&store.ChannelInstanceData{
		BaseModel:   store.BaseModel{ID: idPaused},
		ChannelType: "facebook",
		Config:      json.RawMessage(`{"_backfill":{"version":1,"status":"paused"}}`),
	})

	s := NewStateStore(f)
	n, err := s.MarkStaleAsPaused(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("marked %d, want 1", n)
	}
	st, _ := s.Get(context.Background(), idRunning)
	if st.Status != StatusPaused {
		t.Errorf("expected paused, got %v", st.Status)
	}
	if st.LastError == "" {
		t.Errorf("expected LastError to be set")
	}
	// The already-paused one must not be touched.
	st2, _ := s.Get(context.Background(), idPaused)
	if st2.LastError != "" {
		t.Errorf("already-paused state was mutated: %+v", st2)
	}
}

func TestStateStore_ForwardCompat_MissingVersion(t *testing.T) {
	f := newFakeInstanceStore()
	id := uuid.New()
	// State blob without "version" — simulate a forward-incompatible write
	// or a future format that dropped the field. Reader must still accept.
	f.seed(&store.ChannelInstanceData{
		BaseModel:   store.BaseModel{ID: id},
		ChannelType: "facebook",
		Config:      json.RawMessage(`{"_backfill":{"status":"running","messages_ingested":5}}`),
	})
	s := NewStateStore(f)
	st, err := s.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if st.Version != BackfillStateVersion {
		t.Errorf("Version not defaulted: %d", st.Version)
	}
	if st.Status != StatusRunning || st.MessagesIngested != 5 {
		t.Errorf("other fields dropped: %+v", st)
	}
}

// Smoke test: atomic.Int32 usage to prevent inadvertent imports removal.
var _ = atomic.Int32{}
