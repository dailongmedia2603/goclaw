-- fbcloak: browser-automation re-engagement for fanpage inboxes >7 days idle.
-- See plans/fbcloak-reengagement/ and docs/research/cloak-browser-fanpage-reengagement-2026.md.
-- This feature is Standard-edition only; SQLite has no equivalent schema.

CREATE TABLE IF NOT EXISTS fbcloak_credentials (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    fanpage_id      TEXT NOT NULL,
    fanpage_name    TEXT NOT NULL,
    cookies_enc     TEXT NOT NULL,
    proxy_url_enc   TEXT,
    user_agent      TEXT NOT NULL,
    viewport_w      INT NOT NULL DEFAULT 1366,
    viewport_h      INT NOT NULL DEFAULT 768,
    timezone        TEXT NOT NULL DEFAULT 'Asia/Ho_Chi_Minh',
    status          TEXT NOT NULL DEFAULT 'active',
    last_login_at   TIMESTAMPTZ,
    last_check_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, fanpage_id)
);
CREATE INDEX IF NOT EXISTS idx_fbcloak_creds_tenant_status
    ON fbcloak_credentials (tenant_id, status);

CREATE TABLE IF NOT EXISTS fbcloak_jobs (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    credential_id            UUID NOT NULL REFERENCES fbcloak_credentials(id) ON DELETE CASCADE,
    name                     TEXT NOT NULL,
    template_id              UUID,
    target_min_idle          INTERVAL NOT NULL DEFAULT INTERVAL '7 days',
    target_max_idle          INTERVAL NOT NULL DEFAULT INTERVAL '30 days',
    daily_cap                INT NOT NULL DEFAULT 30 CHECK (daily_cap > 0 AND daily_cap <= 50),
    working_hours            JSONB NOT NULL DEFAULT '{"start":"08:00","end":"21:00","tz":"Asia/Ho_Chi_Minh"}'::jsonb,
    cron_expr                TEXT NOT NULL,
    enabled                  BOOLEAN NOT NULL DEFAULT FALSE,
    dry_run                  BOOLEAN NOT NULL DEFAULT TRUE,
    use_scanner_fallback     BOOLEAN NOT NULL DEFAULT FALSE,
    next_run_at              TIMESTAMPTZ,
    last_run_at              TIMESTAMPTZ,
    last_run_status          TEXT,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_fbcloak_jobs_tenant_enabled
    ON fbcloak_jobs (tenant_id, enabled);
CREATE INDEX IF NOT EXISTS idx_fbcloak_jobs_due
    ON fbcloak_jobs (enabled, next_run_at)
    WHERE enabled = TRUE;

CREATE TABLE IF NOT EXISTS fbcloak_send_log (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    job_id            UUID NOT NULL REFERENCES fbcloak_jobs(id) ON DELETE CASCADE,
    credential_id     UUID NOT NULL REFERENCES fbcloak_credentials(id),
    fanpage_id        TEXT NOT NULL,
    conversation_id   TEXT NOT NULL,
    recipient_psid    TEXT,
    recipient_name    TEXT,
    last_inbound_at   TIMESTAMPTZ,
    message_text      TEXT NOT NULL,
    status            TEXT NOT NULL,
    skip_reason       TEXT,
    error             TEXT,
    screenshot_pre    TEXT,
    screenshot_post   TEXT,
    sent_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_fbcloak_log_tenant_sent
    ON fbcloak_send_log (tenant_id, sent_at DESC);
CREATE INDEX IF NOT EXISTS idx_fbcloak_log_credential_recipient
    ON fbcloak_send_log (credential_id, recipient_psid, sent_at DESC);
CREATE INDEX IF NOT EXISTS idx_fbcloak_log_job_status
    ON fbcloak_send_log (job_id, status);
