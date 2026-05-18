#!/usr/bin/env bash
# install-fbm-bundle.sh — install the Facebook Messenger Personal channel
# on an existing GoClaw docker-compose deployment.
#
# Safe: additive, idempotent, rollback available via uninstall-fbm-bundle.sh.
# Does NOT touch existing channel configurations (Telegram, Discord, etc.).
#
# Usage:
#   sudo bash install-fbm-bundle.sh [--goclaw-dir /path] [--force] [--dry-run] [--skip-interactive]

set -euo pipefail
set +H  # disable history expansion (secrets safety)
umask 0022

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUNDLE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# shellcheck source=./lib/preflight-checks.sh
source "$SCRIPT_DIR/lib/preflight-checks.sh"
# shellcheck source=./lib/compose-detect.sh
source "$SCRIPT_DIR/lib/compose-detect.sh"

# --- CLI args ---
GOCLAW_DIR_HINT=""
FORCE=0
DRY_RUN=0
SKIP_INTERACTIVE=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --goclaw-dir) GOCLAW_DIR_HINT="$2"; shift 2 ;;
    --force) FORCE=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    --skip-interactive) SKIP_INTERACTIVE=1; shift ;;
    -h|--help)
      sed -n '3,13p' "$0" | sed 's/^# //; s/^#//'
      exit 0 ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

# --- Locking (prevent concurrent installs) ---
LOCKFILE="/tmp/fbm-install.lock"
exec 9>"$LOCKFILE"
if ! flock -n 9 2>/dev/null; then
  echo "❌ Another install is running (lockfile: $LOCKFILE)" >&2
  exit 1
fi

cat <<'BANNER'

┌─────────────────────────────────────────────────────────────┐
│  Facebook Messenger (Personal) — Bundle Installer          │
│                                                             │
│  ⚠  Experimental feature. Violates Meta ToS §3.2.3.        │
│  Your Facebook account may be banned. Use burn accounts.    │
│                                                             │
└─────────────────────────────────────────────────────────────┘

BANNER

# --- Read bundle MANIFEST ---
if [[ ! -f "$BUNDLE_DIR/MANIFEST.json" ]]; then
  echo "❌ Bundle MANIFEST.json missing at $BUNDLE_DIR/MANIFEST.json" >&2
  echo "   Are you running this from the extracted bundle directory?" >&2
  exit 1
fi

BUNDLE_VERSION=$(jq -r '.bundle_version' "$BUNDLE_DIR/MANIFEST.json")
UPSTREAM_VERSION=$(jq -r '.goclaw_upstream_version' "$BUNDLE_DIR/MANIFEST.json")

echo "Bundle version:   $BUNDLE_VERSION"
echo "Upstream base:    $UPSTREAM_VERSION"
echo ""

# --- Pre-flight ---
echo "==> Pre-flight checks"
check_docker
check_compose
check_docker_access

GOCLAW_DIR=$(detect_goclaw_dir "$GOCLAW_DIR_HINT")
echo "  ✓ GoClaw dir: $GOCLAW_DIR"

check_disk "$GOCLAW_DIR"
check_ram
check_ports "$(grep -E '^FBM_PORT=' "$BUNDLE_DIR/install/.env.fbm.template" | cut -d= -f2)"
verify_bundle_integrity "$BUNDLE_DIR"
echo ""

# --- Marker check (idempotency) ---
MARKER="$GOCLAW_DIR/.fbm-bundle-installed"
if [[ -f "$MARKER" ]]; then
  INSTALLED_VER=$(jq -r '.bundle_version' "$MARKER")
  if [[ "$INSTALLED_VER" == "$BUNDLE_VERSION" && "$FORCE" != "1" ]]; then
    echo "✓ Bundle $BUNDLE_VERSION đã được cài (nothing to do)."
    echo "  Để cài lại: thêm flag --force"
    exit 0
  fi
  if [[ "$INSTALLED_VER" != "$BUNDLE_VERSION" ]]; then
    echo "ℹ  Đang nâng cấp $INSTALLED_VER → $BUNDLE_VERSION"
  fi
fi

# --- Confirmation ---
if [[ "$SKIP_INTERACTIVE" != "1" && -t 0 ]]; then
  echo ""
  echo "Sắp thực hiện:"
  echo "  1. Backup docker-compose.override.yml (nếu có)"
  echo "  2. Load 3 Docker images (goclaw-fork, goclaw-web-fork, fbm-sidecar)"
  echo "  3. Tạo file $GOCLAW_DIR/.env.fbm (secrets)"
  echo "  4. Thêm docker-compose.fbm.yml overlay"
  echo "  5. Restart goclaw + goclaw-ui, start fbm-sidecar"
  echo ""
  read -r -p "Tiếp tục? [y/N] " reply
  [[ "$reply" =~ ^[Yy]$ ]] || { echo "Cancelled."; exit 0; }
