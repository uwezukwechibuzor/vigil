CREATE TABLE chains (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chain_id        BIGINT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    native_token    TEXT NOT NULL DEFAULT 'ETH',
    block_time_ms   INT NOT NULL DEFAULT 12000,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TYPE endpoint_kind AS ENUM ('rpc', 'indexer', 'oracle', 'custom');

CREATE TABLE endpoints (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    chain_id    UUID REFERENCES chains(id) ON DELETE SET NULL,
    kind        endpoint_kind NOT NULL DEFAULT 'rpc',
    name        TEXT NOT NULL,
    url         TEXT NOT NULL,
    enabled     BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX endpoints_project_id_idx ON endpoints(project_id);
