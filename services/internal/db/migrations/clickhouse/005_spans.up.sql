CREATE TABLE IF NOT EXISTS spans (
    trace_id        String,
    span_id         String,
    parent_span_id  String DEFAULT '',
    name            String,
    kind            String,
    service_name    String,
    attrs           Map(String, String),
    start_time      DateTime64(9),
    end_time        DateTime64(9),
    duration_ms     UInt32 MATERIALIZED toUInt32(dateDiff('millisecond', start_time, end_time)),
    status_code     String DEFAULT 'unset',
    status_message  String DEFAULT '',
    -- GenAI semantic convention columns
    gen_ai_system   String DEFAULT '',
    gen_ai_model    String DEFAULT '',
    gen_ai_op       String DEFAULT '',
    input_tokens    UInt32 DEFAULT 0,
    output_tokens   UInt32 DEFAULT 0,
    created_at      DateTime DEFAULT now()
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (service_name, trace_id, start_time)
TTL created_at + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

-- 1-minute latency rollup per service+operation
CREATE MATERIALIZED VIEW IF NOT EXISTS spans_1m
ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMM(minute)
ORDER BY (service_name, name, minute)
AS SELECT
    service_name,
    name,
    toStartOfMinute(start_time) AS minute,
    countState()                AS total,
    avgState(duration_ms)       AS duration_avg,
    quantileState(0.95)(duration_ms) AS duration_p95
FROM spans
GROUP BY service_name, name, minute;
