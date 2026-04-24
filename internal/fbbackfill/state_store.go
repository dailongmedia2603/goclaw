package fbbackfill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// ErrNoState indicates a channel instance has no backfill state yet
// (fresh channel, never backfilled).
var ErrNoState = errors.New("fbbackfill: no state for channel instance")

// StateStore persists BackfillState in channel_instances.config._backfill.
// It delegates the DB round-trip to the upstream store.ChannelInstanceStore
// so the fork does not need its own SQL or migrations.
//
// Writes are serialized per-instance via an internal mutex map to avoid
// lost updates between the job runner goroutine and RPC commands.
type StateStore struct {
	inner store.ChannelInstanceStore

	mu    sync.Mutex
	locks map[uuid.UUID]*sync.Mutex
}

// NewStateStore constructs a StateStore wrapping the upstream channel
// instance store.
func NewStateStore(inner store.ChannelInstanceStore) *StateStore {
	return &StateStore{
		inner: inner,
		locks: make(map[uuid.UUID]*sync.Mutex),
	}
}

// lockFor returns the per-instance mutex, lazily creating it.
func (s *StateStore) lockFor(id uuid.UUID) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.locks[id]
	if !ok {
		m = &sync.Mutex{}
		s.locks[id] = m
	}
	return m
}

// Get returns the BackfillState for an instance, or ErrNoState if none has
// been written yet. The full instance (including decrypted credentials) is
// returned separately via Load.
func (s *StateStore) Get(ctx context.Context, instanceID uuid.UUID) (*BackfillState, error) {
	inst, err := s.inner.Get(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("fbbackfill: load instance %s: %w", instanceID, err)
	}
	return extractState(inst.Config)
}

// Load returns the full instance with its state. Returns ErrNoState as the
// error if the state key is missing.
func (s *StateStore) Load(ctx context.Context, instanceID uuid.UUID) (*InstanceWithState, error) {
	inst, err := s.inner.Get(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("fbbackfill: load instance %s: %w", instanceID, err)
	}
	iws := &InstanceWithState{
		InstanceID:  inst.ID,
		TenantID:    inst.TenantID,
		AgentID:     inst.AgentID,
		Name:        inst.Name,
		Credentials: inst.Credentials,
		Config:      []byte(inst.Config),
	}
	st, err := extractState(inst.Config)
	if err == nil {
		iws.State = st
	} else if !errors.Is(err, ErrNoState) {
		return nil, err
	}
	return iws, nil
}

// Save atomically updates the _backfill key in the instance config,
// preserving all other config fields.
func (s *StateStore) Save(ctx context.Context, instanceID uuid.UUID, st *BackfillState) error {
	if st == nil {
		return errors.New("fbbackfill: Save called with nil state")
	}
	mu := s.lockFor(instanceID)
	mu.Lock()
	defer mu.Unlock()

	inst, err := s.inner.Get(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("fbbackfill: reload instance %s: %w", instanceID, err)
	}
	st.UpdatedAt = time.Now().UTC()
	merged, err := mergeState(inst.Config, st)
	if err != nil {
		return err
	}
	if err := s.inner.Update(ctx, instanceID, map[string]any{"config": merged}); err != nil {
		return fmt.Errorf("fbbackfill: persist state: %w", err)
	}
	slog.Debug("fb_backfill.state.saved",
		"instance_id", instanceID,
		"status", st.Status,
		"convos_done", st.ConversationsDone,
		"msgs_ingested", st.MessagesIngested)
	return nil
}

// Delete removes the _backfill key entirely. Used when a channel is
// deleted or when clearing stale state.
func (s *StateStore) Delete(ctx context.Context, instanceID uuid.UUID) error {
	mu := s.lockFor(instanceID)
	mu.Lock()
	defer mu.Unlock()

	inst, err := s.inner.Get(ctx, instanceID)
	if err != nil {
		return err
	}
	merged, err := deleteStateKey(inst.Config)
	if err != nil {
		return err
	}
	return s.inner.Update(ctx, instanceID, map[string]any{"config": merged})
}

