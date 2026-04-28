//go:build !sqliteonly

package fbcloak

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// EpisodicSource abstracts the read side of episodic_summaries that fbbackfill
// writes. Production wires a PG impl that filters by fanpage_id (matches
// fbbackfill's source_id convention `fb_backfill:<page>:<conv>`).
type EpisodicSource interface {
	// ListByFanpage returns recipients whose last_message_at is in the idle
	// window (now-maxIdle, now-minIdle). Cross-tenant — caller scopes.
	ListByFanpage(ctx context.Context, tenantID uuid.UUID, fanpageID string,
		minIdle, maxIdle time.Duration, limit int) ([]EpisodicTarget, error)

	// GetByPSID fetches a single recipient's latest summary. Used by Replan
	// to refresh context after the customer replied.
	GetByPSID(ctx context.Context, tenantID uuid.UUID, fanpageID, psid string) (EpisodicTarget, error)
}

// EpisodicTarget is the slim per-recipient context Generator passes to LLM.
type EpisodicTarget struct {
	PSID           string
	ConversationID string
	RecipientName  string
	LastInboundAt  time.Time
	TurnCount      int
	SummaryText    string
	RecentSnippets []string
	SummaryVersion int
}

// CredentialActiveLister returns the active credentials Generator iterates.
// Implemented by extending CredentialStore — see plan_generator's wiring in
// gateway init (5.6) which provides a function adapter.
type CredentialActiveLister interface {
	ListAllActive(ctx context.Context) ([]Credential, error)
}

// PlanGenerator orchestrates: enumerate targets → call LLM → insert Plans.
// One worker per credential (per-credential mutex prevents Active-conflict
// races between Generator + Replan paths).
type PlanGenerator struct {
	Plans           PlanStore
	Credentials     CredentialStore
	ActiveCredsList CredentialActiveLister
	Episodic        EpisodicSource
	LLM             PlanLLM
	Logger          *slog.Logger

	Model         string
	TickInterval  time.Duration
	MaxConcurrent int
	JitterRange   time.Duration
	MinIdle       time.Duration
	MaxIdle       time.Duration
	BatchSize     int

	Killswitch *atomic.Bool

	mu       sync.Mutex
	running  bool
	stopCh   chan struct{}
	credLock sync.Map // credential UUID → *sync.Mutex
	clock    func() time.Time
}

const DefaultPlanGeneratorTick = 7 * 24 * time.Hour

func (g *PlanGenerator) Start(ctx context.Context) error {
	if err := g.validateDeps(); err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.running {
		return nil
	}
	if g.TickInterval == 0 {
		g.TickInterval = DefaultPlanGeneratorTick
	}
	if g.MaxConcurrent <= 0 {
		g.MaxConcurrent = 5
	}
	if g.JitterRange == 0 {
		g.JitterRange = 15 * time.Minute
	}
	if g.MinIdle == 0 {
		g.MinIdle = 7 * 24 * time.Hour
	}
	if g.MaxIdle == 0 {
		g.MaxIdle = 180 * 24 * time.Hour
	}
	if g.BatchSize <= 0 {
		g.BatchSize = 50
	}
	if g.Killswitch == nil {
		g.Killswitch = &atomic.Bool{}
	}
	if g.Logger == nil {
		g.Logger = slog.Default()
	}
	if g.clock == nil {
		g.clock = func() time.Time { return time.Now().UTC() }
	}
	g.stopCh = make(chan struct{})
	g.running = true
	go g.loop(ctx)
	g.Logger.Info("fbcloak.plan_gen.started",
		"tick", g.TickInterval, "model", g.Model, "batch_size", g.BatchSize,
	)
	return nil
}

func (g *PlanGenerator) Stop() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.running {
		return
	}
	close(g.stopCh)
	g.running = false
}

func (g *PlanGenerator) validateDeps() error {
	if g.Plans == nil || g.Credentials == nil || g.ActiveCredsList == nil ||
		g.Episodic == nil || g.LLM == nil {
		return errors.New("plan_generator: missing required deps")
	}
	return nil
}

func (g *PlanGenerator) loop(ctx context.Context) {
	t := time.NewTicker(g.TickInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-g.stopCh:
			return
		case <-t.C:
			if g.Killswitch.Load() {
				continue
			}
			g.tick(ctx)
		}
	}
}

