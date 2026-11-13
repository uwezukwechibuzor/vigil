# Runbook: ServiceDown

**Alert**: `ServiceDown`  
**Severity**: critical  
**Trigger**: Any `vigil-*` scrape target has been `up == 0` for 2 minutes  

---

## What fired

Prometheus cannot reach the named service's `/metrics` endpoint.

## Immediate triage (< 5 min)

```bash
# Replace SERVICE with the value from $labels.job
SERVICE=vigil-api

# 1. Pod status
kubectl -n vigil get pods -l "app=${SERVICE}"

# 2. Events
kubectl -n vigil describe deployment "${SERVICE}" | tail -20

# 3. Recent logs
kubectl -n vigil logs "deployment/${SERVICE}" --previous --tail=50
```

## Decision tree

```
Pods Running?
  NO → Check describe pod for image pull / OOM / config errors
  YES → Pods Ready?
    NO → Check readiness probe: kubectl -n vigil describe pod <pod>
    YES → Networking issue? Check Service and NetworkPolicy:
            kubectl -n vigil get svc,netpol
```

## Common fixes

| Cause | Fix |
|-------|-----|
| ImagePullBackOff | Verify ECR image tag was pushed; check IAM role for node group |
| OOMKilled | Increase `resources.limits.memory` in Helm values, redeploy |
| CrashLoopBackOff (bad config) | Check env vars; External Secrets syncing? `kubectl get externalsecret -n vigil` |
| Node failure | `kubectl get nodes`; replace node or cordon + drain |

## Escalation

- If > 2 pods of the same service crash-loop, page the on-call SRE immediately.
- If the entire cluster is unreachable, escalate to AWS support.

## Recovery verification

Alert resolves when `up{job="<service>"}` returns to 1.
