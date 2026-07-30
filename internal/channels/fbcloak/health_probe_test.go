//go:build !sqliteonly

package fbcloak

import "testing"

func TestClassifyMeURL(t *testing.T) {
	cases := []struct {
		name     string
		url      string
		wantStat CredentialStatus
	}{
		{"logged in profile", "https://www.facebook.com/profile.php?id=100", StatusActive},
		{"vanity logged in", "https://www.facebook.com/zuck", StatusActive},
		{"redirected login", "https://www.facebook.com/login/?next=...", StatusExpired},
		{"redirected checkpoint", "https://www.facebook.com/checkpoint/?next=", StatusCheckpoint},
		{"blank", "", StatusExpired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyMeURL(tc.url)
			if got.Status != tc.wantStat {
				t.Errorf("classifyMeURL(%q).Status = %s, want %s", tc.url, got.Status, tc.wantStat)
			}
		})
	}
}

func TestProbe_NoBrowserConfigured(t *testing.T) {
	h := &HealthProbe{}
	_, err := h.Run(t.Context(), Credential{})
	if err == nil {
		t.Error("expected error when NewBrowser nil")
	}
}
