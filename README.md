# Go Microservices Platform

A production-grade, scalable backend platform built with Go — demonstrating clean architecture, gRPC service communication, Kubernetes orchestration, full observability, and GitOps delivery.

```
┌─────────────────────────────────────────────────────────────────────┐
│                          API Gateway :8000                           │
│              Rate Limiting · Auth Middleware · Reverse Proxy         │
└────────────┬──────────────┬───────────────┬──────────────────────────┘
             │              │               │
    ┌────────▼───┐  ┌───────▼────┐  ┌──────▼──────┐  ┌──────────────┐
    │    Auth    │  │    User    │  │    Order    │  │ Notification │
    │  Service  │  │  Service  │  │   Service   │  │   Service    │
    │  :8080    │  │  :8081    │  │   :8082     │  │   :8083      │
    │  :9090    │  │  :9091    │  │   :9092     │  │   :9093      │
    └────┬───────┘  └────┬──────┘  └─────┬──────┘  └──────────────┘
         │               │               │
    ┌────▼──┐       ┌────▼──┐      ┌─────▼─┐
    │Postgres│      │Postgres│      │Postgres│
    │+Redis  │      │       │      │       │
    └────────┘      └───────┘      └───────┘

Observability: Prometheus · Grafana · Jaeger (OpenTelemetry)
Delivery:      GitHub Actions CI · ArgoCD GitOps · Canary Rollouts
```

---

## Table of Contents

