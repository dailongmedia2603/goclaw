//go:build !sqliteonly

package fbcloak

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// CurrentDisclaimerVersion is the version any tenant must ack to enable a
// job. Bumping this string on a content change forces re-ack on next
// disclaimer_status query — see docs/fbcloak-tos-disclaimer.md for policy.
const CurrentDisclaimerVersion = "v1.0"

// DisclaimerAck is one row in fbcloak_disclaimer_ack.
type DisclaimerAck struct {
	TenantID uuid.UUID  `db:"tenant_id" json:"tenantId"`
	Version  string     `db:"version" json:"version"`
	UserID   *uuid.UUID `db:"user_id" json:"userId,omitempty"`
	AckedAt  time.Time  `db:"acked_at" json:"ackedAt"`
}

// DisclaimerStatus is the response to fbcloak.disclaimer.status. Required
// is true when the tenant has not yet acked CurrentVersion (or never acked
// any version). UI uses Required to gate the modal display.
type DisclaimerStatus struct {
	CurrentVersion string         `json:"currentVersion"`
	Required       bool           `json:"required"`
	Latest         *DisclaimerAck `json:"latest,omitempty"`
}

// DisclaimerStore is the persistence contract for disclaimer acks. Both
// methods are tenant-scoped — Get returns the most recent ack for any
// version (used for `latest` display); GetAtVersion returns nil when the
// tenant has not acked the requested version (used for the toggle gate).
type DisclaimerStore interface {
	Ack(ctx context.Context, tenantID uuid.UUID, version string, userID *uuid.UUID) error
	GetAtVersion(ctx context.Context, tenantID uuid.UUID, version string) (*DisclaimerAck, error)
	GetLatest(ctx context.Context, tenantID uuid.UUID) (*DisclaimerAck, error)
}
