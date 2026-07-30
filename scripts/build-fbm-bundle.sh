#!/usr/bin/env bash
# build-fbm-bundle.sh — build the FBM channel distribution bundle.
#
# Output:
#   dist/goclaw-fbm-bundle-v<VERSION>.tar.gz
#   dist/goclaw-fbm-bundle-v<VERSION>.sha256
#
# Usage:
#   ./scripts/build-fbm-bundle.sh [--version 0.1.0] [--platforms linux/amd64,linux/arm64]
#                                 [--skip-build] [--output-dir dist]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

# shellcheck source=./lib/bundle-helpers.sh
source "$SCRIPT_DIR/lib/bundle-helpers.sh"
# shellcheck source=./lib/manifest-gen.sh
source "$SCRIPT_DIR/lib/manifest-gen.sh"

# --- CLI args ---
BUNDLE_VERSION=""
PLATFORMS="linux/amd64"   # default single-arch; multi-arch opt-in
SKIP_BUILD=0
OUTPUT_DIR="$REPO_ROOT/dist"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) BUNDLE_VERSION="$2"; shift 2 ;;
    --platforms) PLATFORMS="$2"; shift 2 ;;
    --skip-build) SKIP_BUILD=1; shift ;;
    --output-dir) OUTPUT_DIR="$2"; shift 2 ;;
    -h|--help)
      sed -n '3,12p' "$0" | sed 's/^# //; s/^#//'
      exit 0 ;;
    *) log_error "unknown arg: $1"; exit 1 ;;
  esac
done

require_cmd docker jq zstd tar openssl
check_docker_buildx

BUNDLE_VERSION=$(compute_version "$BUNDLE_VERSION")
BUNDLE_VERSION="${BUNDLE_VERSION#v}"  # strip v prefix if present
log_info "Bundle version: $BUNDLE_VERSION"

# --- Gather git state ---
GOCLAW_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
FBM_COMMIT="$GOCLAW_COMMIT"
FBM_BRANCH=$(git branch --show-current 2>/dev/null || echo "unknown")
UPSTREAM_VERSION=$(git tag --sort=-v:refname 2>/dev/null | grep -E '^v[0-9]' | head -1 || echo "unknown")
SOURCE_DATE_EPOCH=$(git log -1 --format=%ct 2>/dev/null || date +%s)
export GOCLAW_COMMIT FBM_COMMIT FBM_BRANCH UPSTREAM_VERSION SOURCE_DATE_EPOCH BUNDLE_VERSION

log_info "GoClaw commit: $GOCLAW_COMMIT"
log_info "Upstream base: $UPSTREAM_VERSION"
log_info "Platforms:     $PLATFORMS"

# --- Staging ---
STAGE_DIR=$(mktemp -d -t fbm-bundle-XXXXXX)
register_cleanup "$STAGE_DIR"
export STAGE_DIR

mkdir -p "$STAGE_DIR/images" "$STAGE_DIR/install" "$STAGE_DIR/install/lib" \
         "$STAGE_DIR/docs" "$STAGE_DIR/bin"

mkdir -p "$OUTPUT_DIR"

# --- Build images ---
build_image() {
  local tag="$1" dockerfile="$2" context="$3"
  shift 3
  log_info "Building $tag"
  if [[ "$SKIP_BUILD" == "1" ]]; then
    docker image inspect "$tag" >/dev/null 2>&1 || {
      log_error "--skip-build but image $tag not present"
      exit 1
    }
    return
  fi

  # Single-arch build via plain `docker build` (faster, works on all hosts).
  # Multi-arch requires buildx + docker-container driver + push to registry.
  # For tar export we stick to single-arch per host.
  docker build \
    --tag "$tag" \
    --file "$dockerfile" \
    --build-arg "VERSION=$BUNDLE_VERSION" \
    --build-arg "SOURCE_DATE_EPOCH=$SOURCE_DATE_EPOCH" \
    "$@" \
    "$context" 2>&1 | tail -5
}

