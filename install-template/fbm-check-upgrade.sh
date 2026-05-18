#!/usr/bin/env bash
# fbm-check-upgrade.sh — verify FBM bundle health vs the current GoClaw deployment.
# Designed to run after a `docker compose pull` or `goclaw upgrade` on recipient host.
#
# Exit codes:
#   0 — everything healthy
#   1 — warnings (non-blocking; feature still works but attention recommended)
#   2 — errors (FBM broken, action required)
#
# Usage: fbm-check-upgrade.sh [GOCLAW_DIR]

set -euo pipefail

GOCLAW_DIR="${1:-/opt/goclaw}"
MARKER="$GOCLAW_DIR/.fbm-bundle-installed"

# Not installed → nothing to check (success)
if [[ ! -f "$MARKER" ]]; then
  echo "ℹ  FBM bundle chưa được cài tại $GOCLAW_DIR (không có marker)."
  exit 0
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "⚠  jq chưa cài — không đọc được marker JSON"
  exit 1
fi

INSTALLED_VER=$(jq -r '.bundle_version' "$MARKER")
INSTALLED_UPSTREAM=$(jq -r '.goclaw_upstream_version' "$MARKER")
INSTALL_DATE=$(jq -r '.install_date' "$MARKER")
RECORDED_HASH=$(jq -r '.upstream_compose_hash // ""' "$MARKER")
SIDECAR_PORT=$(jq -r '.sidecar_port // "29320"' "$MARKER")

WARNINGS=()
ERRORS=()

# 1. Check fork images present
for img in goclaw-fork goclaw-web-fork fbm-sidecar; do
  if ! docker image inspect "$img:$INSTALLED_VER" >/dev/null 2>&1; then
    ERRORS+=("Missing fork image: $img:$INSTALLED_VER")
  fi
done

# 2. Check containers running fork images
GOCLAW_CONTAINER_IMG=$(docker inspect goclaw-goclaw-1 --format '{{.Config.Image}}' 2>/dev/null || echo "")
if [[ -n "$GOCLAW_CONTAINER_IMG" ]]; then
  if [[ "$GOCLAW_CONTAINER_IMG" != "goclaw-fork:$INSTALLED_VER" ]]; then
    ERRORS+=("Container goclaw-goclaw-1 uses image '$GOCLAW_CONTAINER_IMG' but FBM expects 'goclaw-fork:$INSTALLED_VER'")
    ERRORS+=("→ Cause: upstream pull overwrote override. Fix: re-run install-fbm-bundle.sh --force")
  fi
fi

UI_CONTAINER_IMG=$(docker inspect goclaw-goclaw-ui-1 --format '{{.Config.Image}}' 2>/dev/null || echo "")
if [[ -n "$UI_CONTAINER_IMG" && "$UI_CONTAINER_IMG" != "goclaw-web-fork:$INSTALLED_VER" ]]; then
  ERRORS+=("Container goclaw-goclaw-ui-1 uses image '$UI_CONTAINER_IMG' but FBM expects 'goclaw-web-fork:$INSTALLED_VER'")
fi

# 3. Check sidecar running + healthy
SIDECAR_STATE=$(docker inspect fbm-sidecar --format '{{.State.Health.Status}}' 2>/dev/null || echo "missing")
case "$SIDECAR_STATE" in
  healthy) : ;;  # OK
  missing) ERRORS+=("fbm-sidecar container not found") ;;
  starting) WARNINGS+=("fbm-sidecar still starting up") ;;
  unhealthy) ERRORS+=("fbm-sidecar is unhealthy — check docker logs fbm-sidecar") ;;
  *) WARNINGS+=("fbm-sidecar unknown state: $SIDECAR_STATE") ;;
esac

# 4. Check upstream compose files hash drift
if [[ -n "$RECORDED_HASH" ]]; then
  CURRENT_HASH=$(cat \
    "$GOCLAW_DIR"/docker-compose.yml \
    "$GOCLAW_DIR"/docker-compose.postgres.yml \
    "$GOCLAW_DIR"/docker-compose.selfservice.yml \
    2>/dev/null | sha256sum | awk '{print $1}' || echo "")
  if [[ -n "$CURRENT_HASH" && "$RECORDED_HASH" != "$CURRENT_HASH" ]]; then
    WARNINGS+=("Upstream compose files đã thay đổi từ lúc cài FBM ($INSTALL_DATE)")
    WARNINGS+=("→ Khuyến nghị: rebuild bundle từ upstream mới nhất, rồi install --force")
  fi
fi

# 5. Check sidecar can be reached via localhost
if command -v curl >/dev/null 2>&1; then
  if ! curl -sfI "http://localhost:$SIDECAR_PORT/healthz" --max-time 3 \
       -H "Authorization: Bearer invalid" >/dev/null 2>&1; then
    # Expect 401 on invalid token — unreachable means network issue
    if ! curl -sI "http://localhost:$SIDECAR_PORT/healthz" --max-time 3 >/dev/null 2>&1; then
      WARNINGS+=("Sidecar không reachable tại localhost:$SIDECAR_PORT (có thể chưa fully started)")
    fi
  fi
fi

# --- Emit report ---
echo "=== FBM Upgrade Check ==="
echo "Bundle version:      $INSTALLED_VER"
echo "Installed on:        $INSTALL_DATE"
echo "Upstream base:       $INSTALLED_UPSTREAM"
echo ""

if [[ ${#ERRORS[@]} -eq 0 && ${#WARNINGS[@]} -eq 0 ]]; then
  echo "✅ FBM bundle healthy. Không cần hành động."
  exit 0
fi

if [[ ${#WARNINGS[@]} -gt 0 ]]; then
  echo "Warnings:"
  for w in "${WARNINGS[@]}"; do
    echo "  ⚠  $w"
  done
  echo ""
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
  echo "Errors (cần sửa):"
  for e in "${ERRORS[@]}"; do
    echo "  ❌ $e"
  done
  echo ""
  echo "Quick fixes:"
  echo "  1. Cài lại bundle:   sudo bash /path/to/bundle/install/install-fbm-bundle.sh --force"
  echo "  2. Build lại source: bash /path/to/source/install/build-fbm-from-source.sh --upstream-dir $GOCLAW_DIR"
  echo "  3. Gỡ FBM:           sudo bash /path/to/bundle/install/uninstall-fbm-bundle.sh $GOCLAW_DIR"
  exit 2
fi

exit 1
