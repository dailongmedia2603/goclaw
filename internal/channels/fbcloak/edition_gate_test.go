//go:build !sqliteonly

package fbcloak

import (
	"testing"

	"github.com/nextlevelbuilder/goclaw/internal/edition"
)

func TestEditionAllowed_Standard(t *testing.T) {
	defer edition.SetCurrent(edition.Current()) // restore
	edition.SetCurrent(edition.Standard)
	if !EditionAllowed() {
		t.Error("expected EditionAllowed() == true on Standard")
	}
}

func TestEditionAllowed_Lite(t *testing.T) {
	defer edition.SetCurrent(edition.Current())
	edition.SetCurrent(edition.Lite)
	if EditionAllowed() {
		t.Error("expected EditionAllowed() == false on Lite")
	}
}

func TestEditionAllowed_CustomFlag(t *testing.T) {
	defer edition.SetCurrent(edition.Current())
	custom := edition.Standard
	custom.FBCloakEnabled = false
	edition.SetCurrent(custom)
	if EditionAllowed() {
		t.Error("expected EditionAllowed() == false when FBCloakEnabled overridden to false")
	}
}