func (g *PlanGenerator) tick(ctx context.Context) {
	creds, err := g.ActiveCredsList.ListAllActive(ctx)
	if err != nil {
		g.Logger.Warn("fbcloak.plan_gen.list_creds_failed", "err", err)
		return
	}
	for _, c := range creds {
		if g.Killswitch.Load() {
			return
		}
		if _, err := g.RunForCredential(ctx, c.TenantID, c.ID); err != nil {
			g.Logger.Warn("fbcloak.plan_gen.cred_failed",
				"credential", c.ID, "err", err,
			)
		}
	}
}

// GenSummary aggregates the result of a generator run for caller display.
type GenSummary struct {
	Created int
	Skipped int
	Errors  int
}

// RunForCredential generates plans for one credential's idle PSIDs. Exposed
// so RPC `generate-now` can trigger ad-hoc.
func (g *PlanGenerator) RunForCredential(ctx context.Context, tenantID, credentialID uuid.UUID) (GenSummary, error) {
	credLock := g.credentialMutex(credentialID)
	credLock.Lock()
	defer credLock.Unlock()

	cred, err := g.Credentials.Get(ctx, tenantID, credentialID)
	if err != nil {
		return GenSummary{}, err
	}
	if cred.Status != StatusActive {
		return GenSummary{}, fmt.Errorf("credential not active (status=%s)", cred.Status)
	}

	targets, err := g.Episodic.ListByFanpage(ctx, tenantID, cred.FanpageID, g.MinIdle, g.MaxIdle, g.BatchSize)
	if err != nil {
		return GenSummary{}, fmt.Errorf("list targets: %w", err)
	}

	sys := RenderOrchestrateSkill(map[string]string{"fanpage_name": cred.FanpageName})

	summary := GenSummary{}
	for _, t := range targets {
		if g.Killswitch.Load() {
			break
		}
		// Idempotency: skip if active plan already exists.
		if _, err := g.Plans.GetActiveForRecipient(ctx, tenantID, credentialID, t.PSID); err == nil {
			summary.Skipped++
			continue
		} else if !errors.Is(err, ErrPlanNotFound) {
			g.Logger.Warn("fbcloak.plan_gen.active_check_failed",
				"psid", t.PSID, "err", err)
			summary.Errors++
			continue
		}

		decision, err := g.callLLM(ctx, sys, t)
		if err != nil {
			g.Logger.Warn("fbcloak.plan_gen.llm_failed",
				"psid", t.PSID, "err", err)
			summary.Errors++
			continue
		}

		if decision.ShouldSend {
			if err := g.createSendPlan(ctx, tenantID, credentialID, t, decision, cred); err != nil {
				if errors.Is(err, ErrActiveConflict) {
					summary.Skipped++
					continue
				}
				g.Logger.Warn("fbcloak.plan_gen.create_failed",
					"psid", t.PSID, "err", err)
				summary.Errors++
				continue
			}
			summary.Created++
		} else {
			if err := g.createSkipPlan(ctx, tenantID, credentialID, t, decision); err != nil {
				if errors.Is(err, ErrActiveConflict) {
					summary.Skipped++
					continue
				}
				g.Logger.Warn("fbcloak.plan_gen.skip_create_failed",
					"psid", t.PSID, "err", err)
				summary.Errors++
				continue
			}
			summary.Skipped++
		}
	}

	g.Logger.Info("fbcloak.plan_gen.completed",
		"credential", credentialID, "created", summary.Created,
		"skipped", summary.Skipped, "errors", summary.Errors,
	)
	return summary, nil
}

// RunForCredentialPSID is the single-recipient entry used by Replan worker.
// Loads the latest episodic for that PSID and calls the same LLM flow.
func (g *PlanGenerator) RunForCredentialPSID(ctx context.Context, tenantID, credentialID uuid.UUID, psid string) (GenSummary, error) {
	credLock := g.credentialMutex(credentialID)
	credLock.Lock()
	defer credLock.Unlock()

	cred, err := g.Credentials.Get(ctx, tenantID, credentialID)
	if err != nil {
		return GenSummary{}, err
	}
	if cred.Status != StatusActive {
		return GenSummary{}, fmt.Errorf("credential not active (status=%s)", cred.Status)
	}

	t, err := g.Episodic.GetByPSID(ctx, tenantID, cred.FanpageID, psid)
	if err != nil {
		return GenSummary{}, fmt.Errorf("get episodic: %w", err)
	}

	sys := RenderOrchestrateSkill(map[string]string{"fanpage_name": cred.FanpageName})
	summary := GenSummary{}

	decision, err := g.callLLM(ctx, sys, t)
	if err != nil {
		summary.Errors++
		return summary, err
	}

	if decision.ShouldSend {
		if err := g.createSendPlan(ctx, tenantID, credentialID, t, decision, cred); err != nil {
			if errors.Is(err, ErrActiveConflict) {
				summary.Skipped++
				return summary, nil
			}
			summary.Errors++
			return summary, err
		}
		summary.Created++
	} else {
		if err := g.createSkipPlan(ctx, tenantID, credentialID, t, decision); err != nil {
			if !errors.Is(err, ErrActiveConflict) {
				summary.Errors++
				return summary, err
			}
		}
		summary.Skipped++
	}
	return summary, nil
}

