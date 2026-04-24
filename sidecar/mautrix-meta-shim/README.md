# mautrix-meta-shim (GoClaw Facebook Messenger Personal sidecar)

This sidecar bundles [mautrix/meta](https://github.com/mautrix/meta) (a Facebook/Meta/Instagram Matrix bridge) and a small Go HTTP shim that exposes a narrow API for GoClaw to integrate personal Facebook Messenger chats.

## ⚠️ License notice (AGPL-3.0)

The resulting container image embeds **`mautrix/meta`**, which is licensed under **AGPL-3.0-or-later**. Distributing or operating this image as a network service obligates you under AGPL §13 to provide the complete corresponding source code to network users.

- Source of mautrix/meta: https://github.com/mautrix/meta (pinned commit recorded in `Dockerfile`).
- Source of Synapse: https://github.com/matrix-org/synapse (pinned version recorded in `Dockerfile`).
- Source of this shim: this directory.

GoClaw itself does **NOT** link `mautrix/meta` as a Go library. All communication happens over HTTP between separate processes. Consult a lawyer before distributing this image as part of a closed-source commercial product.

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                     fbm-sidecar container                    │
│                                                              │
│  supervisord                                                 │
│  ├── Synapse (matrix homeserver, SQLite)          port 8008  │
│  ├── mautrix-meta (AGPL bridge → FB Messenger)    port 29319 │
│  └── shim (this code, Go HTTP server)             port 29320 │
│                                                              │
└──────────────────────────────────────────────────────────────┘
         ▲                                         │
         │ POST /send, /login, GET /healthz        │ POST <GOCLAW_WEBHOOK_URL>
         │ (Bearer FBM_AUTH_TOKEN)                 │ (signed with FBM_HMAC_SECRET)
         │                                         ▼
  ┌──────┴────────────┐                   ┌──────────────────┐
  │ GoClaw gateway    │                   │ GoClaw webhook   │
  │ (fbm channel)     │                   │ (inbound events) │
  └───────────────────┘                   └──────────────────┘
```

## HTTP API (consumed by GoClaw channel)

All endpoints require `Authorization: Bearer $FBM_AUTH_TOKEN`.

| Method | Path        | Purpose |
|--------|-------------|---------|
| GET    | `/healthz`  | Liveness probe — returns 200 when Synapse + mautrix-meta are reachable |
| POST   | `/login`    | Login to Facebook using cookies (body: `{"cookies": {...}}`) |
| POST   | `/send`     | Send a message to an FB thread (body: `{"chat_id":"...", "content":"...", "media":[...]}`) |

## Outbound webhooks (sidecar → GoClaw)

The shim periodically `/sync`s Matrix on behalf of the admin user, extracts messages from mautrix-meta's portal rooms, and POSTs them to `$FBM_WEBHOOK_URL` with headers:

- `X-Fbm-Api-Version: v1`
- `X-Fbm-Signature: t=<unix>,v1=<hex_hmac_sha256>`

The HMAC algorithm matches `internal/channels/facebookmessenger/signature.go` in the GoClaw repo — identical test vector.

## Environment variables

| Variable | Required | Description |
|----------|----------|-------------|
| `FBM_AUTH_TOKEN` | ✅ | Bearer token GoClaw presents on inbound calls |
| `FBM_HMAC_SECRET` | ✅ | HMAC-SHA256 key for outbound webhook signing |
| `FBM_WEBHOOK_URL` | ✅ | URL of GoClaw's `/channels/facebook_personal/<instance>/webhook` |
| `SYNAPSE_ADMIN_TOKEN` | ✅ | Matrix access token for the `@admin:fbm.local` user |
| `FBM_PORT` | | Shim listen port (default 29320) |
| `LOG_LEVEL` | | `debug`, `info`, `warn`, `error` (default info) |

## Development

```bash
# Build shim only (for unit tests)
cd sidecar/mautrix-meta-shim
go test ./...
go build -o shim .

# Full image (runs Synapse + mautrix-meta + shim via supervisord)
docker build -t fbm-sidecar:dev .
docker run --rm -p 29320:29320 -e FBM_AUTH_TOKEN=t -e FBM_HMAC_SECRET=s \
  -e FBM_WEBHOOK_URL=http://host.docker.internal:18790/channels/facebook_personal/test/webhook \
  fbm-sidecar:dev
```

## Production operations

One sidecar container = one FB account = one residential IP (recommended). Do NOT share an egress IP across multiple tenant accounts — a single ban can poison fingerprint detection for neighbors.

See [docs/channels/facebook-personal.md](../../docs/channels/facebook-personal.md) in the main repo for deployment guidance and ban mitigation checklist.
