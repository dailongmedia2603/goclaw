# Research Report: FBCloak Engagement Orchestration — Plan-Based "Brain Mode"

> **Date:** 2026-04-27
> **Author:** Claude + Hữu Long
> **Status:** Design proposal — implementation pending user approval
> **Related:** [plans/fbcloak-reengagement/](../../plans/fbcloak-reengagement/), [cloak-browser-fanpage-reengagement-2026.md](./cloak-browser-fanpage-reengagement-2026.md)

## Table of Contents
1. [Executive Summary](#executive-summary)
2. [Problem Statement](#problem-statement)
3. [Architecture Comparison](#architecture-comparison)
4. [Recommended: Plan-Based Orchestration](#recommended-plan-based-orchestration)
5. [Data Flow](#data-flow)
6. [Schema Additions](#schema-additions)
7. [Cost Model](#cost-model)
8. [Implementation Phases](#implementation-phases)
9. [Open Questions](#open-questions)
10. [Sources](#sources)

---

## Executive Summary

**Verdict:** Build a **Plan-Based Orchestration** layer on top of existing FBCloak. Read inbox 1× → LLM produces engagement plan (when + what + reason) per conversation → store plan rows in DB → executor cron sends due plans → event-driven replan when khách reply.

**Why this works:**
- `episodic_summaries` đã có sẵn (~800 tokens/conversation) — KHÔNG phải đọc raw messages
- 1 LLM call/conversation tại plan-time (rare), 1 call/event tại replan (sparse) → token cost giảm 50–100× so với naive cron-LLM
- Industry pattern matches: "journey orchestration" trong CRM 2026 (real-time event-driven, AI-curated)
- Reuse hết existing infra (eventbus, scheduler, episodic_summaries, providers.Adapter, send_executor)

**Cost (1000 conversations, Haiku 4.5):**
- Initial plan generation: **~$1.80** one-time (~$0.90 with Batch API 50% off)
- Steady state: **~$2–5/month** (replan after each send + 5-minute prompt cache)
- vs naive cron-read approach: **~$50–100/day** (24× more expensive)

**Time-to-implement:** ~3–4 dev days (~600 LOC + tests + 1 migration + 1 prompt skill).

---

## Problem Statement

User vision (verbatim):
> "Đọc toàn hộp thư 1 lần, lấy ngữ cảnh và xác định ra là ở hộp thư đó cần chăm sóc như nào và thời gian nào là tốt nhất sau đó lên 1 lịch hẹn gởi tin nhắn vào đâu đó, sau đó khi tin nhắn đó được gởi đi thì lúc đó goclaw mới thực hiện vào đọc lại hộp thư đó để tiếp tục lên lịch."

Concrete requirements:
1. Read full inbox 1× → context per conversation
2. Decide: send hay không send? Khi nào? Nội dung gì?
3. Persist quyết định → executor cron gửi đúng giờ
4. Sau khi gửi (hoặc khách reply) → đọc lại "delta" → tái lập lịch
5. Scale to 1000+ conversations
6. Token-efficient

Current FBCloak state (`SimpleTemplateRenderer` in [template_renderer.go](../../internal/channels/fbcloak/template_renderer.go)):
- Cron tick → query `episodic_summaries` by time window → render fixed template (chỉ thay `{customer_name}`) → send
- KHÔNG đọc context, KHÔNG có AI decide, mass-send template

Gap: 100% of the user's vision is missing.

---

## Architecture Comparison

### Approach A: Naive — LLM-on-every-cron-tick

```
Cron tick (hourly) → for each PSID idle 7-30d:
  → load full episodic summary
  → call LLM "should send + what?"
  → if yes, send immediately
```

**Tokens/day:** 1000 conv × 800 tokens × 24 ticks = 19M tokens/day → ~$19/day Haiku, ~$57/day Sonnet
**Verdict:** ❌ Wasteful — same conversation re-evaluated 24×/day with same data.

### Approach B: Plan-Based Orchestration (RECOMMENDED)

```
Phase 1 — Plan Generation (rare, expensive):
  Weekly cron → for each PSID without active plan:
    → load episodic summary
    → call LLM "should send? when? what?"
    → write Plan row {scheduled_at, message_draft, reason, status='pending'}

Phase 2 — Plan Execution (frequent, cheap):
  Hourly cron → SELECT plans WHERE status='pending' AND scheduled_at <= NOW()
    → for each → send via existing send_executor
    → mark status='sent'

Phase 3 — Plan Invalidation (event-driven, no LLM):
  Subscribe to fbm.message.received event
  When khách reply for PSID with active plan:
    → mark status='replan_needed' (NO LLM call yet)

Phase 4 — Replan Worker (sparse):
  Hourly cron → SELECT plans WHERE status='replan_needed'
    → load updated episodic summary
    → call LLM (only changed conversations)
    → update plan row
```

**Tokens/day (1000 conversations, ~10% reply weekly):**
- Initial: 1000 × 800 = 800k input + 1000 × 200 = 200k output → $0.80 + $1.00 = **$1.80 one-time**
- Replan: 100 conv/week × 800 input + 200 output → $0.08 + $0.10 = **$0.18/week** = $0.72/month
- Cron polling: pure SQL, $0
- **Total: ~$3–5/month for 1000 conversations**

**Verdict:** ✅ Đúng ý user, hiệu quả token, scale tốt.

### Approach C: Embedding Cluster + Cohort Templates

Group conversations by embedding similarity → 1 LLM call/cluster generates template → use template for all in cluster.

**Pros:** Lowest token cost (O(K) plans for N conversations, K << N)
**Cons:** Loss of per-recipient personalization, complex bucketing logic, only meaningful at 10k+ scale
**Verdict:** ❌ Premature optimization for 1000-conversation scale; revisit at 10k+.

### Approach D: Hybrid (Trigger + Plan)

Use plan-based as default + add **trigger conditions** for high-priority signals:
- Khách hỏi giá nhưng chưa close → trigger immediate plan with urgency
- Khách hứa "để mình suy nghĩ" → schedule plan +5 days
- Khách complain → KHÔNG re-engage

Implemented as part of Plan Generator's prompt (LLM-driven trigger detection from summary).

**Verdict:** ✅ Subset of Approach B — included as natural prompt-engineering, not a separate system.

---

## Recommended: Plan-Based Orchestration

### Component diagram

```
                  ┌─────────────────────────────────────┐
                  │          GoClaw Brain               │
                  └─────────────────────────────────────┘
                                  │
            ┌─────────────────────┼─────────────────────┐
            ▼                     ▼                     ▼
     ┌──────────────┐    ┌──────────────┐     ┌──────────────┐
     │  Plan Gen    │    │  Executor    │     │  Replan      │
     │  (weekly)    │    │  (hourly)    │     │  (hourly)    │
     │              │    │              │     │              │
     │  • read      │    │  • SELECT    │     │  • SELECT    │
     │    summary   │    │    due plans │     │    needs     │
     │  • LLM call  │    │  • send via  │     │    replan    │
     │  • write     │    │    fbcloak   │     │  • LLM call  │
     │    plan row  │    │  • mark sent │     │  • update    │
     └──────────────┘    └──────────────┘     └──────────────┘
            │                     ▲                     ▲
            ▼                     │                     │
     ┌─────────────────────────────────────────────────┐
     │      fbcloak_engagement_plans (PG)              │
     └─────────────────────────────────────────────────┘
            ▲                     ▲                     │
            │                     │                     │
     ┌──────────────┐    ┌──────────────┐               │
     │  episodic_   │    │  fbcloak_    │               │
     │  summaries   │    │  send_log    │               │
     │  (existing)  │    │  (existing)  │               │
     └──────────────┘    └──────────────┘               │
            ▲                                           │
            │                                           ▼
     ┌──────────────────┐              ┌──────────────────────┐
     │ DomainEventBus   │ ──────────►  │  Invalidator         │
     │ fbm.msg.received │              │  (event subscriber)  │
     └──────────────────┘              │  • mark plan as      │
            ▲                          │    replan_needed     │
            │                          └──────────────────────┘
     ┌──────────────────┐
     │ fbbackfill (live │
     │ webhook + cron)  │
     └──────────────────┘
```

### Concrete responsibilities

| Component | Trigger | Frequency | Token cost/run |
|---|---|---|---|
| **Plan Generator** | Cron + admin "Run now" | Weekly | ~800 input + 200 output per PSID |
| **Plan Executor** | Cron tick | Hourly | 0 (pure SQL + browser send) |
| **Plan Invalidator** | `fbm.message.received` event | Real-time | 0 (just SQL UPDATE) |
| **Replan Worker** | Cron tick | Hourly | ~800 input + 200 output per replan |

### Plan Generator — LLM prompt structure

System prompt (cached via Anthropic 5-min cache, 90% off after first call):
```
You are a Vietnamese customer-care orchestrator for fanpage {fanpage_name}.
For each conversation summary provided, decide:
1. should_send: bool — only true if customer is likely to engage positively
2. send_after_days: int (1-30) — when to send, considering reply patterns
3. message: string (≤500 chars) — the message to send
4. reason: string — why this decision

Skip rules (return should_send=false):
- Customer explicitly said "ngừng nhắn" / "đừng gửi tin nữa"
- Conversation ended in unresolved complaint
- Customer already converted (purchased) within last 30 days
- Less than 7 days since last inbound

Output JSON only.
```

User prompt (per conversation, NOT cached):
```
Customer: {recipient_name}
Last inbound: {last_message_at}
Episodic summary:
{episodic_summary_text}

Recent message snippets:
{last_5_messages}
```

**Token math per conversation:** ~600 cached system + ~600 fresh input + ~200 output = 1400 tokens.

### Replan triggers

Plan Invalidator listens to `EventFBMessageReceived` (or similar — verify exact event name in `internal/eventbus/event_types.go`). When khách reply:

```sql
UPDATE fbcloak_engagement_plans
   SET status = 'replan_needed', updated_at = NOW()
 WHERE psid = $1 AND credential_id = $2 AND status IN ('pending', 'sent');
```

Replan worker picks up `replan_needed` rows next tick. Loads updated `episodic_summaries` (fbbackfill incremental keeps this fresh) → re-runs LLM call → writes new plan with same `psid` (UPSERT or insert+expire-old).

### Key design decisions

| Decision | Choice | Rationale |
|---|---|---|
| Plan storage | PG table `fbcloak_engagement_plans` | Existing PG infra, transactional, good for cron polling |
| Plan generation timing | Weekly cron (configurable) | Bulk batch → use Anthropic Batch API for 50% discount |
| Plan execution timing | Hourly cron | Granularity matches `scheduled_at` precision |
| Replan trigger | Event-driven via `DomainEventBus` | Real-time when khách reply, no cron lag |
| LLM model | Haiku 4.5 (default) / Gemini Flash | Latency + cost; user can override per fbcloak.cfg |
| Personalization | Per-conversation (no clustering) | 1000 scale doesn't need clustering yet |
| Idempotency | UNIQUE (psid, credential_id, status='pending') | At most 1 active plan per recipient |
| Schema migration | New table, no changes to existing | Decoupled from credentials/jobs/send_log |

---

## Data Flow

### Steady-state lifecycle of a single conversation

```
T0:  fbbackfill ingests historical messages → episodic_summaries row created
        token cost: 0 (already happened in fbbackfill phase 4)

T1:  Plan Generator weekly tick picks up PSID → LLM call:
        decision = {should_send: true, send_after_days: 14, message: "...", reason: "..."}
     INSERT INTO fbcloak_engagement_plans
       (psid, credential_id, scheduled_at=T1+14d, message_draft=..., status='pending')
        token cost: ~1400

T2 (= T1+14d):  Plan Executor hourly tick:
     SELECT * WHERE scheduled_at <= NOW() AND status='pending' → finds this plan
     → uses existing fbcloak.SendExecutor.Execute (humanize, verify, send via Rod browser)
     → on success: UPDATE plan SET status='sent', sent_at=NOW()
        token cost: 0

T3 (= T2+3d):  Khách reply via fbm channel
     fbm.OnMessage handler emits EventFBMessageReceived
     Invalidator subscriber: UPDATE plan SET status='replan_needed' for this PSID
     fbbackfill incremental cron updates episodic_summary with new content
        token cost: 0 (just events + SQL)

T4 (= T3 + ~1h):  Replan Worker hourly tick:
     SELECT * WHERE status='replan_needed' → picks this PSID
     → loads UPDATED episodic_summary
     → LLM call with new context
     → INSERT new plan, UPDATE old to status='superseded'
        token cost: ~1400

(Loop continues — at most 1 LLM call/event per recipient)
```

### Batch processing optimization

For initial bulk plan generation across 1000+ conversations, use **Anthropic Batch API**:

```go
// Pseudo-code
batchReq := []BatchRequest{}
for _, psid := range needsPlanning {
    batchReq = append(batchReq, BatchRequest{
        CustomID: psid,
        Body:     buildPrompt(summary),
    })
}
results := anthropic.BatchCreate(ctx, batchReq)
// Wait up to 24h for completion (or poll)
for _, res := range results {
    insertPlan(res.CustomID, parseDecision(res.Output))
}
```

**Saves 50%** on bulk generation. Suitable for: weekly Plan Generator runs (latency-tolerant), initial onboarding of new fanpage.

---

## Schema Additions

### Migration 000058: `fbcloak_engagement_plans`

```sql
-- migrations/000058_create_fbcloak_engagement_plans.up.sql

CREATE TABLE fbcloak_engagement_plans (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    credential_id   UUID NOT NULL REFERENCES fbcloak_credentials(id) ON DELETE CASCADE,

    -- Identity (plan is per-recipient per-fanpage)
    psid                TEXT NOT NULL,
    conversation_id     TEXT,            -- "t_xxxxx" from fbbackfill, nullable
    recipient_name      TEXT,

    -- Decision
    status              TEXT NOT NULL CHECK (status IN
                            ('pending','sent','superseded','cancelled','replan_needed','skipped')),
    scheduled_at        TIMESTAMPTZ NOT NULL,
    message_draft       TEXT NOT NULL,
    reason              TEXT,            -- LLM's explanation
    skip_reason         TEXT,            -- when status='skipped' (e.g. "customer_opted_out")

    -- LLM provenance
    generated_by_model  TEXT,            -- e.g. "claude-haiku-4-5"
    generated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    summary_version     INT NOT NULL DEFAULT 1,  -- bumps when episodic_summary changed

    -- Execution
    sent_at             TIMESTAMPTZ,
    send_log_id         UUID,            -- FK to fbcloak_send_log when sent

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Hot path: Executor's polling query
CREATE INDEX idx_fbcloak_plans_due
    ON fbcloak_engagement_plans(scheduled_at, status)
    WHERE status = 'pending';

-- Replan worker's polling query
CREATE INDEX idx_fbcloak_plans_replan
    ON fbcloak_engagement_plans(updated_at)
    WHERE status = 'replan_needed';

-- At most 1 active plan per (credential, psid)
CREATE UNIQUE INDEX idx_fbcloak_plans_active_unique
    ON fbcloak_engagement_plans(credential_id, psid)
    WHERE status IN ('pending', 'replan_needed');

-- Tenant + status drilldowns from UI
CREATE INDEX idx_fbcloak_plans_tenant_status
    ON fbcloak_engagement_plans(tenant_id, status, scheduled_at DESC);
```

```sql
-- migrations/000058_create_fbcloak_engagement_plans.down.sql
DROP TABLE IF EXISTS fbcloak_engagement_plans CASCADE;
```

Bump `internal/upgrade/version.go` `RequiredSchemaVersion = 58`.

### No changes needed for SQLite

FBCloak is `!sqliteonly` — Lite edition không có feature này, nên không touch `internal/store/sqlitestore/`.

---

## Cost Model

### Assumptions

- 1000 active conversations per fanpage
- Anthropic Haiku 4.5: input $1/M, output $5/M
- Per-conversation prompt: ~600 cached system + 600 fresh input + 200 output = 1400 tokens total
- ~10% conversations get replanned weekly (khách reply)
- 5-min prompt cache for system prompt (90% off cache reads after first miss)

### One-time initial plan generation (1000 conversations)

| Item | Tokens | Cost |
|---|---|---|
| Cached system prompt write (1×) | 600 × 1.25 | $0.00075 |
| Cached system prompt reads (999×) | 999 × 600 × 0.1 | $0.05994 |
| Fresh input (per call × 1000) | 600 × 1000 | $0.60 |
| Output (per call × 1000) | 200 × 1000 | $1.00 |
| **Subtotal** | | **$1.66** |
| With Batch API (-50%) | | **$0.83** |

### Steady-state replan (per week)

| Item | Tokens | Cost |
|---|---|---|
| 100 replans (10% of 1000) | 100 × 1400 | $0.14 input + $0.10 output |
| With cache (system shared) | 100 × 600 × 0.1 + 600 × 0.1 + 200 × 5 | $0.07 |
| **Per week** | | **~$0.07–0.18** |
| **Per month** | | **~$0.30–0.80** |

### Comparison: Naive cron-LLM approach

| Item | Tokens/day | Cost/day | Cost/month |
|---|---|---|---|
| 1000 conv × 800 tokens × 24 ticks | 19.2M input + 4.8M output | $19.2 + $24.0 = $43.2 | **$1296** |

### Verdict

**Plan-based saves ~99%** — $5/month vs $1296/month for 1000-conversation fanpage.

Even if user wants Sonnet (3× more expensive than Haiku):
- Plan-based: ~$15/month
- Naive: ~$3,888/month

---

## Implementation Phases

### Phase 5 (proposed): Plan-Based Orchestration

> Builds on top of merged Phase 1-4 fbcloak. New plan separate from fbcloak-reengagement.

| Step | Files | LOC |
|---|---|---|
| Migration 058 | `migrations/000058_*.sql` | ~50 |
| Plan store | `internal/channels/fbcloak/plan_store.go` | ~150 |
| PG impl | `internal/store/pg/fbcloak_plans.go` | ~200 |
| Plan generator | `internal/channels/fbcloak/plan_generator.go` | ~200 |
| Plan executor | `internal/channels/fbcloak/plan_executor.go` | ~150 |
| Invalidator | `internal/channels/fbcloak/plan_invalidator.go` | ~80 |
| Replan worker | `internal/channels/fbcloak/replan_worker.go` | ~120 |
| LLM prompt skill | `bundled-skills/fbcloak/orchestrate.md` | (config) |
| RPC handlers | `internal/gateway/methods/fbcloak_plans.go` | ~150 |
| UI: Plans tab | `ui/web/src/pages/fbcloak/plans-tab.tsx` | ~250 |
| Tests | `*_test.go` | ~400 |
| **Total** | | **~1750 LOC** |

Time estimate: **~3–4 dev days**.

### Phase 5.5 (optional): Batch API integration

After Phase 5 stable, swap initial plan-gen path to Anthropic Batch API:
- Reduces cost 50%
- Async — admin clicks "Bootstrap plans" → 1–24h later plans appear
- ~100 LOC change in `plan_generator.go`

### Phase 6 (future): Embedding cluster optimization

Only when scale > 10k conversations/fanpage. Defer.

---

## Open Questions

1. **Replan frequency cap**: Khách spam reply 50 lần/giờ → tránh replan thrash. Đề xuất: rate-limit replan = 1× per PSID per 6h (latest reply triggers, intermediate ones queue).

2. **LLM model selection**: Default Haiku or Sonnet? Sonnet output quality cao hơn nhưng 3× cost. Đề xuất: configurable per-tenant; Haiku default.

3. **Multilingual prompts**: Fanpage tiếng Việt vs đa ngôn ngữ? Bundled skill `fbcloak/orchestrate.md` có 1 version VN; admin có thể override.

4. **Plan TTL**: Plan `scheduled_at` cách quá xa (>30 days) có nên auto-expire? Đề xuất: `scheduled_at > now+90d` → auto-cancel + log.

5. **Admin override**: User muốn manual edit plan (sửa message, đổi giờ) trước khi gửi? Cần UI ở plans-tab.tsx — defer Phase 5.5.

6. **Cohort signals across customers**: Nếu LLM detect "X khách hỏi cùng 1 sản phẩm" → có nên alert admin? Defer Phase 6+ (analytics, không phải core engagement).

7. **Disclaimer scope**: Plan-based path có cần re-ack disclaimer không? Đề xuất: NO — disclaimer áp dụng feature-level (đã ack trong Phase 4), không per-plan.

---

## Sources

- [Anthropic API Pricing in 2026: Complete Guide — Models, Caching, Batch & Optimization (finout.io)](https://www.finout.io/blog/anthropic-api-pricing)
- [Prompt caching — Claude API Docs](https://platform.claude.com/docs/en/build-with-claude/prompt-caching)
- [Pricing — Claude API Docs](https://platform.claude.com/docs/en/about-claude/pricing)
- [Revolutionizing CRM Implementation with Event-Driven Architecture (saasmag.com)](https://www.saasmag.com/revolutionizing-crm-implementation/)
- [Why Real-Time Customer Engagement Still Breaks in 2026 (cxtoday.com)](https://www.cxtoday.com/customer-engagement-journey-orchestration/real-time-journey-orchestration/)
- [Leveraging an Event-Driven Architecture to Build Meaningful Customer Relationships (GetYourGuide)](https://www.getyourguide.careers/posts/leveraging-an-event-driven-architecture-to-build-meaningful-customer-relationships)
- [Customer engagement software: 10 best platforms and AI trends for 2026 (monday.com)](https://monday.com/blog/crm-and-sales/customer-engagement-software/)

### Internal references
- [internal/channels/fbcloak/template_renderer.go](../../internal/channels/fbcloak/template_renderer.go) — current state (placeholder)
- [internal/channels/fbcloak/target_resolver.go](../../internal/channels/fbcloak/target_resolver.go) — episodic_summaries query path
- [internal/eventbus/event_types.go](../../internal/eventbus/event_types.go) — DomainEventBus to subscribe for invalidation
- [internal/fbbackfill/](../../internal/fbbackfill/) — incremental episodic_summary writer
- [plans/fbcloak-reengagement/README.md](../../plans/fbcloak-reengagement/README.md) — Phase 1-4 plan + decision log