if [[ "$SKIP_BUILD" != "1" ]]; then
  log_info "==> Building goclaw-fork (backend)"
  build_image "goclaw-fork:$BUNDLE_VERSION" "Dockerfile" "." \
    --build-arg "ENABLE_EMBEDUI=false" \
    --build-arg "ENABLE_PYTHON=true"

  log_info "==> Building goclaw-web-fork (UI)"
  build_image "goclaw-web-fork:$BUNDLE_VERSION" "ui/web/Dockerfile" "ui/web"

  log_info "==> Building fbm-sidecar"
  build_image "fbm-sidecar:$BUNDLE_VERSION" "sidecar/mautrix-meta-shim/Dockerfile" "sidecar/mautrix-meta-shim"
fi

# --- Save images ---
save_image() {
  local tag="$1" dest="$2"
  log_info "Saving $tag → $(basename "$dest")"
  docker save "$tag" | zstd -T0 -19 > "$dest"
}

save_image "goclaw-fork:$BUNDLE_VERSION"     "$STAGE_DIR/images/goclaw-fork.tar.zst"
save_image "goclaw-web-fork:$BUNDLE_VERSION" "$STAGE_DIR/images/goclaw-web-fork.tar.zst"
save_image "fbm-sidecar:$BUNDLE_VERSION"     "$STAGE_DIR/images/fbm-sidecar.tar.zst"

# --- Compute image metadata for MANIFEST ---
IMAGES_JSON="{}"
for img in goclaw-fork goclaw-web-fork fbm-sidecar; do
  tag="$img:$BUNDLE_VERSION"
  tar_path="$STAGE_DIR/images/$img.tar.zst"
  digest=$(docker image inspect "$tag" --format '{{.Id}}' 2>/dev/null || echo "")
  size=$(file_size "$tar_path")
  IMAGES_JSON=$(
    jq --arg k "$tag" \
       --arg d "$digest" \
       --argjson s "$size" \
       --argjson p "[\"$PLATFORMS\"]" \
       '. + {($k): {digest: $d, size_bytes: $s, platforms: $p}}' <<< "$IMAGES_JSON"
  )
done
export IMAGES_JSON

# --- Build fbm-diagnose binary (cross-arch, optional) ---
log_info "==> Building fbm-diagnose (optional)"
DIAG_BUILT=0
if command -v go >/dev/null 2>&1; then
  if (
    cd "$REPO_ROOT"
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
      go build -trimpath -ldflags="-s -w" \
      -o "$STAGE_DIR/bin/fbm-diagnose-linux-amd64" \
      ./cmd/fbm-diagnose 2>&1
    GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
      go build -trimpath -ldflags="-s -w" \
      -o "$STAGE_DIR/bin/fbm-diagnose-linux-arm64" \
      ./cmd/fbm-diagnose 2>&1
  ) >/tmp/fbm-diag-build.log 2>&1; then
    chmod +x "$STAGE_DIR/bin/"fbm-diagnose-linux-* 2>/dev/null || true
    DIAG_BUILT=1
    log_ok "fbm-diagnose built (amd64 + arm64)"
  fi
fi

if [[ "$DIAG_BUILT" != "1" ]]; then
  log_warn "fbm-diagnose build skipped (Go missing or too old)"
  log_warn "See /tmp/fbm-diag-build.log for details. Bundle still functional."
fi

# Wrapper script (arch-selector) — always written so install scripts can reference it
if [[ "$DIAG_BUILT" == "1" ]]; then
  cat > "$STAGE_DIR/bin/fbm-diagnose" <<'WRAPPER'
#!/usr/bin/env bash
# Picks correct fbm-diagnose binary for host arch.
ARCH=$(uname -m)
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
case "$ARCH" in
  x86_64|amd64) exec "$DIR/fbm-diagnose-linux-amd64" "$@" ;;
  aarch64|arm64) exec "$DIR/fbm-diagnose-linux-arm64" "$@" ;;
  *) echo "Unsupported arch: $ARCH" >&2; exit 1 ;;
esac
WRAPPER
else
  cat > "$STAGE_DIR/bin/fbm-diagnose" <<'STUB'
#!/bin/sh
echo "fbm-diagnose not available in this bundle (Go toolchain missing at build time)." >&2
echo "Use 'docker logs fbm-sidecar' or check UI dropdown directly." >&2
exit 0
STUB
fi
chmod +x "$STAGE_DIR/bin/fbm-diagnose"

