# FBCloak — Operator Runbook

> Vận hành sản xuất: cấu hình, monitor, alert, killswitch, recovery. Cho người vận hành cluster GoClaw.

## 1. Cấu hình runtime

### 1.1 Edition gate
FBCloak chỉ active khi `edition.Current().FBCloakEnabled == true`. Lite (desktop) luôn = false. Standard (PG) luôn = true. Không có config flag để tắt — dùng killswitch.

### 1.2 Config block (`config.json5`)
```json5
{
  channels: {
    fbcloak: {
      screenshot_dir: "/var/lib/goclaw/fbcloak/screenshots",  // empty → {dataDir}/fbcloak/screenshots
      screenshot_retention_days: 30,                           // 0 → 30 default
      killswitch_poll_seconds: 30,                             // 0 → 30; <0 → disabled
      max_concurrent: 5                                        // 0 → 5
    }
  }
}
```

### 1.3 Env vars
- `GOCLAW_FBCLOAK_KILLSWITCH=1` → engage killswitch (poll `KillswitchPollSeconds`)
- `GOCLAW_FBCLOAK_KILLSWITCH=0` (hoặc unset) → disengage

## 2. Wiring (cmd/gateway.go) — checklist khi deploy

Phase 3+4 đã đưa các deps vào `fbcloak.Service`:

```go
disclaimerStore := pg.NewPGFBCloakDisclaimerStore(db)
screenshot, _ := fbcloak.NewScreenshotWriter(cfg.DataDir, cfg.Channels.FBCloak.ScreenshotDir, retention)
service, _ := fbcloak.NewService(fbcloak.Deps{
    CredentialStore: credStore,
    HealthProbe:     probe,
    JobStore:        jobStore,
    JobRunner:       runner,
    Events:          domainEventBus,    // Phase 3
    Screenshot:      screenshot,        // Phase 3
    Disclaimer:      disclaimerStore,   // Phase 4
    Logger:          slog.Default(),
})

// Killswitch watcher (Phase 3)
killWatcher, _ := fbcloak.NewKillswitchWatcher(
    service.KillswitchFlag(),
    cfg.Channels.FBCloak.KillswitchPollDuration(),
    slog.Default(),
)
killWatcher.Start(ctx)

// Phase 4 router (optional — wire when fbm Graph API exists)
router := &fbproactive.FBProactiveRouter{
    Resolver: yourLastInboundResolver,    // implement against episodic_summaries
    Graph:    nil,                         // wait for fbm Graph API plan to land
    Cloak:    cloakSenderAdapter{service}, // adapter calling fbcloak.Service.SendProactive
}

// RPC registration
fbcloakMethods := methods.NewFBCloakMethods(service, cfg)
fbcloakMethods.Register(router)
fbcloakMethods.RegisterJobs(router)
methods.NewFBCloakPhase4Methods(service, router).Register(router)
```

> **Note:** GraphSender nil đồng nghĩa `fbcloak.send-proactive` cho ≤7d trả `ErrGraphSenderUnconfigured`. Cloak path (>7d) hoạt động bình thường.

## 3. Metrics dashboard

### 3.1 Counters expose ở `/status`
| Counter | Mô tả | Alert ngưỡng |
|---|---|---|
| `fbcloak_sends_attempted_total` | Mỗi `Execute()` qua guard | tăng đột biến (10× baseline trong 1h) |
| `fbcloak_sends_succeeded_total` | Status = sent | drop về 0 trong 6h khi có job enabled → DOM break |
| `fbcloak_sends_failed_total` | Status = failed | > 5/h liên tiếp → page issue |
| `fbcloak_checkpoint_total` | Detector trip | **> 0 trong 1h → page admin Telegram ngay** |
| `fbcloak_cookie_expired_total` | Credential.status = expired | > 0 → cookie rotation cần |
| `fbcloak_killswitch_aborts` | Job abort vì killswitch | non-zero → confirm killswitch intentional |
| `fbcloak_active_workers` | Gauge in-flight | > MaxConcurrent → bug; thường = 0–MaxConcurrent |
| `fbcloak_screenshot_errors_total` | Screenshot capture fail | > 10/h → disk / Rod issue |
| `fbcloak_job_runs_total` | Mỗi RunOnce | flat 0 khi có job enabled → runner stalled |
| `fbcloak_job_runs_killed_total` | Status = killed | tăng = checkpoint hoặc killswitch active |

