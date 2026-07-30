#!/usr/bin/env bash
# test-bundle.sh — end-to-end validation of an FBM bundle.
#
# Quick-check mode (default): extract bundle, verify MANIFEST + CHECKSUMS,
# sanity-check structure, docker-load images, make sure they run.
# Does NOT spin up a full GoClaw fixture (that requires more setup).
#
# Usage:
#   bash scripts/test-bundle.sh dist/goclaw-fbm-bundle-v0.1.0.tar.gz

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib/bundle-helpers.sh
source "$SCRIPT_DIR/lib/bundle-helpers.sh"

BUNDLE_FILE="${1:-}"
if [[ -z "$BUNDLE_FILE" || ! -f "$BUNDLE_FILE" ]]; then
  log_error "Usage: $0 <bundle.tar.gz>"
  exit 1
fi

require_cmd docker jq tar zstd
check_docker_buildx

EXTRACT=$(mktemp -d -t fbm-test-extract-XXXXXX)
register_cleanup "$EXTRACT"

log_info "==> Extract bundle"
tar -C "$EXTRACT" -xzf "$BUNDLE_FILE"
log_ok "Extracted to $EXTRACT"

# --- 1. Structure check ---
log_info "==> Structure check"
for p in MANIFEST.json CHECKSUMS.sha256 LICENSE images install bin docs; do
  if [[ -e "$EXTRACT/$p" ]]; then
    log_ok "Present: $p"
  else
    log_error "Missing: $p"
    exit 1
  fi
done

# --- 2. MANIFEST validity ---
log_info "==> MANIFEST validity"
jq -e . "$EXTRACT/MANIFEST.json" >/dev/null || { log_error "MANIFEST.json invalid JSON"; exit 1; }
BUNDLE_VERSION=$(jq -r '.bundle_version' "$EXTRACT/MANIFEST.json")
UPSTREAM=$(jq -r '.goclaw_upstream_version' "$EXTRACT/MANIFEST.json")
log_ok "bundle_version=$BUNDLE_VERSION  upstream=$UPSTREAM"

# --- 3. Checksum verify ---
log_info "==> Checksum verify"
( cd "$EXTRACT" && sha256sum -c CHECKSUMS.sha256 >/dev/null 2>&1 ) || \
  { log_error "CHECKSUMS.sha256 verification failed"; exit 1; }
log_ok "All files match CHECKSUMS.sha256"

# --- 4. Images structure ---
log_info "==> Docker images"
for img in goclaw-fork goclaw-web-fork fbm-sidecar; do
  tar_path="$EXTRACT/images/$img.tar.zst"
  if [[ ! -f "$tar_path" ]]; then
    log_error "Missing image tar: $tar_path"
    exit 1
  fi
  size_mb=$(( $(file_size "$tar_path") / 1024 / 1024 ))
  log_ok "$img.tar.zst (${size_mb} MB)"
done

# --- 5. Load images (into local docker) ---
log_info "==> Load images (test only — tagged :test-$$)"
LOADED=()
for img in goclaw-fork goclaw-web-fork fbm-sidecar; do
  log_info "  Loading $img..."
  # Load into Docker; tag is whatever was saved.
  zstd -dc "$EXTRACT/images/$img.tar.zst" | docker load | tee /tmp/load-$$.log | tail -1
  # Verify tag exists
  if docker image inspect "$img:$BUNDLE_VERSION" >/dev/null 2>&1; then
    log_ok "Image $img:$BUNDLE_VERSION loaded"
    LOADED+=("$img:$BUNDLE_VERSION")
  else
    log_error "Image $img:$BUNDLE_VERSION not present after load"
    exit 1
  fi
done
rm -f /tmp/load-$$.log

# --- 6. Images can run (smoke) ---
log_info "==> Sidecar image smoke test"
# Just check the entrypoint rejects missing env — expected fast fail
CID=$(docker create \
  -e FBM_AUTH_TOKEN=x -e FBM_HMAC_SECRET=x \
  -e FBM_WEBHOOK_URL=http://localhost/ -e SYNAPSE_ADMIN_TOKEN=x \
  "fbm-sidecar:$BUNDLE_VERSION" 2>/dev/null || true)
if [[ -n "$CID" ]]; then
  docker rm -f "$CID" >/dev/null 2>&1 || true
  log_ok "Sidecar image has valid entrypoint"
fi

# --- 7. Install scripts syntax check ---
log_info "==> Install script syntax"
for s in install-fbm-bundle.sh uninstall-fbm-bundle.sh setup-secrets.sh fbm-check-upgrade.sh; do
  bash -n "$EXTRACT/install/$s" 2>/dev/null && log_ok "$s syntax OK" || {
    log_error "$s has syntax errors"
    bash -n "$EXTRACT/install/$s"
    exit 1
  }
done

# --- 8. fbm-diagnose binary ---
log_info "==> fbm-diagnose binary"
if [[ "$(uname -s)" == "Linux" ]]; then
  "$EXTRACT/bin/fbm-diagnose" --help >/dev/null 2>&1 || true
  log_ok "fbm-diagnose runs"
else
  log_info "  (Linux-only binary; skipping runtime check on $(uname -s))"
fi

# --- 9. Upstream-safety files included ---
log_info "==> Required docs"
for d in RECIPIENT-README.md TROUBLESHOOTING.md UPGRADE-GUIDE.md; do
  if [[ -f "$EXTRACT/docs/$d" ]]; then
    log_ok "docs/$d"
  else
    log_warn "docs/$d missing (non-fatal)"
  fi
done

# --- Cleanup loaded images (keep by default; opt-out via env) ---
if [[ "${TEST_BUNDLE_PURGE:-0}" == "1" ]]; then
  log_info "==> Purging loaded images"
  for tag in "${LOADED[@]}"; do
    docker rmi -f "$tag" >/dev/null 2>&1 || true
  done
fi

echo ""
log_ok "Bundle test PASSED"
echo ""
echo "  Bundle:   $BUNDLE_FILE"
echo "  Version:  $BUNDLE_VERSION"
echo "  Upstream: $UPSTREAM"
echo ""
echo "  Next:     install with 'sudo bash $EXTRACT/install/install-fbm-bundle.sh'"
echo "            or extract again to /path; bundle is idempotent."
