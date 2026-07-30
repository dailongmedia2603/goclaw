package facebookmessenger

import "github.com/nextlevelbuilder/goclaw/internal/edition"

// EditionAllowed reports whether the facebook_personal channel is available
// in the currently configured edition.
//
// We explicitly block this channel on Lite edition because:
//  1. The sidecar model (separate process + Docker) is incompatible with
//     the single-binary desktop-app spirit of Lite.
//  2. Lite targets non-technical self-hosters who should not be running an
//     AGPL sidecar with ban-risk implications without explicit opt-in.
//
// Called by the gateway wiring to decide whether to register the factory,
// and by the UI (via /edition endpoint) to decide whether to show the
// dropdown entry.
func EditionAllowed() bool {
	return edition.Current().Name != "lite"
}
