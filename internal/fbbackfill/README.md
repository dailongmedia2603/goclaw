# internal/fbbackfill

Facebook Page Messenger history backfill — fork-only feature.

## What this does

When a Facebook Page is connected to GoClaw, this package:

1. Pulls every past conversation (`/{page-id}/conversations`) and every message in each conversation (`/{conversation-id}/messages`) via the Graph API, paginated.
2. Summarizes each per-PSID conversation into an `EpisodicSummary` with an L0 abstract (~50 tokens) and a longer summary body.
3. Writes the summary to the upstream `EpisodicStore`. When the customer sends a new message later, the agent's memory search retrieves this summary and the agent has historical context.

The raw historical messages are **not** written to `sessions.messages`. Storing only the summary avoids clobbering in-flight conversations and keeps token cost predictable.

## Fork contract (important)

This package lives in the fork branch and must not modify upstream code. See [doc.go](./doc.go) for the allowed touch-points and [docs/fork/fb-backfill-fork-contract.md](../../docs/fork/fb-backfill-fork-contract.md) for the full contract.

CI enforces the contract via [scripts/verify-fb-backfill-upstream-safety.sh](../../scripts/verify-fb-backfill-upstream-safety.sh).

## Edition gating

PostgreSQL only. On `sqliteonly` builds (desktop Lite edition), `Register` is a no-op — channels are already disabled in Lite, so there's nothing to backfill.

## Layout

| File | Purpose |
|------|---------|
| `doc.go` | Package doc + fork contract summary |
| `register.go` / `register_lite.go` | Wire-up entry called from `cmd/gateway.go` |
| `types.go` | `BackfillState`, `JobStatus`, `Credentials` duplicated from upstream |
| `graph_client.go` | HTTP client for `/conversations` + `/messages` with pagination, BUC rate-limit, retry |
| `ratelimit.go` | BUC header parser + pacing policy |
| `state.go` / `state_store.go` | State persistence in `channel_instances.config._backfill` JSONB |
| `job_runner.go` | Orchestrator: state machine, resume, concurrency limit |
| `summarizer.go` | Turns `[]Message` per PSID into `EpisodicSummary`, idempotent via `SourceID` |
| `rpc.go` / `events.go` | WS RPC methods + progress broadcasts |
| `metrics.go` | slog event helpers |

Tests colocated as `*_test.go`. Integration tests in `tests/integration/fb_backfill_integration_test.go` (build tag `integration`, requires pgvector pg18 on port 5433).

## Status

Scaffolding — phase 0. Subsequent phases add the functional layers.