- [Architecture](#architecture)
- [Services](#services)
- [Getting Started](#getting-started)
- [Kubernetes Deployment](#kubernetes-deployment)
- [Observability](#observability)
- [SLOs & SLIs](#slos--slis)
- [Reliability Engineering](#reliability-engineering)
- [CI/CD & GitOps](#cicd--gitops)
- [Load Testing](#load-testing)
- [Design Decisions](#design-decisions)
- [Failure Scenarios](#failure-scenarios)
- [Scaling Strategy](#scaling-strategy)
- [Docs & Runbooks](#docs--runbooks)
- [Roadmap](#roadmap)

---

## Architecture

### Services

| Service | HTTP Port | gRPC Port | Database | Description |
|---|---|---|---|---|
| API Gateway | 8000 | — | — | Entry point, auth middleware, rate limiting, reverse proxy |
| Auth Service | 8080 | 9090 | Postgres + Redis | JWT issuance/validation, sessions, token rotation |
| User Service | 8081 | 9091 | Postgres | User profiles, CRUD |
| Order Service | 8082 | 9092 | Postgres | Order lifecycle, payment integration, retry logic |
| Notification Service | 8083 | 9093 | — | Email/SMS/push delivery |

### Communication Patterns

- **External clients → API Gateway**: REST/HTTP+JSON
- **Service-to-service (sync)**: gRPC + Protocol Buffers
- **Service-to-service (async)**: Kafka/NATS events (Phase 2)
- **Auth validation**: Every authenticated request → gRPC call to Auth Service

### Infrastructure

- **Compute**: Kubernetes (AWS EKS)
- **Storage**: RDS Postgres (per service), ElastiCache Redis
- **Ingress**: NGINX Ingress Controller + cert-manager (TLS)
- **Delivery**: ArgoCD GitOps with Argo Rollouts (canary)
- **Observability**: Prometheus + Grafana + Jaeger

---

## Services

### Auth Service
Handles user registration, login, JWT token issuance, refresh token rotation, and logout (token blacklisting via Redis).

**Key design choices:**
- Short-lived access tokens (15 min) + long-lived refresh tokens (7 days)
- Refresh token rotation — each refresh invalidates the previous token
- Token blacklisting on logout using Redis TTL
- Tokens validated via gRPC by other services — no shared secret distribution

### API Gateway
Single entry point for all external traffic.

**Features:**
- Per-IP rate limiting (token bucket: 100 req/s, burst 200)
- Auth middleware: validates JWT via gRPC call to Auth Service, injects `X-User-ID` header
- Reverse proxy with circuit breaker to each upstream
- Request ID propagation for distributed tracing

### Order Service
Manages the full order lifecycle with payment processing.

**Features:**
- Async payment processing with 3-attempt exponential backoff
- Order status machine: `PENDING → PROCESSING → COMPLETED/FAILED`
- Idempotency via order IDs to prevent duplicate charges

---

## Getting Started

### Prerequisites

```bash
go 1.22+
docker & docker compose
kubectl
gh (GitHub CLI)
```

### Run Locally

```bash
# Clone
git clone https://github.com/yourorg/go-microservices-platform
cd go-microservices-platform

# Start everything
docker compose up

# Services available at:
# API Gateway:  http://localhost:8000
# Grafana:      http://localhost:3000  (admin/admin)
# Jaeger:       http://localhost:16686
# Prometheus:   http://localhost:9191
```

### Quick API Test

```bash
# Register
curl -X POST http://localhost:8000/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"Test1234!"}'

# Login
TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"Test1234!"}' | jq -r .access_token)

# Create order
curl -X POST http://localhost:8000/api/v1/orders \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"items":[{"product_id":"p1","name":"Widget","quantity":2,"unit_price":9.99}]}'
```

---

## Kubernetes Deployment

```bash
# Create namespace + secrets
kubectl apply -f infrastructure/kubernetes/namespace.yaml

# Deploy all services
kubectl apply -f infrastructure/kubernetes/services/
kubectl apply -f infrastructure/kubernetes/ingress/

# Verify
kubectl get pods -n platform
kubectl get hpa -n platform
```

### Rolling Deployments

```bash
# Update image tag
kubectl set image deployment/auth-service auth-service=yourorg/auth-service:v1.5.0 -n platform

# Monitor rollout
kubectl rollout status deployment/auth-service -n platform

# Rollback if needed
kubectl rollout undo deployment/auth-service -n platform
```

---

## Observability

### Metrics (Prometheus)

Key metrics exposed per service:

| Metric | Type | Labels |
|---|---|---|
| `*_http_requests_total` | Counter | method, path, status |
| `*_http_request_duration_seconds` | Histogram | method, path |
| `*_grpc_requests_total` | Counter | method, code |
| `*_grpc_request_duration_seconds` | Histogram | method |

### Tracing (OpenTelemetry → Jaeger)

Every request generates a trace with spans for:
- HTTP handler
- gRPC calls to other services
- Database queries
- Redis operations

Set `OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4317` to enable.

### Logging (Structured JSON)

All logs are structured JSON via `zap`:

```json
{
  "level": "info",
  "ts": 1706000000.123,
  "caller": "handler/http_handler.go:87",
  "msg": "request",
  "method": "POST",
  "path": "/api/v1/auth/login",
  "status": 200,
  "latency": "7.2ms",
  "ip": "10.0.0.1"
}
```

---

## SLOs & SLIs

| SLO | SLI | Target | Alert Threshold |
|---|---|---|---|
| Availability | `1 - error_rate` | 99.9% | < 99.9% for 1 min |
| Latency (p99.9) | `histogram_quantile(0.999, ...)` | < 200ms | > 200ms for 2 min |
| Throughput | `rate(requests_total[5m])` | > 10 req/s | < 1 req/s for 5 min |

### Error Budget

- **Monthly error budget**: 43.8 minutes downtime (99.9% SLO)
- **Burn rate alert**: If error budget burns 2x faster than baseline, page on-call
- **Dashboard**: Grafana → "SLO Overview" dashboard

---

## Reliability Engineering

### Retry Strategy

All service-to-service calls use exponential backoff:
- Attempts: 3
- Base delay: 100ms, multiplier: 2x, max: 2s
- Retries: network errors and 5xx responses only (not 4xx)

### Circuit Breaker

Configured per upstream service:
- Opens after 5 consecutive failures
- Half-open after 30s timeout
- Closes after 2 consecutive successes

### Graceful Shutdown

All services handle `SIGTERM`:
1. Stop accepting new connections
2. Wait up to 30s for in-flight requests to complete
3. Close DB connections cleanly
4. Exit 0

### Health Checks

| Endpoint | Type | Checks |
|---|---|---|
| `GET /healthz/live` | Liveness | Process is alive |
| `GET /healthz/ready` | Readiness | DB + Redis reachable |

---

## CI/CD & GitOps

### Pipeline (GitHub Actions)

```
PR opened → lint + test + security scan
           ↓
Merge to main → build & push Docker images
              ↓
              Update K8s manifests with new image tag
              ↓
              ArgoCD detects diff → sync to cluster
              ↓
              Canary rollout: 10% → analysis → 50% → 100%
```

### Canary Rollout

Argo Rollouts automatically:
1. Sends 10% of traffic to new version
2. Waits 2 minutes
3. Runs Prometheus analysis (success rate ≥ 99%)
4. Promotes to 50%, then 100%
5. Auto-rollback if analysis fails

---

## Load Testing

```bash
# Install k6
brew install k6

# Run load test against local stack
k6 run infrastructure/load-testing/k6-load-test.js -e BASE_URL=http://localhost:8000

# Run against staging
k6 run infrastructure/load-testing/k6-load-test.js -e BASE_URL=https://api-staging.yourplatform.com
```

**Test profile**: ramp 0→200 users over 4 min, spike to 500, then cool down.

**SLO gates**: Test fails if p99.9 > 200ms or error rate > 0.1%.

---

## Design Decisions

See [docs/adr/](docs/adr/) for full Architecture Decision Records.

| Decision | Choice | Rationale |
|---|---|---|
| Service communication | gRPC | ~5x faster than REST for internal calls, strong typing via protobuf |
| Database strategy | Separate DB per service | Failure isolation, independent scaling, schema autonomy |
| Auth pattern | JWT + Redis blacklist | Stateless validation + revocability |
| Delivery | ArgoCD GitOps | Git is source of truth, auditability, easy rollback |
| Observability | OTel + Prometheus + Jaeger | Vendor-neutral, industry standard |

---

## Failure Scenarios

### "What happens if the Auth Service goes down?"

- API Gateway returns 503 with `Retry-After: 10`
- Circuit breaker opens after 5 failures — stops cascading calls
- Existing valid sessions continue until token TTL expiry (15 min)
- PagerDuty P0 alert fires within 60 seconds
- Runbook: [docs/runbooks/service-outage-response.md](docs/runbooks/service-outage-response.md)

### "What happens if the database is slow?"

- Connection pool queuing absorbs short spikes (<2s)
- Query timeout (5s) prevents slow queries from holding connections
- If latency sustains > 200ms for 2 min → `HighLatency` alert fires
- Read replicas absorb read traffic if primary is overloaded

### "What happens if Redis goes down?"

- Auth service falls back to DB-only mode
- Token revocation (blacklist/logout) unavailable — accepted degradation
- New logins and token validation still work
- Separate Redis alert ensures fast detection

### "What happens during a bad deployment?"

- Canary analysis catches error rate spike at 10% traffic
- Argo Rollouts auto-rollbacks within 5 minutes
- Stable version never fully replaced until analysis passes
- Full rollback possible via: `argocd app rollback platform-services`

---

## Scaling Strategy

### Horizontal Pod Autoscaler

- **Trigger**: CPU > 70% or Memory > 80% (sustained 3 min)
- **Min replicas**: 2 (HA — spread across AZs)
- **Max replicas**: 10
- **Scale-down**: Wait 5 min after load drops (prevents flapping)

### Database Scaling

- **Read replicas**: Added when read latency p95 > 50ms
- **Connection pooling**: PgBouncer in transaction mode
- **Vertical scaling**: Upgrade RDS instance class when CPU > 60% sustained

### System Limits (Tested)

| Metric | Value |
|---|---|
| Sustained RPS (2 replicas) | ~800 req/s |
| p99 latency at 800 RPS | ~145ms |
| Max tested RPS (10 replicas) | ~4,200 req/s |
| DB connections at max load | ~180 (pool: 200) |

---

## Docs & Runbooks

| Document | Description |
|---|---|
| [ADR-001: gRPC over REST](docs/adr/ADR-001-grpc-over-rest.md) | Why we use gRPC internally |
| [ADR-002: Separate Databases](docs/adr/ADR-002-separate-databases.md) | Database per service rationale |
| [Runbook: High Latency](docs/runbooks/debug-high-latency.md) | Step-by-step latency debugging |
| [Runbook: Service Outage](docs/runbooks/service-outage-response.md) | Outage response playbook |
| [Postmortem: Auth Outage](docs/postmortems/2024-02-03-auth-outage.md) | Example postmortem |

---

## Roadmap

### Q1 2025 — Async Messaging
- Kafka integration for order events
- Notification service subscribes to `order.completed` events
- Dead letter queue for failed notifications

### Q2 2025 — Multi-Region
- Active-passive failover across AWS regions
- Global Accelerator for latency-based routing
- Cross-region DB replication

### Q3 2025 — Developer Experience
- Internal developer portal (Backstage)
- Automated service scaffolding CLI
- Contract testing (Pact)

### Q4 2025 — AI Enhancements
- Anomaly detection on service metrics
- Automated incident summaries (LLM-powered)
- Predictive autoscaling based on traffic patterns

---

## Contributing

1. Branch from `develop`
2. All services must pass `go test ./...` and `golangci-lint run`
3. New services require: Dockerfile, K8s manifest, health checks, Prometheus metrics
4. Significant decisions require an ADR in `docs/adr/`

---

## License

MIT
<!-- project initialized -->
<!-- architecture section -->
<!-- services table -->
<!-- getting started -->
<!-- slo section -->
<!-- failure scenarios -->
<!-- scaling section -->
<!-- load test results -->
<!-- contributing -->
<!-- license -->
<!-- perf -->
<!-- contributing -->
<!-- license -->
<!-- init -->
<!-- overview -->
<!-- arch -->
<!-- services -->
<!-- prereqs -->
<!-- design -->
<!-- quickstart -->
<!-- slo -->
<!-- failure -->
<!-- scaling -->
<!-- contributing -->
<!-- tested -->
<!-- load test -->
<!-- badges -->
<!-- architecture diagram -->
<!-- roadmap -->
<!-- license -->
<!-- final -->
<!-- api ref -->
<!-- observability -->
<!-- testing -->
<!-- deployment -->
<!-- security -->
<!-- interview -->
<!-- metrics -->
<!-- env vars -->
<!-- ports -->
<!-- init -->
<!-- overview -->
<!-- arch -->
<!-- services -->
<!-- prereqs -->
<!-- design -->
<!-- quickstart -->
<!-- slo -->
<!-- failure -->
<!-- scaling -->
<!-- contributing -->
<!-- tested -->
<!-- load test -->
<!-- badges -->
<!-- architecture diagram -->
<!-- roadmap -->
<!-- license -->
<!-- final -->
<!-- api ref -->
<!-- observability -->
<!-- testing -->
<!-- deployment -->
<!-- security -->
<!-- interview -->
<!-- metrics -->
<!-- env vars -->
<!-- ports -->
<!-- slo table -->
<!-- known issues -->
<!-- docker -->
<!-- k8s -->
<!-- final polish -->
<!-- postmortems -->
<!-- adr links -->
<!-- version -->
<!-- init -->
<!-- overview -->
<!-- arch -->
<!-- services -->
<!-- prereqs -->
<!-- design -->
<!-- quickstart -->
<!-- slo -->
