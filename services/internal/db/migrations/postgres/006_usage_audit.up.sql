CREATE TABLE usage_counters (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    day         DATE NOT NULL,
    traces      BIGINT NOT NULL DEFAULT 0,
    flight_checks BIGINT NOT NULL DEFAULT 0,
    api_calls   BIGINT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, day)
);

CREATE TABLE audit_log (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID REFERENCES orgs(id) ON DELETE SET NULL,
    user_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    api_key_id  UUID REFERENCES api_keys(id) ON DELETE SET NULL,
    action      TEXT NOT NULL,
    resource    TEXT NOT NULL DEFAULT '',
    resource_id TEXT NOT NULL DEFAULT '',
    ip_address  INET,
    user_agent  TEXT NOT NULL DEFAULT '',
    metadata    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX audit_log_org_id_idx    ON audit_log(org_id);
CREATE INDEX audit_log_user_id_idx   ON audit_log(user_id);
CREATE INDEX audit_log_created_at_idx ON audit_log(created_at DESC);
