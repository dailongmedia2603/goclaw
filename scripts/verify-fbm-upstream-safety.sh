#!/usr/bin/env bash
# verify-fbm-upstream-safety.sh
#
# Enforces the upstream-safety contract for the facebook_personal channel:
#   - Only 3 core files may be touched (cmd/gateway.go, channel.go const, UI constants)
#   - Schema files (channel-schemas.ts, wizard-registry.tsx) accept additive entries
#   - A hard blocklist of core files MUST remain untouched
#
# Exit 0 if safe, non-zero otherwise. Run in CI on every PR that modifies
# internal/channels/facebookmessenger/.
#
# Usage:
#   ./scripts/verify-fbm-upstream-safety.sh [base-ref]
#
# base-ref defaults to origin/main. Compares HEAD against the base ref.

set -euo pipefail

# Modes:
#   (default) BASE=origin/main  → CI mode: compare PR vs main
#   --local                     → skip diff-based checks, verify artifact presence only
LOCAL_ONLY=0
if [[ "${1:-}" == "--local" ]]; then
  LOCAL_ONLY=1
  BASE=""
else
  BASE="${1:-origin/main}"
  # Ensure BASE exists (in CI we may need to fetch).
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
else
  echo "== Local mode: skipping diff-based checks (FBM artifact presence only) =="
  echo ""
fi

# --- Hard blocklist: these core files must never be modified ---
FORBIDDEN=(
  "internal/channels/manager.go"
  "internal/channels/dispatch.go"
  "internal/channels/instance_loader.go"
  "internal/channels/channel.go"   # only additive type const allowed — checked separately
  "internal/bus/types.go"
  "internal/bus/bus.go"
  "internal/store/pg/channel_instances.go"
  "internal/crypto/aes.go"
)

if [[ "$LOCAL_ONLY" == "1" ]]; then
  # Skip diff-based checks entirely; jump to artifact presence checks
  SKIP_DIFF_CHECKS=1
fi

if [[ -z "${SKIP_DIFF_CHECKS:-}" ]]; then
echo "== Forbidden-file check =="
for f in "${FORBIDDEN[@]}"; do
  if echo "$TOUCHED" | grep -qxF "$f"; then
    # Exception: channel.go may be touched to add the TypeFacebookPersonal constant.
    if [[ "$f" == "internal/channels/channel.go" ]]; then
      # Verify the diff only adds lines and nothing is deleted.
      DELETED=$(git diff "$BASE"..HEAD -- "$f" | grep -c "^-[^-]" || true)
      if [[ "$DELETED" -gt 0 ]]; then
        echo "❌ FAIL: $f had $DELETED deleted line(s) — only additive constant add is allowed"
        VIOLATIONS=$((VIOLATIONS + 1))
      else
        # Check the added lines are only the TypeFacebookPersonal constant area.
        ADDED=$(git diff "$BASE"..HEAD -- "$f" | grep "^+" | grep -v "^+++")
        if ! echo "$ADDED" | grep -qE 'TypeFacebookPersonal\s*='; then
          echo "❌ FAIL: $f modifications do not look like a TypeFacebookPersonal add"
          echo "$ADDED" | head -10
          VIOLATIONS=$((VIOLATIONS + 1))
        else
          echo "✓ $f — additive TypeFacebookPersonal only"
        fi
      fi
    else
      echo "❌ FAIL: $f was modified (forbidden)"
      VIOLATIONS=$((VIOLATIONS + 1))
    fi
  else
    echo "✓ $f untouched"
  fi
done

echo ""
echo "== Expected-touch audit =="
EXPECTED_TOUCHES=(
  "cmd/gateway.go"                                          # +1 import, +1 RegisterFactory
  "ui/web/src/constants/channels.ts"                        # +1 entry
  "ui/web/src/pages/channels/channel-schemas.ts"            # +2 entries
  "ui/web/src/pages/channels/channel-wizard-registry.tsx"   # +2 imports +2 entries
)
for f in "${EXPECTED_TOUCHES[@]}"; do
  if echo "$TOUCHED" | grep -qxF "$f"; then
    COUNT=$(git diff --numstat "$BASE"..HEAD -- "$f" | awk '{print $1}')
    echo "  $f: +$COUNT line(s) added"
  fi
done

fi  # SKIP_DIFF_CHECKS

echo ""
echo "== FBM artifact presence check =="
REQUIRED_PATHS=(
  "internal/channels/facebookmessenger/channel.go"
  "internal/channels/facebookmessenger/factory.go"
  "internal/channels/facebookmessenger/policy.go"
  "internal/channels/facebookmessenger/signature.go"
  "internal/channels/facebookmessenger/edition_gate.go"
  "sidecar/mautrix-meta-shim/main.go"
  "sidecar/mautrix-meta-shim/Dockerfile"
  "sidecar/mautrix-meta-shim/LICENSE"
  "cmd/fbm-diagnose/main.go"
  "docs/channels/facebook-personal.md"
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
  echo "❌ $MISSING required FBM artifact(s) missing"
  VIOLATIONS=$((VIOLATIONS + MISSING))
fi

echo ""
echo "== Touch-point inline marker check =="
grep -q "TypeFacebookPersonal" internal/channels/channel.go 2>/dev/null || {
  echo "❌ TypeFacebookPersonal constant missing in internal/channels/channel.go"
  VIOLATIONS=$((VIOLATIONS + 1))
}
grep -q "facebookmessenger\.Factory" cmd/gateway.go 2>/dev/null || {
  echo "❌ facebookmessenger.Factory registration missing in cmd/gateway.go"
  VIOLATIONS=$((VIOLATIONS + 1))
}
grep -q "facebook_personal" ui/web/src/constants/channels.ts 2>/dev/null || {
  echo "❌ facebook_personal entry missing in ui/web/src/constants/channels.ts"
  VIOLATIONS=$((VIOLATIONS + 1))
}
[[ $VIOLATIONS -eq 0 ]] && echo "  ✓ All touch points intact"

echo ""
if [[ $VIOLATIONS -eq 0 ]]; then
  echo "✅ Upstream-safety check PASSED"
  exit 0
fi
echo "❌ Upstream-safety check FAILED ($VIOLATIONS violations)"
exit 1
