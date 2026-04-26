package browser

import (
	"strings"
	"testing"
)

func TestStealthJS_HasCriticalPatches(t *testing.T) {
	// Sanity-check the JS source contains the named overrides. This is a
	// characterization test: if someone edits StealthJS to remove a critical
	// patch, this test fails loudly.
	// We check substrings present in the actual JS source. The properties are
	// defined via Object.defineProperty(navigator, '<name>', ...) so we look
	// for the property-name literals.
	required := []string{
		"'webdriver'",
		"'languages'",
		"'plugins'",
		"WebGLRenderingContext",
		"chrome.runtime",
		"permissions.query",
	}
	for _, needle := range required {
		if !strings.Contains(StealthJS, needle) {
			t.Errorf("StealthJS missing critical patch: %s", needle)
		}
	}
}

func TestStealthJS_NotEmpty(t *testing.T) {
	if len(StealthJS) < 200 {
		t.Errorf("StealthJS suspiciously short: %d bytes", len(StealthJS))
	}
}

func TestApplyStealth_NilPage(t *testing.T) {
	if err := ApplyStealth(nil); err == nil {
		t.Error("expected error for nil page")
	}
}
