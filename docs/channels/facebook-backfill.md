# Facebook Messenger History Backfill

End-user guide for the conversation history backfill feature on the official Facebook Page channel.

## What it does

When a Facebook Page is connected to GoClaw, backfill scans every past Messenger conversation via the Graph API, summarizes each per-customer thread, and writes the summary into the agent's episodic memory. When a returning customer next messages the Page, the agent already has context from the prior conversation — no lost history, no awkward restart.

Backfill does **not** replay tin nhắn cũ as session messages and never sends anything to the customer. It writes only to the agent's memory layer.

## Prerequisites

- A connected `facebook` channel instance (not `facebook_personal` — those use a different backend)
- Page Access Token with these scopes:
  - `pages_messaging`
  - `pages_read_engagement`
  - `pages_manage_metadata`
- App approved by Meta App Review and Business Verification (for production Page usage)
- Postgres deployment — the desktop Lite edition does not have channels

## How to use

### Auto-start on channel create

1. When creating a new Facebook channel instance, tick **"Scan conversation history after creating"** in the config section.
2. Submit the create form.
3. Open the newly-created channel's detail page. The backfill panel auto-starts on first visit.

### Manual trigger

1. Open an existing Facebook channel's detail page.
2. Scroll to the **"Conversation History Backfill"** panel below the tabs.
3. Click **"Start Backfill"**.

Progress updates in real time: conversations done vs. total, messages ingested, summaries created.

### Pause / Resume / Cancel

- **Pause** — stops between conversations. Progress is preserved; click Resume to continue.
- **Resume** — picks up from the saved cursor, no duplicate work.
- **Cancel** — terminates the job. The state stays cancelled; click Retry to start fresh.
- **Re-sync** — (only after completion) re-creates all summaries. Useful if the underlying LLM improved or conversations have evolved since the last run.

### Auto-pause on rate limit

The Graph API enforces a ~4800 calls/24h/page Business Use Case quota. When the BUC peak reaches 100%, the job auto-pauses and can be resumed after the reset window (typically 15-60 min).

## Defaults and limits

- **Max conversations per run**: 500. Covers nearly all realistic Messenger inboxes. Can be overridden via RPC (not exposed in the UI form).
- **LLM summarization**: conversations with >20 messages use the tenant's background LLM. Shorter conversations use a cheaper concat-only path.
- **Dedup**: re-runs skip conversations that already have a summary (idempotent via `SourceID`). Use **Re-sync** to force recreate.
- **Retention**: episodic summaries expire after 180 days.
- **Edition**: Postgres only. SQLite/Lite (desktop) has no channels.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|-------------|-----|
| "Page Access Token expired or invalid — please re-connect the channel" | PAT revoked or expired | Re-generate the PAT in Facebook Developer Console and update the channel credentials tab |
| Job paused with "Graph API rate limit" | BUC quota saturated | Wait for the window to reset (max 1h). Job auto-resumes. |
| Progress stalls at 0 conversations total | Page has no conversations, or the app lacks `pages_read_engagement` | Verify scopes; test with the Graph API Explorer |
| Agent still missing context after completion | Episodic embedding not yet indexed | Wait 1-2 minutes after completion — embeddings generate async |

## Architecture (brief)

```
UI panel ──RPC──► JobRunner ──goroutine──► BackfillClient ──HTTPS──► Graph API
                      │                            │
                      ├──persists state──► channel_instances.config._backfill
                      │
                      └──Summarize──► Summarizer ──► EpisodicStore
                                         │
                                         └─if long convo─► Background LLM
```

Details: see [internal/fbbackfill/README.md](../../internal/fbbackfill/README.md) and [the fork contract](../fork/fb-backfill-fork-contract.md).

## Rollout guidance (operators)

1. **Stage 1 — internal**: deploy + test on a Page with <50 conversations. Watch `slog` for `fb_backfill.job.*` events.
2. **Stage 2 — pilot**: enable for 1-2 tenants. Monitor `fb_backfill.summarize.llm_failed` rate (should be <5%).
3. **Stage 3 — general**: announce to all tenants. Watch support tickets for the first week.

## Disabling the feature

If a tenant needs the panel hidden, remove their `facebook` channel or keep it disabled. The feature cannot be globally disabled without rebuilding the binary (and it is harmless if unused — no background work without a Start call).