### 3.2 Eventbus subscribers
Subscribe các event để alert real-time:

| Event | Source | Khuyến nghị handler |
|---|---|---|
| `fbcloak.checkpoint` | `internal/channels/fbcloak/events.go` | Telegram admin + Slack #ops; đính ScreenshotPath |
| `fbcloak.blocked` | Policy skip | Aggregate hourly; alert khi spike |
| `fbcloak.sent` | Successful send | Optional — audit log to S3 |
| `fbcloak.job_completed` | Run cycle done | Alert khi `Status == "fail"` 3 lần liên tiếp |

## 4. Killswitch — runbook

### 4.1 Khi nào kích hoạt
- Page bị Meta restrict → **NGAY**
- Multiple `fbcloak_checkpoint_total` trong 30 phút
- Operator yêu cầu hard stop
- Pre-deploy migration (DB lock có thể làm runner stall)

### 4.2 Cách kích hoạt
```bash
# Nếu chạy systemd
sudo systemctl set-environment GOCLAW_FBCLOAK_KILLSWITCH=1
sudo systemctl restart goclaw-gateway   # nếu KillswitchPollSeconds < 0

# Hoặc nếu poll watcher đang chạy (default)
echo "GOCLAW_FBCLOAK_KILLSWITCH=1" | sudo tee -a /etc/goclaw.env
sudo systemctl reload goclaw-gateway   # picks up env on next poll (≤30s)
```

Watcher log:
```
WARN security.fbcloak.killswitch_changed engaged=true source=env_watcher
```

### 4.3 Verify đã engage
- WS RPC nào của fbcloak → trả `ErrUnavailable` + `MsgFBCloakKillswitch`
- Job runner tick: gặp `Killed=true` → return `JobStatusKilled` + `IncJobRunsKilled`
- UI hiển thị banner đỏ ở /fbcloak

### 4.4 Disengage
```bash
unset GOCLAW_FBCLOAK_KILLSWITCH
# hoặc
sudo systemctl unset-environment GOCLAW_FBCLOAK_KILLSWITCH
sudo systemctl reload goclaw-gateway
```

## 5. DOM break recovery

Khi Meta đổi DOM Business Inbox và `fbcloak_sends_failed_total` spike:

1. **Engage killswitch** (mục 4.2).
2. **Thu thập fixture mới**:
   ```bash
   ./bin/fbcloak-spike --cookies cookies.json --capture-html > new-inbox.html
   ```
3. So sánh với fixture cũ ở `internal/channels/fbcloak/testdata/`.
4. Update selector trong [target_resolver.go](../internal/channels/fbcloak/target_resolver.go) hoặc [send_executor.go](../internal/channels/fbcloak/send_executor.go).
5. Update fixture HTML trong `testdata/`.
6. Run `go test ./internal/channels/fbcloak/...`.
7. PR + deploy.
8. **Disengage killswitch** sau khi smoke pass trên fanpage test.

## 6. Cookie rotation

Trigger:
- `fbcloak_cookie_expired_total` tăng
- Credential health probe trả `ok=false`
- User báo từ UI

Quy trình:
1. Liên hệ tenant admin để export cookie mới (xem user-guide §3).
2. UI: /fbcloak → Credentials → xoá credential cũ (cascade jobs nếu cần) hoặc thêm credential mới (cùng `fanpage_id` sẽ conflict UNIQUE → phải xoá cũ).
3. Verify health probe → status `active`.
4. Re-enable jobs đã pause khi credential expired.

## 7. Per-fanpage disable

Khi 1 fanpage bị Meta restrict mà tenant chạy nhiều fanpage:

