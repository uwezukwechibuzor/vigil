# Runbook: FlightCheckLatencySLOBreached

**Alert**: `FlightCheckLatencySLOBreached`  
**Severity**: critical  
**SLO**: p95 flight check latency < 1 500 ms  

---

## What fired

Flight check p95 response time has exceeded 1.5 s for 5 minutes.

## Immediate triage (< 5 min)

```bash
# 1. Pod health
kubectl -n vigil get pods -l app=vigil-flightcheck

# 2. Latency breakdown in logs (look for slow RPC calls)
kubectl -n vigil logs -l app=vigil-flightcheck --since=5m | grep '"duration_ms"' | \
  jq -c '{chain, endpoint, duration_ms}' | sort -t: -k3 -rn | head -20

# 3. Current p95
kubectl -n vigil exec deploy/vigil-api -- \
  curl -s http://vigil-flightcheck:8083/metrics | \
  grep http_request_duration_seconds
```

## Common causes

| Symptom | Likely cause | Fix |
|---------|-------------|-----|
| Specific chain showing high latency | RPC endpoint slow or rate-limited | Check RPC provider status; consider failover endpoint |
| All chains slow | Redis latency high (rate limit overhead) | Check Redis CloudWatch; `kubectl -n vigil describe pod -l app=redis` |
| Latency only on cold start | Pod not warmed up after deploy | Normal; alert should clear within 2 min |
| Increasing pod CPU | CPU throttling under load | HPA should scale; check `kubectl top pods -n vigil` |

## RPC failover

If a specific RPC endpoint is degraded:

```bash
# Update RPC URL for chain in Secrets Manager, then restart pods
aws secretsmanager update-secret --secret-id vigil/prod --secret-string '{"ETHEREUM_RPC_URL":"https://backup-rpc.example.com"}'
kubectl -n vigil rollout restart deployment/vigil-flightcheck
```

## Recovery verification

Alert resolves when `vigil:flightcheck:p95_latency5m < 1.5`.

## Linked alerts

- [ServiceDown](service-down.md)
- [HighErrorRate](high-error-rate.md)
