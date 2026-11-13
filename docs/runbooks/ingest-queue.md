# Runbook: IngestQueueBackpressure / SpanDropRate

**Alerts**: `IngestQueueBackpressure`, `SpanDropRate`  
**Severity**: warning  

---

## What fired

- **IngestQueueBackpressure**: Redis Streams queue depth > 50 000 spans — ingest is producing faster than ClickHouse can consume.
- **SpanDropRate**: Spans are actively being discarded (> 100 drops/s).

## Immediate triage

```bash
# Queue depth
kubectl -n vigil exec deploy/vigil-ingest -- \
  curl -s localhost:8084/metrics | grep vigil_ingest_queue_depth

# Consumer lag
kubectl -n vigil logs -l app=vigil-collector --since=5m | grep '"level":"WARN"'

# ClickHouse write latency
kubectl -n vigil logs -l app=vigil-ingest --since=5m | grep '"op":"clickhouse_write"'
```

## Common causes

| Cause | Fix |
|-------|-----|
| ClickHouse slow (high disk I/O) | Check ClickHouse host metrics; consider increasing instance size |
| Collector pods not scaling | HPA may be lagging; manually scale: `kubectl -n vigil scale deploy/vigil-collector --replicas=5` |
| Redis connection exhaustion | Check Redis CloudWatch; increase `maxmemory` or cluster size |
| Burst of large traces from one agent | Identify agent via top spans metric; apply per-agent rate limit |

## Recovery verification

- `vigil_ingest_queue_depth` returns below 10 000
- `vigil_ingest_spans_dropped_total` rate returns to 0
