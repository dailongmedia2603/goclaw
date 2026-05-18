#!/usr/bin/env bash
# Pre-flight checks for install-fbm-bundle.sh. Sourced only.
# Each function exits 1 with a Vietnamese error on failure.
# shellcheck shell=bash

check_docker() {
  command -v docker >/dev/null 2>&1 || {
    echo "❌ Docker chưa được cài. Xem: https://docs.docker.com/engine/install/" >&2
    exit 1
  }
  local ver
  ver=$(docker version --format '{{.Server.Version}}' 2>/dev/null || echo "0.0.0")
  if ! printf '%s\n%s\n' "24.0.0" "$ver" | sort -V -C 2>/dev/null; then
    echo "❌ Docker $ver quá cũ (cần ≥ 24.0.0). Cập nhật: https://docs.docker.com/engine/install/" >&2
    exit 1
  fi
  echo "  ✓ Docker: $ver"
}

check_compose() {
  docker compose version >/dev/null 2>&1 || {
    echo "❌ Docker Compose v2 chưa có. Xem: https://docs.docker.com/compose/install/" >&2
    exit 1
  }
  local ver
  ver=$(docker compose version --short 2>/dev/null || echo "0.0.0")
  if ! printf '%s\n%s\n' "2.20.0" "$ver" | sort -V -C 2>/dev/null; then
    echo "❌ Compose $ver quá cũ (cần ≥ 2.20.0)" >&2
    exit 1
  fi
  echo "  ✓ Compose: $ver"
}

check_disk() {
  local dir="$1" avail_gb
  if [[ "$(uname)" == "Darwin" ]]; then
    avail_gb=$(df -g "$dir" | awk 'NR==2 {print $4}')
  else
    avail_gb=$(df -BG "$dir" | awk 'NR==2 {gsub("G","",$4); print $4}')
  fi
  if [[ "${avail_gb:-0}" -lt 3 ]]; then
    echo "❌ Đĩa chỉ còn ${avail_gb}GB (cần ≥ 3GB). Dọn: docker system prune -a" >&2
    exit 1
  fi
  echo "  ✓ Disk: ${avail_gb}GB free"
}

check_ram() {
  local ram_mb
  if [[ "$(uname)" == "Darwin" ]]; then
    ram_mb=$(( $(sysctl -n hw.memsize 2>/dev/null || echo 0) / 1024 / 1024 ))
  else
    ram_mb=$(awk '/MemTotal/ {printf "%d", $2/1024}' /proc/meminfo 2>/dev/null || echo 0)
  fi
  if [[ "$ram_mb" -lt 2048 ]]; then
    echo "❌ RAM ${ram_mb}MB (cần ≥ 2048MB) cho Synapse + mautrix-meta" >&2
    exit 1
  fi
  echo "  ✓ RAM: ${ram_mb}MB"
}

check_ports() {
  local ports=("$@")
  for port in "${ports[@]}"; do
    if port_in_use "$port"; then
      echo "❌ Port $port đang bị chiếm. Dùng lệnh sau để xem:" >&2
      echo "     sudo ss -tlnp | grep :$port   # Linux" >&2
      echo "     sudo lsof -i :$port           # macOS" >&2
      exit 1
    fi
  done
  echo "  ✓ Ports (${ports[*]}) free"
}

port_in_use() {
  local port="$1"
  if command -v ss >/dev/null 2>&1; then
    ss -tln 2>/dev/null | awk '{print $4}' | grep -qE "[.:]${port}\$"
  elif command -v lsof >/dev/null 2>&1; then
    lsof -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1
  else
    return 1
  fi
}

check_docker_access() {
  if ! docker ps >/dev/null 2>&1; then
    echo "❌ Không chạy được 'docker ps'. Chạy script với sudo HOẶC thêm user vào group docker:" >&2
    echo "     sudo usermod -aG docker \$USER && newgrp docker" >&2
    exit 1
  fi
  echo "  ✓ Docker access OK"
}

# detect_goclaw_dir [HINT] → echo path or exit 1
detect_goclaw_dir() {
  local hint="${1:-}" candidate
  local candidates=()
  [[ -n "$hint" ]] && candidates+=("$hint")
  [[ -n "${GOCLAW_DIR:-}" ]] && candidates+=("$GOCLAW_DIR")
  candidates+=(/opt/goclaw /srv/goclaw "$HOME/goclaw")

  for candidate in "${candidates[@]}"; do
    [[ -z "$candidate" ]] && continue
    if [[ -f "$candidate/docker-compose.yml" ]]; then
      echo "$candidate"
      return 0
    fi
  done

  echo "❌ Không tìm thấy GoClaw installation." >&2
  echo "   Đã tìm: ${candidates[*]}" >&2
  echo "   Truyền --goclaw-dir /path/to/goclaw" >&2
  exit 1
}

verify_bundle_integrity() {
  local bundle_dir="$1"
  if [[ ! -f "$bundle_dir/CHECKSUMS.sha256" ]]; then
    echo "❌ Bundle thiếu CHECKSUMS.sha256 — file có thể không phải bundle FBM hợp lệ" >&2
    exit 1
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    ( cd "$bundle_dir" && sha256sum -c CHECKSUMS.sha256 ) >/dev/null 2>&1 || {
      echo "❌ Checksum verification FAILED. Bundle corrupt. Tải lại." >&2
      exit 1
    }
  elif command -v shasum >/dev/null 2>&1; then
    ( cd "$bundle_dir" && shasum -a 256 -c CHECKSUMS.sha256 ) >/dev/null 2>&1 || {
      echo "❌ Checksum verification FAILED. Bundle corrupt. Tải lại." >&2
      exit 1
    }
  else
    echo "⚠  sha256sum/shasum không có — skip integrity check" >&2
  fi
  echo "  ✓ Bundle integrity verified"
}
