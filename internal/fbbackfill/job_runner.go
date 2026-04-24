package fbbackfill

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// ClientFactory constructs a BackfillClient for a specific Page. Exposed
// as an interface so tests can inject a client that talks to a mock
// Graph server instead of the public Graph API.
type ClientFactory interface {
	Build(pageAccessToken, pageID string) GraphAPIBackfill
}

// GraphAPIBackfill is the narrow surface of BackfillClient the runner uses.
// Defined as an interface so tests can substitute a fake.
type GraphAPIBackfill interface {
	ListConversations(ctx context.Context, cursor string) (*ListConversationsPage, error)
	ListMessages(ctx context.Context, conversationID, cursor string) (*ListMessagesPage, error)
	BUCTracker() *bucTracker
}

// defaultClientFactory builds a real BackfillClient.
type defaultClientFactory struct{}

// NewDefaultClientFactory returns a factory that constructs live
// BackfillClient instances against the public Graph API.
func NewDefaultClientFactory() ClientFactory { return defaultClientFactory{} }

func (defaultClientFactory) Build(token, pageID string) GraphAPIBackfill {
	return NewBackfillClient(token, pageID)
}

// RunnerDeps bundles the dependencies a JobRunner needs at construction.
type RunnerDeps struct {
	StateStore    *StateStore
	Instances     store.ChannelInstanceStore // used to load credentials + config
	ClientFactory ClientFactory
	Summarizer    Summarizer
	Emitter       EventEmitter

	// MaxConcurrentJobs caps how many jobs can run in parallel process-wide.
	// 0 defaults to 3 to keep DB + LLM load bounded.
	MaxConcurrentJobs int
}

// JobRunner orchestrates backfill jobs. One instance per gateway process.
// Commands (Start/Pause/Resume/Cancel/Retry) are thread-safe and can be
// called from RPC handlers while the main loop runs in a goroutine.
type JobRunner struct {
	deps RunnerDeps

	mu   sync.Mutex
	jobs map[uuid.UUID]*jobController
	sem  chan struct{}

	// clock + sleep injectable for deterministic tests.
	clock func() time.Time
	sleep func(context.Context, time.Duration) error
}

// jobController is the in-memory handle for a running goroutine.
type jobController struct {
	instanceID uuid.UUID
	pauseCh    chan struct{} // signal to pause
	resumeCh   chan struct{} // signal to resume (unblocks the paused loop)
	cancelCh   chan struct{} // signal to cancel
	doneCh    chan struct{} // closed when goroutine exits
	paused     bool
}

// NewJobRunner constructs a runner with the given dependencies.
func NewJobRunner(deps RunnerDeps) *JobRunner {
	if deps.MaxConcurrentJobs <= 0 {
		deps.MaxConcurrentJobs = 3
	}
	if deps.Emitter == nil {
		deps.Emitter = noopEmitter{}
	}
	return &JobRunner{
		deps:  deps,
		jobs:  make(map[uuid.UUID]*jobController),
		sem:   make(chan struct{}, deps.MaxConcurrentJobs),
		clock: time.Now,
		sleep: func(ctx context.Context, d time.Duration) error {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
				return nil
			}
		},
	}
}

// withClock lets tests inject a fake clock. Not exported — tests use the
// package-internal helper newTestRunner.
func (r *JobRunner) withClock(now func() time.Time) *JobRunner { r.clock = now; return r }

// Running reports whether an instance has an active goroutine right now.
// Used by tests and RPC.
func (r *JobRunner) Running(instanceID uuid.UUID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.jobs[instanceID]
	return ok
}