// ListActive returns all channel instances whose status is running or
// paused. Used at gateway startup to flip crashed jobs to paused.
func (s *StateStore) ListActive(ctx context.Context) ([]InstanceWithState, error) {
	all, err := s.inner.ListAllInstances(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]InstanceWithState, 0)
	for i := range all {
		inst := &all[i]
		if inst.ChannelType != "facebook" {
			continue
		}
		st, err := extractState(inst.Config)
		if err != nil {
			continue
		}
		if st.Status != StatusRunning && st.Status != StatusPaused && st.Status != StatusPending {
			continue
		}
		out = append(out, InstanceWithState{
			InstanceID:  inst.ID,
			TenantID:    inst.TenantID,
			AgentID:     inst.AgentID,
			Name:        inst.Name,
			Credentials: inst.Credentials,
			Config:      []byte(inst.Config),
			State:       st,
		})
	}
	return out, nil
}

// MarkStaleAsPaused flips any state.status==running to paused at gateway
// startup. Rationale: a running state after a fresh boot means the
// previous gateway crashed mid-run. We do not auto-resume — user must
// explicitly Resume via UI, which is the safe default under multi-replica
// deployments (avoids a double-run if another replica is still alive).
func (s *StateStore) MarkStaleAsPaused(ctx context.Context) (int, error) {
	active, err := s.ListActive(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, iws := range active {
		if iws.State == nil || iws.State.Status != StatusRunning {
			continue
		}
		iws.State.Status = StatusPaused
		iws.State.LastError = "resumed from gateway restart — please click Resume"
		if err := s.Save(ctx, iws.InstanceID, iws.State); err != nil {
			slog.Warn("fb_backfill.state.mark_stale_failed",
				"instance_id", iws.InstanceID, "err", err)
			continue
		}
		slog.Info("fb_backfill.state.marked_stale",
			"instance_id", iws.InstanceID, "name", iws.Name)
		n++
	}
	return n, nil
}

// extractState reads the _backfill key out of a config JSONB blob.
// Returns ErrNoState if the key is absent.
func extractState(cfg json.RawMessage) (*BackfillState, error) {
	if len(cfg) == 0 || string(cfg) == "null" {
		return nil, ErrNoState
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(cfg, &m); err != nil {
		return nil, fmt.Errorf("fbbackfill: parse config: %w", err)
	}
	raw, ok := m[BackfillConfigKey]
	if !ok || len(raw) == 0 {
		return nil, ErrNoState
	}
	var st BackfillState
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("fbbackfill: parse _backfill: %w", err)
	}
	// Fill in default Version for forward compat if a malformed write
	// omitted it.
	if st.Version == 0 {
		st.Version = BackfillStateVersion
	}
	return &st, nil
}

// mergeState writes the given state into the _backfill key of the config
// blob, preserving other keys.
func mergeState(cfg json.RawMessage, st *BackfillState) (json.RawMessage, error) {
	m := map[string]json.RawMessage{}
	if len(cfg) > 0 && string(cfg) != "null" {
		if err := json.Unmarshal(cfg, &m); err != nil {
			return nil, fmt.Errorf("fbbackfill: parse existing config: %w", err)
		}
	}
	blob, err := json.Marshal(st)
	if err != nil {
		return nil, err
	}
	m[BackfillConfigKey] = blob
	out, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// deleteStateKey removes the _backfill key from a config blob.
func deleteStateKey(cfg json.RawMessage) (json.RawMessage, error) {
	if len(cfg) == 0 || string(cfg) == "null" {
		return json.RawMessage("{}"), nil
	}
	m := map[string]json.RawMessage{}
	if err := json.Unmarshal(cfg, &m); err != nil {
		return nil, err
	}
	delete(m, BackfillConfigKey)
	return json.Marshal(m)
}
