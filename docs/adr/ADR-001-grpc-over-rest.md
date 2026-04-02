# ADR-001: Use gRPC for Service-to-Service Communication

**Date:** 2024-01-15  
**Status:** Accepted  
**Deciders:** Platform Team

---

## Context

Our microservices need to communicate with each other. We evaluated REST/HTTP+JSON, gRPC, and async messaging (Kafka/NATS) for synchronous service-to-service calls.

## Decision

Use **gRPC with Protocol Buffers** for all synchronous service-to-service communication. Expose **REST/HTTP+JSON** only at the API Gateway for external clients.

## Rationale

| Concern | REST | gRPC |
|---|---|---|
| Performance | JSON parsing overhead | Binary protobuf, ~5–10x faster |
| Type safety | Manual validation | Generated, strongly-typed clients |
| Contract | OpenAPI (optional) | `.proto` files (enforced) |
| Streaming | Polling or SSE | Native bi-directional streaming |
| Code gen | Partial | Full client/server generation |
| Browser support | Native | Requires grpc-web proxy |

**Key drivers:**
1. Auth service validates tokens on every request — latency matters
2. Protobuf contracts eliminate entire class of integration bugs
3. Generated clients remove hand-rolled HTTP wiring code
4. HTTP/2 multiplexing reduces connection overhead at scale

## Consequences

**Positive:**
- ~40% latency reduction on token validation calls (benchmark: 12ms → 7ms p95)
- Schema evolution is explicit and backward-compatible
- Service contracts are version-controlled `.proto` files

**Negative:**
- Harder to debug with `curl` — need tools like `grpcurl` or Postman gRPC
- gRPC requires HTTP/2, which needs additional infra config
- Teams unfamiliar with protobuf have a learning curve

## Alternatives Considered

- **REST everywhere**: Simpler, but performance and type-safety trade-offs unacceptable at our scale targets
- **Kafka for everything**: Async messaging is excellent for events but wrong for request/response patterns (e.g., auth validation must be synchronous)
<!-- updated -->
<!-- v1 -->
<!-- context -->
<!-- comparison -->
<!-- decision -->
<!-- performance -->
<!-- context -->
<!-- comparison -->
<!-- decision -->
<!-- performance -->
<!-- migration -->
<!-- protobuf -->
<!-- context -->
<!-- comparison -->
