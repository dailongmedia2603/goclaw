//go:build !sqliteonly

package fbcloak

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Target is one re-engagement candidate produced by Resolver. The struct is
// shared with the inbox scanner fallback so Job runner can iterate either
// source uniformly.
type Target struct {
	ConversationID string    // "t_xxx" if known; empty when resolver-from-DB cannot recover it
	RecipientPSID  string    // FB PSID — always present from DB path
	RecipientName  string    // optional; pulled from agent metadata when available
	LastMessageAt  time.Time // best-known idle anchor — created_at of the most-recent episodic summary
	Source         string    // "fbbackfill" | "scanner"
}

// ResolveOpts narrows a resolver pass to the credential's fanpage and the
// idle window the job is configured for.
type ResolveOpts struct {
	PageID            string
	MinIdle, MaxIdle  time.Duration
	Limit             int
	ExcludeRecipients []string
	// Now lets tests inject a fixed clock; zero value means time.Now().
	Now time.Time
}

// Resolver queries fbbackfill-produced episodic_summaries to find PSIDs that
// fall inside [now-MaxIdle, now-MinIdle]. fbbackfill's source_id convention
// is "fb_backfill:{page_id}:{psid}" — we filter on a LIKE pattern. The
// timestamp anchor is episodic_summaries.created_at because that table does
// NOT carry a last-message timestamp (we use the freshest summary as a proxy).
type Resolver struct {
	DB *sql.DB
}

// NewResolver constructs the resolver around a *sql.DB shared with the rest
// of the gateway. Callers wire it from store.Stores.DB.
func NewResolver(db *sql.DB) *Resolver { return &Resolver{DB: db} }

// Resolve returns up to opts.Limit targets, oldest-first by LastMessageAt.
// Tenant-scoped via TenantID parameter — never trust client-supplied PageID alone.
func (r *Resolver) Resolve(ctx context.Context, tenantID uuid.UUID, opts ResolveOpts) ([]Target, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("resolver: DB is nil")
	}
	if tenantID == uuid.Nil {
		return nil, fmt.Errorf("resolver: tenantID required")
	}
	if opts.PageID == "" {
		return nil, fmt.Errorf("resolver: PageID required")
	}
	if opts.MinIdle <= 0 || opts.MaxIdle <= opts.MinIdle {
		return nil, fmt.Errorf("resolver: invalid idle window: min=%s max=%s", opts.MinIdle, opts.MaxIdle)
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 200
	}

	// We collapse to one row per PSID via DISTINCT ON, taking the freshest
	// summary as the LastMessageAt anchor. This is the row that fbbackfill
	// last wrote for the (page, psid) pair, and matches the user's mental
	// model of "last contact" within fbcloak's tolerance.
	const baseSQL = `
		SELECT DISTINCT ON (user_id)
		       user_id, MAX(created_at) OVER (PARTITION BY user_id) AS last_at
		  FROM episodic_summaries
		 WHERE tenant_id = $1
		   AND source_id LIKE $2
		   AND created_at BETWEEN $3 AND $4
		 ORDER BY user_id, last_at DESC
	`
	likePattern := "fb_backfill:" + escapeLike(opts.PageID) + ":%"
	from := now.Add(-opts.MaxIdle)
	to := now.Add(-opts.MinIdle)

	rows, err := r.DB.QueryContext(ctx, baseSQL, tenantID, likePattern, from, to)
	if err != nil {
		return nil, fmt.Errorf("resolver query: %w", err)
	}
	defer rows.Close()

	exclude := make(map[string]struct{}, len(opts.ExcludeRecipients))
	for _, p := range opts.ExcludeRecipients {
		exclude[p] = struct{}{}
	}

	out := make([]Target, 0, limit)
	for rows.Next() {
		var psid string
		var lastAt time.Time
		if err := rows.Scan(&psid, &lastAt); err != nil {
			return nil, fmt.Errorf("resolver scan: %w", err)
		}
		if _, skip := exclude[psid]; skip {
			continue
		}
		out = append(out, Target{
			RecipientPSID: psid,
			LastMessageAt: lastAt,
			Source:        "fbbackfill",
		})
		if len(out) >= limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("resolver rows: %w", err)
	}

	// Sort oldest-first (we want to re-engage the longest-idle conversations
	// first; the SQL ORDER BY ran by user_id for DISTINCT ON, not by date).
	sortTargetsOldestFirst(out)
	return out, nil
}

// escapeLike escapes a value for use inside a LIKE pattern. The fbbackfill
// page_id is a numeric string so the surface area is minimal — we still
// guard against `_` and `%` so a malicious ID cannot widen the pattern.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

func sortTargetsOldestFirst(ts []Target) {
	for i := 1; i < len(ts); i++ {
		for j := i; j > 0 && ts[j-1].LastMessageAt.After(ts[j].LastMessageAt); j-- {
			ts[j-1], ts[j] = ts[j], ts[j-1]
		}
	}
}
