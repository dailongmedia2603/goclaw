#!/usr/bin/env bash
# Parse recipient's GoClaw compose files to determine merge strategy.
# shellcheck shell=bash

# detect_compose_files GOCLAW_DIR → echoes "-f file1 -f file2 ..."
detect_compose_files() {
  local d="$1"
  local args=()
  for f in docker-compose.yml docker-compose.postgres.yml docker-compose.selfservice.yml docker-compose.browser.yml docker-compose.claude-cli.yml docker-compose.otel.yml; do
    [[ -f "$d/$f" ]] && args+=("-f" "$f")
  done
  echo "${args[@]}"
}

# detect_has_embedded_ui GOCLAW_DIR → "embedded" | "separate" | "unknown"
detect_has_embedded_ui() {
  local d="$1"
  if [[ -f "$d/docker-compose.selfservice.yml" ]] && \
     docker ps --format '{{.Names}}' 2>/dev/null | grep -q "goclaw-ui"; then
    echo "separate"
    return
  fi
  if grep -q "ENABLE_EMBEDUI" "$d/docker-compose.yml" 2>/dev/null; then
    local val
    val=$(awk '/ENABLE_EMBEDUI:/ {print; exit}' "$d/docker-compose.yml" | grep -oE '"[a-z]+"' | tr -d '"')
    if [[ "$val" == "false" ]]; then
      echo "separate"
    else
      echo "embedded"
    fi
    return
  fi
  echo "unknown"
}

# detect_cpu_limit_conflict GOCLAW_DIR → 0 if conflict (limit > host), 1 if safe
detect_cpu_limit_conflict() {
  local d="$1"
  local host_cpus
  host_cpus=$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 2)
  local declared
  declared=$(grep -hoE "cpus: *['\"]?[0-9.]+" "$d"/docker-compose*.yml 2>/dev/null \
    | grep -oE "[0-9.]+" | sort -rn | head -1)
  [[ -z "$declared" ]] && return 1
  awk -v d="$declared" -v h="$host_cpus" 'BEGIN { exit !(d > h) }'
}

# get_host_cpus → print integer
get_host_cpus() {
  nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 1
}

# detect_gateway_port GOCLAW_DIR → best guess (default 18790)
detect_gateway_port() {
  local d="$1"
  local port
  port=$(grep -hE '\${GOCLAW_PORT:-[0-9]+}' "$d"/docker-compose*.yml 2>/dev/null \
    | grep -oE '[0-9]+' | head -1)
  echo "${port:-18790}"
}
