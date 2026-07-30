//go:build !sqliteonly

package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/nextlevelbuilder/goclaw/internal/channels/fbcloak"
)

// PGFBCloakEpisodicSource bridges fbcloak.EpisodicSource to the PG
// episodic_summaries table that fbbackfill writes. Convention:
//   source_id = "fb_backfill:<page_id>:<conversation_id>"
//   user_id   = psid (string)
type PGFBCloakEpisodicSource struct {
	db *sql.DB
}

func NewPGFBCloakEpisodicSource(db *sql.DB) *PGFBCloakEpisodicSource {
	return &PGFBCloakEpisodicSource{db: db}
}

// Compile-time guard.
var _ fbcloak.EpisodicSource = (*PGFBCloakEpisodicSource)(nil)

// ListByFanpage returns recipients whose latest episodic summary's
// created_at falls in the idle window [now-maxIdle, now-minIdle]. We use
// `created_at` as the proxy for last_inbound_at because fbbackfill writes
// one episodic_summary per conversation snapshot and re-creates a fresh
// row when the conversation gets new content.
func (s *PGFBCloakEpisodicSource) ListByFanpage(ctx context.Context, tenantID uuid.UUID, fanpageID string, minIdle, maxIdle time.Duration, limit int) ([]fbcloak.EpisodicTarget, error) {
	if fanpageID == "" {
		return nil, errors.New("fanpageID required")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	now := time.Now().UTC()
	maxAge := now.Add(-minIdle) // oldest acceptable inbound timestamp (more recent than this = too recent)
	minAge := now.Add(-maxIdle) // newest acceptable inbound timestamp (older than this = out of window)

	// `source_id` pattern: 'fb_backfill:<page_id>:%'
	// Use DISTINCT ON (user_id) to pick the most-recent summary per recipient.
	const q = `
		SELECT DISTINCT ON (user_id)
		       user_id,
		       session_key,
		       summary,
		       turn_count,
		       extract(epoch from created_at)::bigint as version,
		       created_at
		  FROM episodic_summaries
		 WHERE tenant_id = $1
		   AND source_id LIKE $2
		   AND created_at <= $3
		   AND created_at >= $4
		 ORDER BY user_id, created_at DESC
		 LIMIT $5`

	rows, err := s.db.QueryContext(ctx, q,
		tenantID,
		"fb_backfill:"+fanpageID+":%",
		maxAge,
		minAge,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("episodic_source list: %w", err)
	}
	defer rows.Close()

	var out []fbcloak.EpisodicTarget
	for rows.Next() {
		var (
			psid, sessionKey, summary string
			turnCount                 int
			version                   int64
			createdAt                 time.Time
		)
		if err := rows.Scan(&psid, &sessionKey, &summary, &turnCount, &version, &createdAt); err != nil {
			return nil, err
		}
		out = append(out, fbcloak.EpisodicTarget{
			PSID:           psid,
			ConversationID: sessionKey,
			LastInboundAt:  createdAt,
			TurnCount:      turnCount,
			SummaryText:    summary,
			SummaryVersion: int(version % 2147483647), // truncate to int32 range
		})
	}
	return out, rows.Err()
}

// GetByPSID returns the most-recent episodic summary for one recipient.
// Used by Replan worker to refresh context after the customer replied.
func (s *PGFBCloakEpisodicSource) GetByPSID(ctx context.Context, tenantID uuid.UUID, fanpageID, psid string) (fbcloak.EpisodicTarget, error) {
	const q = `
		SELECT user_id, session_key, summary, turn_count,
		       extract(epoch from created_at)::bigint as version, created_at
		  FROM episodic_summaries
		 WHERE tenant_id = $1
		   AND source_id LIKE $2
		   AND user_id = $3
		 ORDER BY created_at DESC
		 LIMIT 1`
	var (
		uid, sessionKey, summary string
		turnCount                int
		version                  int64
		createdAt                time.Time
	)
	err := s.db.QueryRowContext(ctx, q,
		tenantID,
		"fb_backfill:"+fanpageID+":%",
		psid,
	).Scan(&uid, &sessionKey, &summary, &turnCount, &version, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fbcloak.EpisodicTarget{}, fmt.Errorf("no episodic summary for psid %s", psid)
		}
		// pq: relation might not exist on older deploys — surface clearly.
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "42P01" {
			return fbcloak.EpisodicTarget{}, errors.New("episodic_summaries table not provisioned")
		}
		return fbcloak.EpisodicTarget{}, err
	}
	return fbcloak.EpisodicTarget{
		PSID:           uid,
		ConversationID: sessionKey,
		LastInboundAt:  createdAt,
		TurnCount:      turnCount,
		SummaryText:    summary,
		SummaryVersion: int(version % 2147483647),
	}, nil
}