fi

if [[ "$DRY_RUN" == "1" ]]; then
  echo "DRY RUN — không thay đổi gì. Exit."
  exit 0
fi

# --- Step 1: Backup existing override ---
echo ""
echo "==> Backup compose override"
BAK_TS=$(date +%Y%m%d%H%M%S)
BAK_PATH=""
if [[ -f "$GOCLAW_DIR/docker-compose.override.yml" ]]; then
  BAK_PATH="$GOCLAW_DIR/docker-compose.override.yml.bak-fbm-$BAK_TS"
  cp -a "$GOCLAW_DIR/docker-compose.override.yml" "$BAK_PATH"
  echo "  ✓ Backed up to $BAK_PATH"
else
  echo "  ℹ No existing override.yml to back up"
fi

# --- Step 2: Load Docker images ---
echo ""
echo "==> Load Docker images"
for img_tar in "$BUNDLE_DIR/images"/*.tar.zst; do
  [[ -f "$img_tar" ]] || { echo "❌ No image tar found in $BUNDLE_DIR/images/"; exit 1; }
  echo "  Loading $(basename "$img_tar")..."
  zstd -dc "$img_tar" | docker load | tail -1
done

# Verify 3 images present
for img in goclaw-fork goclaw-web-fork fbm-sidecar; do
  if ! docker image inspect "$img:$BUNDLE_VERSION" >/dev/null 2>&1; then
    echo "❌ Image $img:$BUNDLE_VERSION missing after load" >&2
    exit 1
  fi
done
echo "  ✓ All 3 images loaded"

# --- Step 3: Generate secrets ---
echo ""
echo "==> Generate secrets"
bash "$SCRIPT_DIR/setup-secrets.sh" "$GOCLAW_DIR"

# --- Step 4: Write compose overlay ---
echo ""
echo "==> Install compose overlay"
cp -a "$SCRIPT_DIR/docker-compose.fbm.yml" "$GOCLAW_DIR/docker-compose.fbm.yml"
echo "  ✓ Wrote $GOCLAW_DIR/docker-compose.fbm.yml"

# Update .env.fbm with bundle version + ensure FBM_BUNDLE_VERSION present
ENV_FILE="$GOCLAW_DIR/.env.fbm"
if grep -q '^FBM_BUNDLE_VERSION=' "$ENV_FILE"; then
  sed -i.tmp "s/^FBM_BUNDLE_VERSION=.*/FBM_BUNDLE_VERSION=$BUNDLE_VERSION/" "$ENV_FILE"
  rm -f "$ENV_FILE.tmp"
else
  echo "FBM_BUNDLE_VERSION=$BUNDLE_VERSION" >> "$ENV_FILE"
fi

# --- Step 4.5: CPU-limit conflict mitigation (1-core VPS etc) ---
if detect_cpu_limit_conflict "$GOCLAW_DIR"; then
  echo "  ⚠  CPU limit conflict detected (compose declares > host CPUs)"
  HOST_CPUS=$(get_host_cpus)
  SAFE_CPUS=$(awk -v h="$HOST_CPUS" 'BEGIN { printf "%.2f", h * 0.95 }')
  # Append CPU override to docker-compose.fbm.yml
  cat >> "$GOCLAW_DIR/docker-compose.fbm.yml" <<EOF

# Auto-added by install-fbm-bundle.sh on $(date -u +%Y-%m-%dT%H:%M:%SZ)
# Host has $HOST_CPUS CPU(s); base compose declares > 1 — cap to $SAFE_CPUS to avoid Docker error
x-cpu-override: &cpu-override
  deploy:
    resources:
      limits:
        cpus: "$SAFE_CPUS"
EOF
  # Apply via YAML merge — simplest: write a supplementary file
  cat > "$GOCLAW_DIR/docker-compose.fbm-cpu.yml" <<EOF
services:
  goclaw:
    deploy:
      resources:
        limits:
          cpus: "$SAFE_CPUS"
  goclaw-ui:
    deploy:
      resources:
        limits:
          cpus: "$SAFE_CPUS"
EOF
  echo "  ✓ Added CPU safety cap ($SAFE_CPUS)"
fi

