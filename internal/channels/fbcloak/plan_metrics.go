//go:build !sqliteonly

package fbcloak

import "sync/atomic"

// Phase 5 plan-mode counters. Same atomic.Int64 pattern as Phase 1-4
// metrics.go — no external Prometheus dependency. Names follow the
// fbcloak_plans_* convention so a scrape adapter maps them 1:1.
var (
	metricPlanSentTotal             atomic.Int64
	metricPlanExecErrorTotal        atomic.Int64
	metricPlanSkippedByPolicyTotal  atomic.Int64
	metricPlanReplanMarkedTotal     atomic.Int64
	metricPlanReplanCompleteTotal   atomic.Int64
	metricPlanReplanErrorTotal      atomic.Int64
)

func IncPlanSent()             { metricPlanSentTotal.Add(1) }
func IncPlanExecError()        { metricPlanExecErrorTotal.Add(1) }
func IncPlanSkippedByPolicy()  { metricPlanSkippedByPolicyTotal.Add(1) }
func IncPlanReplanMarked()     { metricPlanReplanMarkedTotal.Add(1) }
func IncReplanComplete()       { metricPlanReplanCompleteTotal.Add(1) }
func IncReplanError()          { metricPlanReplanErrorTotal.Add(1) }

// PlanMetricsSnapshot returns a snapshot of the Phase 5 plan counters for
// the gateway /status endpoint or admin diagnostics.
type PlanMetricsSnapshot struct {
	Sent            int64 `json:"plans_sent_total"`
	ExecError       int64 `json:"plans_exec_error_total"`
	SkippedByPolicy int64 `json:"plans_skipped_by_policy_total"`
	ReplanMarked    int64 `json:"plans_replan_marked_total"`
	ReplanComplete  int64 `json:"plans_replan_complete_total"`
	ReplanError     int64 `json:"plans_replan_error_total"`
}

func PlanMetrics() PlanMetricsSnapshot {
	return PlanMetricsSnapshot{
		Sent:            metricPlanSentTotal.Load(),
		ExecError:       metricPlanExecErrorTotal.Load(),
		SkippedByPolicy: metricPlanSkippedByPolicyTotal.Load(),
		ReplanMarked:    metricPlanReplanMarkedTotal.Load(),
		ReplanComplete:  metricPlanReplanCompleteTotal.Load(),
		ReplanError:     metricPlanReplanErrorTotal.Load(),
	}
}
