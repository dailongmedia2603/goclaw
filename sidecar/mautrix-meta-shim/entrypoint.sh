#!/bin/sh
# mautrix-meta-shim container entrypoint.
# Validates required env vars and launches supervisord.

set -eu

# Required env vars
: "${FBM_AUTH_TOKEN:?FBM_AUTH_TOKEN is required}"
: "${FBM_HMAC_SECRET:?FBM_HMAC_SECRET is required}"
: "${FBM_WEBHOOK_URL:?FBM_WEBHOOK_URL is required}"
: "${SYNAPSE_ADMIN_TOKEN:?SYNAPSE_ADMIN_TOKEN is required}"

# Optional env vars with defaults
export FBM_PORT="${FBM_PORT:-29320}"
export LOG_LEVEL="${LOG_LEVEL:-info}"

# Validate Synapse config + mautrix-meta config are mounted
if [ ! -f /data/homeserver.yaml ]; then
  echo "FATAL: /data/homeserver.yaml missing — mount Synapse data volume" >&2
  exit 1
fi
if [ ! -f /data/mautrix-meta/config.yaml ]; then
  echo "FATAL: /data/mautrix-meta/config.yaml missing — mount mautrix-meta data volume" >&2
  exit 1
fi

echo "Starting fbm-sidecar (shim port=$FBM_PORT, webhook=$FBM_WEBHOOK_URL)"
exec /usr/bin/supervisord -c /etc/supervisor/conf.d/fbm.conf
