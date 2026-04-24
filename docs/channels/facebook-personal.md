# Facebook Messenger (Personal) channel

> ⚠️ **Experimental — not covered by any SLA. Your Facebook account may be banned. This channel violates [Meta Terms of Service §3.2.3](https://www.facebook.com/legal/terms).**
>
> Only enable if you have accepted the risk for a **burn / test** Facebook account, understand local anti-spam laws, and are running this in production at your own risk.

This channel lets a GoClaw agent auto-reply to personal Facebook Messenger DMs (1:1 + group chat) of a single Facebook user. Not to be confused with the `facebook` channel, which is for Facebook **Pages / Fanpages** via the official Graph API.

Implementation: GoClaw talks over HTTP to an external [mautrix-meta](https://github.com/mautrix/meta) sidecar (AGPL-3.0, separate process — never linked into the GoClaw binary). The sidecar handles the Meta protocol (MQTT, Labyrinth E2EE, cookie auth).

---

## Prerequisites

1. **Burn/test Facebook account** aged ≥ 30 days, with avatar + a few posts + a real friends list. Do **NOT** use your personal/main account. Do **NOT** use a brand-new account created 5 minutes ago — Meta flags these instantly.
2. **Residential IP**: one outbound IP per account. Do not share egress IP across multiple tenants or multiple accounts. Datacenter IPs (AWS, Digital Ocean, Hetzner) trigger ban detection within minutes.
3. **Deployment target** with Docker + 1GiB RAM free (Synapse + mautrix-meta + shim).
4. **GoClaw Standard edition**. Lite edition blocks this channel by design (sidecar model incompatible with single-binary desktop app).

## Setup

### 1. Deploy the sidecar

```bash
cd sidecar/mautrix-meta-shim
cp docker-compose.yml docker-compose.override.yml  # edit as needed

export FBM_AUTH_TOKEN=$(openssl rand -hex 32)
export FBM_HMAC_SECRET=$(openssl rand -hex 32)
export FBM_WEBHOOK_URL="http://goclaw-gateway:18790/channels/facebook_personal/INSTANCE/webhook"
export SYNAPSE_ADMIN_TOKEN=...  # from Synapse first-run admin register

docker compose up -d
```

See `sidecar/mautrix-meta-shim/README.md` for full config.

### 2. Create the GoClaw channel instance

Via the web UI:

1. **Channels → Add Channel**
2. Type: **Facebook Messenger (Personal)**
3. Fill credentials:
   - **Sidecar URL**: `http://fbm-sidecar:29320` (or wherever the sidecar is reachable from the gateway)
   - **Sidecar Auth Token**: same value as `$FBM_AUTH_TOKEN` above
   - **Webhook HMAC Secret**: same value as `$FBM_HMAC_SECRET`
4. Fill config:
   - **Account Label**: e.g. `Alice's FB`
   - **DM Policy**: `pairing` (default) or `open`
   - **Group Policy**: `disabled` recommended (group chats have higher ban risk)
   - **Rate Limit**: `20` messages/minute (do not exceed 30)
   - **✅ I acknowledge the risks** — MUST be checked to save

### 3. Login with cookies

After the instance is created, the wizard prompts for FB cookies:

1. Open `messenger.com` in a **private/incognito window**.
2. Log in with your burn account.
3. Open DevTools → **Application** (Chrome) / **Storage** (Firefox) → **Cookies** → `https://www.messenger.com`.
4. Copy values for: `c_user`, `xs`, `datr`, `sb`, `fr` (fr is optional).
5. Paste each into the wizard and submit.

The shim forwards the cookies to mautrix-meta, which opens a Messenger session. Confirmed when bridge logs show `Logged in as <Your Name>`.

## Operations

| Task | Where |
|------|-------|
| View agent replies | Channels page → Diagnostics tab |
| Re-authenticate (cookies expired) | Channels page → ⋯ menu → Re-authenticate |
| Pause (without deleting instance) | Channels page → toggle Enabled off |
| View metrics | Gateway `/status` endpoint → `fbm_*` counters |

### Metrics worth watching

| Counter | Meaning | Action when elevated |
|---------|---------|---------------------|
| `fbm_checkpoint_total` | FB challenge/checkpoint was triggered | Stop the instance immediately; account may be flagged |
| `fbm_signature_fail_total` | Webhook signature rejected | Verify sidecar `FBM_HMAC_SECRET` matches GoClaw channel config |
| `fbm_reconnect_total` | Sidecar reconnect count | Investigate network / sidecar stability |
| `fbm_rate_limited_total` | Outbound messages blocked by rate limit | Lower agent response rate or raise limit (carefully) |

## Ban avoidance checklist

- [ ] 1 account = 1 residential IP (no sharing)
- [ ] Rate limit ≤ 20 msgs/min, ≤ 200 msgs/hour
- [ ] No identical template messages to >10 recipients in an hour
- [ ] Typing-indicator-like natural delays (agent replies should take a few seconds, not 100ms)
- [ ] Do not run this in EU unless you have a legal basis under GDPR Art. 6 for automated messaging to third parties
- [ ] Don't enable group policy by default
- [ ] Monitor `fbm_checkpoint_total` — any non-zero value is a signal to stop

## If your account gets banned

1. Stop the instance: Channels → disable.
2. Do NOT retry login from the same IP for at least 72 hours — this makes the fingerprint worse.
3. Do NOT spin up a new account from the same IP — they will be linked.
4. Create a new burn account via a **different residential IP + different browser profile** and re-register.

## Known limitations

- **No message streaming** (Messenger doesn't support message edits for personal DMs — agent replies are sent as-is once complete).
- **Media**: inbound media URLs point at the sidecar; outbound media are sent via Matrix content repo upload and may not work for files >20 MiB on default Synapse config.
- **Threads in groups**: reply-to metadata works in 1:1 but may be stripped in groups depending on FB's server-side handling.
- **Multi-device**: logging in via cookies while the same account is active on the real Messenger mobile app may cause conflicts — use a dedicated burn account or log out the real device first.

## Troubleshooting

**"cookie expired" error** → Cookies are usually valid 3-14 days for active sessions, less for inactive ones. Re-auth via the wizard.

**Agent never receives messages** → Check:
1. Sidecar logs: `docker compose logs -f fbm-sidecar | grep -i error`
2. Bridge state: `curl http://sidecar:29320/healthz -H "Authorization: Bearer $FBM_AUTH_TOKEN"`
3. Gateway logs: `journalctl -u goclaw | grep facebook_personal`

**"webhook signature invalid"** in gateway logs → secret mismatch. Re-verify `FBM_HMAC_SECRET` (sidecar env) == `Webhook HMAC Secret` (channel credentials).

**Sidecar reconnect loop** → `docker compose restart fbm-sidecar`. If persistent, check Synapse storage (SQLite db full) and mautrix-meta config.

---

See also:
- [facebook-personal-security.md](facebook-personal-security.md) — threat model + audit trail
- [../../sidecar/mautrix-meta-shim/README.md](../../sidecar/mautrix-meta-shim/README.md) — sidecar ops guide
- [../05-channels-messaging.md](../05-channels-messaging.md) — general channel architecture
