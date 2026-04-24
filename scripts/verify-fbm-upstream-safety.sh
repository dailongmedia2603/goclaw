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

BASE="${1:-origin/main}"

# Ensure BASE exists (in CI we may need to fetch).
if ! git rev-parse --verify "$BASE" >/dev/null 2>&1; then
  echo "⚠️  Base ref '$BASE' not found; attempting 'git fetch origin main'..."
  git fetch origin main --depth=50 2>/dev/null || true
fi

echo "== Touched files since $BASE =="
TOUCHED=$(git diff --name-only "$BASE"..HEAD || true)
echo "$TOUCHED"
echo ""

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

echo "== Forbidden-file check =="
VIOLATIONS=0
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

echo ""
if [[ $VIOLATIONS -eq 0 ]]; then
  echo "✅ Upstream-safety check PASSED"
  exit 0
fi

echo "❌ Upstream-safety check FAILED ($VIOLATIONS violations)"
exit 1
