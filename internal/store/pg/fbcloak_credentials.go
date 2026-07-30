//go:build !sqliteonly

package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/channels/fbcloak"
	"github.com/nextlevelbuilder/goclaw/internal/crypto"
)

// PGFBCloakCredentialStore implements fbcloak.CredentialStore against
// PostgreSQL. Cookies and proxy URL are stored encrypted via AES-256-GCM
// (internal/crypto). Tenant scoping is enforced in every WHERE clause.
type PGFBCloakCredentialStore struct {
	db     *sql.DB
	encKey string
}

// NewPGFBCloakCredentialStore constructs a store. encKey must be the same
// value used elsewhere (config.Crypto.Key) so existing decryption keeps
// working across services.
func NewPGFBCloakCredentialStore(db *sql.DB, encKey string) *PGFBCloakCredentialStore {
	return &PGFBCloakCredentialStore{db: db, encKey: encKey}
}

// Compile-time interface check.
var _ fbcloak.CredentialStore = (*PGFBCloakCredentialStore)(nil)

func (s *PGFBCloakCredentialStore) Create(ctx context.Context, c fbcloak.Credential) (fbcloak.Credential, error) {
	cookiesEnc, err := crypto.Encrypt(c.Cookies, s.encKey)
	if err != nil {
		return fbcloak.Credential{}, fmt.Errorf("encrypt cookies: %w", err)
	}
	var proxyEnc *string
	if c.ProxyURL != "" {
		v, err := crypto.Encrypt(c.ProxyURL, s.encKey)
		if err != nil {
			return fbcloak.Credential{}, fmt.Errorf("encrypt proxy: %w", err)
		}
		proxyEnc = &v
	}

	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	now := time.Now().UTC()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	if c.Status == "" {
		c.Status = fbcloak.StatusActive
	}
	if c.ViewportW == 0 {
		c.ViewportW = 1366
	}
	if c.ViewportH == 0 {
		c.ViewportH = 768
	}
	if c.Timezone == "" {
		c.Timezone = "Asia/Ho_Chi_Minh"
	}

	const q = `
		INSERT INTO fbcloak_credentials (
			id, tenant_id, fanpage_id, fanpage_name,
			cookies_enc, proxy_url_enc, user_agent,
			viewport_w, viewport_h, timezone, status,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`
	if _, err := s.db.ExecContext(ctx, q,
		c.ID, c.TenantID, c.FanpageID, c.FanpageName,
		cookiesEnc, proxyEnc, c.UserAgent,
		c.ViewportW, c.ViewportH, c.Timezone, string(c.Status),
		c.CreatedAt, c.UpdatedAt,
	); err != nil {
		return fbcloak.Credential{}, fmt.Errorf("insert credential: %w", err)
	}
	c.CookiesEnc = cookiesEnc
	if proxyEnc != nil {
		c.ProxyURLEnc = *proxyEnc
	}
	return c, nil
}

func (s *PGFBCloakCredentialStore) Get(ctx context.Context, tenantID, id uuid.UUID) (fbcloak.Credential, error) {
	const q = `
		SELECT id, tenant_id, fanpage_id, fanpage_name, cookies_enc, proxy_url_enc,
		       user_agent, viewport_w, viewport_h, timezone, status,
		       last_login_at, last_check_at, created_at, updated_at
		  FROM fbcloak_credentials
		 WHERE tenant_id = $1 AND id = $2
	`
	row := s.db.QueryRowContext(ctx, q, tenantID, id)
	return s.scanCredential(row)
}

func (s *PGFBCloakCredentialStore) GetByFanpage(ctx context.Context, tenantID uuid.UUID, fanpageID string) (fbcloak.Credential, error) {
	const q = `
		SELECT id, tenant_id, fanpage_id, fanpage_name, cookies_enc, proxy_url_enc,
		       user_agent, viewport_w, viewport_h, timezone, status,
		       last_login_at, last_check_at, created_at, updated_at
		  FROM fbcloak_credentials
		 WHERE tenant_id = $1 AND fanpage_id = $2
	`
	row := s.db.QueryRowContext(ctx, q, tenantID, fanpageID)
	return s.scanCredential(row)
}

