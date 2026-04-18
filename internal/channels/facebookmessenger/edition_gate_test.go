package facebookmessenger

import (
	"testing"

	"github.com/nextlevelbuilder/goclaw/internal/edition"
)

func TestEditionAllowed_Standard(t *testing.T) {
	edition.SetCurrent(edition.Standard)
	if !EditionAllowed() {
		t.Error("Standard edition should allow facebook_personal")
	}
}

func TestEditionAllowed_Lite(t *testing.T) {
	edition.SetCurrent(edition.Lite)
	if EditionAllowed() {
		t.Error("Lite edition must NOT allow facebook_personal")
	}
	// Reset to not pollute other tests.
	edition.SetCurrent(edition.Standard)
}
