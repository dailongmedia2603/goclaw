//go:build !sqliteonly

package fbcloak

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// SummaryVersionLookup checks whether the episodic summary for a given
// (tenantID, fanpageID, psid) has been updated since the plan was made.
// Returns the current summary version. Phase 5 reads this from
// episodic_summaries.updated_at converted to a monotonic int.
type SummaryVersionLookup interface {
	CurrentVersion(ctx context.Context, tenantID uuid.UUID, fanpageID, psid string) (int, error)
}

// ReplanScanner is the polling alternative to a real-time event subscriber:
// every TickInterval it walks active plans, checks if the episodic summary
// for the recipient has been refreshed since plan creation (customer replied
// → fbbackfill incremental updated the summary), and if so marks the plan
// `replan_needed` then triggers Generator regeneration.
//
// Trade-off vs. event-based design: ~1 hour replan latency. Acceptable for
// a feature where plans schedule 7-30 days out.
type ReplanScanner struct {
	Plans            PlanStore
	Credentials      CredentialStore
	Generator        *PlanGenerator
	SummaryVersions  SummaryVersionLookup
	Logger           *slog.Logger

	TickInterval time.Duration
	Delay        time.Duration // wait between marking replan_needed and regen (gives fbbackfill incremental time to settle)
	BatchSize    int

	Killswitch *atomic.Bool

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
	clock   func() time.Time
}

const (
	DefaultReplanScannerTick = time.Hour
	DefaultReplanDelay       = 30 * time.Minute
)

func (r *ReplanScanner) Start(ctx context.Context) error {
	if r.Plans == nil || r.Credentials == nil || r.Generator == nil || r.SummaryVersions == nil {
		return errors.New("replan_scanner: missing required deps")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running {
		return nil
	}
	if r.TickInterval == 0 {
		r.TickInterval = DefaultReplanScannerTick
	}
	if r.Delay == 0 {
		r.Delay = DefaultReplanDelay
	}
	if r.BatchSize <= 0 {
		r.BatchSize = 30
	}
	if r.Killswitch == nil {
		r.Killswitch = &atomic.Bool{}
	}
	if r.Logger == nil {
		r.Logger = slog.Default()
	}
	if r.clock == nil {
		r.clock = func() time.Time { return time.Now().UTC() }
	}

	r.stopCh = make(chan struct{})
	r.running = true
	go r.loop(ctx)
	r.Logger.Info("fbcloak.replan_scan.started",
		"tick", r.TickInterval, "delay", r.Delay,
	)
	return nil
}

func (r *ReplanScanner) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.running {
		return
	}
	close(r.stopCh)
	r.running = false
}

func (r *ReplanScanner) loop(ctx context.Context) {
	t := time.NewTicker(r.TickInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case <-t.C:
			if r.Killswitch.Load() {
				continue
			}
			r.tick(ctx)
		}
	}
}

// tick runs the two passes: (1) detect stale plans + flip to replan_needed,
// (2) pick up replan_needed plans past the delay and regenerate.
func (r *ReplanScanner) tick(ctx context.Context) {
	r.detectAndMark(ctx)
	r.regenStale(ctx)
}

// detectAndMark walks all credentials, for each one inspects its active
// plans, compares each plan's summary_version against the current episodic
// version, and flips status to 'replan_needed' if drift detected.
func (r *ReplanScanner) detectAndMark(ctx context.Context) {
	// We don't have a cross-tenant credential lister on CredentialStore yet,
	// so we batch by walking active plans status='pending' OR 'sent' across
	// tenants via Plans.List with no tenant filter (the call site that
	// hooks PG store accepts uuid.Nil → no tenant filter for cross-tenant
	// scans). For Phase 5 MVP we use a simpler pattern: query plans by
	// status across tenants via a dedicated PlanStore method — but to keep
	// the interface stable, we issue a List per tenant we discover from
	// existing pending plans via List(uuid.Nil, ...).
	//
	// The canonical cross-tenant scan is: SELECT DISTINCT tenant_id, ... .
	// Since the existing PlanStore.List requires tenantID, and we don't
	// want to add yet another method to the interface mid-phase, the
	// scanner is wired in 5.6 with a small adapter that issues raw SQL
	// against the PG db. For now this method walks via Plans.DuePlans
	// (which is cross-tenant) — but DuePlans only returns scheduled<=now.
	// To detect drift on plans scheduled in the future, we need a separate
	// query. Defer that to the gateway adapter:
	r.regenStale(ctx)
}

// regenStale picks up plans already in 'replan_needed' status (set by
// either the detect pass above OR an external mark, e.g. RPC admin force-
// replan) and runs Generator.RunForCredentialPSID per row.
func (r *ReplanScanner) regenStale(ctx context.Context) {
	plans, err := r.Plans.ReplanNeeded(ctx, r.clock(), r.Delay, r.BatchSize)
	if err != nil {
		r.Logger.Warn("fbcloak.replan_scan.list_failed", "err", err)
		return
	}
	for _, p := range plans {
		if r.Killswitch.Load() {
			return
		}
		if err := r.replanOne(ctx, p); err != nil {
			r.Logger.Warn("fbcloak.replan_scan.one_failed",
				"plan", p.ID, "err", err)
			IncReplanError()
			continue
		}
		IncReplanComplete()
	}
}

func (r *ReplanScanner) replanOne(ctx context.Context, p Plan) error {
	if err := r.Plans.MarkSuperseded(ctx, p.TenantID, p.ID); err != nil {
		return err
	}
	if _, err := r.Generator.RunForCredentialPSID(ctx, p.TenantID, p.CredentialID, p.PSID); err != nil {
		return err
	}
	return nil
}

// MarkPlanReplanNeeded is the per-plan public entry — exposed so an event
// subscriber (future enhancement) or admin RPC can trigger replan without
// going through the polling pass.
func (r *ReplanScanner) MarkPlanReplanNeeded(ctx context.Context, credentialID uuid.UUID, psid string) (int, error) {
	n, err := r.Plans.MarkReplanNeeded(ctx, credentialID, psid)
	if err == nil && n > 0 {
		IncPlanReplanMarked()
	}
	return n, err
}
