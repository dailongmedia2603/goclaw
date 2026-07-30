//go:build !sqliteonly

package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PGLastInboundResolver implements fbproactive.LastInboundResolver against
// the same episodic_summaries table that fbbackfill writes into. The
// `source_id` format is `fb_backfill:{page_id}:{psid}` (per
// internal/channels/fbcloak/target_resolver.go), so we can resolve the
// "last inbound" for a single PSID by selecting MAX(created_at) of rows
// matching the tenant + composite source_id.
//
// Returns (zero time, nil) when no row matches — caller treats that as
// fbcloak.ErrNoConversationHistory in the router.
type PGLastInboundResolver struct {
	db *sql.DB
}

func NewPGLastInboundResolver(db *sql.DB) *PGLastInboundResolver {
	return &PGLastInboundResolver{db: db}
}

// LastInboundAt returns MAX(created_at) for the (tenant, fanpage, psid)
// triple. Empty / nil-UUID inputs are rejected so callers don't
// accidentally widen the query into a tenant-wide aggregate.
func (r *PGLastInboundResolver) LastInboundAt(ctx context.Context, tenantID uuid.UUID, fanpageID, recipientPSID string) (time.Time, error) {
	if tenantID == uuid.Nil {
		return time.Time{}, errors.New("last_inbound: tenantID required")
	}
	if fanpageID == "" || recipientPSID == "" {
		return time.Time{}, errors.New("last_inbound: fanpageID and recipientPSID required")
	}
	const q = `
		SELECT MAX(created_at)
		  FROM episodic_summaries
		 WHERE tenant_id = $1
		   AND source_id = $2
	`
	sourceID := "fb_backfill:" + fanpageID + ":" + recipientPSID
	var t sql.NullTime
	if err := r.db.QueryRowContext(ctx, q, tenantID, sourceID).Scan(&t); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("last_inbound query: %w", err)
	}
	if !t.Valid {
		return time.Time{}, nil
	}
	return t.Time, nil
}
