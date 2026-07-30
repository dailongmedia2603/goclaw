# Facebook Backfill — Operations Runbook

Ops-oriented reference for support and incident response.

## Health signals

slog events to watch (JSON logs grep recipes):

```bash
# Normal operation (one line per job state transition)
grep 'fb_backfill.job' /var/log/goclaw.log

# Rate-limit warnings (job paused, will auto-resume)
grep 'fb_backfill.client.rate_limit' /var/log/goclaw.log

# Auth expired — user action required
grep 'fb_backfill.client.auth_expired' /var/log/goclaw.log

# LLM summarization failures (job continues with concat fallback)
grep 'fb_backfill.summarize.llm_failed' /var/log/goclaw.log

# State persistence failures (rare — indicates DB issue)
grep 'fb_backfill.state.save_failed' /var/log/goclaw.log
```

## Inspecting job state via SQL

State is embedded in `channel_instances.config` under key `_backfill`.

```sql
-- List all instances with an active backfill job
SELECT id, name, config->'_backfill'->>'status' AS status,
       config->'_backfill'->>'conversations_done' AS done,
       config->'_backfill'->>'messages_ingested' AS msgs
FROM channel_instances
WHERE channel_type = 'facebook'
  AND config ? '_backfill';

-- Jobs stuck in running for > 2 hours (suspect crashed goroutine)
SELECT id, name,
       config->'_backfill'->>'status' AS status,
       config->'_backfill'->>'updated_at' AS updated
FROM channel_instances
WHERE channel_type = 'facebook'
  AND config->'_backfill'->>'status' = 'running'
  AND (config->'_backfill'->>'updated_at')::timestamptz < NOW() - INTERVAL '2 hours';

-- Cancel a specific job via direct SQL (user cannot click Cancel)
UPDATE channel_instances
SET config = jsonb_set(config, '{_backfill,status}', '"cancelled"')
WHERE id = '<instance-uuid>';
```

## Manual actions

### Clear a stuck job

The gateway flips any `running` state to `paused` on startup (the `fb_backfill.startup.stale_paused` log event). If a job appears stuck in `running` during a live gateway:

1. SSH to the gateway host, check `ps` for the goroutine (it's inside the main binary — you cannot see it separately)
2. Tail `fb_backfill.*` logs for the instance_id to see the last activity
3. If truly stuck, the safest fix is to restart the gateway — startup will auto-pause it
4. User can then click **Resume** from the UI

### Force-delete backfill state (reset)

If a job enters a corrupted state (shouldn't happen, but safety net):

```sql
UPDATE channel_instances
SET config = config - '_backfill'
WHERE id = '<instance-uuid>';
```

After this, the panel treats the instance as "never backfilled" — user can click **Start Backfill** to begin fresh.

### Delete bad episodic entries

If a re-sync is needed but the user cannot click **Re-sync** (edge case):

```sql
DELETE FROM episodic_summaries
WHERE source_type = 'fb_backfill'
  AND agent_id = '<agent-uuid>';
```

Then force state reset above and Start fresh.

## Common escalations

| Report | Diagnosis steps | Owner |
|--------|----------------|-------|
| "Backfill started but never finishes" | Check `fb_backfill.client.rate_limit` — user needs to wait for window reset | user |
| "Agent still does not recognize returning customer" | Verify episodic row exists for PSID (see SQL above); if missing, check `fb_backfill.summarize.*` for errors | ops |
| "Token expired" banner in UI | User must re-connect the channel with a fresh PAT | user |
| "Backfill panel missing on FB channel" | Is the channel type `facebook` (Page) or `facebook_personal` (mautrix)? Backfill only for `facebook`. | product |

## Metrics to watch (if OTel enabled)

When the `otel` build tag is active (not default), the runner emits:
- Counter `fb_backfill_jobs_total{status}` — job completions by terminal status
- Counter `fb_backfill_conversations_processed_total`
- Counter `fb_backfill_graph_api_calls_total{endpoint,status}`

Alert on spike in `status="failed"` — indicates systemic issue (bad deploy, upstream Graph outage, etc.).

## Disable feature at deploy level

Not currently supported via config flag. To fully disable:

1. Build with the OSS upstream (which does not include `internal/fbbackfill`). The `Register(...)` call in `cmd/gateway.go` is the single gate — remove it to deactivate without rebuilding the package.
2. Or revert the fork branch. The gateway continues without the feature; existing backfill state rows become dormant (harmless).