# --- Step 5: Compose up ---
echo ""
echo "==> Restart goclaw + goclaw-ui + start fbm-sidecar"
cd "$GOCLAW_DIR"
# shellcheck disable=SC2046
COMPOSE_BASE_FILES=($(detect_compose_files "$GOCLAW_DIR"))

EXTRA_FILES=(-f docker-compose.fbm.yml)
[[ -f docker-compose.fbm-cpu.yml ]] && EXTRA_FILES+=(-f docker-compose.fbm-cpu.yml)

# Reload env vars before compose up
set -a
# shellcheck disable=SC1091
source "$GOCLAW_DIR/.env.fbm"
set +a

docker compose "${COMPOSE_BASE_FILES[@]}" "${EXTRA_FILES[@]}" \
  --env-file "$GOCLAW_DIR/.env.fbm" \
  up -d --no-build \
  goclaw goclaw-ui fbm-sidecar

# --- Step 6: Wait healthy ---
echo ""
echo "==> Wait for healthy (up to 120s)"
HEALTHY=0
for i in $(seq 1 40); do
  sleep 3
  STATUS=$(docker inspect fbm-sidecar --format '{{.State.Health.Status}}' 2>/dev/null || echo "starting")
  if [[ "$STATUS" == "healthy" ]]; then
    HEALTHY=1
    break
  fi
  [[ $((i % 5)) == 0 ]] && echo "  ... ($((i*3))s, sidecar=$STATUS)"
done

if [[ "$HEALTHY" != "1" ]]; then
  echo "⚠  fbm-sidecar not healthy after 120s. Recent logs:" >&2
  docker compose logs --tail=30 fbm-sidecar 2>&1 | tail -30 >&2
  echo ""
  echo "  Installation finished but sidecar not yet healthy." >&2
  echo "  Check: docker logs fbm-sidecar -f" >&2
  echo "  Or rollback: sudo bash $SCRIPT_DIR/uninstall-fbm-bundle.sh $GOCLAW_DIR" >&2
  # Don't fail — could just be slow first boot
fi

# --- Step 7: Compute upstream compose hash (for upgrade detection) ---
UPSTREAM_HASH=$(cat "${COMPOSE_BASE_FILES[@]/-f/$GOCLAW_DIR\/}" 2>/dev/null | sha256sum | awk '{print $1}' || echo "")

# --- Step 8: Write install marker ---
cat > "$MARKER" <<EOF
{
  "bundle_version": "$BUNDLE_VERSION",
  "install_date": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "goclaw_upstream_version": "$UPSTREAM_VERSION",
  "sidecar_port": "${FBM_PORT:-29320}",
  "instance_name": "${FBM_INSTANCE_NAME:-default}",
  "compose_backup_path": "$BAK_PATH",
  "upstream_compose_hash": "$UPSTREAM_HASH",
  "installer_script": "$SCRIPT_DIR/install-fbm-bundle.sh"
}
EOF
chmod 644 "$MARKER"

# --- Step 9: Final diagnostics ---
if [[ -x "$BUNDLE_DIR/bin/fbm-diagnose" ]]; then
  echo ""
  echo "==> Post-install diagnostic"
  "$BUNDLE_DIR/bin/fbm-diagnose" \
    --marker "$MARKER" \
    --env "$ENV_FILE" \
    --sidecar-url "http://localhost:${FBM_PORT:-29320}" \
    --gateway-url "http://localhost:18790" \
    2>&1 || true
fi

echo ""
echo "┌─────────────────────────────────────────────────────────────┐"
echo "│  ✅ FBM bundle $BUNDLE_VERSION installed successfully!        │"
echo "│                                                             │"
echo "│  Next steps:                                                │"
echo "│  1. Open GoClaw UI (usually http://localhost:3000)          │"
echo "│  2. HARD REFRESH browser (Cmd/Ctrl + Shift + R)             │"
echo "│  3. Channels → Add → choose 'Facebook Messenger (Personal)' │"
echo "│  4. Credentials:                                            │"
echo "│     - Sidecar URL: http://fbm-sidecar:29320                 │"
echo "│     - Auth Token + HMAC Secret: from $GOCLAW_DIR/.env.fbm   │"
echo "│  5. Paste FB cookies via wizard to login                    │"
echo "│                                                             │"
echo "│  Docs:        $BUNDLE_DIR/docs/RECIPIENT-README.md          │"
echo "│  Marker:      $MARKER                                       │"
echo "│  Rollback:    sudo bash uninstall-fbm-bundle.sh $GOCLAW_DIR │"
echo "└─────────────────────────────────────────────────────────────┘"
