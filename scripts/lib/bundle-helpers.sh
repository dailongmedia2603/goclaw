#!/usr/bin/env bash
# Shared functions for the FBM bundle scripts. Source-only (not executable).
#
# Provides: log_*, require_cmd, sha256_file, compute_version, atomic_mv, register_cleanup

# shellcheck shell=bash

# --- Logging ---
log_info()  { printf '\033[0;34m[INFO]\033[0m  %s\n' "$*" >&2; }
log_warn()  { printf '\033[0;33m[WARN]\033[0m  %s\n' "$*" >&2; }
log_error() { printf '\033[0;31m[ERROR]\033[0m %s\n' "$*" >&2; }
log_ok()    { printf '\033[0;32m[OK]\033[0m    %s\n' "$*" >&2; }

# require_cmd CMD [CMD ...] — exit 1 if any missing
require_cmd() {
  local missing=()
  for c in "$@"; do
    command -v "$c" >/dev/null 2>&1 || missing+=("$c")
  done
  if [[ ${#missing[@]} -gt 0 ]]; then
    log_error "missing required command(s): ${missing[*]}"
    log_info  "On macOS: brew install ${missing[*]}"
    log_info  "On Ubuntu: sudo apt-get install -y ${missing[*]}"
    exit 1
  fi
}

# sha256_file FILE → prints hex digest (cross-platform)
sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    log_error "no sha256sum/shasum found"
    exit 1
  fi
}

# compute_version [OVERRIDE] → echo a version string
# Priority: arg → VERSION file → git describe → "0.0.0-dev"
compute_version() {
  if [[ -n "${1:-}" ]]; then
    echo "$1"
    return
  fi
  if [[ -f VERSION ]]; then
    cat VERSION
    return
  fi
  if command -v git >/dev/null 2>&1; then
    git describe --tags --always --dirty 2>/dev/null && return
  fi
  echo "0.0.0-dev"
}

# atomic_mv SRC DST — move via temp name to avoid partial-write readers
atomic_mv() {
  local src="$1" dst="$2"
  mv "$src" "$dst.tmp.$$" && mv "$dst.tmp.$$" "$dst"
}

# human_bytes N → human-readable size (e.g. 512MB)
human_bytes() {
  local bytes="$1"
  if [[ "$bytes" -lt 1024 ]]; then
    printf '%dB' "$bytes"
  elif [[ "$bytes" -lt 1048576 ]]; then
    printf '%dKB' $((bytes / 1024))
  elif [[ "$bytes" -lt 1073741824 ]]; then
    printf '%dMB' $((bytes / 1048576))
  else
    printf '%d.%02dGB' $((bytes / 1073741824)) $(((bytes * 100 / 1073741824) % 100))
  fi
}

# file_size FILE → bytes (cross-platform)
file_size() {
  if [[ "$(uname)" == "Darwin" ]]; then
    stat -f%z "$1"
  else
    stat -c%s "$1"
  fi
}

# --- Cleanup trap ---
_CLEANUP_DIRS=()
_cleanup_on_exit() {
  local rc=$?
  # macOS bash 3 is strict about empty arrays — guard the expansion.
  if [[ ${#_CLEANUP_DIRS[@]} -gt 0 ]]; then
    for d in "${_CLEANUP_DIRS[@]}"; do
      [[ -d "$d" ]] && rm -rf "$d"
    done
  fi
  exit "$rc"
}
trap _cleanup_on_exit EXIT INT TERM

register_cleanup() {
  _CLEANUP_DIRS+=("$1")
}

# --- Docker helpers ---

# check_docker_buildx — ensures buildx available
check_docker_buildx() {
  if ! docker buildx version >/dev/null 2>&1; then
    log_error "Docker buildx not available. Install Docker Desktop or docker-buildx-plugin."
    exit 1
  fi
}

# ensure_buildx_builder NAME — create + use the named builder if missing
ensure_buildx_builder() {
  local name="${1:-fbm-builder}"
  if ! docker buildx inspect "$name" >/dev/null 2>&1; then
    docker buildx create --name "$name" --driver docker-container --use >/dev/null
  else
    docker buildx use "$name"
  fi
}

# image_size_bytes IMAGE_TAG → size in bytes from docker images
image_size_bytes() {
  local tag="$1"
  docker image inspect "$tag" --format '{{.Size}}' 2>/dev/null || echo 0
}
