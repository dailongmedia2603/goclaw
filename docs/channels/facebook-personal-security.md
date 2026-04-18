# Facebook Messenger (Personal) — Security Model

> Scope: how GoClaw authenticates, encrypts, and audits the `facebook_personal` channel. For ban-mitigation operational guidance see [facebook-personal.md](facebook-personal.md).

## Trust boundaries

```
┌─────────────┐  Bearer token  ┌─────────────┐   HTTP over MQTT   ┌──────────┐
│   GoClaw    │ ─────────────→ │   Sidecar   │ ──────────────────→│   Meta   │
│   gateway   │ ←─HMAC-signed─ │ (mautrix +  │ ←──────────────────│          │
│             │     webhook    │   Synapse)  │                    │          │
└─────────────┘                └─────────────┘                    └──────────┘
      ▲                              ▲
      │                              │
      │ HTTPS (TLS)                  │ Cookie auth (posted once via /login)
      │                              │
┌─────────────┐                ┌─────────────┐
│    User     │                │  Burn FB    │
│  (admin)    │                │   account   │
└─────────────┘                └─────────────┘
```

| Link | Auth | Integrity | Confidentiality | Mitigations |
|------|------|-----------|-----------------|------------|
| User → GoClaw UI | Session cookie / JWT | TLS | TLS | Standard GoClaw auth (see [20-api-keys-auth.md](../20-api-keys-auth.md)) |
| GoClaw → Sidecar `/send /login /healthz` | Bearer token | TLS (if configured) or localhost | TLS or localhost | AES-GCM encrypted creds at rest; token rotation via channel edit |
| Sidecar → GoClaw `/webhook/...` | HMAC-SHA256 body signature + 60s timestamp window | HMAC | TLS or localhost | Replay-window enforced; 4 MiB body limit; unauthenticated inbound rejected 401 |
| Sidecar ↔ Meta | FB session cookies (`c_user`, `xs`) | Meta MQTT TLS | E2EE where Meta supports (Labyrinth) | Cookies stored in Synapse DB on sidecar volume; rotate regularly |

## Credential storage

- **In GoClaw DB**: `channel_instances.credentials` BYTEA — AES-256-GCM encrypted at rest via [internal/crypto](../../internal/crypto/aes.go). Master key from env var. Payload shape: `{sidecar_url, auth_token, webhook_secret, fb_cookies?}`.
- **On sidecar disk**: mautrix-meta session database at `/data/mautrix-meta/mautrix-meta.db` (SQLite). Contains encrypted cookie bag. Do not back up to untrusted storage. `chmod 600` by default.
- **Never logged**: GoClaw's slog pipeline filters the `auth_token` / `webhook_secret` fields (they're inside `Credentials` struct; not interpolated into log strings anywhere).

## Webhook HMAC scheme (reference)

```
signed_payload   = "<unix_seconds>.<raw_body_bytes>"
signature_hex    = hex( hmac_sha256(secret, signed_payload) )
header_value     = "t=<unix_seconds>,v1=<signature_hex>"
```

- Constant-time compare (`hmac.Equal` in Go, `secrets.compare_digest` equivalent).
- 60-second timestamp window — older/newer timestamps are rejected as replay.
- Signed body is the RAW inbound payload, before any JSON parsing.
- Header sent via `X-Fbm-Signature`. API version in `X-Fbm-Api-Version` (currently `v1`).

## Audit log events

All security-relevant events use the `security.facebook_personal.*` slog prefix so they can be filtered from general logs:

| Event | Trigger | Fields |
|-------|---------|--------|
| `security.facebook_personal.webhook.signature_failed` | HMAC mismatch, timestamp expired, or malformed header | `channel`, `err` |
| `security.facebook_personal.webhook.api_version_mismatch` | Unexpected `X-Fbm-Api-Version` | `channel`, `got`, `want` |
| `facebook_personal.health.disconnected` | Sidecar health ping failed | `channel`, `err` |
| `facebook_personal.health.reconnected` | Sidecar recovered | `channel` |

## Threat model

### In-scope threats

1. **Webhook forgery** — an attacker posts fake inbound messages to the gateway webhook.
   - Mitigation: HMAC body signature with 60s replay window.
2. **Replay** — valid webhook captured and re-sent later.
   - Mitigation: timestamp window enforced by `VerifyWebhookSignature`.
3. **Sidecar impersonation** — attacker on the same network pretends to be the sidecar and sends /send calls posing as GoClaw.
   - Mitigation: Bearer token auth on `/send /login /healthz`. Token rotates via channel edit.
4. **Cross-tenant message leak** — tenant A's inbound message reaches tenant B's agent.
   - Mitigation: webhook path `/channels/facebook_personal/<name>/webhook` is name-scoped; only the channel instance for `<name>` mounts its handler; `InboundMessage.TenantID` is set from `BaseChannel.TenantID()` which is sourced from `channel_instances.tenant_id`.
5. **Creds-at-rest disclosure** — DB dump leaks raw cookies.
   - Mitigation: AES-GCM encryption with env-injected master key.

### Out-of-scope (explicit non-goals)

- **Meta banning your account** — this is a fundamental risk of unofficial automation. No amount of GoClaw-side hardening prevents Meta's anti-automation heuristics from flagging your session.
- **Matrix Olm/Megolm encryption** — not enabled; ignored because FB's own Labyrinth is handled internally by mautrix-meta. If tenants need audit-grade message confidentiality against a compromised sidecar host, this channel is not the right fit.
- **Supply-chain integrity of mautrix/meta or Synapse** — pin commits, verify signatures out-of-band if required.

## Tenant isolation invariants

Enforced by code paths:

1. **Factory always sets Type**: `Channel` struct's `channelType = "facebook_personal"` via `SetType` in `New()` — used by `dispatch.go` routing.
2. **Webhook path is name-scoped**: `WebhookHandler()` returns `/channels/facebook_personal/<c.Name()>/webhook`. Multiple instances → distinct paths.
3. **Inbound messages carry TenantID**: `mapEventToInbound(..., c.TenantID(), ...)` stamps every message. Agent pipeline filters by tenant downstream.
4. **Admin API (creating/editing the instance)** is gated by `requireTenantAdmin` in [internal/http/tenant_auth_helpers.go](../../internal/http/tenant_auth_helpers.go) — standard channel admin RPC. No special path added.

## Incident response playbook

| Signal | Severity | First action |
|--------|----------|--------------|
| `fbm_checkpoint_total` > 0 | High | Disable instance. Do NOT retry login from same IP within 72h. |
| `fbm_signature_fail_total` rising fast | High | Possible attacker or HMAC key rotation mistake. Check secrets match. |
| Sidecar container OOM / crashloop | Medium | Restart sidecar; check RAM budget; review logs for panics. |
| FB account banned | High | Stop instance. Burn + replace account via new residential IP. |
