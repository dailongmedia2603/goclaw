//go:build !sqliteonly

package fbcloak

import "sync/atomic"

// Process-wide counters surfaced via Metrics() for the gateway /status
// endpoint. No external Prometheus dependency — atomic.Int64 is enough for
// single-binary deployments. Names follow Prometheus convention (snake_case
// + _total suffix) so a future scrape adapter can map them 1:1.
//
// Cardinality intentionally low: no per-tenant or per-fanpage labels. If
// per-tenant attribution is needed, the operator scrapes /status from a
// process running one tenant; multi-tenant breakdown belongs in the SQL
// `fbcloak_send_log` audit table, not in counters.
var (
	metricSendsAttempted   atomic.Int64 // every Execute() call that passes guard
	metricSendsSucceeded   atomic.Int64 // status=sent
	metricSendsDryRun      atomic.Int64 // status=dry_run
	metricSendsSkipped     atomic.Int64 // status=skipped (policy / verify)
	metricSendsFailed      atomic.Int64 // status=failed
	metricCheckpointTotal  atomic.Int64 // checkpoint detector triggered
	metricCookieExpired    atomic.Int64 // credential.status flipped to expired mid-run
	metricKillswitchAborts atomic.Int64 // job aborts because killswitch flipped on
	metricJobRunsTotal     atomic.Int64 // every RunOnce() entry
	metricJobRunsKilled    atomic.Int64 // RunOnce returned JobStatusKilled
	metricActiveWorkers    atomic.Int64 // gauge: current in-flight job runs
	metricScreenshotErrors atomic.Int64 // failed to capture pre/post screenshot
)

// Counter mutators. Exported so wiring in send_executor / job_runner /
// checkpoint_detector can call them without exposing the underlying vars.
func IncSendsAttempted()   { metricSendsAttempted.Add(1) }
func IncSendsSucceeded()   { metricSendsSucceeded.Add(1) }
func IncSendsDryRun()      { metricSendsDryRun.Add(1) }
func IncSendsSkipped()     { metricSendsSkipped.Add(1) }
func IncSendsFailed()      { metricSendsFailed.Add(1) }
func IncCheckpoint()       { metricCheckpointTotal.Add(1) }
func IncCookieExpired()    { metricCookieExpired.Add(1) }
func IncKillswitchAbort()  { metricKillswitchAborts.Add(1) }
func IncJobRunsTotal()     { metricJobRunsTotal.Add(1) }
func IncJobRunsKilled()    { metricJobRunsKilled.Add(1) }
func IncScreenshotError()  { metricScreenshotErrors.Add(1) }

// Active worker gauge — paired Inc/Dec around the in-flight critical
// section. Underflow is impossible if every Inc has a deferred Dec.
func IncActiveWorker() { metricActiveWorkers.Add(1) }
func DecActiveWorker() { metricActiveWorkers.Add(-1) }

// IncSendStatus is a convenience that maps a SendStatus value to the
// matching success/failure counter. Call sites in send_executor only have
// the typed status — keeps wiring readable.
func IncSendStatus(s SendStatus) {
	switch s {
	case SendStatusSent:
		metricSendsSucceeded.Add(1)
	case SendStatusDryRun:
		metricSendsDryRun.Add(1)
	case SendStatusSkipped:
		metricSendsSkipped.Add(1)
	case SendStatusFailed:
		metricSendsFailed.Add(1)
	}
}

// Metrics returns a read-only snapshot of the current counter values.
// Safe to call concurrently.
func Metrics() map[string]int64 {
	return map[string]int64{
		"fbcloak_sends_attempted_total":  metricSendsAttempted.Load(),
		"fbcloak_sends_succeeded_total":  metricSendsSucceeded.Load(),
		"fbcloak_sends_dry_run_total":    metricSendsDryRun.Load(),
		"fbcloak_sends_skipped_total":    metricSendsSkipped.Load(),
		"fbcloak_sends_failed_total":     metricSendsFailed.Load(),
		"fbcloak_checkpoint_total":       metricCheckpointTotal.Load(),
		"fbcloak_cookie_expired_total":   metricCookieExpired.Load(),
		"fbcloak_killswitch_aborts":      metricKillswitchAborts.Load(),
		"fbcloak_job_runs_total":         metricJobRunsTotal.Load(),
		"fbcloak_job_runs_killed_total":  metricJobRunsKilled.Load(),
		"fbcloak_active_workers":         metricActiveWorkers.Load(),
		"fbcloak_screenshot_errors_total": metricScreenshotErrors.Load(),
	}
}

// ResetMetrics zeroes all counters. For tests only.
func ResetMetrics() {
	metricSendsAttempted.Store(0)
	metricSendsSucceeded.Store(0)
	metricSendsDryRun.Store(0)
	metricSendsSkipped.Store(0)
	metricSendsFailed.Store(0)
	metricCheckpointTotal.Store(0)
	metricCookieExpired.Store(0)
	metricKillswitchAborts.Store(0)
	metricJobRunsTotal.Store(0)
	metricJobRunsKilled.Store(0)
	metricActiveWorkers.Store(0)
	metricScreenshotErrors.Store(0)
}
