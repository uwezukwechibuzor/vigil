# Runbook: APIAvailabilityBelowSLO

**Alert**: `APIAvailabilityBelowSLO`  
**Severity**: critical  
**SLO**: API success rate ≥ 99.9% over any 5-minute window  

---

## What fired

The API service is returning ≥ 0.1% HTTP 5xx responses, sustained for 5 minutes.

## Immediate triage (< 5 min)

```bash
# 1. Check pod health
kubectl -n vigil get pods -l app=vigil-api

# 2. Recent error logs
kubectl -n vigil logs -l app=vigil-api --since=5m | grep '"level":"ERROR"'

# 3. Check upstream dependencies
kubectl -n vigil exec deploy/vigil-api -- curl -s localhost:8081/readyz
```

## Common causes

| Symptom | Likely cause | Fix |
|---------|-------------|-----|
| Pods CrashLoopBackOff | OOM or panic | Check `kubectl describe pod`; increase memory limit or fix bug |
| DB connection errors | Postgres unavailable or connection pool exhausted | Check RDS CloudWatch; restart pods to reset pool |
| `context deadline exceeded` | Downstream timeout (ClickHouse, Redis) | Check those services; circuit-breaker will auto-recover |
| 429 from upstream | Anthropic API rate limit | Insights service backs off; not user-facing |

## Escalation

- Page database on-call if Postgres is unreachable
- Notify Slack `#vigil-incidents` with current error rate
- If error rate > 5% for > 10 min, consider rolling back the last deploy:

```bash
helm -n vigil rollback vigil
```

## Recovery verification

Alert should auto-resolve once `vigil:api:request_success_rate5m` returns to ≥ 0.999.

## Linked alerts

- [ServiceDown](service-down.md)
- [DBSlowQueries](db-slow-queries.md)
