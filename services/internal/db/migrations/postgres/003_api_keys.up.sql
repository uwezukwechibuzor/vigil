-- API keys: only the SHA-256 hash + a short prefix are stored; the plaintext is shown once at creation.
CREATE TABLE api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    project_id      UUID REFERENCES projects(id) ON DELETE CASCADE,
    created_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    name            TEXT NOT NULL,
    key_prefix      TEXT NOT NULL,          -- first 8 chars of the key, for display
    key_hash        TEXT NOT NULL UNIQUE,   -- SHA-256 hex of the full key
    scopes          TEXT[] NOT NULL DEFAULT '{}',
    expires_at      TIMESTAMPTZ,
    last_used_at    TIMESTAMPTZ,
    revoked_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX api_keys_org_id_idx    ON api_keys(org_id);
CREATE INDEX api_keys_key_hash_idx  ON api_keys(key_hash);
