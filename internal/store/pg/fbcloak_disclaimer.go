//go:build !sqliteonly

package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/channels/fbcloak"
)

// PGFBCloakDisclaimerStore implements fbcloak.DisclaimerStore against
// the fbcloak_disclaimer_ack table introduced in migration 000057.
type PGFBCloakDisclaimerStore struct {
	db *sql.DB
}

func NewPGFBCloakDisclaimerStore(db *sql.DB) *PGFBCloakDisclaimerStore {
	return &PGFBCloakDisclaimerStore{db: db}
}

var _ fbcloak.DisclaimerStore = (*PGFBCloakDisclaimerStore)(nil)

// Ack upserts an acknowledgement row for (tenant_id, version). userID is
// nullable — admins ack on behalf of the tenant; we record who clicked
// "I agree" purely for transparency in the UI and audit.
func (s *PGFBCloakDisclaimerStore) Ack(ctx context.Context, tenantID uuid.UUID, version string, userID *uuid.UUID) error {
	const q = `
		INSERT INTO fbcloak_disclaimer_ack (tenant_id, version, user_id, acked_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (tenant_id, version) DO UPDATE SET
		    user_id  = EXCLUDED.user_id,
		    acked_at = EXCLUDED.acked_at
	`
	if _, err := s.db.ExecContext(ctx, q, tenantID, version, userID); err != nil {
		return fmt.Errorf("ack disclaimer: %w", err)
	}
	return nil
}

// GetAtVersion returns the row matching (tenant_id, version) or nil when
// not acked. Errors other than ErrNoRows are surfaced.
func (s *PGFBCloakDisclaimerStore) GetAtVersion(ctx context.Context, tenantID uuid.UUID, version string) (*fbcloak.DisclaimerAck, error) {
	const q = `
		SELECT tenant_id, version, user_id, acked_at
		  FROM fbcloak_disclaimer_ack
		 WHERE tenant_id = $1 AND version = $2
	`
	row := s.db.QueryRowContext(ctx, q, tenantID, version)
	a, err := scanDisclaimerAck(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get disclaimer at version: %w", err)
	}
	return &a, nil
}

// GetLatest returns the most recent ack regardless of version. Used by the
// UI to render "acked at X by user Y" for transparency. nil when never
// acked.
func (s *PGFBCloakDisclaimerStore) GetLatest(ctx context.Context, tenantID uuid.UUID) (*fbcloak.DisclaimerAck, error) {
	const q = `
		SELECT tenant_id, version, user_id, acked_at
		  FROM fbcloak_disclaimer_ack
		 WHERE tenant_id = $1
		 ORDER BY acked_at DESC
		 LIMIT 1
	`
	row := s.db.QueryRowContext(ctx, q, tenantID)
	a, err := scanDisclaimerAck(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest disclaimer: %w", err)
	}
	return &a, nil
}

func scanDisclaimerAck(scanner rowScanner) (fbcloak.DisclaimerAck, error) {
	var a fbcloak.DisclaimerAck
	var userID uuid.NullUUID
	if err := scanner.Scan(&a.TenantID, &a.Version, &userID, &a.AckedAt); err != nil {
		return fbcloak.DisclaimerAck{}, err
	}
	if userID.Valid {
		uid := userID.UUID
		a.UserID = &uid
	}
	return a, nil
}
