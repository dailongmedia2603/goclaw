#!/usr/bin/env bash
# uninstall-fbm-bundle.sh — remove FBM bundle cleanly, restore upstream state.
# Leaves existing channels (Telegram, Discord, Zalo, etc.) untouched.
# Downtime: < 30s.
#
# Usage:
#   sudo bash uninstall-fbm-bundle.sh [GOCLAW_DIR] [--purge]
#
# --purge → also remove fork images (saves disk; normally keep for quick re-install)

set -euo pipefail

GOCLAW_DIR="${1:-/opt/goclaw}"
PURGE=0
for arg in "$@"; do
  [[ "$arg" == "--purge" ]] && PURGE=1
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib/compose-detect.sh
source "$SCRIPT_DIR/lib/compose-detect.sh" 2>/dev/null || true

MARKER="$GOCLAW_DIR/.fbm-bundle-installed"
if [[ ! -f "$MARKER" ]]; then
  echo "ℹ  FBM bundle not installed at $GOCLAW_DIR (no marker file). Nothing to do."
  exit 0
fi

BUNDLE_VERSION=$(jq -r '.bundle_version // "unknown"' "$MARKER")
BAK_PATH=$(jq -r '.compose_backup_path // ""' "$MARKER")

echo "==> Uninstalling FBM bundle $BUNDLE_VERSION from $GOCLAW_DIR"

# 1. Stop + remove sidecar first (fast)
echo "  → Stopping fbm-sidecar"
if [[ -f "$GOCLAW_DIR/docker-compose.fbm.yml" ]]; then
  cd "$GOCLAW_DIR"
  docker compose -f docker-compose.fbm.yml stop fbm-sidecar 2>/dev/null || true
  docker compose -f docker-compose.fbm.yml rm -f fbm-sidecar 2>/dev/null || true
fi
docker rm -f fbm-sidecar 2>/dev/null || true

# 2. Remove FBM compose file(s)
echo "  → Removing FBM compose overlay"
rm -f "$GOCLAW_DIR/docker-compose.fbm.yml" "$GOCLAW_DIR/docker-compose.fbm-cpu.yml"

# 3. Restore backed-up override.yml (if exists)
if [[ -n "$BAK_PATH" && -f "$BAK_PATH" ]]; then
  echo "  → Restoring override from $BAK_PATH"
  cp -a "$BAK_PATH" "$GOCLAW_DIR/docker-compose.override.yml"
else
  # No backup → the install added `image: goclaw-fork:...` into override;
  # if override exists, strip the FBM-specific lines; else do nothing.
  if [[ -f "$GOCLAW_DIR/docker-compose.override.yml" ]]; then
    if grep -q "goclaw-fork\|goclaw-web-fork\|fbm-sidecar" "$GOCLAW_DIR/docker-compose.override.yml"; then
      echo "  ⚠  override.yml appears modified by FBM but no backup — leaving as-is"
      echo "      (manually restore or delete if needed)"
    fi
  fi
fi

# 4. Restart goclaw + goclaw-ui with upstream images
echo "  → Restarting goclaw + goclaw-ui (upstream images)"
cd "$GOCLAW_DIR"
# shellcheck disable=SC2046
COMPOSE_ARGS=($(detect_compose_files "$GOCLAW_DIR" 2>/dev/null || echo "-f docker-compose.yml"))
docker compose "${COMPOSE_ARGS[@]}" up -d --no-build goclaw goclaw-ui 2>&1 | tail -10

# 5. Remove .env.fbm (keep backup in case reinstall)
if [[ -f "$GOCLAW_DIR/.env.fbm" ]]; then
  mv "$GOCLAW_DIR/.env.fbm" "$GOCLAW_DIR/.env.fbm.uninstalled-$(date +%Y%m%d%H%M%S)"
  echo "  → Moved .env.fbm to .env.fbm.uninstalled-* (not deleted — for easy reinstall)"
fi

# 6. Remove marker
rm -f "$MARKER"

# 7. Optional: purge fork images
if [[ "$PURGE" == "1" ]]; then
  echo "  → Purging fork images"
  for img in "goclaw-fork:$BUNDLE_VERSION" "goclaw-web-fork:$BUNDLE_VERSION" "fbm-sidecar:$BUNDLE_VERSION"; do
    docker rmi -f "$img" 2>/dev/null || true
  done
fi

# 8. Remove sidecar volume (user data)
if [[ "$PURGE" == "1" ]]; then
  docker volume rm -f goclaw_fbm-sidecar-data 2>/dev/null || true
fi

echo ""
echo "✅ FBM bundle uninstalled successfully."
echo "   Existing channels (Telegram/Discord/Zalo/...) unaffected."
if [[ "$PURGE" != "1" ]]; then
  echo ""
  echo "   Fork images retained. To free disk: re-run with --purge"
fi
