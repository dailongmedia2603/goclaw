//go:build !sqliteonly

package fbcloak

import (
	"testing"
	"time"
)

func TestEscapeLike(t *testing.T) {
	cases := map[string]string{
		"100":     "100",
		"50%":     `50\%`,
		"a_b":     `a\_b`,
		`a\b`:     `a\\b`,
		`%_\`:     `\%\_\\`,
	}
	for in, want := range cases {
		if got := escapeLike(in); got != want {
			t.Errorf("escapeLike(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSortTargetsOldestFirst(t *testing.T) {
	now := time.Now()
	ts := []Target{
		{RecipientPSID: "B", LastMessageAt: now.Add(-1 * time.Hour)},
		{RecipientPSID: "A", LastMessageAt: now.Add(-3 * time.Hour)},
		{RecipientPSID: "C", LastMessageAt: now.Add(-2 * time.Hour)},
	}
	sortTargetsOldestFirst(ts)
	want := []string{"A", "C", "B"}
	for i, target := range ts {
		if target.RecipientPSID != want[i] {
			t.Errorf("position %d: got %s, want %s (full: %+v)", i, target.RecipientPSID, want[i], ts)
		}
	}
}

func TestResolver_NilDB(t *testing.T) {
	r := &Resolver{}
	_, err := r.Resolve(t.Context(), [16]byte{1}, ResolveOpts{PageID: "1", MinIdle: time.Hour, MaxIdle: 2 * time.Hour})
	if err == nil {
		t.Error("expected error for nil DB")
	}
}

func TestResolver_RejectsBadOpts(t *testing.T) {
	r := &Resolver{DB: nil}
	cases := []struct {
		name string
		opts ResolveOpts
	}{
		{"missing page", ResolveOpts{MinIdle: time.Hour, MaxIdle: 2 * time.Hour}},
		{"window inverted", ResolveOpts{PageID: "1", MinIdle: 2 * time.Hour, MaxIdle: time.Hour}},
		{"min zero", ResolveOpts{PageID: "1", MinIdle: 0, MaxIdle: time.Hour}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.Resolve(t.Context(), [16]byte{1}, tc.opts)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}
