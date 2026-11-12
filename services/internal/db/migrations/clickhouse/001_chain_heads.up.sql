CREATE TABLE IF NOT EXISTS chain_heads (
    chain       String,
    endpoint_id String,
    block_number UInt64,
    block_time  DateTime,
    observed_at DateTime DEFAULT now(),
    latency_ms  UInt32
) ENGINE = MergeTree()
ORDER BY (chain, endpoint_id, observed_at)
TTL observed_at + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

-- 1-minute rollup
CREATE MATERIALIZED VIEW IF NOT EXISTS chain_heads_1m
ENGINE = AggregatingMergeTree()
ORDER BY (chain, endpoint_id, minute)
AS SELECT
    chain,
    endpoint_id,
    toStartOfMinute(observed_at) AS minute,
    maxState(block_number)       AS block_number_max,
    avgState(latency_ms)         AS latency_ms_avg
FROM chain_heads
GROUP BY chain, endpoint_id, minute;
