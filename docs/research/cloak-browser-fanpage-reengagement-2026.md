# Research Report: Cloak Browser Auto-Message cho Fanpage (Re-engagement >7 ngày)

**Ngày nghiên cứu:** 2026-04-26
**Phạm vi:** Triển khai kênh thứ 2 cho GoClaw — dùng browser automation (Rod) để gửi tin chăm sóc tới hộp thư fanpage Facebook không hoạt động > 7 ngày. Bổ sung (không thay thế) channel `facebookmessenger` hiện có (Graph API + sidecar mautrix-meta).
**Báo cáo liên quan:** [`fb-messenger-proactive-2026.md`](./fb-messenger-proactive-2026.md) — đã khuyến nghị HUMAN_AGENT tag (window 7 ngày). Báo cáo này tiếp nối cho **case ngoài 7 ngày**, nơi không còn API path hợp pháp tại VN.

---

## Executive Summary

**Vấn đề:** HUMAN_AGENT tag chỉ phủ tới 7 ngày kể từ tin cuối khách gửi. Sau 7 ngày, tại VN không có path API hợp pháp ổn định: Marketing Messages API chưa rollout VN, Recurring Notifications đã chết 2026-02-10, các tag legacy chết 2026-04-27. Sponsored Messages tốn ads. **Đây là gap thực sự** — và đó chính xác là lý do user yêu cầu browser automation.

**Trade-off rõ ràng phải nhận thức:**

> **Browser automation Pages inbox VI PHẠM Meta TOS hiện hành** (cập nhật 2025-01-01): cấm "access or collect data using automated means without prior permission". Rủi ro thực: page bị disable, account admin bị lock 24h–vĩnh viễn, IP bị block. Đây là **calculated risk** business chấp nhận đổi lấy giữ liên hệ với khách lâu năm — không phải kỹ thuật bypass an toàn.

**Khuyến nghị triển khai:**

