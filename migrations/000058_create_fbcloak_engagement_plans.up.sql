-- FBCloak Phase 5: Plan-based engagement orchestration.
--
-- Each row = one AI-curated re-engagement plan for one (credential, psid).
-- Lifecycle: pending → sent | replan_needed → superseded; cancelled / skipped are terminal.
-- Invariant: at most ONE row per (credential_id, psid) in non-terminal status.

CREATE TABLE fbcloak_engagement_plans (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    credential_id   UUID NOT NULL REFERENCES fbcloak_credentials(id) ON DELETE CASCADE,

    psid                TEXT NOT NULL,
    conversation_id     TEXT,
    recipient_name      TEXT,

    status              TEXT NOT NULL CHECK (status IN
                            ('pending','sent','superseded','cancelled','replan_needed','skipped')),
    scheduled_at        TIMESTAMPTZ NOT NULL,
    message_draft       TEXT NOT NULL CHECK (length(message_draft) <= 2000),
    reason              TEXT NOT NULL DEFAULT '',
    skip_reason         TEXT,

    generated_by_model  TEXT NOT NULL DEFAULT '',
    generated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    summary_version     INT NOT NULL DEFAULT 1,

    sent_at             TIMESTAMPTZ,
    send_log_id         UUID,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_fbcloak_plans_due
    ON fbcloak_engagement_plans(scheduled_at, status)
    WHERE status = 'pending';

CREATE INDEX idx_fbcloak_plans_replan
    ON fbcloak_engagement_plans(updated_at)
    WHERE status = 'replan_needed';

CREATE UNIQUE INDEX idx_fbcloak_plans_active_unique
    ON fbcloak_engagement_plans(credential_id, psid)
    WHERE status IN ('pending', 'replan_needed');

CREATE INDEX idx_fbcloak_plans_tenant_status
    ON fbcloak_engagement_plans(tenant_id, status, scheduled_at DESC);

CREATE INDEX idx_fbcloak_plans_psid
    ON fbcloak_engagement_plans(credential_id, psid);
