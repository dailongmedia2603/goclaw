#!/usr/bin/env bash
# Dry-run all patches against an upstream clone.
# Report each patch's applicability without modifying the tree.
#
# Usage: verify-patches.sh UPSTREAM_DIR PATCH_DIR
# Exit code: 0 if all clean, 1 if any conflict

set -euo pipefail

UPSTREAM="${1:?upstream dir required}"
PATCH_DIR="${2:?patch dir required}"

cd "$UPSTREAM"

ok_count=0
already_count=0
conflict_count=0
conflicts=()

for p in "$PATCH_DIR"/*.patch; do
  [[ -f "$p" && -s "$p" ]] || continue
  name=$(basename "$p")
  if patch -p1 --dry-run --silent < "$p" >/dev/null 2>&1; then
    ok_count=$((ok_count + 1))
    echo "  ✓ $name (applies cleanly)"
  elif patch -p1 -R --dry-run --silent < "$p" >/dev/null 2>&1; then
    already_count=$((already_count + 1))
    echo "  ≡ $name (already applied)"
  else
    conflict_count=$((conflict_count + 1))
    conflicts+=("$name")
    echo "  ✗ $name (CONFLICT)"
  fi
done

echo ""
echo "Summary: $ok_count clean | $already_count already applied | $conflict_count conflicts"

if [[ "$conflict_count" -gt 0 ]]; then
  echo ""
  echo "❌ Conflicts detected. Likely causes:"
  echo "   1. Upstream diverged from fork's target version"
  echo "   2. Patch files were corrupted during download"
  echo ""
  echo "Conflicting patches:"
  for c in "${conflicts[@]}"; do
    echo "   - $c"
  done
  echo ""
  echo "Fix:"
  echo "   1. Check upstream version: cd $UPSTREAM && git describe"
  echo "   2. Regenerate patches from a fork aligned to that version"
  echo "   3. OR use the bundle (Cách 1) instead of source build"
  exit 1
fi

echo "✓ All patches applicable"
