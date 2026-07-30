//go:build sqliteonly

package fbbackfill

import "context"

// Register is a no-op on sqliteonly builds. Lite edition has no channels,
// so Facebook backfill is unreachable. The Deps struct is defined in
// deps.go (no build tag) so the caller's code shape is identical across
// builds.
func Register(_ context.Context, _ *Deps) error { return nil }
