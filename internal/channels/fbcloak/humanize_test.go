//go:build !sqliteonly

package fbcloak

import (
	"testing"
	"time"
)

func TestHumanizer_TypingDelay_Bounds(t *testing.T) {
	h := NewHumanizer(42, HumanizeConfig{
		TypingMinMS:           80,
		TypingMaxMS:           180,
		HesitationProbability: 0, // disable hesitation for bounds check
	})
	for range 200 {
		d := h.TypingDelay('a')
		if d < 80*time.Millisecond || d > 180*time.Millisecond {
			t.Fatalf("typing delay out of base range: got %s", d)
		}
	}
}

func TestHumanizer_TypingDelay_SpaceExtra(t *testing.T) {
	h := NewHumanizer(42, HumanizeConfig{TypingMinMS: 100, TypingMaxMS: 100, SpacePauseExtraMS: 50})
	d := h.TypingDelay(' ')
	if d != 150*time.Millisecond {
		t.Errorf("expected exactly 150ms (100 base + 50 space extra), got %s", d)
	}
}

func TestHumanizer_TypingDelay_DotExtra(t *testing.T) {
	h := NewHumanizer(42, HumanizeConfig{TypingMinMS: 100, TypingMaxMS: 100, DotPauseExtra: 200})
	if d := h.TypingDelay('.'); d != 300*time.Millisecond {
		t.Errorf("expected 300ms after dot, got %s", d)
	}
	if d := h.TypingDelay('!'); d != 300*time.Millisecond {
		t.Errorf("expected 300ms after !, got %s", d)
	}
	if d := h.TypingDelay('?'); d != 300*time.Millisecond {
		t.Errorf("expected 300ms after ?, got %s", d)
	}
}

func TestHumanizer_PreActionDelay_Bounds(t *testing.T) {
	h := NewHumanizer(7, HumanizeConfig{PreActionMin: time.Second, PreActionMax: 5 * time.Second})
	for range 100 {
		d := h.PreActionDelay()
		if d < time.Second || d > 5*time.Second {
			t.Fatalf("pre-action delay out of range: %s", d)
		}
	}
}

func TestHumanizer_DeterministicWithSeed(t *testing.T) {
	h1 := NewHumanizer(42, HumanizeConfig{TypingMinMS: 80, TypingMaxMS: 180})
	h2 := NewHumanizer(42, HumanizeConfig{TypingMinMS: 80, TypingMaxMS: 180})
	for range 10 {
		if h1.TypingDelay('a') != h2.TypingDelay('a') {
			t.Fatal("seeded RNG should produce identical sequences")
		}
	}
}

func TestHumanizer_IsWithinWorkingHours(t *testing.T) {
	h := NewHumanizer(0, HumanizeConfig{
		WorkingHours: WorkingHours{Start: "08:00", End: "21:00", TZ: "UTC"},
	})
	cases := []struct {
		name string
		when time.Time
		want bool
	}{
		{"morning ok", time.Date(2026, 4, 26, 9, 0, 0, 0, time.UTC), true},
		{"evening edge", time.Date(2026, 4, 26, 21, 0, 0, 0, time.UTC), true},
		{"after hours", time.Date(2026, 4, 26, 22, 0, 0, 0, time.UTC), false},
		{"early morning", time.Date(2026, 4, 26, 7, 59, 0, 0, time.UTC), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := h.IsWithinWorkingHours(tc.when); got != tc.want {
				t.Errorf("IsWithinWorkingHours(%s) = %v, want %v", tc.when, got, tc.want)
			}
		})
	}
}

func TestHumanizer_IsWithinWorkingHours_AlwaysOnWhenZero(t *testing.T) {
	h := NewHumanizer(0, HumanizeConfig{}) // zero WorkingHours
	if !h.IsWithinWorkingHours(time.Now()) {
		t.Error("zero WorkingHours should mean always-on")
	}
}

func TestHumanizer_BezierPath(t *testing.T) {
	h := NewHumanizer(42, HumanizeConfig{BezierSteps: 30, BezierJitterPx: 1.0})
	path := h.BezierPath(10, 10, 100, 100)
	if len(path) != 31 {
		t.Errorf("expected 31 points (steps+1), got %d", len(path))
	}
	// Path should start near source and end near target (within jitter).
	if absInt(path[0][0]-10) > 3 || absInt(path[0][1]-10) > 3 {
		t.Errorf("path start too far from source: %v", path[0])
	}
	if absInt(path[len(path)-1][0]-100) > 3 || absInt(path[len(path)-1][1]-100) > 3 {
		t.Errorf("path end too far from target: %v", path[len(path)-1])
	}
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func TestParseHHMM(t *testing.T) {
	cases := []struct {
		in     string
		h, m   int
		wantOK bool
	}{
		{"08:00", 8, 0, true},
		{"23:59", 23, 59, true},
		{"00:00", 0, 0, true},
		{"24:00", 0, 0, false},
		{"12:60", 0, 0, false},
		{"abc", 0, 0, false},
		{"", 0, 0, false},
	}
	for _, tc := range cases {
		hh, mm, ok := parseHHMM(tc.in)
		if ok != tc.wantOK {
			t.Errorf("parseHHMM(%q) ok=%v, want %v", tc.in, ok, tc.wantOK)
			continue
		}
		if ok && (hh != tc.h || mm != tc.m) {
			t.Errorf("parseHHMM(%q) = %d:%d, want %d:%d", tc.in, hh, mm, tc.h, tc.m)
		}
	}
}