func (g *PlanGenerator) createSendPlan(ctx context.Context, tenantID, credentialID uuid.UUID, t EpisodicTarget, d PlanDecision, _ Credential) error {
	jitter := jitterFor(t.PSID, g.JitterRange)
	_, err := g.Plans.Create(ctx, tenantID, PlanInput{
		CredentialID:     credentialID,
		PSID:             t.PSID,
		ConversationID:   t.ConversationID,
		RecipientName:    t.RecipientName,
		ScheduledAt:      d.ScheduleAt(g.clock(), jitter),
		MessageDraft:     d.Message,
		Reason:           d.Reason,
		GeneratedByModel: g.Model,
		SummaryVersion:   t.SummaryVersion,
	})
	return err
}

func (g *PlanGenerator) createSkipPlan(ctx context.Context, tenantID, credentialID uuid.UUID, t EpisodicTarget, d PlanDecision) error {
	// SECURITY: insert directly in status='skipped' (single SQL) rather than
	// the previous Create-then-MarkSkipped flow. A transient failure between
	// the two calls would otherwise leave a status='pending' row with the
	// placeholder message ("(skipped by orchestrator)") that PlanExecutor
	// could later pick up and SEND to the customer. CreateSkipped is
	// transactional + bypasses the active-uniqueness index by writing
	// terminal status from the start.
	_, err := g.Plans.CreateSkipped(ctx, tenantID, PlanInput{
		CredentialID:     credentialID,
		PSID:             t.PSID,
		ConversationID:   t.ConversationID,
		RecipientName:    t.RecipientName,
		ScheduledAt:      g.clock(),
		Reason:           d.Reason,
		GeneratedByModel: g.Model,
		SummaryVersion:   t.SummaryVersion,
	}, d.SkipReason)
	return err
}

func (g *PlanGenerator) callLLM(ctx context.Context, sys string, t EpisodicTarget) (PlanDecision, error) {
	user := buildUserPrompt(t, g.clock())

	out, err := g.LLM.Generate(ctx, PlanLLMInput{
		SystemPrompt: sys,
		UserPrompt:   user,
		Model:        g.Model,
	})
	if err != nil {
		return PlanDecision{}, err
	}

	g.Logger.Debug("fbcloak.plan_gen.llm_call",
		"psid", t.PSID, "model", out.Model,
		"input_tokens", out.InputTokens, "output_tokens", out.OutputTokens,
		"cached_tokens", out.CachedTokens,
	)
	return ParseDecision(out.Text)
}

func (g *PlanGenerator) credentialMutex(id uuid.UUID) *sync.Mutex {
	v, _ := g.credLock.LoadOrStore(id, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// --- helpers ---

func buildUserPrompt(t EpisodicTarget, now time.Time) string {
	daysAgo := int(now.Sub(t.LastInboundAt).Hours() / 24)
	snippets := joinSnippets(t.RecentSnippets, 5)
	name := t.RecipientName
	if name == "" {
		name = "khách"
	}
	return fmt.Sprintf(`Khách hàng: %s
Lần khách nhắn cuối: %s (%d ngày trước)
Tổng số lượt: %d

Tóm tắt episodic memory:
%s

5 tin gần nhất (snippets):
%s`,
		name,
		t.LastInboundAt.Format("2006-01-02"),
		daysAgo,
		t.TurnCount,
		t.SummaryText,
		snippets,
	)
}

func joinSnippets(s []string, n int) string {
	if len(s) > n {
		s = s[:n]
	}
	if len(s) == 0 {
		return "(không có dữ liệu)"
	}
	out := ""
	for i, x := range s {
		out += fmt.Sprintf("%d. %s\n", i+1, x)
	}
	return out
}

// jitterFor produces a deterministic jitter in [-max/2, +max/2] from a string seed.
// Deterministic so tests can predict the jitter for a given PSID.
func jitterFor(seed string, max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	h := uint64(0)
	for _, b := range []byte(seed) {
		h = h*131 + uint64(b)
	}
	half := int64(max / 2)
	if half == 0 {
		return 0
	}
	return time.Duration(int64(h%uint64(2*half)) - half)
}