// Start transitions a channel instance's backfill job to running and
// spawns the goroutine. Errors:
//   - ErrMissingAccessToken / ErrMissingPageID for bad creds/config
//   - already-running error when a goroutine for this instance exists
func (r *JobRunner) Start(ctx context.Context, instanceID uuid.UUID, opts StartOpts) error {
	r.mu.Lock()
	if _, ok := r.jobs[instanceID]; ok {
		r.mu.Unlock()
		return fmt.Errorf("fbbackfill: job for %s is already running", instanceID)
	}
	r.mu.Unlock()

	// Load instance + creds. This also validates the channel is type=facebook.
	inst, err := r.deps.Instances.Get(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("fbbackfill: load instance: %w", err)
	}
	if inst.ChannelType != "facebook" {
		return fmt.Errorf("fbbackfill: channel type %q is not facebook", inst.ChannelType)
	}
	creds, err := parseCreds(inst.Credentials)
	if err != nil {
		return err
	}
	cfg, err := parseConfig(inst.Config)
	if err != nil {
		return err
	}

	// Initialize or reset state.
	st, loadErr := r.deps.StateStore.Get(ctx, instanceID)
	if errors.Is(loadErr, ErrNoState) || (st != nil && st.Status.IsTerminal()) {
		st = NewBackfillState(opts)
	} else if st != nil {
		// Resume semantics: if paused or pending, adopt but refresh options.
		st.Status = StatusPending
		if opts.MaxConversations != 0 {
			st.MaxConversations = opts.MaxConversations
		}
		if opts.TriggeredBy != "" {
			st.TriggeredBy = opts.TriggeredBy
		}
		st.LastError = ""
	} else if loadErr != nil {
		return loadErr
	}

	started := r.clock().UTC()
	st.Status = StatusRunning
	if st.StartedAt == nil {
		st.StartedAt = &started
	}
	if err := r.deps.StateStore.Save(ctx, instanceID, st); err != nil {
		return err
	}

	client := r.deps.ClientFactory.Build(creds.PageAccessToken, cfg.PageID)
	ctl := &jobController{
		instanceID: instanceID,
		pauseCh:    make(chan struct{}, 1),
		resumeCh:   make(chan struct{}, 1),
		cancelCh:   make(chan struct{}, 1),
		doneCh:     make(chan struct{}),
	}
	r.mu.Lock()
	r.jobs[instanceID] = ctl
	r.mu.Unlock()

	slog.Info("fb_backfill.job.started",
		"instance_id", instanceID, "tenant_id", inst.TenantID,
		"page_id", cfg.PageID, "triggered_by", st.TriggeredBy,
		"max_conversations", st.MaxConversations)
	r.deps.Emitter.EmitStarted(inst.TenantID, instanceID)

	// Run in a goroutine; the sem is acquired inside runJob so that the
	// RPC caller returns immediately with status=running.
	go r.runJob(ctl, &InstanceWithState{
		InstanceID:  instanceID,
		TenantID:    inst.TenantID,
		AgentID:     inst.AgentID,
		Name:        inst.Name,
		Credentials: inst.Credentials,
		Config:      []byte(inst.Config),
		State:       st,
	}, client, cfg.PageID)
	return nil
}

// Pause sends a pause signal. The goroutine flips status to paused at its
// next checkpoint (between Graph calls). No-op if not running.
func (r *JobRunner) Pause(_ context.Context, instanceID uuid.UUID) error {
	r.mu.Lock()
	ctl, ok := r.jobs[instanceID]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("fbbackfill: no running job for %s", instanceID)
	}
	select {
	case ctl.pauseCh <- struct{}{}:
	default:
	}
	return nil
}

// Resume either wakes a paused goroutine or starts a fresh one if the
// previous goroutine has already exited (gateway restart case).
func (r *JobRunner) Resume(ctx context.Context, instanceID uuid.UUID) error {
	r.mu.Lock()
	ctl, ok := r.jobs[instanceID]
	r.mu.Unlock()
	if ok {
		// Goroutine is alive — send resume signal.
		select {
		case ctl.resumeCh <- struct{}{}:
		default:
		}
		return nil
	}
	// Goroutine is gone (post-restart). Start a fresh one that will pick
	// up cursors from the persisted state.
	return r.Start(ctx, instanceID, StartOpts{TriggeredBy: "resume"})
}

// Cancel signals the goroutine to exit without completing. State
// transitions to cancelled.
func (r *JobRunner) Cancel(_ context.Context, instanceID uuid.UUID) error {
	r.mu.Lock()
	ctl, ok := r.jobs[instanceID]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("fbbackfill: no running job for %s", instanceID)
	}
	select {
	case ctl.cancelCh <- struct{}{}:
	default:
	}
	return nil
}

