CREATE TABLE IF NOT EXISTS oracle_updates (
    feed        String,
    chain       String,
    answer      Float64,
    updated_at  DateTime,
    observed_at DateTime DEFAULT now(),
    staleness_s UInt32
) ENGINE = MergeTree()
ORDER BY (feed, chain, observed_at)
TTL observed_at + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;
