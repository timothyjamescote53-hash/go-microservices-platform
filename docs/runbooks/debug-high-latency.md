# Runbook: Debugging High Latency

**Owner:** Platform Team  
**Last Updated:** 2024-01-15  
**Severity Trigger:** p99 latency > 200ms for 2+ minutes (PagerDuty alert: `HighLatency`)

---

## 1. Acknowledge & Assess (0–2 min)

```bash
# Check which service is affected
kubectl get pods -n platform

# Look at recent alert
# Prometheus: http://prometheus:9090/alerts
```

**Is this affecting users?**
- Check error rate: `rate(auth_http_requests_total{status=~"5.."}[5m])`
- Check 5xx in API Gateway logs

---

## 2. Identify the Bottleneck (2–10 min)

### Check service latency breakdown
```bash
# Grafana dashboard: Platform Overview → Request Latency by Service
# Or query directly:
histogram_quantile(0.99, rate(auth_http_request_duration_seconds_bucket[5m]))
```

### Check downstream dependencies
```bash
# Is it the DB?
kubectl exec -n platform deploy/auth-service -- \
  sh -c 'time psql $DATABASE_URL -c "SELECT 1"'

# Is it Redis?
kubectl exec -n platform deploy/auth-service -- \
  sh -c 'redis-cli -u $REDIS_URL PING'
```

### Check traces in Jaeger
```
http://jaeger:16686 → Search: service=auth-service, min-duration=100ms
```
Look for: long DB spans, slow gRPC calls to upstream services, GC pauses.

---

## 3. Common Causes & Fixes

### Cause A: Database connection pool exhausted
```bash
# Symptom: DB queries take >50ms, pool_wait_time metric is high
# Fix: Increase pool size (ConfigMap → DB_MAX_CONNS) or add read replica
kubectl set env deployment/auth-service -n platform DB_MAX_CONNS=20
```

### Cause B: Redis latency spike
```bash
# Check Redis slow log
redis-cli -u $REDIS_URL SLOWLOG GET 10
# Fix: Check for large key scans, KEYS command usage, memory pressure
```

### Cause C: Pod CPU throttling
```bash
kubectl top pods -n platform
# If CPU near limit, increase resources or scale out:
kubectl scale deployment auth-service -n platform --replicas=5
```

### Cause D: gRPC timeout cascade
```bash
# Check if auth-service is timing out calling another service
kubectl logs -n platform deploy/auth-service --since=5m | grep "context deadline"
# Fix: Check circuit breaker state — it should open, not cascade
```

### Cause E: Noisy neighbor (other pod on same node)
```bash
kubectl get pods -n platform -o wide  # check node assignments
kubectl describe node <node-name> | grep -A5 "Allocated resources"
# Fix: Add pod anti-affinity rules or cordon the noisy node
```

---

## 4. Escalation

If not resolved in 20 min:
1. Page on-call engineer
2. Consider enabling maintenance mode (return 503 with Retry-After header)
3. Roll back last deployment: `argocd app rollback platform-services`

---

## 5. Post-Incident

After resolving: open a postmortem ticket. Use `docs/postmortems/TEMPLATE.md`.
<!-- updated -->
<!-- v1 -->
<!-- assess -->
<!-- causes -->
<!-- escalation -->
<!-- grafana -->
<!-- assess -->
<!-- causes -->
<!-- escalation -->
<!-- grafana -->
<!-- p99 -->
<!-- cache -->
<!-- assess -->
<!-- causes -->
