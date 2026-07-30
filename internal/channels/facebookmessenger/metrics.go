package facebookmessenger

import "sync/atomic"

// Package-level lightweight counters. No external metrics dependency
// — we expose a snapshot via Metrics() so the gateway's /status endpoint
// can surface the numbers without wiring Prometheus.
//
// These are *process-wide*, not per-instance. That's intentional for a
// v1 hardening pass; a multi-instance deployment can scrape them and
// attribute to the single tenant running this channel.

var (
	metricInboundTotal       atomic.Int64
	metricOutboundTotal      atomic.Int64
	metricRateLimitedTotal   atomic.Int64
	metricSignatureFailTotal atomic.Int64
	metricReconnectTotal     atomic.Int64
	metricCheckpointTotal    atomic.Int64 // explicit FB checkpoint detections
)

// IncInbound, IncOutbound, etc. are package-internal but exported so
// the sidecar shim integration tests can verify increments.
func IncInbound()       { metricInboundTotal.Add(1) }
func IncOutbound()      { metricOutboundTotal.Add(1) }
func IncRateLimited()   { metricRateLimitedTotal.Add(1) }
func IncSignatureFail() { metricSignatureFailTotal.Add(1) }
func IncReconnect()     { metricReconnectTotal.Add(1) }
func IncCheckpoint()    { metricCheckpointTotal.Add(1) }

// Metrics returns a read-only snapshot of the current counter values.
// Safe to call concurrently.
func Metrics() map[string]int64 {
	return map[string]int64{
		"fbm_inbound_total":        metricInboundTotal.Load(),
		"fbm_outbound_total":       metricOutboundTotal.Load(),
		"fbm_rate_limited_total":   metricRateLimitedTotal.Load(),
		"fbm_signature_fail_total": metricSignatureFailTotal.Load(),
		"fbm_reconnect_total":      metricReconnectTotal.Load(),
		"fbm_checkpoint_total":     metricCheckpointTotal.Load(),
	}
}

// ResetMetrics zeroes all counters. For tests only.
func ResetMetrics() {
	metricInboundTotal.Store(0)
	metricOutboundTotal.Store(0)
	metricRateLimitedTotal.Store(0)
	metricSignatureFailTotal.Store(0)
	metricReconnectTotal.Store(0)
	metricCheckpointTotal.Store(0)
}
