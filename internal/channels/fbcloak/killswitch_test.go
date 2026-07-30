//go:build !sqliteonly

package fbcloak

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestParseKillswitchEnv(t *testing.T) {
	cases := map[string]bool{
		"1":      true,
		"true":   true,
		"TRUE":   true,
		"yes":    true,
		"on":     true,
		" 1 ":    true,
		"":       false,
		"0":      false,
		"false":  false,
		"no":     false,
		"random": false,
	}
	for in, want := range cases {
		if got := parseKillswitchEnv(in); got != want {
			t.Errorf("parseKillswitchEnv(%q): got %v, want %v", in, got, want)
		}
	}
}

func TestNewKillswitchWatcher_RejectsNilTarget(t *testing.T) {
	if _, err := NewKillswitchWatcher(nil, time.Second, nil); err == nil {
		t.Fatal("expected error for nil target")
	}
}

func TestKillswitchWatcher_AppliesInitialEnv(t *testing.T) {
	target := &atomic.Bool{}
	w, err := NewKillswitchWatcher(target, time.Second, nil)
	if err != nil {
		t.Fatalf("NewKillswitchWatcher: %v", err)
	}
	w.getenv = func(string) string { return "1" }

	w.applyOnce()
	if !target.Load() {
		t.Errorf("expected killswitch engaged after applyOnce")
	}

	w.getenv = func(string) string { return "" }
	w.applyOnce()
	if target.Load() {
		t.Errorf("expected killswitch disengaged after env cleared")
	}
}

func TestKillswitchWatcher_LoopReactsToEnvChange(t *testing.T) {
	target := &atomic.Bool{}
	w, _ := NewKillswitchWatcher(target, 10*time.Millisecond, nil)

	envVal := atomic.Pointer[string]{}
	off := ""
	envVal.Store(&off)
	w.getenv = func(string) string {
		if v := envVal.Load(); v != nil {
			return *v
		}
		return ""
	}

	ctx := t.Context()
	w.Start(ctx)
	defer w.Stop()

	// Wait for at least one tick after engaging.
	on := "true"
	envVal.Store(&on)

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if target.Load() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !target.Load() {
		t.Errorf("watcher did not pick up env change within deadline")
	}

	// Disengage.
	envVal.Store(&off)
	deadline = time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if !target.Load() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if target.Load() {
		t.Errorf("watcher did not clear killswitch after env unset")
	}
}

func TestKillswitchWatcher_StopIsIdempotent(t *testing.T) {
	target := &atomic.Bool{}
	w, _ := NewKillswitchWatcher(target, 10*time.Millisecond, nil)
	ctx := t.Context()
	w.Start(ctx)
	w.Stop()
	w.Stop() // should not panic
}