# --- Stage install/ files ---
log_info "==> Stage install scripts + docs"
cp -a "$REPO_ROOT/install-template/"*.sh "$STAGE_DIR/install/"
cp -a "$REPO_ROOT/install-template/docker-compose.fbm.yml" "$STAGE_DIR/install/"
cp -a "$REPO_ROOT/install-template/.env.fbm.template" "$STAGE_DIR/install/"
cp -a "$REPO_ROOT/install-template/lib/"*.sh "$STAGE_DIR/install/lib/"

# --- Stage docs ---
if [[ -d "$REPO_ROOT/docs/channels" ]]; then
  cp "$REPO_ROOT/docs/channels/facebook-personal.md" "$STAGE_DIR/docs/" 2>/dev/null || true
  cp "$REPO_ROOT/docs/channels/facebook-personal-security.md" "$STAGE_DIR/docs/" 2>/dev/null || true
fi
# Copy recipient-facing docs (from install-template/docs/ once Phase 5 writes them)
if [[ -d "$REPO_ROOT/install-template/docs" ]]; then
  cp -a "$REPO_ROOT/install-template/docs/"*.md "$STAGE_DIR/docs/" 2>/dev/null || true
fi

# --- Copy LICENSE + NOTICE ---
cp "$REPO_ROOT/sidecar/mautrix-meta-shim/LICENSE" "$STAGE_DIR/LICENSE" 2>/dev/null || \
  cat > "$STAGE_DIR/LICENSE" <<EOF
This bundle contains the fbm-sidecar image which embeds mautrix/meta,
licensed under AGPL-3.0-or-later.
See docs/facebook-personal-security.md and the mautrix-meta-shim README
for source offer details (AGPL §13).
EOF

# --- Generate MANIFEST + CHECKSUMS ---
log_info "==> Generate MANIFEST.json"
manifest_gen

log_info "==> Compute CHECKSUMS.sha256"
(
  cd "$STAGE_DIR"
  find . -type f ! -name CHECKSUMS.sha256 -print0 | sort -z | \
    while IFS= read -r -d '' f; do
      hash=$(sha256_file "$f")
      printf '%s  %s\n' "$hash" "${f#./}"
    done > CHECKSUMS.sha256
)

# --- Tar + gzip ---
BUNDLE_OUT="$OUTPUT_DIR/goclaw-fbm-bundle-v$BUNDLE_VERSION.tar.gz"
log_info "==> Tar + gzip → $BUNDLE_OUT"

# Set mtime for reproducibility
find "$STAGE_DIR" -exec touch -d "@$SOURCE_DATE_EPOCH" {} + 2>/dev/null || \
  find "$STAGE_DIR" -exec touch -t "$(date -r "$SOURCE_DATE_EPOCH" '+%Y%m%d%H%M.%S' 2>/dev/null || echo 197001010000)" {} + 2>/dev/null || true

tar \
  --sort=name \
  --owner=0 --group=0 --numeric-owner \
  --mtime="@$SOURCE_DATE_EPOCH" \
  -C "$STAGE_DIR" \
  -cf - . 2>/dev/null \
| gzip -9 > "$BUNDLE_OUT"

# --- Final checksum ---
BUNDLE_SHA=$(sha256_file "$BUNDLE_OUT")
echo "$BUNDLE_SHA  $(basename "$BUNDLE_OUT")" > "${BUNDLE_OUT%.tar.gz}.sha256"

BUNDLE_SIZE=$(file_size "$BUNDLE_OUT")
BUNDLE_SIZE_HUMAN=$(human_bytes "$BUNDLE_SIZE")

if [[ "$BUNDLE_SIZE" -gt 524288000 ]]; then
  log_warn "Bundle size $BUNDLE_SIZE_HUMAN exceeds 500 MB target"
fi

# --- Done ---
echo ""
log_ok "Bundle built successfully"
echo ""
echo "  File:     $BUNDLE_OUT"
echo "  Size:     $BUNDLE_SIZE_HUMAN"
echo "  SHA256:   $BUNDLE_SHA"
echo "  Version:  $BUNDLE_VERSION"
echo "  Upstream: $UPSTREAM_VERSION"
echo ""
echo "  Verify:   sha256sum -c ${BUNDLE_OUT%.tar.gz}.sha256"
echo ""
echo "  Next:     scripts/test-bundle.sh $BUNDLE_OUT"
