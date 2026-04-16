# ADR-002: gRPC as the API Protocol

## Status
Accepted

## Context
We need a communication protocol between the mobile client and the server. Options considered:
1. REST (JSON over HTTP)
2. GraphQL
3. gRPC (Protocol Buffers over HTTP/2)

## Decision
**gRPC** with Protocol Buffers for all client-server communication.

- WebSocket is used alongside gRPC for real-time in-app updates (push events, AI action notifications, sync events).
- Push notifications (APNs/FCM) are used for background/closed-app alerts.

## Consequences
- **Performance**: Binary serialization is significantly more efficient than JSON. Smaller payloads, faster parsing.
- **Type safety**: Proto definitions serve as the single source of truth for the API contract. Code generation for both Go (server) and TypeScript (client).
- **Streaming**: Server-side streaming enables real-time AI response delivery without a separate protocol.
- **Versioning**: Package-level versioning (`zee6do.v1`, `zee6do.v2`) with Buf breaking change detection in CI.
- **Proto management**: Buf CLI + Buf Schema Registry for linting, breaking change checks, code generation, and distribution.
- **Trade-off**: gRPC is less human-debuggable than REST. We accept this for the performance and type-safety benefits. `grpcurl` and Buf Studio are available for debugging.
- **Trade-off**: React Native gRPC support requires grpc-web or a bridge layer. This adds setup complexity but is well-supported.