1. **Module mới:** `internal/channels/fbcloak/` — channel song song với `facebookmessenger/`, KHÔNG phải tool LLM gọi tự do.
2. **Trigger:** Cron-driven (`internal/cron/`) → enqueue task (`internal/tasks/`) → spawn worker per fanpage (lane riêng trong `internal/scheduler/`). KHÔNG để LLM tự ý kích hoạt.
3. **Browser foundation:** Tận dụng `pkg/browser/Manager` đã có (Rod + tenant incognito + idle reaper). Bổ sung 4 thứ: cookie inject, per-profile proxy, [`go-rod/stealth`](https://github.com/go-rod/stealth) JS patches, humanization layer (bezier mouse, jitter typing).
4. **Schema:** 3 bảng mới — `fbcloak_credentials` (cookie+proxy encrypted), `fbcloak_jobs` (cron run state), `fbcloak_send_log` (audit). Tất cả tenant-scoped.
5. **Safety nets bắt buộc:** dry-run mode, daily cap per fanpage (≤30 tin), working hours window, screenshot every send, kill-switch khi phát hiện checkpoint/captcha, token health probe trước mỗi run.
6. **Edition gating:** Chỉ Standard. Lite (Wails desktop) bị chặn vì user single-tenant không cần và RAM Chrome quá nặng cho desktop.
7. **Dual-mode mặc định:** Channel gateway thử Graph API (`HUMAN_AGENT`) trước; chỉ rớt sang Cloak Browser khi `last_inbound_at > 7d`. Code phía caller không biết kênh nào được chọn.

---

## 1. Bối cảnh: Tại sao cần Cloak Browser dù đã có HUMAN_AGENT

| Khoảng cách kể từ tin cuối khách | Path API | Path Cloak Browser |
|---|---|---|
| ≤ 24h | `messaging_type=RESPONSE` | (không cần) |
| 24h – 7 ngày | `MESSAGE_TAG` + `HUMAN_AGENT` | (không cần) |
| **> 7 ngày, ≤ 6 tháng** | **Không có** (Marketing Messages chưa VN, RN đã chết) | **Đây** |
| > 6 tháng | (Sponsored Messages/Ads — paid) | Vẫn được nhưng risk cao hơn |

User bài toán "hộp thư cũ quá 7 ngày" rơi đúng vào ô màu xám không có API. Đó là khoảng trống kinh doanh thật.

> **Note:** Báo cáo `fb-messenger-proactive-2026.md` mục 3.4 ghi *"❌ Browser automation business.facebook.com Inbox — user đã reject"*. Tại thời điểm đó user đã reject; nay user yêu cầu lại với scope hẹp hơn (chỉ re-engagement >7d, không phải reply realtime). Phạm vi mới hợp lệ hơn vì không cạnh tranh với channel chính.

---

## 2. Kiến trúc: Channel hay Tool hay Cron Job?

### 2.1 Tổng quan 3 lựa chọn

| Phương án | Ưu | Nhược | Verdict |
|---|---|---|---|
| **A. Tool mới trong `internal/tools/`** (kiểu `fb_cloak_send`) | LLM trực tiếp gọi được, linh hoạt | LLM có thể gọi sai → ban page; khó rate-limit; khó dry-run | ❌ — risky, agent tự gọi qua sandbox sẽ phá daily cap |
| **B. Cron job thuần trong `internal/cron/`** | Đơn giản, không lộ qua RPC/tool | Không reusable; khó test riêng; không lifecycle | ⚠️ — quá thô |
| **C. Channel mới `internal/channels/fbcloak/`** | Theo pattern hiện có (channels là pluggable, có factory, edition_gate, ratelimit, metrics) | Tên hơi sai — "channel" trong codebase = inbox 2 chiều; cloak chỉ outbound | ✅ — best fit |

**Chọn C.** Channel pattern (tham chiếu [`internal/channels/facebookmessenger/`](../../internal/channels/facebookmessenger/) — đã thấy `factory.go`, `edition_gate.go`, `ratelimit.go`, `metrics.go`, `policy.go`) cung cấp đúng các primitives cần. Outbound-only là thiểu số nhưng không vi phạm interface.

### 2.2 Layout đề xuất

```
internal/channels/fbcloak/
├── channel.go            # Channel struct + lifecycle (Start/Stop)
├── credential_store.go   # CRUD fbcloak_credentials, encrypt/decrypt cookie+proxy
├── browser_runner.go     # Spawn rod page, inject cookie, navigate inbox
├── humanize.go           # bezier mouse, jitter typing, random delay
├── stealth.go            # apply go-rod/stealth + extra patches
├── inbox_scanner.go      # navigate Business Suite inbox, list conversations, parse last_activity
├── send_executor.go      # one conversation: open thread, type, send, screenshot
├── job_runner.go         # orchestrator: pick fanpage, run scan→send loop
├── policy.go             # daily cap, working hours, content filter
├── ratelimit.go          # per-fanpage + per-recipient cooldown
├── metrics.go            # Prometheus counters (sends, blocks, captchas)
├── edition_gate.go       # block Lite
├── errors.go             # sentinel errors (ErrCheckpoint, ErrCookieExpired, ...)
├── types.go              # config struct + DB models
└── factory.go            # builds Channel from config + dependencies

pkg/browser/
├── (existing files)
├── cookie.go             # NEW: SetCookies(page, []Cookie) helper
├── proxy.go              # NEW: per-profile proxy launcher option
└── stealth_inject.go     # NEW: lightweight rod-stealth wrapper
```

`pkg/browser/` ưu tiên thêm low-level primitives (cookie, proxy, stealth inject) — generic, dùng được cho profile khác (shopee, tiktok…). Logic Facebook-specific nằm trong `internal/channels/fbcloak/`.

---

## 3. Browser Foundation — Đã có gì, cần thêm gì

### 3.1 Đã có trong `pkg/browser/Manager` (xác nhận từ `browser.go`, `registry.go`)

| Primitive | Trạng thái |
|---|---|
| Rod connection + reconnect on dead | ✅ `Manager.Start()` |
| Tenant isolation qua incognito | ✅ `tenantBrowserLocked()` |
| Profile registry với domain match | ✅ `ProfileRegistry.Resolve()` |
| Idle page reaper | ✅ `WithIdleTimeout` |
| Per-action timeout | ✅ `WithActionTimeout` |
| Max pages per tenant | ✅ `WithMaxPages` |
| Snapshot AX tree + ref-based click | ✅ `snapshot.go`, `actions.go` |
| Console capture | ✅ `console map` |
| VNC URL field cho manual login | ✅ `Profile.VNCURL` |
| Shared mode (preserve cookies) | ✅ `WithShared` |
| Remote CDP sidecar | ✅ `WithRemoteURL` |

### 3.2 Cần thêm

| Primitive | Vì sao | Đặt ở đâu |
|---|---|---|
| **Cookie inject API** | FB auth dựa hoàn toàn vào cookie set | `pkg/browser/cookie.go` |
| **Per-profile proxy** | Mỗi fanpage 1 proxy riêng để giảm cluster IP | `pkg/browser/proxy.go` (kiểu `Profile.Proxy string` + per-profile launcher) |
| **Stealth JS patches** | Hide `navigator.webdriver`, spoof canvas/WebGL | `pkg/browser/stealth_inject.go` dùng `github.com/go-rod/stealth` |
| **Humanization helpers** | Bezier mouse, typing jitter, scroll variance | `internal/channels/fbcloak/humanize.go` (Facebook-specific patterns OK riêng) |
| **Per-profile launch flags** | `--proxy-server=...`, `--user-agent=...` | Refactor: `Profile.LauncherFlags map[string]string` |

**Quyết định proxy:** Đề xuất per-profile launcher (mỗi fanpage spawn Chrome riêng) thay vì shared browser + per-context proxy. Lý do: Chrome proxy-per-context cần CDP `Fetch.requestPaused` interception phức tạp; spawn riêng đơn giản, nhưng tốn RAM (~150MB/Chrome). Cap ≤5 fanpages active đồng thời.

### 3.3 Stealth strategy

**Tier 1 — bắt buộc (`go-rod/stealth`):**
- `--disable-blink-features=AutomationControlled` (đã không có trong launcher hiện tại — thêm vào)
- Override `navigator.webdriver` = undefined
- Spoof `navigator.plugins`, `navigator.languages`, `navigator.permissions.query`
- Patch canvas/WebGL noise

**Tier 2 — nên thêm:**
- User-Agent match đúng Chrome version đang chạy (Rod default UA hơi lệch)
- Viewport realistic (1366×768, 1920×1080 — random per profile, persisted)
- Timezone match proxy IP geolocation (`Emulation.setTimezoneOverride`)
- Screen resolution match viewport
- Skip `webgl-renderer` lộ "ANGLE (Mesa)" / "SwiftShader" (signature headless)

**Tier 3 — defer (chỉ nếu Tier 1+2 fail):**
- Chuyển sang [Nodriver](https://github.com/ultrafunkamsterdam/nodriver) hoặc patched chromium
- Theo bài [Castle.io 6/2025](https://blog.castle.io/from-puppeteer-stealth-to-nodriver-how-anti-detect-frameworks-evolved-to-evade-bot-detection/): puppeteer-stealth/rod-stealth đã bị nhiều WAF bắt từ 2024. Nếu FB siết, Tier 3 là exit plan.

---

## 4. Cookie & Proxy: Schema và quản lý

### 4.1 Cookie Facebook (xác nhận từ techexpertise.medium + cookielibrary)

| Cookie | Mục đích | Lifetime | Bắt buộc? |
|---|---|---|---|
| `c_user` | User ID logged-in | 365d | ✅ Bắt buộc |
| `xs` | Session secret | 365d (refresh 30–40min khi tab open) | ✅ Bắt buộc |
| `datr` | Browser identity (anti-fraud) | 2y | ✅ Bắt buộc — thiếu = checkpoint ngay |
| `fr` | Ads tracking + impression count | 90d | ⚠️ Recommended |
| `sb` | Browser security | 2y | ⚠️ Recommended |
| `wd` | Window dimension | session | ❌ Không cần (browser tự set) |
| `presence` | Online status | session | ❌ |

**Quy trình lấy cookie ban đầu (manual, 1 lần / fanpage):**
1. User mở browser thật, login admin Page (có thể qua VNC nếu remote).
2. Export cookies qua extension (EditThisCookie, Cookie-Editor) → JSON.
3. Upload JSON vào GoClaw qua RPC `fbcloak.add_credential` (encrypted at rest).

**Auto-refresh:** Trên mỗi run thành công, dump cookies từ rod page về DB (xs có thể đã rotate). Nếu run fail với checkpoint → mark credential `expired`, alert admin.

### 4.2 Schema DB (PostgreSQL — Standard only)

```sql
-- migrations/00XX_create_fbcloak_tables.up.sql

CREATE TABLE fbcloak_credentials (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    fanpage_id      TEXT NOT NULL,                  -- FB numeric page ID
    fanpage_name    TEXT NOT NULL,                  -- display only
    cookies_enc     TEXT NOT NULL,                  -- aes-gcm: encrypted JSON []Cookie
    proxy_url_enc   TEXT,                           -- aes-gcm: socks5://user:pass@host:port
    user_agent      TEXT NOT NULL,
    viewport_w      INT NOT NULL DEFAULT 1366,
    viewport_h      INT NOT NULL DEFAULT 768,
    timezone        TEXT NOT NULL DEFAULT 'Asia/Ho_Chi_Minh',
    status          TEXT NOT NULL DEFAULT 'active', -- active|expired|disabled|checkpoint
    last_login_at   TIMESTAMPTZ,
    last_check_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, fanpage_id)
);
CREATE INDEX idx_fbcloak_creds_tenant_status ON fbcloak_credentials(tenant_id, status);

CREATE TABLE fbcloak_jobs (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    credential_id    UUID NOT NULL REFERENCES fbcloak_credentials(id) ON DELETE CASCADE,
    name             TEXT NOT NULL,                 -- "Re-engage 7-30d"
    template_id      UUID,                          -- ref skill/template; nullable for ad-hoc
    target_min_idle  INTERVAL NOT NULL DEFAULT '7 days',
    target_max_idle  INTERVAL NOT NULL DEFAULT '30 days',
    daily_cap        INT NOT NULL DEFAULT 30,
    working_hours    JSONB NOT NULL DEFAULT '{"start":"08:00","end":"21:00","tz":"Asia/Ho_Chi_Minh"}',
    cron_expr        TEXT NOT NULL,                 -- '0 9 * * *'
    enabled          BOOLEAN NOT NULL DEFAULT FALSE,
    dry_run          BOOLEAN NOT NULL DEFAULT TRUE, -- default safe!
    last_run_at      TIMESTAMPTZ,
    last_run_status  TEXT,                          -- ok|partial|fail|killed
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_fbcloak_jobs_tenant_enabled ON fbcloak_jobs(tenant_id, enabled);

CREATE TABLE fbcloak_send_log (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    job_id            UUID NOT NULL REFERENCES fbcloak_jobs(id) ON DELETE CASCADE,
    credential_id     UUID NOT NULL REFERENCES fbcloak_credentials(id),
    fanpage_id        TEXT NOT NULL,
    conversation_id   TEXT NOT NULL,                 -- FB thread ID
    recipient_psid    TEXT,                          -- nếu parse được
    recipient_name    TEXT,
    last_inbound_at   TIMESTAMPTZ,                   -- snapshot tại lúc gửi
    message_text      TEXT NOT NULL,
    status            TEXT NOT NULL,                 -- sent|skipped|failed|dry_run
    error             TEXT,
    screenshot_path   TEXT,                          -- pre-send + post-send (storage path)
    sent_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_fbcloak_log_tenant_sent ON fbcloak_send_log(tenant_id, sent_at DESC);
CREATE INDEX idx_fbcloak_log_credential_recipient ON fbcloak_send_log(credential_id, recipient_psid, sent_at DESC);
```

**Encryption:** dùng `internal/crypto.Encrypt(json, key)` từ [`aes.go`](../../internal/crypto/aes.go) (đã verify: AES-256-GCM, prefix `aes-gcm:`). Key từ env `GOCLAW_ENCRYPTION_KEY` đang dùng cho LLM API key.

**SQLite:** KHÔNG bổ sung schema cho Lite — feature gated off.

### 4.3 Cookie health probe

Trước mỗi run job:
1. Spawn page, inject cookies, navigate `https://www.facebook.com/me`.
2. Check redirect: nếu URL chứa `/login` hoặc `/checkpoint` → mark `status=expired|checkpoint`, abort job, alert.
3. Nếu `<title>` chứa tên admin user → OK, continue.
4. `last_check_at` update.

---

## 5. Workflow Inbox: Chiến lược URL/DOM ổn định nhất

### 5.1 So sánh 3 entry points

| URL | Pros | Cons |
|---|---|---|
| `business.facebook.com/latest/inbox?asset_id={fanpage_id}` (Meta Business Suite mới) | UI hiện đại, có timestamp rõ "X ngày trước" | DOM thay đổi 1–2 tháng/lần |
| `facebook.com/messages/t/{conv_id}` (Messenger web cho Page) | Stable hơn, single thread URL ổn | Khó iterate inbox; phải vào Pages Manager |
| `facebook.com/{page}/inbox` (Pages classic) | Đang dần bị Meta retire | Có thể chết bất kỳ lúc nào |

**Khuyến nghị:** `business.facebook.com/latest/inbox` chính, fallback `facebook.com/messages/t/...` cho từng thread khi cần.

### 5.2 Strategy parse last_activity

Vì DOM Meta Business Suite hay đổi, KHÔNG selector cứng theo class `_xy123`. Thay vào:

1. **AX tree (đã có `pkg/browser/snapshot.go`)** — snapshot accessibility tree, tìm `role="listitem"` + `name` chứa pattern thời gian.
2. **JS evaluate**: chạy JS đọc internal React state qua `__REACT_INTERNAL_INSTANCE_KEY`. Brittle nhưng cho đúng timestamp.
3. **Fallback regex**: parse text "X giờ", "Y ngày", "Tuần trước" → quy ra ms.

Thực hiện *cả 3* và pick first success. Nếu cả 3 fail trong 1 run → mark job failed + screenshot full-page evidence.

### 5.3 Pseudocode flow

```
PER FANPAGE JOB RUN:
  1. health_probe(credential)               -> abort if expired
  2. open https://business.facebook.com/latest/inbox?asset_id={fp}
  3. wait_for(role="listitem" count >= 1, timeout=15s)
  4. simulate idle 2-5s (human reading)
  5. scroll inbox slowly, collect all conversation cards
     until last_activity < target_min_idle OR scrolled_count > 200
  6. filter: last_activity ∈ [target_min_idle, target_max_idle]
  7. respect daily_cap, working_hours, per-recipient cooldown(30d)
  8. for each conversation (random order, max=daily_cap):
        a. random delay 30-180s before opening
        b. click conversation row (bezier mouse)
        c. wait thread loaded
        d. screenshot pre-send
        e. type message (template + personalization) with jitter
        f. read-back typed text — abort if mismatch
        g. random pause 1-3s
        h. click Send
        i. wait sent state confirm (DOM check)
        j. screenshot post-send
        k. INSERT INTO fbcloak_send_log
        l. random delay 60-300s before next
  9. dump cookies back to DB (xs may have rotated)
  10. release page, mark job last_run_status
```

---

## 6. Anti-detection & Humanization

| Behavior | Spec |
|---|---|
| Mouse path | Cubic Bezier với 2 control points random ±15% length, 30–60 steps, jitter ±2.5px |
| Typing speed | 80–180ms/char base, +50ms sau dấu cách, +200ms sau `.`, hesitation 5% chance |
| Scroll | wheel events, 300–800px/tick, pause 800–1500ms giữa ticks |
| Pre-action delay | random 1.5–4s |
| Working hours | configurable per job, default 08:00–21:00 GMT+7 |
| Daily cap | configurable, default 30/fanpage, hard cap 50 |
| Per-recipient cooldown | 30 ngày kể từ lần gửi gần nhất qua cloak |
| IP rotation | per-fanpage proxy fixed (không rotate giữa run — bị FB nghi); rotate khi `status=checkpoint` reset |
| User-Agent | match Chrome major version + OS, persistent per credential |
| Concurrency | 1 worker / fanpage. Tối đa 5 fanpages parallel toàn cluster (config `fbcloak.max_concurrent`) |

**Reference:** [humanization-playwright](https://github.com/saksham-personal/humanization-playwright), [ghost-cursor](https://github.com/Xetera/ghost-cursor), Bezier paper [IJIRT 183343](https://ijirt.org/publishedpaper/IJIRT183343_PAPER.pdf).

**Self-imposed restrictions:**
- KHÔNG concurrent send trên cùng 1 fanpage.
- KHÔNG override device fingerprint mid-session.
- KHÔNG spoof headless detection bằng cách hijack timing API (đã bị Castle.io flag từ 2024).

---

## 7. Template & Personalization

**Quan điểm:** KHÔNG mở browser tool cho LLM gọi tự do (xem mục 2.1.A). Thay vào, LLM **sinh template trước**, system gửi.

### 7.1 Pipeline render

```
job.template_id → skills/skill_search → SKILL.md "fbcloak/reengagement-7d-vi"
          ↓
   placeholder list: {customer_name}, {last_topic}, {fanpage_name}
          ↓
   per conversation: enrich placeholders từ:
     - parse thread name → customer_name
     - last 2-3 messages from cloak's own scrape (NOT Graph API to avoid double-touch)
     - LLM gọi 1 lần để tóm tắt "last_topic" (provider Anthropic/OpenAI tùy config)
          ↓
   final text → content_filter (mục 7.2) → send
```

### 7.2 Content filter (kế thừa research cũ)

Block keywords (case-insensitive, có dấu/không dấu): "khuyến mãi", "sale", "giảm giá", "deal", "coupon", "voucher", "miễn phí", "%", "đăng ký", "click", "link". Pancake/Smax dùng tương tự cho HUMAN_AGENT — ngoài 7d FB không enforce kỹ keyword nữa, nhưng giữ filter để tránh user bị mark spam phía recipient.

### 7.3 Tool exposure

KHÔNG đăng ký tool `fb_cloak_send` trong `internal/tools/`. RPC method mới trong `internal/gateway/methods/`:

- `fbcloak.list_credentials`
- `fbcloak.add_credential` (cookie + proxy)
- `fbcloak.test_credential` (health probe sync)
- `fbcloak.create_job`, `fbcloak.update_job`, `fbcloak.toggle_job`
- `fbcloak.run_job_now` (manual trigger, vẫn theo cap)
- `fbcloak.list_send_log`

User control qua UI (sau).

---

## 8. Observability & Safety

| Concern | Mechanism |
|---|---|
| Tracing | `internal/tracing/` — span per job run, child span per send |
| Domain events | `internal/eventbus/` — emit `FBCloakSent`, `FBCloakBlocked`, `FBCloakCheckpoint` |
| Security logs | `slog.Warn("security.fbcloak.checkpoint", ...)` khi gặp checkpoint, captcha, expired cookie |
| Metrics | Prometheus: `fbcloak_sends_total{status,fanpage}`, `fbcloak_run_duration_seconds`, `fbcloak_checkpoint_total` |
| Screenshots | Lưu `~/.goclaw/data/fbcloak/screenshots/{job_id}/{send_id}_{pre|post}.png` (Standard); rotation 30 ngày |
| Kill-switch | Env `GOCLAW_FBCLOAK_KILLSWITCH=1` → ALL jobs return `disabled` ngay từ entry. Dùng khi Meta đang siết hàng loạt |
| Dry-run | Default `dry_run=TRUE` ở job level. Khi true: render template + parse inbox + log "dry_run" status, KHÔNG type/send. |
| Per-tenant isolation | tenant_id filter mọi query, incognito browser context per tenant |
| Audit | `fbcloak_send_log` immutable (không UPDATE/DELETE qua RPC) |
| Alerting | `eventbus` → notification channel (admin Telegram) khi `fbcloak_checkpoint_total++` |

---

## 9. Edition Gating

Theo [`internal/edition/edition.go`](../../internal/edition/edition.go) (đã verify):

| Edition | fbcloak feature |
|---|---|
| **Standard** | ✅ Full |
| **Lite** (Wails desktop, SQLite) | ❌ Block via `edition_gate.go` returning `ErrFeatureNotInLite` |

**Lý do block Lite:**
- Desktop user single-tenant — workflow re-engagement enterprise
- Chrome ~150MB RAM × 5 fanpages = quá nặng cho desktop
- Không có proxy infra
- ToS risk → enterprise calculated; desktop end-user tự gánh không hợp lý

Để gate sạch: thêm field `Edition.FBCloakEnabled bool`, set `true` cho `Standard`, `false` cho `Lite`. Build tag `sqliteonly` không compile package này (`//go:build !sqliteonly` ở channel.go).

---

## 10. Đối chiếu Graph API vs Cloak Browser — Dual-mode

### 10.1 Bảng so sánh

| Khía cạnh | Graph API (HUMAN_AGENT) | Cloak Browser |
|---|---|---|
| Cửa sổ tối đa | 7 ngày | Không giới hạn (giảm hiệu ứng theo thời gian) |
| TOS-compliant | ✅ Có | ❌ Vi phạm 2025 ToS |
| Tốc độ gửi | ~5 tin/s | ~1 tin/2-5min (humanized) |
| Risk page bị restrict | Thấp | Trung bình – Cao |
| Yêu cầu | App Review `human_agent_messaging` | Cookie + proxy + ngụy trang |
| Maintenance | Thấp (API stable) | Cao (DOM đổi, anti-bot upgrade) |
| Cost per send | $0 | RAM Chrome, proxy fee, dev maintenance |
| Có message tag log phía FB | Có | Không |

### 10.2 Dual-mode logic (đề xuất)

```
RPC fbcloak.send_proactive(psid, message)
  ↓
  resolve last_inbound_at(psid)
  ↓
  if delta <= 7d:
      → channel facebookmessenger.SendViaGraph(MESSAGE_TAG, HUMAN_AGENT)
      log channel_used="api"
  else:
      → resolve credential cho fanpage có conv với psid
      → channel fbcloak.Send(credential, conversation_id, message)
      log channel_used="cloak"
```

Caller layer (UI / agent) **không cần biết** kênh nào. Logic chọn nằm trong adapter unified `internal/channels/fb_proactive_router.go` (mới).

---

## 11. Risk Assessment & Mitigation

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Page bị disable do Meta phát hiện automation | M | High | daily_cap thấp, working hours, humanization, killswitch, dry-run mặc định |
| Cookie expire / xs rotated → fail mid-run | H | Med | health_probe, dump cookie sau mỗi run, alert admin |
| DOM Meta Business Suite đổi → scraper break | H | Med | 3-tier parse (AX, React, regex), screenshot khi fail, version-tag selector |
| Proxy IP bị FB blacklist | M | Med | per-fanpage fixed proxy, rotate khi status=checkpoint |
| Recipient mark spam → fanpage rating drop | M | High | content filter, per-recipient 30d cooldown, opt-out keyword detect ("STOP","HỦY","KHÔNG") trong reply trước → exclude |
| Captcha xuất hiện | M | Med | abort + screenshot + alert; KHÔNG tự solve (đẩy thêm risk) |
| LLM-generated message vi phạm Meta community | L | Med | content filter blocklist, length cap 500 chars, no URL/emoji excessive |
| GDPR/PII concern khi log message | L | Med | encrypt fbcloak_send_log.message_text? (overhead) — defer trừ khi tenant EU |
| Tenant lạm dụng (gửi spam) | M | High | global daily cap per tenant + admin review job approval |
| ToS lawsuit / account ban admin user | L | Critical | Disclaimer rõ trong UI: "Tính năng này dùng browser automation có thể vi phạm ToS. Sử dụng tự chịu trách nhiệm." Yêu cầu confirm checkbox trước khi enable. |

---

## 12. Action Items & Sequencing

### Phase 0 — Spike (1–2 ngày)
1. POC: dùng `pkg/browser` hiện tại + `go-rod/stealth` → login facebook.com bằng cookie inject thủ công → load Pages inbox → scroll → parse 1 conversation timestamp. Đánh giá DOM stability + detection tại thời điểm hiện tại.
2. Đo RAM/CPU per Chrome instance trên VPS test.
3. Quyết định proxy strategy: SOCKS5 dedicated VN IP (Bright Data, Lunaproxy, Smartproxy) — verify Meta không flag IP range nhanh.

### Phase 1 — Foundation (3–5 ngày)
4. `pkg/browser/cookie.go` + `proxy.go` + `stealth_inject.go`.
5. Migration `00XX_create_fbcloak_tables`.
6. `internal/channels/fbcloak/credential_store.go` + `factory.go` + RPC `add/list/test_credential`.
7. UI tab "FB Cloak Browser" → list credentials, manual cookie upload.

### Phase 2 — Job runner (5–7 ngày)
8. `inbox_scanner.go` + `send_executor.go` + `humanize.go` + `policy.go`.
9. `job_runner.go` + integration với `internal/cron/`.
10. Dry-run end-to-end test: log đầy đủ nhưng không send.

### Phase 3 — Production-ready (3–4 ngày)
11. Metrics + tracing + eventbus integration.
12. Killswitch + checkpoint detection + screenshot pipeline.
13. UI job CRUD + send log viewer.
14. Disclaimer modal + confirm flow.

### Phase 4 — Dual-mode + polish (2–3 ngày)
15. `fb_proactive_router.go` chọn API hoặc Cloak theo last_inbound_at.
16. Documentation user-facing (giới hạn, ToS warning, best practices).

---

## 13. Resources & References

### Browser automation & stealth
- [go-rod/stealth (GitHub)](https://github.com/go-rod/stealth)
- [stealth package — Go Packages](https://pkg.go.dev/github.com/go-rod/stealth)
- [From Puppeteer stealth to Nodriver — Castle.io blog (2025-06)](https://blog.castle.io/from-puppeteer-stealth-to-nodriver-how-anti-detect-frameworks-evolved-to-evade-bot-detection/)
- [Stealth Scraping with Puppeteer or Playwright — Browserless](https://www.browserless.io/blog/stealth-scraping-puppeteer-playwright)
- [Puppeteer Real Browser: Anti-Bot Scraping Guide — Bright Data](https://brightdata.com/blog/web-data/puppeteer-real-browser)

### Humanization
- [humanization-playwright (GitHub)](https://github.com/saksham-personal/humanization-playwright)
- [HumanCursor (PyPI)](https://pypi.org/project/HumanCursor/)
- [Bezier Mouse Movement (IJIRT paper)](https://ijirt.org/publishedpaper/IJIRT183343_PAPER.pdf)
- [Ghost Cursor Tutorial — Round Proxies](https://roundproxies.com/blog/ghost-cursor/)

### Facebook cookies & auth
- [Facebook Cookies Analysis — techexpertise/Medium](https://techexpertise.medium.com/facebook-cookies-analysis-e1cf6ffbdf8a)
- [Facebook Cookie Library (cookielibrary.org)](https://cookielibrary.org/service/facebook/)
- [c_user cookie — Cookiedatabase.org](https://cookiedatabase.org/cookie/facebook/c_user/)
- [_js_datr Cookie explained — Captain Compliance](https://captaincompliance.com/education/_js_datr-cookie-facebooks-cookie-explained/)
- [Adventures with Facebook's session cookie — Medium](https://medium.com/swlh/adventures-with-facebooks-session-cookie-3a6e10783070)

### Meta TOS
- [Meta Automated Data Collection Terms](https://www.facebook.com/legal/automated_data_collection_terms)
- [Meta Terms of Service](https://www.facebook.com/terms/)
- [Meta Updates Platform Terms 2025 — Swipe Insight](https://web.swipeinsight.app/posts/new-platform-terms-and-developer-policies-update-11744)

### Internal references (verified in codebase)
- [`pkg/browser/browser.go`](../../pkg/browser/browser.go) — Manager
- [`pkg/browser/registry.go`](../../pkg/browser/registry.go) — ProfileRegistry
- [`internal/channels/facebookmessenger/`](../../internal/channels/facebookmessenger/) — Channel pattern reference
- [`internal/crypto/aes.go`](../../internal/crypto/aes.go) — AES-256-GCM
- [`internal/edition/edition.go`](../../internal/edition/edition.go) — Edition gating
- [`internal/cron/service.go`](../../internal/cron/service.go) — Cron service
- [`internal/scheduler/lanes.go`](../../internal/scheduler/lanes.go) — Lane queue
- [`internal/eventbus/domain_event_bus.go`](../../internal/eventbus/domain_event_bus.go) — Domain events
- [`docs/research/fb-messenger-proactive-2026.md`](./fb-messenger-proactive-2026.md) — Báo cáo trước (HUMAN_AGENT path)

---

## Unresolved Questions

1. **Proxy provider concrete?** VN IPs từ Bright Data ngày càng bị Meta flag. Có nên test residential proxy VN giá tầm $150/GB hay đi dedicated VPS VN $20/tháng làm proxy? Cần benchmark thực tế.
2. **Threshold daily_cap ban đầu?** 30 là educated guess. Cần A/B test 10/20/30/50 trên 1 fanpage thật để định công thức tỷ lệ ban.
3. **Captcha frequency?** Chưa rõ Meta hiện tại challenge bao nhiều % session với cookie + proxy hợp lệ. POC Phase 0 sẽ trả lời.
4. **Marketing Messages API VN rollout date?** Nếu Meta bật trong 6 tháng tới, Cloak Browser có thể chuyển từ "primary >7d path" → "fallback only". Theo dõi changelog hàng tuần.
5. **DOM versioning?** Meta Business Suite có thể có internal A/B test (DOM khác per user). Cần plan để detect và fork selectors hay không?
6. **Lưu screenshot có vi phạm GDPR khi tin nhắn chứa PII?** Encrypt at rest hay base path tenant-scoped đủ chưa? Cần legal check trước khi rollout EU.
7. **HUMAN_AGENT có còn được sau 27 Apr 2026?** Báo cáo cũ note một Genesys announcement title gây hiểu lầm. Verify lại sau ngày 27 trước khi build path API.
