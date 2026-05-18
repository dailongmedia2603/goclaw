# FBCloak Spike Report — 2026-04-XX

> **Status:** template (draft) — fill in after running `fbcloak-spike` against a real test fanpage.

**Phase:** 0 of `plans/fbcloak-reengagement/`
**Plan reference:** [phase-0-spike.md](../../plans/fbcloak-reengagement/phase-0-spike.md)
**Research base:** [cloak-browser-fanpage-reengagement-2026.md](./cloak-browser-fanpage-reengagement-2026.md)

---

## How to run

```bash
# 1. Build the throwaway binary
go build -tags spike -o ./bin/fbcloak-spike ./cmd/fbcloak-spike/

# 2. Export cookies from a logged-in admin browser session using
#    EditThisCookie or Cookie-Editor extension. Save as ./cookies.json.
#    REQUIRED cookie names: c_user, xs, datr (binary will refuse otherwise).
#    DO NOT commit cookies.json — already in .gitignore.

# 3. (Optional) Get a `t_<id>` conversation_id from Graph API or DB:
#       SELECT conversation_id FROM episodic_summaries
#        WHERE source_id LIKE 'fb_backfill:<PAGE_ID>:%' LIMIT 5;

# 4. Run with full visual debug
./bin/fbcloak-spike \
    --cookies-file ./cookies.json \
    --fanpage-id <PAGE_ID> \
    --conversation-id t_xxxxxxxx \
    --proxy-url socks5://user:pass@host:port \
    --headless=false \
    --idle-minutes 30

# 5. Inspect output:
#    - Screenshots in ./docs/research/assets/fbcloak-spike/
#    - JSON summary in ./docs/research/assets/fbcloak-spike/summary.json
#    - Stdout JSON: pipe to jq for inspection
```

**SAFETY:**
- Use a **test fanpage**, not a production page. The spike does not send any messages, but it does load the inbox and idle for 30 minutes — Meta may notice.
- If `--proxy-url` is empty, the spike connects from your local IP. Use a residential VN proxy to mirror production conditions.

---

## Summary

> Fill in after run. Each line is a hard yes/no.

- **Cookie inject (S0.2):** PASS / FAIL — note: ___
- **Stealth probes (S0.3):** sannysoft pass score __ / __, creepjs trust score __ — note: ___
- **Direct thread URL (S0.4) — DECISIVE:** pattern `business_suite` / `messenger_web` / `pages_classic` / **all failed** — note: ___
- **Inbox scanner fallback (S0.4b):** only run if S0.4 fully failed. Tier chosen: AX / React / regex — note: ___
- **30-minute idle session (S0.5):** 0 checkpoint / N checkpoint, 0 captcha / N captcha — note: ___
- **Resource use:** RAM peak __ MB, CPU avg __ % — note: ___

---

## Decision

> Apply rule from plan: ≥3 of 4 critical pass → GREEN.
> Critical = (a) cookie inject, (b) stealth captures + visual review pass, (c) ≥1 direct thread URL works, (d) idle session 0 checkpoint.

**Decision:** GREEN / YELLOW / RED

- **GREEN** → proceed Phase 1. Update `plans/fbcloak-reengagement/README.md` with winning URL pattern + final stealth config.
- **YELLOW** → identify which check failed. Spike second iteration with mitigation (e.g. switch proxy provider, add fingerprint patches). Re-run.
- **RED** → escalate. Possible re-route: switch to monitoring Marketing Messages API VN rollout instead. Pause Phase 1.

---

## Screenshots

> Path examples — actual filenames will have ISO timestamp prefix.

- `assets/fbcloak-spike/*_01_cookie_health.png` — `/me` after cookie inject
- `assets/fbcloak-spike/*_02_sannysoft.png`
- `assets/fbcloak-spike/*_03_creepjs.png`
- `assets/fbcloak-spike/*_04_business_suite.png` — direct URL pattern 1
- `assets/fbcloak-spike/*_05_messenger_web.png` — direct URL pattern 2
- `assets/fbcloak-spike/*_06_pages_classic.png` — direct URL pattern 3 (only if `--page-username` set)
- `assets/fbcloak-spike/*_07_inbox_scanner.png` — fallback probe (only if S0.4 failed)
- `assets/fbcloak-spike/*_08_idle_final.png` — final state after 30-min idle

---

## Evidence

- `summary.json` produced by the binary
- HAR file (record manually via Chrome DevTools Network tab → Save All as HAR; **strip cookies** before committing)
- Console log: redirect `cmd > stderr` to capture launcher logs

---

## Resource measurement

> Use `top -pid <chrome_pid>` while the spike runs. PID is printed by rod's launcher to stderr.

| Sample | RAM (MB) | CPU (%) | Time |
|---|---|---|---|
| start | | | |
| 5 min | | | |
| 15 min | | | |
| 30 min (end) | | | |

Average CPU: __ %  ·  Peak RAM: __ MB

Capacity estimate: VPS with X GB RAM can run __ concurrent fanpages (RAM peak × profile cap + headroom).

---

## Notes & follow-ups

> Free-form. Anything surprising. Anything that needs to enter Phase 1 plan as a new Step.

- ___
- ___

---

## Unresolved questions

1. Does the winning URL pattern survive Meta DOM rollouts? (Re-run monthly during Phase 1+2 dev.)
2. Stealth `Object.defineProperty` patches — do they survive Meta's React-based hot-replacement? (Test by reloading thread in same session.)
3. Idle frequency: 90s scroll seemed to keep session alive — is shorter (30s) better, or does that *increase* suspicion? (Defer to Phase 2 humanize tuning.)
