//go:build !sqliteonly

package fbcloak

import (
	"context"
	"errors"
)

// InboxScanner is the FALLBACK target source — used only when the database
// resolver returns zero targets and the operator opts in via
// fbcloak_jobs.use_scanner_fallback. Phase 2 ships the interface + a stub
// "not implemented" impl; Phase 3 wires the rod-backed scanner that traverses
// Meta Business Suite listitems and parses timestamps via the 3-tier strategy.
type InboxScanner interface {
	Scan(ctx context.Context, fanpageID string, opts ScanOpts) ([]Target, error)
}

// ScanOpts narrows the scan to the desired idle window and caps the number
// of conversations returned (hard cap 200 in implementations).
type ScanOpts struct {
	MaxConversations int
	MinIdle, MaxIdle int64 // seconds; runtime converts from time.Duration to keep wire shape simple
}

// ErrScannerUnavailable is returned by the stub scanner so the runner can
// fall through cleanly when no real implementation is wired.
var ErrScannerUnavailable = errors.New("fbcloak: inbox scanner not available in this build")

// StubScanner is the no-op default. Always returns ErrScannerUnavailable.
type StubScanner struct{}

// Compile-time guard.
var _ InboxScanner = (*StubScanner)(nil)

// Scan returns ErrScannerUnavailable.
func (StubScanner) Scan(_ context.Context, _ string, _ ScanOpts) ([]Target, error) {
	return nil, ErrScannerUnavailable
}
