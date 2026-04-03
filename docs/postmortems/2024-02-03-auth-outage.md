# Postmortem: Auth Service Outage — 2024-02-03

**Severity:** P0  
**Duration:** 14 minutes (14:22 UTC – 14:36 UTC)  
**Impact:** 100% of authenticated requests failed. ~12,000 users affected.  
**Author:** Platform Team  
**Status:** Complete

---

## Summary

A misconfigured Redis connection pool caused the auth service to exhaust all connections during a traffic spike, making token validation unavailable for 14 minutes. The issue was triggered by a deployment that accidentally reduced `REDIS_POOL_SIZE` from 20 to 2.

---

## Timeline

| Time (UTC) | Event |
|---|---|
| 14:18 | Deployment of `auth-service:v1.4.2` begins (canary: 10%) |
| 14:22 | PagerDuty fires: `HighErrorRate` on auth-service (error rate: 98%) |
| 14:23 | On-call engineer acknowledges alert |
| 14:25 | Engineer checks pod logs — sees `redis: connection pool timeout` errors |
| 14:27 | Root cause identified: `REDIS_POOL_SIZE=2` in new deployment config |
| 14:30 | Rollback initiated via `kubectl rollout undo` |
| 14:34 | All pods running v1.4.1, error rate drops to 0% |
| 14:36 | Monitoring confirms recovery, alert resolved |

---

## Root Cause

During a config cleanup in PR #412, a developer renamed `REDIS_MAX_CONNECTIONS` to `REDIS_POOL_SIZE` in the Kubernetes ConfigMap but forgot to update the corresponding env var in the Deployment manifest. The service defaulted to `REDIS_POOL_SIZE=2` (hardcoded default in code), which was insufficient for production traffic.

---

## Contributing Factors

1. **No config validation at startup**: The service accepted `REDIS_POOL_SIZE=2` without warning
2. **Canary analysis didn't catch it**: The Argo Rollouts analysis template only checked HTTP success rate, not Redis-specific metrics
3. **No integration test for pool exhaustion**: Load tests didn't simulate Redis pool saturation

---

## Impact

- 12,400 users received 401/500 errors during the window
- Estimated ~340 lost sessions (users had to re-login)
- No data loss or corruption

---

## Action Items

| Action | Owner | Due | Status |
|---|---|---|---|
| Add startup config validation (fail fast on pool size < 5) | @alice | Feb 10 | ✅ Done |
| Add `redis_pool_connections_active` to canary analysis template | @bob | Feb 10 | In Progress |
| Add Redis pool exhaustion test to load test suite | @charlie | Feb 17 | Planned |
| Add PR template checklist: "Did you update all env var references?" | @alice | Feb 10 | ✅ Done |
| Document Redis pool sizing guidelines in runbook | @david | Feb 17 | Planned |

---

## Lessons Learned

**What went well:**
- Alert fired within 1 minute of impact
- Root cause identified in under 5 minutes due to structured logs
- Rollback was clean and fast (4 minutes end-to-end)

**What could be better:**
- Config changes need better cross-referencing between ConfigMaps and Deployment specs
- Canary rollouts should include dependency health checks, not just HTTP status

---

## Prevention

We will implement a config schema validation library that:
1. Validates required env vars at boot time
2. Enforces minimum values for pool sizes and timeouts
3. Fails the readiness probe if config is invalid — preventing bad deployments from receiving traffic
<!-- updated -->
<!-- v1 -->
<!-- timeline -->
<!-- root cause -->
<!-- actions -->
<!-- prevention -->
<!-- timeline -->
<!-- root cause -->
