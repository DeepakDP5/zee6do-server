# ADR-001: Modular Monolith Architecture

## Status
Accepted

## Context
The Zee6do server needs to support ~1,000 users at launch (closed beta) and scale to ~100,000 at maturity. The team is small. We need to choose between:
1. Microservices from day one
2. A traditional monolith
3. A modular monolith designed for future extraction

## Decision
**Modular monolith** — a single Go binary with clean internal module boundaries, designed to split into microservices when scale or team structure demands it.

Modules communicate via Go interfaces (not HTTP/gRPC internally). Dependencies are wired via Google Wire. Each module owns its own types, repositories, and service logic.

## Consequences
- Simpler deployment and operations — one binary, one Docker image, one ECS service.
- No distributed system complexity at launch — no service discovery, distributed tracing, or eventual consistency headaches.
- Clean module boundaries mean extraction to microservices is a deployment change, not a code rewrite.
- Risk: if module boundaries are not maintained, the codebase degrades into a coupled monolith. Mitigation: enforce package dependency rules in code review and CI.