// Retry resets the state cursors and re-starts a failed job.
func (r *JobRunner) Retry(ctx context.Context, instanceID uuid.UUID) error {
	st, err := r.deps.StateStore.Get(ctx, instanceID)
	if err != nil && !errors.Is(err, ErrNoState) {
		return err
	}
	if st != nil {
		// Clear cursors + error so Start() treats it as a fresh run.
		st.Status = StatusPending
		st.ConversationCursor = ""
		st.CurrentConvoID = ""
		st.MessageCursor = ""
		st.ConversationsDone = 0
		st.MessagesIngested = 0
		st.EpisodicsCreated = 0
		st.ConversationsSkipped = 0
		st.LastError = ""
		st.FinishedAt = nil
		if err := r.deps.StateStore.Save(ctx, instanceID, st); err != nil {
			return err
		}
	}
	return r.Start(ctx, instanceID, StartOpts{TriggeredBy: "retry"})
}

// Status returns the current persisted BackfillState.
func (r *JobRunner) Status(ctx context.Context, instanceID uuid.UUID) (*BackfillState, error) {
	return r.deps.StateStore.Get(ctx, instanceID)
}

// Wait blocks until the goroutine for instanceID exits (or immediately if
// not running). Used by tests.
func (r *JobRunner) Wait(instanceID uuid.UUID) {
	r.mu.Lock()
	ctl, ok := r.jobs[instanceID]
	r.mu.Unlock()
	if !ok {
		return
	}
	<-ctl.doneCh
}

// --- Main loop ---

