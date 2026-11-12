CREATE TABLE IF NOT EXISTS endpoint_probes (
    endpoint_id String,
    method      String,
    status      String,
    latency_ms  UInt32,
    error       String DEFAULT '',
    observed_at DateTime DEFAULT now()
) ENGINE = MergeTree()
ORDER BY (endpoint_id, observed_at)
TTL observed_at + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS endpoint_probes_1m
ENGINE = AggregatingMergeTree()
ORDER BY (endpoint_id, method, minute)
AS SELECT
    endpoint_id,
    method,
    toStartOfMinute(observed_at) AS minute,
    countState()                  AS total,
    countIfState(status = 'ok')   AS ok_count,
    avgState(latency_ms)          AS latency_ms_avg
FROM endpoint_probes
GROUP BY endpoint_id, method, minute;
