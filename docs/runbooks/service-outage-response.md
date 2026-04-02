# Runbook: Service Outage Response

**Owner:** Platform Team  
**Last Updated:** 2024-01-15  
**Trigger:** Service health check failing, error rate > 5%, or pod crash-looping

---

## Severity Classification

| Severity | Condition | Response Time |
|---|---|---|
| P0 | Auth service down (all users locked out) | 5 min |
| P1 | Order/Payment service down (revenue impact) | 15 min |
| P2 | User or Notification service down | 30 min |
| P3 | Degraded performance, partial impact | Next business day |

---

## Immediate Response (0–5 min)

```bash
# 1. Get pod status
kubectl get pods -n platform -l app=<service-name>

# 2. Check recent events
kubectl describe pods -n platform -l app=<service-name> | tail -30

# 3. Check logs (last 100 lines)
kubectl logs -n platform deploy/<service-name> --tail=100

# 4. Check if it's a rollout issue
kubectl rollout history deployment/<service-name> -n platform
```

---

## Scenario A: CrashLoopBackOff

```bash
# Get the error
kubectl logs -n platform <pod-name> --previous

# Common causes:
# - Missing env var / secret → check ConfigMap and Secrets
# - DB connection failed → check database pod is healthy
# - OOM killed → check memory limits, increase if needed

# Quick fix: Roll back
kubectl rollout undo deployment/<service-name> -n platform

# Or via ArgoCD:
argocd app rollback platform-services <revision>
```

---

## Scenario B: Service Running but Returning Errors

```bash
# Check error details
kubectl logs -n platform deploy/<service-name> --since=10m | grep '"level":"error"' | jq .

# Check readiness probe
kubectl describe pod <pod-name> -n platform | grep -A10 "Readiness"

# Exec into pod for live debugging
kubectl exec -it deploy/<service-name> -n platform -- sh
```

---

## Scenario C: Auth Service Down (P0)

**Impact:** All authenticated requests fail across entire platform.

```bash
# 1. Scale up immediately (buy time while debugging)
kubectl scale deployment auth-service -n platform --replicas=5

# 2. Check JWT secret and Redis connectivity
kubectl exec deploy/auth-service -n platform -- \
  redis-cli -u $REDIS_URL PING

# 3. If DB is down, Redis-only mode can still validate non-revoked tokens
# (tokens will remain valid until TTL, logout won't work — acceptable degradation)

# 4. If Redis is down:
#    - Auth service will still issue tokens
#    - Token revocation (logout/blacklist) won't work
#    - Communicate this to users if needed

# 5. Emergency: enable read-only mode via feature flag
kubectl set env deployment/auth-service -n platform READ_ONLY_MODE=true
```

---

## Scenario D: Database is Down

```bash
# Check postgres pod
kubectl get pods -n platform -l app=<service>-postgres
kubectl logs deploy/<service>-postgres -n platform --tail=50

# If pod is healthy but service can't connect:
kubectl exec deploy/<service-name> -n platform -- \
  sh -c 'psql $DATABASE_URL -c "SELECT 1"'

# Restart postgres pod (it has persistent volume — data is safe)
kubectl rollout restart deploy/<service>-postgres -n platform

# If PVC issue, check:
kubectl get pvc -n platform
kubectl describe pvc <pvc-name> -n platform
```

---

## Communication Template

**Status page update (P0/P1):**
```
[INVESTIGATING] We are experiencing issues with [service]. 
Users may [describe impact]. Our team is actively investigating.
Next update in 15 minutes.
```

**Resolution:**
```
[RESOLVED] The issue with [service] has been resolved at [time].
Root cause: [brief description]. 
A full postmortem will be published within 48 hours.
```

---

## Post-Incident Checklist

- [ ] Service restored and stable for 30+ minutes
- [ ] Alert resolved in PagerDuty
- [ ] Postmortem ticket created (due within 48h for P0/P1)
- [ ] Immediate mitigations documented
- [ ] Action items assigned with owners and due dates
<!-- updated -->
<!-- v1 -->
<!-- severity -->
<!-- scenarios -->
<!-- comms -->
<!-- checklist -->
<!-- severity -->
<!-- scenarios -->
<!-- comms -->
<!-- checklist -->
<!-- rollback -->
<!-- notify -->
<!-- severity -->
<!-- scenarios -->
