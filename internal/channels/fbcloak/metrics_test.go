//go:build !sqliteonly

package fbcloak

import (
	"sync"
	"testing"
)

func TestMetrics_Increments(t *testing.T) {
	ResetMetrics()
	defer ResetMetrics()

	IncSendsAttempted()
	IncSendsAttempted()
	IncSendsSucceeded()
	IncSendsDryRun()
	IncSendsSkipped()
	IncSendsFailed()
	IncCheckpoint()
	IncCookieExpired()
	IncKillswitchAbort()
	IncJobRunsTotal()
	IncJobRunsKilled()
	IncScreenshotError()

	got := Metrics()
	want := map[string]int64{
		"fbcloak_sends_attempted_total":   2,
		"fbcloak_sends_succeeded_total":   1,
		"fbcloak_sends_dry_run_total":     1,
		"fbcloak_sends_skipped_total":     1,
		"fbcloak_sends_failed_total":      1,
		"fbcloak_checkpoint_total":        1,
		"fbcloak_cookie_expired_total":    1,
		"fbcloak_killswitch_aborts":       1,
		"fbcloak_job_runs_total":          1,
		"fbcloak_job_runs_killed_total":   1,
		"fbcloak_active_workers":          0,
		"fbcloak_screenshot_errors_total": 1,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("metric %s: got %d, want %d", k, got[k], v)
		}
	}
}

func TestMetrics_IncSendStatus(t *testing.T) {
	ResetMetrics()
	defer ResetMetrics()

	IncSendStatus(SendStatusSent)
	IncSendStatus(SendStatusDryRun)
	IncSendStatus(SendStatusSkipped)
	IncSendStatus(SendStatusFailed)
	IncSendStatus(SendStatus("unknown")) // ignored

	got := Metrics()
	if got["fbcloak_sends_succeeded_total"] != 1 {
		t.Errorf("sent: got %d, want 1", got["fbcloak_sends_succeeded_total"])
	}
	if got["fbcloak_sends_dry_run_total"] != 1 {
		t.Errorf("dry_run: got %d, want 1", got["fbcloak_sends_dry_run_total"])
	}
	if got["fbcloak_sends_skipped_total"] != 1 {
		t.Errorf("skipped: got %d, want 1", got["fbcloak_sends_skipped_total"])
	}
	if got["fbcloak_sends_failed_total"] != 1 {
		t.Errorf("failed: got %d, want 1", got["fbcloak_sends_failed_total"])
	}
}

func TestMetrics_ActiveWorkerGauge(t *testing.T) {
	ResetMetrics()
	defer ResetMetrics()

	IncActiveWorker()
	IncActiveWorker()
	IncActiveWorker()
	if got := Metrics()["fbcloak_active_workers"]; got != 3 {
		t.Errorf("after 3 inc: got %d, want 3", got)
	}
	DecActiveWorker()
	DecActiveWorker()
	if got := Metrics()["fbcloak_active_workers"]; got != 1 {
		t.Errorf("after 2 dec: got %d, want 1", got)
	}
	DecActiveWorker()
	if got := Metrics()["fbcloak_active_workers"]; got != 0 {
		t.Errorf("after 3rd dec: got %d, want 0", got)
	}
}

func TestMetrics_ConcurrentSafe(t *testing.T) {
	ResetMetrics()
	defer ResetMetrics()

	const goroutines = 50
	const each = 100
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			for range each {
				IncSendsAttempted()
				IncActiveWorker()
				DecActiveWorker()
			}
		})
	}
	wg.Wait()

	got := Metrics()
	if got["fbcloak_sends_attempted_total"] != int64(goroutines*each) {
		t.Errorf("sends_attempted: got %d, want %d", got["fbcloak_sends_attempted_total"], goroutines*each)
	}
	if got["fbcloak_active_workers"] != 0 {
		t.Errorf("active_workers: got %d, want 0", got["fbcloak_active_workers"])
	}
}
