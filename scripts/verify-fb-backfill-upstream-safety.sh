#!/usr/bin/env bash
# verify-fb-backfill-upstream-safety.sh
#
# Enforces the upstream-safety contract for the Facebook History Backfill
# feature (internal/fbbackfill/):
#
#   - A hard blocklist of upstream files MUST remain untouched.
#   - Additive touches are allowed only on a whitelist (cmd/gateway.go,
#     ui/web/src/pages/channels/channel-schemas.ts, at most one more UI
#     mount point).
#   - Required fork artifacts MUST be present (checked locally).
#
# Exit 0 if safe, non-zero otherwise. Run in CI on every PR that modifies
# internal/fbbackfill/ or touches the whitelist.
#
# Usage:
#   ./scripts/verify-fb-backfill-upstream-safety.sh [base-ref]
#   ./scripts/verify-fb-backfill-upstream-safety.sh --local
#
# base-ref defaults to origin/main. --local skips diff-based checks and
# verifies only that required fork artifacts exist.

set -euo pipefail

LOCAL_ONLY=0
if [[ "${1:-}" == "--local" ]]; then
  LOCAL_ONLY=1
  BASE=""
else
  BASE="${1:-origin/main}"
  if ! git rev-parse --verify "$BASE" >/dev/null 2>&1; then
    echo "⚠️  Base ref '$BASE' not found; attempting 'git fetch origin main'..."
    git fetch origin main --depth=50 2>/dev/null || true
  fi
fi

VIOLATIONS=0
TOUCHED=""

if [[ "$LOCAL_ONLY" == "0" ]]; then
  echo "== Touched files since $BASE =="
  TOUCHED=$(git diff --name-only "$BASE"..HEAD || true)
  echo "$TOUCHED"
  echo ""
fi

# --- Hard blocklist: upstream areas the fork must not modify ---
# Paths here are prefix-matched: any file under the prefix is forbidden.
FORBIDDEN_PREFIXES=(
  "internal/channels/facebook/"
  "internal/bus/"
  "internal/sessions/"
  "internal/memory/"
  "internal/pipeline/"
  "internal/consolidation/"
  "migrations/"
  "internal/store/sqlitestore/"
)

# Individual files that are forbidden even if not under a forbidden prefix.
FORBIDDEN_FILES=(
  "internal/store/episodic_store.go"
  "internal/store/session_store.go"
  "internal/store/context.go"
)

# --- Whitelist: additive touches are allowed only in these files ---
ALLOWED_TOUCHES=(
  "cmd/gateway.go"
  "ui/web/src/pages/channels/channel-schemas.ts"
  # Phase 6 may add one of these for the detail mount point. Either — not both:
  "ui/web/src/pages/channels/FacebookChannelDetail.tsx"
  "ui/web/src/pages/channels/ChannelDetail.tsx"
  # i18n catalog additions (additive only) for UI strings:
  "ui/web/src/i18n/locales/en/channels.json"
  "ui/web/src/i18n/locales/vi/channels.json"
  "ui/web/src/i18n/locales/zh/channels.json"
)

file_under_prefix() {
  local f="$1" p
  for p in "${FORBIDDEN_PREFIXES[@]}"; do
    [[ "$f" == "$p"* ]] && return 0
  done
  return 1
}

file_in_list() {
  local f="$1" entry
  shift
  for entry in "$@"; do
    [[ "$f" == "$entry" ]] && return 0
  done
  return 1
}

if [[ "$LOCAL_ONLY" == "0" ]]; then
  echo "== Forbidden-path check =="
  while IFS= read -r f; do
    [[ -z "$f" ]] && continue
    if file_under_prefix "$f" || file_in_list "$f" "${FORBIDDEN_FILES[@]}"; then
      echo "❌ FAIL: $f is on the forbidden list"
      VIOLATIONS=$((VIOLATIONS + 1))
    fi
  done <<< "$TOUCHED"
  [[ $VIOLATIONS -eq 0 ]] && echo "  ✓ No forbidden-path touches"
  echo ""

  echo "== Allowed touch-point audit =="
  while IFS= read -r f; do
    [[ -z "$f" ]] && continue
    # Only audit files outside internal/fbbackfill/ and outside tests/
    # (fbbackfill dir is fork-owned, free to change).
    [[ "$f" == internal/fbbackfill/* ]] && continue
    [[ "$f" == scripts/verify-fb-backfill-upstream-safety.sh ]] && continue
    [[ "$f" == docs/fork/fb-backfill-fork-contract.md ]] && continue
    [[ "$f" == docs/channels/facebook-backfill.md ]] && continue
    [[ "$f" == tests/integration/fb_backfill_* ]] && continue
    [[ "$f" == plans/* ]] && continue
    [[ "$f" == ui/web/src/features/fb-backfill/* ]] && continue

    if ! file_in_list "$f" "${ALLOWED_TOUCHES[@]}"; then
      echo "⚠️  WARN: $f touched but not on whitelist — review manually"
      # Do not fail the build here; the forbidden-path check above is authoritative.
      # This warning surfaces unexpected touches for reviewer attention.
    fi
  done <<< "$TOUCHED"
  echo ""
fi

echo "== Required fork artifact presence =="
REQUIRED_PATHS=(
  "internal/fbbackfill/doc.go"
  "internal/fbbackfill/register.go"
  "internal/fbbackfill/register_lite.go"
  "internal/fbbackfill/README.md"
  "scripts/verify-fb-backfill-upstream-safety.sh"
  "docs/fork/fb-backfill-fork-contract.md"
)
MISSING=0
for p in "${REQUIRED_PATHS[@]}"; do
  if [[ -f "$p" ]]; then
    echo "  ✓ $p"
  else
    echo "  ❌ MISSING: $p"
    MISSING=$((MISSING + 1))
  fi
done
if [[ $MISSING -gt 0 ]]; then
  echo ""
  echo "❌ $MISSING required fb-backfill artifact(s) missing"
  VIOLATIONS=$((VIOLATIONS + MISSING))
fi
echo ""

echo "== Build-tag separation =="
# register.go MUST be !sqliteonly, register_lite.go MUST be sqliteonly.
if ! head -3 internal/fbbackfill/register.go | grep -q '//go:build !sqliteonly'; then
  echo "❌ internal/fbbackfill/register.go missing '//go:build !sqliteonly' tag"
  VIOLATIONS=$((VIOLATIONS + 1))
fi
if ! head -3 internal/fbbackfill/register_lite.go | grep -q '//go:build sqliteonly'; then
  echo "❌ internal/fbbackfill/register_lite.go missing '//go:build sqliteonly' tag"
  VIOLATIONS=$((VIOLATIONS + 1))
fi
[[ $VIOLATIONS -eq 0 ]] && echo "  ✓ Build-tag separation intact"
echo ""

if [[ $VIOLATIONS -eq 0 ]]; then
  echo "✅ fb-backfill upstream-safety check PASSED"
  exit 0
fi
echo "❌ fb-backfill upstream-safety check FAILED ($VIOLATIONS violations)"
exit 1
