//go:build !sqliteonly

package fbcloak

import (
	"errors"
	"testing"
	"time"
)

func TestParseTier1AX(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		ax     string
		expect time.Duration // ago
	}{
		{"3 days", "Jane Doe • 3 ngày", 3 * 24 * time.Hour},
		{"5 hours", "John • 5 giờ", 5 * time.Hour},
		{"30 minutes", "X • 30 phút", 30 * time.Minute},
		{"2 weeks", "Y • 2 tuần", 14 * 24 * time.Hour},
		{"1 month", "Z • 1 tháng", 30 * 24 * time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseTier1AX(tc.ax, now)
			if err != nil {
				t.Fatal(err)
			}
			want := now.Add(-tc.expect)
			if !got.At.Equal(want) {
				t.Errorf("got %s, want %s", got.At, want)
			}
			if got.Source != "ax" {
				t.Errorf("source: got %s, want ax", got.Source)
			}
			if got.Confidence != 1.0 {
				t.Errorf("confidence: got %v, want 1.0", got.Confidence)
			}
		})
	}
}

func TestParseTier1AX_Empty(t *testing.T) {
	_, err := ParseTier1AX("", time.Now())
	if !errors.Is(err, ErrTimestampUnparsable) {
		t.Errorf("expected ErrTimestampUnparsable, got %v", err)
	}
}

func TestParseTier2React_HappyPath(t *testing.T) {
	now := time.Now()
	dump := `{"props":{"name":"Jane","lastActivityTimestampMs":1712345678901,"foo":"bar"}}`
	got, err := ParseTier2React(dump, now)
	if err != nil {
		t.Fatal(err)
	}
	want := time.UnixMilli(1712345678901).UTC()
	if !got.At.Equal(want) {
		t.Errorf("got %s, want %s", got.At, want)
	}
	if got.Source != "react" {
		t.Errorf("source: got %s, want react", got.Source)
	}
}

func TestParseTier2React_KeyMissing(t *testing.T) {
	_, err := ParseTier2React(`{"foo":"bar"}`, time.Now())
	if !errors.Is(err, ErrTimestampUnparsable) {
		t.Errorf("expected ErrTimestampUnparsable, got %v", err)
	}
}

func TestParseTier3Regex(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	got, err := ParseTier3Regex("Last message: Hello! • 7 ngày", now)
	if err != nil {
		t.Fatal(err)
	}
	want := now.Add(-7 * 24 * time.Hour)
	if !got.At.Equal(want) {
		t.Errorf("got %s, want %s", got.At, want)
	}
	if got.Confidence != 0.6 {
		t.Errorf("confidence: got %v, want 0.6 for tier3", got.Confidence)
	}
}

func TestParseLastActivity_FallbackChain(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

	// Tier 1 wins.
	got, err := ParseLastActivity("• 5 giờ", "", "", now)
	if err != nil || got.Source != "ax" {
		t.Fatalf("tier1 should win: %+v err=%v", got, err)
	}

	// Tier 1 fails (empty), Tier 2 wins.
	dump := `{"lastActivityTimestampMs":1712345678901}`
	got, err = ParseLastActivity("", dump, "", now)
	if err != nil || got.Source != "react" {
		t.Fatalf("tier2 should win: %+v err=%v", got, err)
	}

	// Tier 1 + 2 fail, Tier 3 wins.
	got, err = ParseLastActivity("", "", "lorem 2 tuần ipsum", now)
	if err != nil || got.Source != "regex" {
		t.Fatalf("tier3 should win: %+v err=%v", got, err)
	}

	// All fail.
	_, err = ParseLastActivity("", "", "", now)
	if !errors.Is(err, ErrTimestampUnparsable) {
		t.Errorf("expected ErrTimestampUnparsable, got %v", err)
	}
}

func TestParseRelativeVN_UnknownUnit(t *testing.T) {
	_, ok := parseRelativeVN("3 fortnights", time.Now())
	if ok {
		t.Error("unknown unit should not parse")
	}
}
