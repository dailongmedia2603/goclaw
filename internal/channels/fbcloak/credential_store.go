//go:build !sqliteonly

package fbcloak

import (
	"context"

	"github.com/google/uuid"
)

// CredentialStore is the persistence contract for fbcloak_credentials. The
// store is responsible for transparent encryption/decryption of cookies and
// proxy URLs (callers see/give plaintext).
type CredentialStore interface {
	Create(ctx context.Context, c Credential) (Credential, error)
	Get(ctx context.Context, tenantID, id uuid.UUID) (Credential, error)
	GetByFanpage(ctx context.Context, tenantID uuid.UUID, fanpageID string) (Credential, error)
	ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]Credential, error)
	UpdateStatus(ctx context.Context, tenantID, id uuid.UUID, status CredentialStatus) error
	UpdateCookies(ctx context.Context, tenantID, id uuid.UUID, cookiesJSON string) error
	UpdateLastCheck(ctx context.Context, tenantID, id uuid.UUID) error
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}
