#!/usr/bin/env bash
# Apply the 5 touch-point patches against an upstream GoClaw clone.
# Idempotent: re-running a patch that's already applied is a no-op (detected via reverse dry-run).
#
# Usage: apply-patches.sh UPSTREAM_DIR PATCH_DIR

set -euo pipefail

UPSTREAM="${1:?upstream dir required}"
PATCH_DIR="${2:?patch dir required}"

[[ -d "$UPSTREAM" ]] || { echo "❌ UPSTREAM not a directory: $UPSTREAM" >&2; exit 1; }
[[ -d "$PATCH_DIR" ]] || { echo "❌ PATCH_DIR not a directory: $PATCH_DIR" >&2; exit 1; }
[[ -f "$PATCH_DIR/APPLY-ORDER.txt" ]] || { echo "❌ APPLY-ORDER.txt missing in $PATCH_DIR" >&2; exit 1; }

cd "$UPSTREAM"

while IFS= read -r patchname; do
  [[ -z "$patchname" || "$patchname" == \#* ]] && continue
  patch_file="$PATCH_DIR/$patchname"
  if [[ ! -f "$patch_file" ]]; then
    echo "❌ Missing patch: $patch_file" >&2
    exit 1
  fi

  # Skip if patch is empty
  if [[ ! -s "$patch_file" ]]; then
    echo "  ⏭  $patchname is empty, skipping"
    continue
  fi

  if patch -p1 --dry-run --silent < "$patch_file" >/dev/null 2>&1; then
    patch -p1 --silent < "$patch_file" >/dev/null
    echo "  ✓ Applied $patchname"
  elif patch -p1 -R --dry-run --silent < "$patch_file" >/dev/null 2>&1; then
    echo "  ✓ $patchname already applied (skipping)"
  else
    echo "❌ Cannot apply $patchname — upstream may have diverged" >&2
    echo "   Run verify-patches.sh first to diagnose." >&2
    exit 1
  fi
done < "$PATCH_DIR/APPLY-ORDER.txt"

echo "✓ All patches applied"