1. Vào /fbcloak → Credentials → tìm credential fanpage đó.
2. Database direct (nếu UI bị block):
   ```sql
   UPDATE fbcloak_credentials SET status = 'disabled'
    WHERE tenant_id = '...' AND fanpage_id = '...';
   ```
3. Job runner sẽ skip credential disabled với error `credential status=disabled, abort`.
4. Re-enable bằng `UPDATE ... SET status = 'active'` sau khi Meta gỡ restrict.

## 8. Disclaimer version bump

Khi nội dung [docs/fbcloak-tos-disclaimer.md](./fbcloak-tos-disclaimer.md) thay đổi:

1. Bump `CurrentDisclaimerVersion` trong [internal/channels/fbcloak/disclaimer.go](../internal/channels/fbcloak/disclaimer.go) (vd. `v1.0` → `v1.1`).
2. Update i18n (`fbcloak.json` 3 locale + Markdown bản tiếng Việt nguồn).
3. Deploy. Mọi tenant bật Job sau bump sẽ thấy modal "Required" và bị block tới khi ack v1.1.
4. (Optional) Background job query tenants có ack=v1.0 + jobs.enabled=true → email/notify trước.

## 9. Escalation

| Severity | Hiện tượng | Action |
|---|---|---|
| **P0** | Page Meta xoá / khóa vĩnh viễn | Engage killswitch + page on-call + ticket Legal |
| **P1** | `fbcloak_checkpoint_total` > 5/h | Engage killswitch + Telegram admin + cookie rotation |
| **P2** | DOM break (sends_failed > 5/h) | Engage killswitch + DOM recovery (mục 5) |
| **P3** | Cookie expired single fanpage | UI rotation |
| **P4** | UI lỗi load | Frontend hotfix |

## 10. Phụ lục

- Sentry / OTel traces: span name `fbcloak.job.run`, `fbcloak.send` — filter theo TenantID
- Log levels: `slog.Warn("security.fbcloak.*", ...)` cho mọi sự kiện security
- DB cleanup: send_log không có TTL — chạy cron prune tuỳ chính sách (ví dụ `DELETE FROM fbcloak_send_log WHERE sent_at < NOW() - INTERVAL '90 days'`)

## 11. Phase 5 (Plan-Based Brain Mode) — known limitations

> Phase 5 (commits `f6ec1ee9` → `8e8cc42e` + hotfix `8693d3e6`) ships AI-curated
> per-recipient engagement plans. Default disabled (`channels.fbcloak.orchestrator.enabled=false`).

### Replan-on-customer-reply NOT yet automatic
`ReplanScanner` polls `status='replan_needed'` rows but nothing flips a plan to that
status today. `MarkPlanReplanNeeded` is exposed but unwired. Until the inbound-message
hook lands, plans **fire on schedule even if the customer already replied**.

**Operator workaround:** if monitoring shows a plan firing after a customer reply, the
admin must Cancel it manually via the UI (`/fbcloak` → Plans tab → row Cancel button)
before `scheduled_at` triggers Executor.

**Tracking:** follow-up commit will wire either:
- a `SummaryVersionLookup` against `episodic_summaries.created_at` so the scanner
  detects drift between plan generation time and current summary mtime, OR
- a direct subscriber on the inbound-message bus that calls
  `PlanStore.MarkReplanNeeded(credentialID, psid)`.

### Other Phase 5 caveats
- `PlanCleanup` only auto-cancels plans scheduled > now+90d (Service.CreatePlan
  rejects these at insert time anyway). Forgotten plans (credential disabled, plan
  scheduled in past, never executed) are NOT auto-cleaned. Operator must SQL-prune
  via vps-command if they accumulate: `UPDATE fbcloak_engagement_plans SET status='cancelled' WHERE status='pending' AND scheduled_at < NOW() - INTERVAL '30 days';`
- Anthropic Batch API integration deferred — bulk plan generation runs synchronously
  with prompt cache (still 90% off cache reads).
- Embedding-cluster optimization deferred — only meaningful at >10k conversations/fanpage.
