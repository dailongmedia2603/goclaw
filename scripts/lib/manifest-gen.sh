#!/usr/bin/env bash
# Emit MANIFEST.json for the bundle.
# shellcheck shell=bash
#
# Inputs (env vars required):
#   BUNDLE_VERSION       — e.g. 0.1.0
#   STAGE_DIR            — staging directory containing images/ + install/
#   GOCLAW_COMMIT        — e.g. f49d6fa3
#   FBM_COMMIT           — e.g. 626e8ceb
#   UPSTREAM_VERSION     — e.g. v3.9.1
#   IMAGES_JSON          — a JSON object of image metadata (built by caller)
#
# Output: writes $STAGE_DIR/MANIFEST.json

manifest_gen() {
  : "${BUNDLE_VERSION:?required}"
  : "${STAGE_DIR:?required}"
  : "${GOCLAW_COMMIT:?required}"
  : "${FBM_COMMIT:?required}"
  : "${UPSTREAM_VERSION:?required}"

  local build_date
  build_date=$(date -u +%Y-%m-%dT%H:%M:%SZ)

  local build_host
  build_host=$(uname -sm | tr ' ' '-')

  local images_json="${IMAGES_JSON:-{\}}"

  # Build per-file checksums for everything in STAGE_DIR except MANIFEST.json + CHECKSUMS.sha256
  local checksums_json="{}"
  while IFS= read -r -d '' f; do
    local rel="${f#"$STAGE_DIR"/}"
    local h
    h=$(sha256_file "$f")
    checksums_json=$(jq --arg k "$rel" --arg v "sha256:$h" '. + {($k): $v}' <<< "$checksums_json")
  done < <(
    find "$STAGE_DIR" -type f \
      ! -name MANIFEST.json \
      ! -name CHECKSUMS.sha256 \
      -print0 \
    | sort -z
  )

  jq -n \
    --arg ver "$BUNDLE_VERSION" \
    --arg date "$build_date" \
    --arg host "$build_host" \
    --arg upstream "$UPSTREAM_VERSION" \
    --arg gcommit "$GOCLAW_COMMIT" \
    --arg fbranch "${FBM_BRANCH:-feature/fbm}" \
    --arg fcommit "$FBM_COMMIT" \
    --argjson images "$images_json" \
    --argjson checksums "$checksums_json" \
    '{
      bundle_version: $ver,
      bundle_format_version: "1",
      build_date: $date,
      build_host: $host,
      goclaw_upstream_version: $upstream,
      goclaw_upstream_commit: $gcommit,
      fbm_branch: $fbranch,
      fbm_feature_commit: $fcommit,
      images: $images,
      checksums: $checksums,
      upstream_safety: {
        core_files_touched: 5,
        touched_files: [
          "cmd/gateway.go",
          "internal/channels/channel.go",
          "ui/web/src/constants/channels.ts",
          "ui/web/src/pages/channels/channel-schemas.ts",
          "ui/web/src/pages/channels/channel-wizard-registry.tsx"
        ]
      }
    }' > "$STAGE_DIR/MANIFEST.json"
}

# manifest_emit_image_entry TAG DIGEST SIZE PLATFORMS → JSON fragment
manifest_emit_image_entry() {
  local tag="$1" digest="$2" size="$3" platforms="$4"
  jq -n \
    --arg tag "$tag" \
    --arg digest "$digest" \
    --argjson size "$size" \
    --argjson platforms "$platforms" \
    '{($tag): {digest: $digest, size_bytes: $size, platforms: $platforms}}'
}
