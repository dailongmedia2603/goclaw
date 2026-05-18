#!/usr/bin/env bash
# build-fbm-source-tarball.sh — produce a small source-only tarball for Cách 4 distribution.
#
# Output: dist/goclaw-fbm-source-v<VERSION>.tar.gz (≤ 80 MB)
#
# Recipient has: their own upstream GoClaw clone. They extract this tarball and run
# install-template/build-fbm-from-source.sh → it applies patches + builds 3 images.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

# shellcheck source=./lib/bundle-helpers.sh
source "$SCRIPT_DIR/lib/bundle-helpers.sh"

BUNDLE_VERSION=""
UPSTREAM_TAG=""
OUTPUT_DIR="$REPO_ROOT/dist"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) BUNDLE_VERSION="$2"; shift 2 ;;
    --upstream-tag) UPSTREAM_TAG="$2"; shift 2 ;;
    --output-dir) OUTPUT_DIR="$2"; shift 2 ;;
    *) log_error "unknown arg: $1"; exit 1 ;;
  esac
done

require_cmd git jq tar gzip

BUNDLE_VERSION=$(compute_version "$BUNDLE_VERSION")
BUNDLE_VERSION="${BUNDLE_VERSION#v}"

if [[ -z "$UPSTREAM_TAG" ]]; then
  UPSTREAM_TAG=$(git tag --sort=-v:refname | grep -E '^v[0-9]' | head -1)
fi

SOURCE_DATE_EPOCH=$(git log -1 --format=%ct 2>/dev/null || date +%s)

STAGE=$(mktemp -d -t fbm-source-XXXXXX)
register_cleanup "$STAGE"
mkdir -p "$OUTPUT_DIR"

log_info "Building source tarball v$BUNDLE_VERSION (upstream base: $UPSTREAM_TAG)"

# --- Copy new files (these are wholly new, not modified) ---
log_info "==> Copy new feature source"
mkdir -p "$STAGE/source"
cp -a internal/channels/facebookmessenger "$STAGE/source/"
cp -a sidecar/mautrix-meta-shim "$STAGE/source/"
cp -a ui/web/src/pages/channels/facebook-messenger "$STAGE/source/" 2>/dev/null || true
cp -a cmd/fbm-diagnose "$STAGE/source/" 2>/dev/null || true

# Clean up build artifacts that may have leaked
rm -f "$STAGE/source/mautrix-meta-shim/goclaw-fbm-sidecar"

# --- i18n fragments (partial file changes — extract only facebook_personal keys) ---
log_info "==> Extract i18n fragments"
mkdir -p "$STAGE/source/i18n-fragments"
for locale in en vi zh; do
  src="ui/web/src/i18n/locales/$locale/channels.json"
  out="$STAGE/source/i18n-fragments/$locale-channels.json"
  if [[ -f "$src" ]]; then
    jq '{facebook_personal: .facebook_personal}' "$src" > "$out" 2>/dev/null || \
      echo '{}' > "$out"
  fi
done

# --- Go i18n fragments (Go keys for error messages) ---
# Extract just the lines matching Fbm or Facebook_personal — small diff file
log_info "==> Extract Go i18n fragments"
for f in internal/i18n/keys.go internal/i18n/catalog_en.go internal/i18n/catalog_vi.go internal/i18n/catalog_zh.go; do
  if [[ -f "$f" ]]; then
    grep -n -E 'FBM|Facebook.*Messenger|fbm\.|facebook_personal' "$f" > \
      "$STAGE/source/i18n-fragments/$(basename "$f").txt" 2>/dev/null || true
  fi
done

# --- Generate patches ---
log_info "==> Generate patches"
bash "$SCRIPT_DIR/extract-fbm-patches.sh" "$UPSTREAM_TAG" "$STAGE/patches" 2>&1 | tail -10

# --- Install-template + build scripts ---
log_info "==> Copy install scripts"
mkdir -p "$STAGE/install"
cp -a "$REPO_ROOT/install-template/"* "$STAGE/install/"

# --- Source build script (bundled with source tarball) ---
# build-fbm-from-source.sh is copied by apply-patches.sh
mkdir -p "$STAGE/scripts"
for s in build-fbm-from-source.sh apply-patches.sh verify-patches.sh; do
  if [[ -f "$REPO_ROOT/install-template/$s" ]]; then
    cp -a "$REPO_ROOT/install-template/$s" "$STAGE/scripts/"
  fi
done

# --- Docs ---
if [[ -d "$REPO_ROOT/docs/channels" ]]; then
  mkdir -p "$STAGE/docs"
  cp -a "$REPO_ROOT/docs/channels/"facebook-personal*.md "$STAGE/docs/" 2>/dev/null || true
fi

# --- MANIFEST ---
log_info "==> Write MANIFEST-SOURCE.json"
PATCHES_JSON=$(ls "$STAGE/patches"/*.patch 2>/dev/null | xargs -n1 basename | jq -R . | jq -s . || echo '[]')
cat > "$STAGE/MANIFEST-SOURCE.json" <<EOF
{
  "bundle_version": "$BUNDLE_VERSION",
  "format": "source",
  "upstream_base": "$UPSTREAM_TAG",
  "goclaw_fork_commit": "$(git rev-parse --short HEAD 2>/dev/null || echo unknown)",
  "build_date": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "patches": $PATCHES_JSON,
  "notes": [
    "Apply patches against upstream $UPSTREAM_TAG with 'patch -p1 < <patch>'",
    "Source files under source/ are new files — copy as-is",
    "i18n-fragments/ contain partial additions to existing files"
  ]
}
EOF

# --- CHECKSUMS ---
log_info "==> Compute CHECKSUMS.sha256"
(
  cd "$STAGE"
  find . -type f ! -name CHECKSUMS.sha256 -print0 | sort -z | \
    while IFS= read -r -d '' f; do
      hash=$(sha256_file "$f")
      printf '%s  %s\n' "$hash" "${f#./}"
    done > CHECKSUMS.sha256
)

# --- Tar + gzip ---
OUT="$OUTPUT_DIR/goclaw-fbm-source-v$BUNDLE_VERSION.tar.gz"
log_info "==> Tar + gzip → $OUT"
if tar --version 2>&1 | grep -q GNU; then
  TAR=(tar --sort=name --owner=0 --group=0 --numeric-owner --mtime="@$SOURCE_DATE_EPOCH")
else
  # BSD tar (macOS default) — no --sort, no --owner numeric; rely on find sort for determinism
  TAR=(tar)
fi
( cd "$STAGE" && find . -type f -print0 | sort -z | xargs -0 "${TAR[@]}" -cf - ) 2>/dev/null | gzip -9 > "$OUT"

SHA=$(sha256_file "$OUT")
echo "$SHA  $(basename "$OUT")" > "${OUT%.tar.gz}.sha256"
SIZE=$(file_size "$OUT")
SIZE_H=$(human_bytes "$SIZE")

if [[ "$SIZE" -gt 83886080 ]]; then
  log_warn "Source tarball size $SIZE_H exceeds 80 MB target"
fi

log_ok "Source tarball built"
echo ""
echo "  File:    $OUT"
echo "  Size:    $SIZE_H"
echo "  SHA256:  $SHA"
echo "  Version: $BUNDLE_VERSION"
echo ""
echo "  Recipient install flow:"
echo "    1. Recipient has existing goclaw clone at <upstream-dir>"
echo "    2. Extract source tarball"
echo "    3. Run: bash install/build-fbm-from-source.sh --upstream-dir <upstream-dir>"
