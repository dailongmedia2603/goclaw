#!/usr/bin/env bash
# fbm-post-upgrade.sh — run after `docker compose pull` / `goclaw upgrade`.
# Thin wrapper around fbm-check-upgrade.sh that prints actionable next-step hints.
#
# Add to cron or run manually:
#   bash /path/to/install/fbm-post-upgrade.sh /opt/goclaw

set -euo pipefail
GOCLAW_DIR="${1:-/opt/goclaw}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Nothing installed → nothing to do
if [[ ! -f "$GOCLAW_DIR/.fbm-bundle-installed" ]]; then
  exit 0
fi

echo "=== Post-upgrade FBM check ==="
set +e
bash "$SCRIPT_DIR/fbm-check-upgrade.sh" "$GOCLAW_DIR"
RC=$?
set -e

if [[ $RC -eq 2 ]]; then
  cat <<'EOF'

⚠  FBM cần action sau upgrade upstream

  Upstream GoClaw mới có thể đã thay đổi compose files hoặc schema.
  Cài lại bundle để đồng bộ:

    sudo bash /path/to/bundle/install/install-fbm-bundle.sh --force

  HOẶC build lại từ source:

    bash /path/to/source/install/build-fbm-from-source.sh \
      --upstream-dir /opt/goclaw --version <new>

  Nếu muốn gỡ FBM:

    sudo bash /path/to/bundle/install/uninstall-fbm-bundle.sh /opt/goclaw

EOF
elif [[ $RC -eq 1 ]]; then
  echo ""
  echo "ℹ  FBM hoạt động nhưng có warnings. Kiểm tra chi tiết ở trên."
fi

exit $RC
