# Facebook History Backfill — Fork Contract

This document describes how the Facebook History Backfill feature is kept **upstream-safe** so it survives `git merge` from `nextlevelbuilder/goclaw`.

Feature code lives in [`internal/fbbackfill/`](../../internal/fbbackfill/) (fork-only). This contract governs what the fork may touch **outside** that directory.

## Summary

| Area | Policy |
|------|--------|
| `internal/fbbackfill/` | Fork-owned — free to modify |
| `cmd/gateway.go` | Whitelisted — 1 import + 1 `fbbackfill.Register(ctx, deps)` call |
| `ui/web/src/pages/channels/channel-schemas.ts` | Whitelisted — 1 additive field (`backfill_on_create`) |
| `ui/web/src/pages/channels/FacebookChannelDetail.tsx` *or* `ChannelDetail.tsx` | Whitelisted — 1 additive `<FbBackfillPanel />` mount (only one of these files, depending on which exists upstream) |
| `ui/web/src/features/fb-backfill/` | Fork-owned — free to create |
| `ui/web/src/i18n/locales/{en,vi,zh}/channels.json` | Whitelisted — additive keys under `fbBackfill.*` |
| `docs/channels/facebook-backfill.md`, `docs/fork/fb-backfill-fork-contract.md` | Fork-owned |
| `scripts/verify-fb-backfill-upstream-safety.sh` | Fork-owned |
| `tests/integration/fb_backfill_*_test.go` | Fork-owned |
| **Everything else** | **Do not modify** |

## Forbidden paths (enforced by CI)

The verify script (`scripts/verify-fb-backfill-upstream-safety.sh`) fails the build if any of these are touched:

- `internal/channels/facebook/` — upstream channel; the fork has its own Graph client
- `internal/bus/`, `internal/sessions/`, `internal/memory/`, `internal/pipeline/`, `internal/consolidation/`
- `internal/store/sqlitestore/` — feature is PG-only; Lite edition gated out via build tag
- `internal/store/episodic_store.go`, `internal/store/session_store.go`, `internal/store/context.go` — upstream interfaces; call them, do not modify them
- `migrations/` — **no new migration files**. State is embedded in `channel_instances.config._backfill` JSONB

## Why no new migrations?

Upstream migration counter is at `000055_*` (at the time of writing). If the fork adds `000056_fork_fb_backfill.up.sql`, the next upstream release will almost certainly add a `000056_*` file too and collide fatally. The whole state machine for backfill jobs therefore lives in a JSONB subtree of `channel_instances.config` under the key `_backfill`. The leading underscore signals "fork-private" by convention.

This is fine because:

- Backfill state is strictly per channel-instance (1:1)
- We do not need cross-instance query capability
- Episodic memory entries (the actual backfill output) use the upstream `episodic_summaries` table via `EpisodicStore`, no new schema required

## Upstream-merge SOP

When the upstream releases a new version:

```bash
# 1. Fetch
git fetch upstream

# 2. Peek at what changed in our whitelisted touch-points
git log HEAD..upstream/main -- cmd/gateway.go \
  ui/web/src/pages/channels/channel-schemas.ts \
  ui/web/src/pages/channels/FacebookChannelDetail.tsx \
  ui/web/src/pages/channels/ChannelDetail.tsx

# 3. Merge
git merge upstream/main

# 4. Resolve conflicts in whitelisted files:
#    - cmd/gateway.go: keep the fbbackfill.Register(...) line + any new upstream factory registrations
#    - channel-schemas.ts: keep the backfill_on_create field; merge with any upstream additions
#    - i18n JSON files: keep fbBackfill.* keys; merge with upstream keys

# 5. Verify
./scripts/verify-fb-backfill-upstream-safety.sh
go build ./...
go build -tags sqliteonly ./...
go vet ./...
go test ./internal/fbbackfill/...

# 6. Run upstream regression
go test ./...
cd ui/web && pnpm build && pnpm lint

# 7. Commit merge + tag
git push origin main
```

If the verify script fails:

- **Forbidden-path violation** → move the change into `internal/fbbackfill/` or `ui/web/src/features/fb-backfill/`, or widen the contract in this document (requires maintainer review)
- **Missing artifact** → the fork branch is in a broken state; likely a bad merge. `git status` and restore

## Contributing this feature upstream (future work)

If the upstream project wants to adopt this, the porting path is:

1. Move `internal/fbbackfill/` → upstream as a sibling of `internal/channels/facebook/`
2. Extend `internal/channels/facebook/graph_api.go` with `ListConversations` / `ListMessages` (the fork's private client can delete its copy)
3. Add a proper `channel_instances` migration (if upstream prefers a relational state table over JSONB)
4. Wire the `Register(ctx, deps)` call and UI mount the same way — those bits do not need to change

Until that happens, the fork keeps its own self-contained implementation.

## Verify script reference

Run locally before pushing:

```bash
./scripts/verify-fb-backfill-upstream-safety.sh --local        # artifact presence + build tag checks
./scripts/verify-fb-backfill-upstream-safety.sh                # full check vs origin/main
./scripts/verify-fb-backfill-upstream-safety.sh upstream/main  # full check vs upstream
```

CI runs this on every PR that touches `internal/fbbackfill/` or any whitelisted file.
