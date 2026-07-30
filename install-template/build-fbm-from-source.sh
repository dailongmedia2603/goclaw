#!/usr/bin/env bash
# build-fbm-from-source.sh — build 3 FBM images on recipient host from source tarball.
#
# 2 modes:
#   (1) --upstream-dir /path  → apply patches into existing clone, build
#   (2) --fresh-clone          → git clone upstream fresh, apply, build
#
# Usage:
#   bash build-fbm-from-source.sh --upstream-dir /opt/goclaw
#   bash build-fbm-from-source.sh --fresh-clone --version 0.1.0

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SRC_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

UPSTREAM_DIR=""
FRESH_CLONE=0
BUILD_VERSION=""
KEEP_TEMP=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --upstream-dir) UPSTREAM_DIR="$2"; shift 2 ;;
    --fresh-clone) FRESH_CLONE=1; shift ;;
    --version) BUILD_VERSION="$2"; shift 2 ;;
    --keep-temp) KEEP_TEMP=1; shift ;;
    -h|--help)
      sed -n '3,14p' "$0" | sed 's/^# //; s/^#//'
      exit 0 ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

# --- Validate ---
for c in git docker jq patch; do
  command -v "$c" >/dev/null 2>&1 || { echo "❌ missing: $c" >&2; exit 1; }
done

if [[ ! -f "$SRC_DIR/MANIFEST-SOURCE.json" ]]; then
  echo "❌ Cần chạy từ thư mục đã giải nén source tarball (MANIFEST-SOURCE.json missing)" >&2
  exit 1
fi

UPSTREAM_TAG=$(jq -r '.upstream_base' "$SRC_DIR/MANIFEST-SOURCE.json")
[[ -z "$BUILD_VERSION" || "$BUILD_VERSION" == "null" ]] && \
  BUILD_VERSION=$(jq -r '.bundle_version' "$SRC_DIR/MANIFEST-SOURCE.json")

echo "==> Building from source"
echo "    Source bundle version: $BUILD_VERSION"
echo "    Upstream base:         $UPSTREAM_TAG"
echo ""

# --- Step 1: Prepare upstream workspace ---
if [[ "$FRESH_CLONE" == "1" ]]; then
  UPSTREAM_DIR=$(mktemp -d -t goclaw-upstream-XXXXXX)
  if [[ "$KEEP_TEMP" != "1" ]]; then
    trap 'rm -rf "$UPSTREAM_DIR"' EXIT
  fi
  echo "==> Cloning upstream $UPSTREAM_TAG..."
  git clone --depth 1 --branch "$UPSTREAM_TAG" \
    https://github.com/nextlevelbuilder/goclaw.git "$UPSTREAM_DIR"
  echo "  ✓ Cloned to $UPSTREAM_DIR"
elif [[ -z "$UPSTREAM_DIR" ]]; then
  echo "❌ Cần truyền --upstream-dir /path HOẶC --fresh-clone" >&2
  exit 1
fi

[[ -d "$UPSTREAM_DIR" ]] || { echo "❌ UPSTREAM_DIR not a directory: $UPSTREAM_DIR" >&2; exit 1; }

# --- Step 2: Verify patches applicable ---
echo ""
echo "==> Verify patches"
bash "$SCRIPT_DIR/verify-patches.sh" "$UPSTREAM_DIR" "$SRC_DIR/patches"

# --- Step 3: Copy new (non-patched) files ---
echo ""
echo "==> Copy new feature files"
cp -a "$SRC_DIR/source/facebookmessenger" "$UPSTREAM_DIR/internal/channels/"
cp -a "$SRC_DIR/source/mautrix-meta-shim" "$UPSTREAM_DIR/sidecar/"
if [[ -d "$SRC_DIR/source/facebook-messenger" ]]; then
  mkdir -p "$UPSTREAM_DIR/ui/web/src/pages/channels/"
  cp -a "$SRC_DIR/source/facebook-messenger" "$UPSTREAM_DIR/ui/web/src/pages/channels/"
fi
if [[ -d "$SRC_DIR/source/fbm-diagnose" ]]; then
  mkdir -p "$UPSTREAM_DIR/cmd/"
  cp -a "$SRC_DIR/source/fbm-diagnose" "$UPSTREAM_DIR/cmd/"
fi
echo "  ✓ New files copied"

# --- Step 4: Merge i18n JSON fragments ---
echo ""
echo "==> Merge i18n fragments"
for locale in en vi zh; do
  frag="$SRC_DIR/source/i18n-fragments/$locale-channels.json"
  target="$UPSTREAM_DIR/ui/web/src/i18n/locales/$locale/channels.json"
  if [[ -f "$frag" && -f "$target" ]]; then
    jq -s '.[0] * .[1]' "$target" "$frag" > /tmp/fbm-merged.json
    mv /tmp/fbm-merged.json "$target"
    echo "  ✓ Merged $locale-channels.json"
  fi
done

# --- Step 5: Apply patches ---
echo ""
echo "==> Apply patches"
bash "$SCRIPT_DIR/apply-patches.sh" "$UPSTREAM_DIR" "$SRC_DIR/patches"

# --- Step 6: Build 3 images ---
echo ""
echo "==> Build Docker images (this takes 5-15 minutes)"
cd "$UPSTREAM_DIR"

echo "  → Building goclaw-fork:$BUILD_VERSION"
docker build \
  --build-arg "VERSION=$BUILD_VERSION" \
  --build-arg "ENABLE_EMBEDUI=false" \
  --build-arg "ENABLE_PYTHON=true" \
  -t "goclaw-fork:$BUILD_VERSION" \
  . 2>&1 | tail -3

echo "  → Building goclaw-web-fork:$BUILD_VERSION"
docker build -t "goclaw-web-fork:$BUILD_VERSION" ui/web/ 2>&1 | tail -3

echo "  → Building fbm-sidecar:$BUILD_VERSION"
docker build -t "fbm-sidecar:$BUILD_VERSION" sidecar/mautrix-meta-shim/ 2>&1 | tail -3

# --- Step 7: Summary ---
echo ""
echo "✅ Built 3 images:"
docker images | grep -E "goclaw-fork|goclaw-web-fork|fbm-sidecar" | grep "$BUILD_VERSION"

echo ""
echo "Next steps:"
echo "  1. (Optional) Export to tar bundle: docker save ... | zstd > ..."
echo "  2. Install with existing bundle install script (skip image load step):"
echo "     bash install/install-fbm-bundle.sh --force"
echo "  3. Or wire into your compose directly with pull_policy: never"