// runJob is the goroutine entry point. It acquires the concurrency slot,
// then loops: check signals → advance one conversation → save → repeat.
func (r *JobRunner) runJob(ctl *jobController, iws *InstanceWithState, client GraphAPIBackfill, pageID string) {
	// Acquire slot. This can block if MaxConcurrentJobs already saturated.
	r.sem <- struct{}{}
	defer func() { <-r.sem }()
	defer close(ctl.doneCh)
	defer func() {
		r.mu.Lock()
		delete(r.jobs, ctl.instanceID)
		r.mu.Unlock()
	}()

	// Background context — request context from Start is not valid here.
	ctx := context.Background()
	// Propagate tenant/agent/user via store context helpers so any call
	// into stores (summarizer, state store) is tenant-scoped.
	ctx = store.WithTenantID(ctx, iws.TenantID)
	ctx = store.WithAgentID(ctx, iws.AgentID)

	st := iws.State

	// Helper to persist + emit progress.
	save := func() {
		if err := r.deps.StateStore.Save(ctx, ctl.instanceID, st); err != nil {
			slog.Warn("fb_backfill.state.save_failed", "instance_id", ctl.instanceID, "err", err)
			return
		}
		r.deps.Emitter.EmitProgress(iws.TenantID, ctl.instanceID, st)
	}

	finish := func(status JobStatus, errMsg string) {
		st.Status = status
		if errMsg != "" {
			st.LastError = errMsg
		}
		now := r.clock().UTC()
		st.FinishedAt = &now
		save()
		switch status {
		case StatusCompleted:
			slog.Info("fb_backfill.job.completed",
				"instance_id", ctl.instanceID,
				"convos_done", st.ConversationsDone,
				"msgs_ingested", st.MessagesIngested,
				"episodics_created", st.EpisodicsCreated)
			r.deps.Emitter.EmitCompleted(iws.TenantID, ctl.instanceID, st)
		case StatusFailed:
			slog.Error("fb_backfill.job.failed",
				"instance_id", ctl.instanceID, "err", errMsg,
				"convos_done", st.ConversationsDone)
			r.deps.Emitter.EmitFailed(iws.TenantID, ctl.instanceID, errMsg)
		case StatusCancelled:
			slog.Info("fb_backfill.job.cancelled", "instance_id", ctl.instanceID)
		}
	}

	for {
		// Check signals non-blocking between iterations.
		select {
		case <-ctl.cancelCh:
			finish(StatusCancelled, "")
			return
		case <-ctl.pauseCh:
			st.Status = StatusPaused
			save()
			slog.Info("fb_backfill.job.paused", "instance_id", ctl.instanceID, "reason", "user")
			r.deps.Emitter.EmitPaused(iws.TenantID, ctl.instanceID, "user")
			// Block until resume or cancel.
			select {
			case <-ctl.resumeCh:
				st.Status = StatusRunning
				st.LastError = ""
				save()
				slog.Info("fb_backfill.job.resumed", "instance_id", ctl.instanceID,
					"cursor", st.ConversationCursor)
				r.deps.Emitter.EmitResumed(iws.TenantID, ctl.instanceID)
			case <-ctl.cancelCh:
				finish(StatusCancelled, "")
				return
			}
		default:
		}

		// Cap reached?
		if st.MaxConversations > 0 && st.ConversationsDone >= st.MaxConversations {
			st.LastError = fmt.Sprintf("reached max_conversations cap (%d)", st.MaxConversations)
			finish(StatusCompleted, st.LastError)
			return
		}

		// If we are mid-conversation (resume case), finish it first.
		if st.CurrentConvoID != "" {
			if err := r.processOneConversation(ctx, client, pageID, iws, st); err != nil {
				if r.handleAPIError(err, st, ctl, iws) {
					save()
					return
				}
			}
			save()
			continue
		}

		// Fetch next page of conversations.
		page, err := client.ListConversations(ctx, st.ConversationCursor)
		if err != nil {
			if r.handleAPIError(err, st, ctl, iws) {
				save()
				return
			}
			continue
		}

		if len(page.Data) == 0 && page.Next == "" {
			finish(StatusCompleted, "")
			return
		}

		for _, convo := range page.Data {
			// Check cancel/pause between conversations for responsiveness.
			select {
			case <-ctl.cancelCh:
				finish(StatusCancelled, "")
				return
			default:
			}

			st.CurrentConvoID = convo.ID
			st.MessageCursor = ""
			st.ConversationsTotal++ // best-effort grows with each page

			// Fast-path skip when source already has an episodic entry.
			if st.SkipExisting && !st.ForceRecreate {
				psid := extractPSIDFromParticipants(convo, pageID)
				if psid != "" {
					srcID := SourceIDFor(pageID, psid)
					exists, err := r.deps.Summarizer.AlreadySummarized(ctx, iws.AgentID, psid, srcID)
					if err == nil && exists {
						slog.Debug("fb_backfill.summarize.skip_existing", "source_id", srcID)
						st.ConversationsSkipped++
						st.ConversationsDone++
						st.CurrentConvoID = ""
						save()
						continue
					}
				}
			}

			if err := r.processOneConversation(ctx, client, pageID, iws, st); err != nil {
				if r.handleAPIError(err, st, ctl, iws) {
					save()
					return
				}
				// non-terminal (e.g., single-convo failure) — record and continue
				st.LastError = err.Error()
				st.ConversationsDone++
				st.CurrentConvoID = ""
				save()
				continue
			}
			st.ConversationsDone++
			st.CurrentConvoID = ""
			save()

			if st.MaxConversations > 0 && st.ConversationsDone >= st.MaxConversations {
				st.LastError = fmt.Sprintf("reached max_conversations cap (%d)", st.MaxConversations)
				finish(StatusCompleted, st.LastError)
				return
			}
		}

		st.ConversationCursor = page.Next
		save()
		if page.Next == "" {
			finish(StatusCompleted, "")
			return
		}
	}
}

