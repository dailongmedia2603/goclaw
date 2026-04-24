//go:build sqliteonly

package fbbackfill

import "context"

// Deps is a stub for the sqliteonly (Lite desktop) build. Lite does not
// support channels, so Facebook backfill is unreachable and the
// dependency shape is not exercised.
type Deps struct{}

// Register is a no-op on sqliteonly builds. Lite edition has no channels.
func Register(_ context.Context, _ *Deps) error { return nil }
