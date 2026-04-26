//go:build !sqliteonly

package fbcloak

import "github.com/nextlevelbuilder/goclaw/internal/edition"

// EditionAllowed reports whether fbcloak is available in the current edition.
//
// fbcloak is gated to Standard for two reasons:
//  1. Lite (desktop, single-tenant) has no proxy / multi-fanpage workflow.
//  2. Each Chrome instance costs ~150MB RAM; >5 fanpages on a desktop is
//     infeasible.
//
// Build tag !sqliteonly already prevents the package from compiling into the
// Lite binary; this function is the runtime guard for the Standard binary
// when an operator manually overrides edition.SetCurrent(Lite) at startup.
func EditionAllowed() bool {
	return edition.Current().FBCloakEnabled
}