// processOneConversation pulls all messages for the current convo (starting
// from any saved message cursor), hands them to the summarizer, and
// updates counters.
func (r *JobRunner) processOneConversation(
	ctx context.Context, client GraphAPIBackfill, pageID string,
	iws *InstanceWithState, st *BackfillState,
) error {
	convoID := st.CurrentConvoID
	var allMessages []Message
	cursor := st.MessageCursor

	for {
		page, err := client.ListMessages(ctx, convoID, cursor)
		if err != nil {
			return err
		}
		allMessages = append(allMessages, page.Data...)
		st.MessagesIngested += len(page.Data)
		st.MessageCursor = page.Next
		cursor = page.Next
		if cursor == "" {
			break
		}
	}

	// Graph returns newest-first; summarizer wants chronological order.
	sort.SliceStable(allMessages, func(i, j int) bool {
		return allMessages[i].CreatedTime.Before(allMessages[j].CreatedTime)
	})

	if len(allMessages) == 0 {
		return nil
	}

	psid := extractPSIDFromMessages(allMessages, pageID)
	if psid == "" {
		return errors.New("fbbackfill: could not identify PSID for conversation")
	}
	srcID := SourceIDFor(pageID, psid)
	// Propagate user_id=psid for episodic write scoping.
	ctx = store.WithUserID(ctx, psid)
	err := r.deps.Summarizer.Summarize(ctx, SummarizeInput{
		InstanceID:    iws.InstanceID,
		TenantID:      iws.TenantID,
		AgentID:       iws.AgentID,
		PageID:        pageID,
		PSID:          psid,
		SourceID:      srcID,
		Messages:      allMessages,
		ForceRecreate: st.ForceRecreate,
	})
	if err == nil {
		st.EpisodicsCreated++
	}
	return err
}

// handleAPIError classifies an error returned from the Graph client and
// transitions the job state appropriately. Returns true if the caller
// should stop the run loop (job transitioned to a terminal or paused
// state), false if it should continue.
func (r *JobRunner) handleAPIError(
	err error, st *BackfillState, ctl *jobController, iws *InstanceWithState,
) bool {
	switch {
	case errors.Is(err, ErrAuthExpired):
		st.Status = StatusFailed
		st.LastError = "Page Access Token expired or invalid — please re-connect the channel"
		now := r.clock().UTC()
		st.FinishedAt = &now
		slog.Error("fb_backfill.job.failed",
			"instance_id", ctl.instanceID, "reason", "auth_expired", "err", err)
		r.deps.Emitter.EmitFailed(iws.TenantID, ctl.instanceID, st.LastError)
		return true

	case errors.Is(err, ErrRateLimit):
		st.Status = StatusPaused
		st.LastError = "Graph API rate limit — auto-resume scheduled"
		slog.Warn("fb_backfill.job.paused",
			"instance_id", ctl.instanceID, "reason", "rate_limit")
		r.deps.Emitter.EmitPaused(iws.TenantID, ctl.instanceID, "rate_limit")
		return true

	case errors.Is(err, ErrBadRequest) || errors.Is(err, ErrNotFound):
		// Single-conversation failures (e.g. 404 for a deleted convo)
		// should not fail the whole job. Return false to continue.
		slog.Warn("fb_backfill.client.bad_request",
			"instance_id", ctl.instanceID, "err", err)
		st.LastError = err.Error()
		return false

	case errors.Is(err, ErrTransient):
		// Client already exhausted retries. Treat as non-fatal; record
		// and move on to the next conversation.
		slog.Warn("fb_backfill.client.transient_exhausted",
			"instance_id", ctl.instanceID, "err", err)
		st.LastError = err.Error()
		return false
	}
	// Unknown error — bail out.
	st.Status = StatusFailed
	st.LastError = err.Error()
	now := r.clock().UTC()
	st.FinishedAt = &now
	slog.Error("fb_backfill.job.failed",
		"instance_id", ctl.instanceID, "err", err)
	r.deps.Emitter.EmitFailed(iws.TenantID, ctl.instanceID, err.Error())
	return true
}

// extractPSIDFromParticipants returns the non-Page participant ID from a
// Conversation object. Messenger DMs are 1:1 (page + one user); group
// threads would have >2 but Messenger Graph API does not currently
// expose group threads for Pages, so 2-person is the expected shape.
func extractPSIDFromParticipants(c Conversation, pageID string) string {
	for _, p := range c.Participants.Data {
		if p.ID != "" && p.ID != pageID {
			return p.ID
		}
	}
	return ""
}

// extractPSIDFromMessages falls back to scanning messages when the
// conversation participants block is unavailable. Returns the first
// non-Page 'from' ID encountered.
func extractPSIDFromMessages(msgs []Message, pageID string) string {
	for _, m := range msgs {
		if m.From.ID != "" && m.From.ID != pageID {
			return m.From.ID
		}
	}
	return ""
}
