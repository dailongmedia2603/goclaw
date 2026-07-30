// Package fbbackfill implements Facebook Page Messenger conversation history
// backfill for GoClaw. When a Page is connected to the gateway, this package
// pulls all historical conversations via the Graph API, summarizes each
// per-PSID thread, and writes the result to episodic memory so the agent has
// context when returning customers send new messages.
//
// # Fork-only package
//
// This package is maintained in the fork (feature/fbm branch) and MUST NOT
// modify any upstream code. Touch-points on upstream are limited to:
//
//   - cmd/gateway.go                                       (1 import + 1 Register call)
//   - ui/web/src/pages/channels/channel-schemas.ts         (1 additive field)
//   - ui/web/src/pages/channels/<FacebookChannelDetail>.tsx (mount <FbBackfillPanel />, if detail page exists)
//
// Forbidden touch-points (verified by scripts/verify-fb-backfill-upstream-safety.sh):
//
//   - internal/channels/facebook/*      (do not extend GraphClient — fork has its own)
//   - internal/bus/*, internal/sessions/*, internal/memory/*, internal/pipeline/*, internal/consolidation/*
//   - migrations/*                      (state lives in channel_instances.config._backfill JSONB)
//   - internal/store/sqlitestore/*      (feature is PG-only; Lite edition gated out)
//
// See docs/fork/fb-backfill-fork-contract.md for the full contract and
// upstream-merge SOP.
package fbbackfill
