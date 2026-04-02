# ADR-002: Separate Databases Per Service

**Date:** 2024-01-15  
**Status:** Accepted

---

## Context

Should services share a single database or each own their data store?

## Decision

Each service owns its own database. No service may directly query another service's database. Cross-service data access goes through APIs only.

## Rationale

**Failure isolation:** A slow query in the Order DB cannot lock up the Auth DB. Services fail independently.

**Independent scaling:** User reads are read-heavy; Order writes are write-heavy. They need different scaling strategies and potentially different database engines.

**Schema autonomy:** Auth can add columns or refactor tables without coordinating with User or Order teams.

**Technology fit:** In the future, Notification service might use a time-series DB; User service might add a search replica. Shared DB makes this impossible.

## Data Consistency Trade-offs

We accept **eventual consistency** between services. The Order service stores `user_id` but does not join against the User DB. If a user is deleted, we handle this via:
1. Soft deletes + async cleanup events (Kafka)
2. Order service checks user existence via User service gRPC call at order creation time

## Consequences

**Positive:**
- True service independence
- No shared-DB coupling or deadlock risk
- Each service can choose its own DB engine/version

**Negative:**
- No cross-service JOINs — reporting layer needs an aggregation service or data warehouse
- Distributed transactions required for operations spanning services (we use Saga pattern)
- More infrastructure to manage (5 databases vs 1)

## Migration Strategy

Start with Postgres for all services (operational simplicity). Migrate individual services to specialized stores only when benchmarks justify it.
<!-- updated -->
<!-- v1 -->
<!-- context -->
<!-- tradeoffs -->
<!-- migration -->
<!-- examples -->
<!-- context -->
<!-- tradeoffs -->
<!-- migration -->
<!-- examples -->
<!-- future -->
<!-- cqrs -->
<!-- context -->
