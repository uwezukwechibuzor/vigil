CREATE TABLE IF NOT EXISTS flight_checks (
    id          String,
    project_id  String,
    chain       String,
    verdict     String,
    score       Float32,
    checks      String,   -- JSON blob
    latency_ms  UInt32,
    created_at  DateTime DEFAULT now()
) ENGINE = MergeTree()
ORDER BY (project_id, created_at)
TTL created_at + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;
