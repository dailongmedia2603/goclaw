//go:build !sqliteonly

package pg

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nextlevelbuilder/goclaw/internal/channels/fbcloak"
	"github.com/nextlevelbuilder/goclaw/internal/crypto"
)

// CredentialActiveLister adapter: wraps a PGFBCloakCredentialStore and
// exposes ListAllActive. Cross-tenant scan — used by Plan Generator's
// scheduled tick to enumerate every fanpage with an active credential.
//
// Defined as a separate file so the adapter is co-located with the SQL
// query but doesn't pollute the existing CredentialStore interface
// (which is tenant-scoped by design).
type FBCloakActiveCredsLister struct {
	db     *sql.DB
	encKey string
}

func NewFBCloakActiveCredsLister(db *sql.DB, encKey string) *FBCloakActiveCredsLister {
	return &FBCloakActiveCredsLister{db: db, encKey: encKey}
}

// Compile-time guard.
var _ fbcloak.CredentialActiveLister = (*FBCloakActiveCredsLister)(nil)

// ListAllActive returns every credential with status='active' across all
// tenants. Caller (Generator) loops these and processes one at a time
// under the per-credential mutex.
func (l *FBCloakActiveCredsLister) ListAllActive(ctx context.Context) ([]fbcloak.Credential, error) {
	const q = `
		SELECT id, tenant_id, fanpage_id, fanpage_name, cookies_enc, proxy_url_enc,
		       user_agent, viewport_w, viewport_h, timezone, status,
		       last_login_at, last_check_at, created_at, updated_at
		  FROM fbcloak_credentials
		 WHERE status = 'active'
		 ORDER BY tenant_id, id`
	rows, err := l.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list active credentials: %w", err)
	}
	defer rows.Close()

	var out []fbcloak.Credential
	for rows.Next() {
		c, err := scanActiveCredential(rows, l.encKey)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// scanActiveCredential mirrors PGFBCloakCredentialStore.scanCredentialRow
// without depending on the unexported helper. Only the fields Generator
// needs are decrypted (cookies); proxy stays encrypted because Generator
// doesn't open browser sessions itself.
func scanActiveCredential(rows *sql.Rows, encKey string) (fbcloak.Credential, error) {
	var c fbcloak.Credential
	var proxyEnc sql.NullString
	var lastLogin, lastCheck sql.NullTime
	var status string
	if err := rows.Scan(
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
	}
	if c.CookiesEnc != "" && encKey != "" {
		dec, err := crypto.Decrypt(c.CookiesEnc, encKey)
		if err == nil {
			c.Cookies = dec
		}
	}
	return c, nil
}
