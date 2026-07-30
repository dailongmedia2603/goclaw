#!/usr/bin/env bash
# extract-fbm-patches.sh — generate per-file patch files for the 5 FBM touch points.
#
# Called by build-fbm-source-tarball.sh. Output goes to OUT_DIR, one .patch per touched file.
#
# Usage:
#   ./scripts/extract-fbm-patches.sh [UPSTREAM_TAG] [OUT_DIR]
#
# UPSTREAM_TAG defaults to the latest vX.Y.Z tag reachable from HEAD.
# OUT_DIR defaults to dist/patches-vX.Y.Z.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib/bundle-helpers.sh
source "$SCRIPT_DIR/lib/bundle-helpers.sh"

UPSTREAM_TAG="${1:-}"
OUT_DIR="${2:-}"

if [[ -z "$UPSTREAM_TAG" ]]; then
  UPSTREAM_TAG=$(git tag --sort=-v:refname | grep -E '^v[0-9]' | head -1)
  [[ -z "$UPSTREAM_TAG" ]] && { log_error "cannot detect upstream tag"; exit 1; }
fi
if [[ -z "$OUT_DIR" ]]; then
  OUT_DIR="dist/patches-$UPSTREAM_TAG"
fi

mkdir -p "$OUT_DIR"

log_info "Upstream base: $UPSTREAM_TAG"
log_info "Output dir:    $OUT_DIR"

# Touch-point map: "path_in_repo:patch_filename"
declare -a TOUCH_POINTS=(
  "cmd/gateway.go:01-cmd-gateway.patch"
  "internal/channels/channel.go:02-channels-channel.patch"
  "ui/web/src/constants/channels.ts:03-ui-constants-channels.patch"
  "ui/web/src/pages/channels/channel-schemas.ts:04-ui-channel-schemas.patch"
  "ui/web/src/pages/channels/channel-wizard-registry.tsx:05-ui-channel-wizard-registry.patch"
)

for tp in "${TOUCH_POINTS[@]}"; do
  file="${tp%:*}"
  patch="${tp##*:}"
  out="$OUT_DIR/$patch"
  log_info "Extracting → $patch"
  if ! git diff "$UPSTREAM_TAG"..HEAD -- "$file" > "$out"; then
    log_error "git diff failed for $file"
    exit 1
  fi
  if [[ ! -s "$out" ]]; then
    log_warn "patch for $file is empty — upstream identical or file missing"
  fi
done

# Apply order
cat > "$OUT_DIR/APPLY-ORDER.txt" <<EOF
01-cmd-gateway.patch
02-channels-channel.patch
03-ui-constants-channels.patch
04-ui-channel-schemas.patch
05-ui-channel-wizard-registry.patch
EOF

# Apply instructions
cat > "$OUT_DIR/README.md" <<EOF
# FBM Patches — applied against $UPSTREAM_TAG

Apply in order (see APPLY-ORDER.txt):

\`\`\`bash
cd /path/to/goclaw-$UPSTREAM_TAG-clone
for p in \$(cat APPLY-ORDER.txt); do
  patch -p1 < "\$p"
done
\`\`\`

If any hunk fails to apply cleanly, the upstream source may have diverged from
$UPSTREAM_TAG. Regenerate patches from a fork aligned with your target upstream
version, or use the bundle (Cách 1) instead.
EOF

log_ok "Extracted $((${#TOUCH_POINTS[@]})) patches to $OUT_DIR"
