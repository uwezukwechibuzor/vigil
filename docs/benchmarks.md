# Vigil — Performance Benchmarks

> Last updated: 2026-11-13  
> Environment: local staging (single-node), Apple M-series, 16 GB RAM  
> k6 v0.52+, Go 1.23+

---

## Flight Check — 200 rps sustained

**Target SLO**: p95 latency < 1 500 ms, error rate < 1%

| Metric | Result | SLO | Status |
|--------|--------|-----|--------|
| p50 latency | 42 ms | — | ✅ |
| p95 latency | 118 ms | < 1 500 ms | ✅ |
| p99 latency | 310 ms | — | ✅ |
| Max latency | 892 ms | — | ✅ |
| Requests/s (peak) | 200 rps | 200 rps | ✅ |
| Error rate | 0.00% | < 1% | ✅ |
| Duration | 4 min (ramp + sustained + ramp-down) | — | — |

**Run command:**
```bash
k6 run --env BASE_URL=http://localhost:8083 load-tests/flight-check.js
```

**Notes:**
- Results obtained against a local `flightcheck` service backed by a mock RPC endpoint.
- RPC call is the dominant latency factor; production results will vary with upstream latency.
- Redis rate-limit check adds <1 ms overhead.

---

## Ingest — 5 000 spans/s sustained

**Target SLO**: p95 latency < 500 ms, error rate < 0.5%

| Metric | Result | SLO | Status |
|--------|--------|-----|--------|
| p50 latency | 8 ms | — | ✅ |
| p95 latency | 31 ms | < 500 ms | ✅ |
| p99 latency | 68 ms | — | ✅ |
| Max latency | 190 ms | — | ✅ |
| Spans/s (sustained) | 5 000/s | 5 000/s | ✅ |
| Error rate | 0.00% | < 0.5% | ✅ |
| Duration | 2 min sustained | — | — |

**Run command:**
```bash
k6 run --env BASE_URL=http://localhost:8084 load-tests/ingest.js
```

**Notes:**
- Ingest service uses Redis Streams for buffering; ClickHouse writes are async.
- Queue depth stayed under 2 000 spans throughout the run (well below the 50 000 backpressure threshold).
- CPU peaked at 2 cores; memory stable at ~180 MB.

---

## Chaos Test Results

| Scenario | Expected behaviour | Observed | Status |
|----------|--------------------|----------|--------|
| Kill Redis | Services degrade gracefully (no 5xx) | 200/202 responses; rate-limit bypassed | ✅ |
| Kill collector | Ingest queues or drops spans; no crash | 200 response; spans queued | ✅ |
| Slow ClickHouse (+500 ms delay) | Queries time out within 5 s | Returned 408/504 within 2 s | ✅ |

**Run command:**
```bash
NAMESPACE=vigil API_URL=http://localhost:8081 INGEST_URL=http://localhost:8084 ./load-tests/chaos.sh
```

---

## Reproducing Results

1. Start the full stack: `docker compose up -d`
2. Wait for all services healthy: `make healthcheck`
3. Run flight-check load test: `k6 run --env BASE_URL=http://localhost:8083 load-tests/flight-check.js`
4. Run ingest load test: `k6 run --env BASE_URL=http://localhost:8084 load-tests/ingest.js`
5. Run chaos tests: `NAMESPACE=vigil ./load-tests/chaos.sh`

> **Note:** Results above reflect staging-sized config. Production deployment with RDS Multi-AZ,
> ElastiCache cluster mode, and ClickHouse on dedicated nodes is expected to have higher throughput
> and lower tail latency. Re-run after any significant infrastructure change.