func (s *PGFBCloakCredentialStore) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]fbcloak.Credential, error) {
	const q = `
		SELECT id, tenant_id, fanpage_id, fanpage_name, cookies_enc, proxy_url_enc,
		       user_agent, viewport_w, viewport_h, timezone, status,
		       last_login_at, last_check_at, created_at, updated_at
		  FROM fbcloak_credentials
		 WHERE tenant_id = $1
		 ORDER BY created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, q, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list credentials: %w", err)
	}
	defer rows.Close()

	var out []fbcloak.Credential
	for rows.Next() {
		c, err := s.scanCredentialRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *PGFBCloakCredentialStore) UpdateStatus(ctx context.Context, tenantID, id uuid.UUID, status fbcloak.CredentialStatus) error {
	const q = `
		UPDATE fbcloak_credentials
		   SET status = $3, updated_at = NOW()
		 WHERE tenant_id = $1 AND id = $2
	`
	res, err := s.db.ExecContext(ctx, q, tenantID, id, string(status))
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	return checkAffected(res, fbcloak.ErrCredentialNotFound)
}

func (s *PGFBCloakCredentialStore) UpdateCookies(ctx context.Context, tenantID, id uuid.UUID, cookiesJSON string) error {
	enc, err := crypto.Encrypt(cookiesJSON, s.encKey)
	if err != nil {
		return fmt.Errorf("encrypt cookies: %w", err)
	}
	const q = `
		UPDATE fbcloak_credentials
		   SET cookies_enc = $3, updated_at = NOW()
		 WHERE tenant_id = $1 AND id = $2
	`
	res, err := s.db.ExecContext(ctx, q, tenantID, id, enc)
	if err != nil {
		return fmt.Errorf("update cookies: %w", err)
	}
	return checkAffected(res, fbcloak.ErrCredentialNotFound)
}

func (s *PGFBCloakCredentialStore) UpdateLastCheck(ctx context.Context, tenantID, id uuid.UUID) error {
	const q = `
		UPDATE fbcloak_credentials
		   SET last_check_at = NOW(), updated_at = NOW()
		 WHERE tenant_id = $1 AND id = $2
	`
	res, err := s.db.ExecContext(ctx, q, tenantID, id)
	if err != nil {
		return fmt.Errorf("update last_check: %w", err)
	}
	return checkAffected(res, fbcloak.ErrCredentialNotFound)
}

func (s *PGFBCloakCredentialStore) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	const q = `DELETE FROM fbcloak_credentials WHERE tenant_id = $1 AND id = $2`
	res, err := s.db.ExecContext(ctx, q, tenantID, id)
	if err != nil {
		return fmt.Errorf("delete credential: %w", err)
	}
	return checkAffected(res, fbcloak.ErrCredentialNotFound)
}

// --- helpers ---

// rowScanner abstracts *sql.Row and *sql.Rows so scanCredentialRow can be
// reused for both single-row and iteration paths.
type rowScanner interface {
	Scan(dst ...any) error
}

func (s *PGFBCloakCredentialStore) scanCredential(row *sql.Row) (fbcloak.Credential, error) {
	c, err := s.scanCredentialRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fbcloak.Credential{}, fbcloak.ErrCredentialNotFound
		}
		return fbcloak.Credential{}, err
	}
	return c, nil
}

func (s *PGFBCloakCredentialStore) scanCredentialRow(scanner rowScanner) (fbcloak.Credential, error) {
	var c fbcloak.Credential
	var proxyEnc sql.NullString
	var lastLogin, lastCheck sql.NullTime
	var status string
	if err := scanner.Scan(
		&c.ID, &c.TenantID, &c.FanpageID, &c.FanpageName,
		&c.CookiesEnc, &proxyEnc, &c.UserAgent,
		&c.ViewportW, &c.ViewportH, &c.Timezone, &status,
		&lastLogin, &lastCheck, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return fbcloak.Credential{}, err
	}
	c.Status = fbcloak.CredentialStatus(status)
	if lastLogin.Valid {
		c.LastLoginAt = &lastLogin.Time
	}
	if lastCheck.Valid {
		c.LastCheckAt = &lastCheck.Time
	}
	if proxyEnc.Valid {
		c.ProxyURLEnc = proxyEnc.String
		dec, err := crypto.Decrypt(proxyEnc.String, s.encKey)
		if err != nil {
			return fbcloak.Credential{}, fmt.Errorf("decrypt proxy: %w", err)
		}
		c.ProxyURL = dec
	}
	if c.CookiesEnc != "" {
		dec, err := crypto.Decrypt(c.CookiesEnc, s.encKey)
		if err != nil {
			return fbcloak.Credential{}, fmt.Errorf("decrypt cookies: %w", err)
		}
		c.Cookies = dec
	}
	return c, nil
}

func checkAffected(res sql.Result, notFoundErr error) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return notFoundErr
	}
	return nil
}
