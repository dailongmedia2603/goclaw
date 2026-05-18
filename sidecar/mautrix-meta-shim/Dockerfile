# Build mautrix-meta-shim sidecar image.
# Embeds Synapse + mautrix/meta + this Go shim + supervisord to orchestrate all three.
#
# DO NOT use :latest tags — pin everything for reproducible behavior and to satisfy
# AGPL §13 "corresponding source" via commit pins.

# --- Stage 1: build the Go shim ---
FROM golang:1.26-alpine AS shim-build
WORKDIR /src
# Shim has no external dependencies — go.sum may not exist. Copy go.mod alone.
COPY go.mod ./
RUN go mod download || true
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/shim .

# --- Stage 2: runtime ---
# Use Synapse's image as base (already has Python + Synapse preinstalled).
FROM matrixdotorg/synapse:v1.106.0

USER root

# Pin mautrix-meta commit for reproducibility. Matches "v26.04" tag (2026-04-16).
ARG MAUTRIX_META_TAG=v26.04

RUN apt-get update -qq && \
    apt-get install -y --no-install-recommends \
      supervisor curl ca-certificates wget && \
    rm -rf /var/lib/apt/lists/*

# Fetch mautrix-meta binary. The upstream image only ships `/usr/bin/mautrix-meta`.
# We copy it out via a transient build stage.
COPY --from=dock.mau.dev/mautrix/meta:v26.04 /usr/bin/mautrix-meta /usr/local/bin/mautrix-meta
COPY --from=shim-build /out/shim /usr/local/bin/shim

COPY supervisord.conf /etc/supervisor/conf.d/fbm.conf
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh /usr/local/bin/shim /usr/local/bin/mautrix-meta

EXPOSE 29320

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD curl -sf -H "Authorization: Bearer $FBM_AUTH_TOKEN" http://localhost:29320/healthz || exit 1

ENTRYPOINT ["/entrypoint.sh"]
