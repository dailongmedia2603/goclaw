//go:build !sqliteonly

package fbcloak

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// KillswitchEnvVar names the env variable the watcher polls. Setting it
// to a truthy value ("1", "true", "yes") engages the killswitch on the
// next tick — every Service entry guard then short-circuits with
// ErrKillswitchActive.
const KillswitchEnvVar = "GOCLAW_FBCLOAK_KILLSWITCH"

// DefaultKillswitchPoll is the fallback interval when config leaves it
// at zero. 30s balances responsiveness against `os.Getenv` syscall cost.
const DefaultKillswitchPoll = 30 * time.Second

// KillswitchWatcher polls the env var and mirrors its value into a
// shared *atomic.Bool. Designed to be wired alongside Service so the
// guard() check picks up the new state without a process restart.
type KillswitchWatcher struct {
	Target   *atomic.Bool
	Interval time.Duration
	Logger   *slog.Logger

	getenv func(string) string // overridable in tests
	stopCh chan struct{}
}

// NewKillswitchWatcher constructs a watcher targeting the given atomic
// flag. Interval ≤ 0 falls back to DefaultKillswitchPoll. A nil logger
// silently uses slog.Default(). Returns an error when target is nil so
// callers can't accidentally watch a phantom flag.
func NewKillswitchWatcher(target *atomic.Bool, interval time.Duration, logger *slog.Logger) (*KillswitchWatcher, error) {
	if target == nil {
		return nil, errKillswitchNilTarget
	}
	if interval <= 0 {
		interval = DefaultKillswitchPoll
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &KillswitchWatcher{
		Target:   target,
		Interval: interval,
		Logger:   logger,
		getenv:   os.Getenv,
		stopCh:   make(chan struct{}),
	}, nil
}

// Start launches the polling goroutine and returns immediately. The
// initial value is read synchronously so the killswitch reflects the env
// state by the time Start returns. Idempotent — a second call before
// Stop is a no-op (caller's responsibility, not enforced here).
func (w *KillswitchWatcher) Start(ctx context.Context) {
	w.applyOnce()
	go w.loop(ctx)
}

// Stop signals the loop to exit. Safe to call multiple times — the
// channel close is wrapped in a recover; subsequent reads are harmless.
func (w *KillswitchWatcher) Stop() {
	defer func() { _ = recover() }()
	close(w.stopCh)
}

// applyOnce reads the env once and updates the atomic flag. Logs
// transitions only — repeated reads with no change are silent.
func (w *KillswitchWatcher) applyOnce() {
	want := parseKillswitchEnv(w.getenv(KillswitchEnvVar))
	prev := w.Target.Swap(want)
	if prev != want {
		w.Logger.Warn("security.fbcloak.killswitch_changed",
			"engaged", want, "source", "env_watcher",
		)
	}
}

func (w *KillswitchWatcher) loop(ctx context.Context) {
	t := time.NewTicker(w.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-t.C:
			w.applyOnce()
		}
	}
}

// parseKillswitchEnv returns true when the raw env value is one of the
// recognised "on" tokens. Any unrecognised value (including empty)
// disengages — fail-open here is intentional: a typo'd env should not
// silently disable the feature for everyone.
func parseKillswitchEnv(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// errKillswitchNilTarget is package-internal so tests can assert against
// it without leaking the symbol.
var errKillswitchNilTarget = errStr("killswitch: nil target")

type errStr string

func (e errStr) Error() string { return string(e) }
