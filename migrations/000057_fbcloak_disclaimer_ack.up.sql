-- Phase 4: per-tenant disclaimer acknowledgement for fbcloak jobs.
-- Any tenant admin acks the current disclaimer version on behalf of the
-- tenant. Bumping `version` (e.g. v1.0 → v1.1) re-requires acknowledgement.
-- Server-side gate: ToggleJob(enabled=true) checks for a row matching
-- (tenant_id, current_version) before allowing the flip.

CREATE TABLE IF NOT EXISTS fbcloak_disclaimer_ack (
    tenant_id  UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    version    TEXT NOT NULL,
    user_id    UUID,
    acked_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, version)
);

CREATE INDEX IF NOT EXISTS idx_fbcloak_disclaimer_ack_tenant
    ON fbcloak_disclaimer_ack (tenant_id);
