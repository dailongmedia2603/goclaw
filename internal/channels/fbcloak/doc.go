//go:build !sqliteonly

// Package fbcloak implements the browser-automation re-engagement channel for
// Facebook fanpage inboxes idle for more than 7 days — the gap that the Graph
// API HUMAN_AGENT tag (≤7d) and Marketing Messages API (no VN rollout) do not
// cover.
//
// Architecture: Standard-edition only. Wired in cmd/gateway.go as a service
// (not a channels.ChannelFactory) because the workflow is outbound-batch on a
// cron, not bidirectional. See plans/fbcloak-reengagement/ for full design.
//
// ToS note: this feature operates against Meta Terms of Service ("automated
// access without prior permission", 2025-01-01). It exists because the
// business need (re-engaging customers idle >7d) has no compliant path in VN
// at the time of writing. Operators must accept the risk explicitly via the
// disclaimer flow (Phase 4) and the GOCLAW_FBCLOAK_KILLSWITCH env var must
// remain available.
package fbcloak
