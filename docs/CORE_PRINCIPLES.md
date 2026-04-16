# Core Principles

> Architectural and engineering principles that guide every decision in the Zee6do server codebase.

---

## 1. Modular Monolith — Split Later, Not Sooner

The server is a single Go binary with clean internal module boundaries. It is not a monolith by accident -- it is a monolith by design.

**What this means in practice:**
- Each domain module (auth, tasks, ai, scheduler, connectors, etc.) owns its own types, interfaces, repositories, and services.
- Modules communicate via Go interfaces, not HTTP/gRPC internally.
- Dependencies between modules flow through Wire-injected interfaces. A module never instantiates another module's concrete types directly.
- If a module needs to be extracted into a separate service, the change is a deployment and infrastructure change -- not a rewrite of business logic.
- Do not introduce service meshes, message buses, or distributed coordination until the system demonstrates a concrete need for them.

---

## 2. Interfaces at Boundaries, Concrete Code Everywhere Else

Interfaces exist at module boundaries and external dependency seams. They do not exist for the sake of abstraction.

**What this means in practice:**
- Every repository is an interface. Every external service client (LLM provider, S3, Slack API) is an interface.
- Internal implementation code within a module is concrete. Do not create interfaces for internal helper functions or private types.
- If a type is only used in one place and has only one implementation, it does not need an interface.
- Interfaces enable testability (mock the repository) and flexibility (swap the LLM provider). If neither applies, skip the interface.

---

## 3. Fail Fast, Recover Gracefully

The server fails fast on startup and recovers gracefully at runtime.

**What this means in practice:**
- **Startup:** If a required config value is missing, a database connection fails, or a migration cannot be applied, the server panics immediately with a clear error message. Do not start serving requests with a broken dependency.
- **Runtime:** If a request fails, return a proper gRPC error code. If a background job fails, log the error and retry. If the database is temporarily unreachable, the circuit breaker trips and requests fail fast until recovery.
- **Shutdown:** The server drains gracefully through the full lifecycle sequence. In-flight requests complete. Worker pools drain. Connections close cleanly.

---

## 4. AI is Orchestrated, Not Embedded

The server orchestrates AI operations -- it does not embed AI logic into business code.

**What this means in practice:**
- The `ai` module is the single entry point for all LLM interactions. No other module calls an LLM provider directly.
- The `ai` module exposes high-level operations: `ParseNaturalLanguage`, `SuggestPriority`, `GenerateDailyPlan`, `ResolveConflict`. Callers describe what they need, not how to prompt the model.
- Prompt templates, context assembly, token management, and provider selection are internal to the `ai` module.
- The LLM provider is swappable via the `LLMProvider` interface. Switching from OpenAI to Anthropic requires changing a config value, not business logic.
- AI operations that are not latency-sensitive (analytics narratives, end-of-day reviews, knowledge base re-indexing) are dispatched to the job queue, not executed inline.

---

## 5. The Client is Untrusted

Every request from the mobile client is treated as potentially malicious or malformed.

**What this means in practice:**
- Structural validation via proto annotations catches malformed requests before they reach handler code.
- Business validation (authorization, ownership, state transitions) is enforced in the service layer.
- JWT tokens are validated on every request. Device fingerprints are checked against token claims.
- Rate limiting is applied at multiple layers (per-RPC, per-user, per-IP) to prevent abuse.
- Never trust client-supplied IDs for authorization decisions. Always verify ownership server-side.
- Error messages returned to the client never leak internal details (stack traces, database errors, config values).

---

## 6. Domain-Driven Package Design

Code is organized by domain, not by technical layer.

**What this means in practice:**
- The `tasks` package contains the task model, task repository interface, task service, and task gRPC handler. Not a separate `models/`, `repositories/`, `services/`, `handlers/` folder tree.
- Each package is self-contained. A developer working on the task domain opens the `tasks/` folder and finds everything they need.
- Cross-cutting concerns (auth middleware, logging, config) live in `pkg/`.
- Sub-packages exist only when there's a genuine sub-domain (e.g., `connectors/slack/`, `connectors/clickup/`).

---

## 7. Context Propagation is Non-Negotiable

Every operation carries a `context.Context`. It is the thread that ties together request lifecycle, cancellation, timeouts, logging, and tracing.

**What this means in practice:**
- Every function that does I/O (database, network, AI, file) accepts `context.Context` as its first parameter.
- The gRPC interceptor chain injects a request-scoped zap logger, correlation ID, and user context into the context.
- All downstream code extracts the logger from context -- it never creates its own.
- Background jobs create their own contexts with appropriate timeouts.
- When context is cancelled, goroutines exit promptly. Long-running operations check `ctx.Done()` periodically.

---

## 8. Explicit Error Chains

Errors tell a story. When an error reaches the log, the full chain must explain what happened, where, and why.

**What this means in practice:**
- Every error is wrapped with context: `fmt.Errorf("taskService.Create: %w", err)`.
- Sentinel errors (`ErrTaskNotFound`, `ErrUnauthorized`) are defined for known, checkable conditions.
- Error-to-gRPC-status mapping happens at the handler boundary, nowhere else.
- Errors are logged once at the top level (handler or worker), not at every layer they pass through.
- Never silently discard errors. If an error is intentionally ignored, a comment explains why.

---

## 9. Background Work is First-Class

Background jobs (AI processing, connector sync, notification delivery, cleanup) are not second-class citizens hidden in goroutines. They are managed, monitored, and recoverable.

**What this means in practice:**
- Worker pools manage concurrency with configured limits. No unbounded goroutine creation.
- Heavy or non-urgent tasks are dispatched to the job queue (SQS/Redis) for decoupled processing.
- Every background job has a timeout and a retry policy.
- Failed jobs are logged with full context and can be retried.
- Worker pools participate in graceful shutdown -- they drain before the server exits.
- Job metrics (queue depth, processing time, failure rate) are exposed to the monitoring stack.

---

## 10. Configuration is Validated, Not Assumed

Every configuration value is loaded into a typed struct and validated at startup. The server never reads a config value at runtime and hopes it exists.

**What this means in practice:**
- Viper loads config from environment variables, YAML files, and AWS Secrets Manager.
- The config is parsed into typed Go structs with required/optional annotations.
- Missing required values cause a startup panic with a clear message naming the missing field.
- Secrets never live in env vars or config files in production -- only AWS Secrets Manager.
- Config structs are injected via Wire, not accessed through global variables or singletons.

---

## 11. Observability is Built In, Not Bolted On

Every request, every background job, and every external call is observable from day one.

**What this means in practice:**
- Every gRPC request is logged with method, duration, status code, user ID, and correlation ID.
- Every external call (LLM, MongoDB, S3, Slack, Calendar) is instrumented with latency and error metrics.
- AI operations track token usage and cost per request for budget monitoring.
- Worker pools expose queue depth, processing time, and failure rate metrics.
- Health check endpoints expose readiness (can serve traffic) and liveness (process is alive) separately.

---

## 12. Security is Standard Practice, Not Theater

Security follows established standard practices without over-engineering.

**What this means in practice:**
- Input validation on all endpoints. No raw string construction for queries.
- TLS everywhere. JWT on every request. Rate limiting on every endpoint.
- Secrets in Secrets Manager. Encryption at rest for sensitive fields (MongoDB CSFLE).
- No security features that add complexity without proportional risk reduction.
- Security scanning (gosec, govulncheck, container scanning) runs in CI automatically.
- When in doubt, follow the simplest secure approach. Do not add layers of defense that complicate the system unless the threat model justifies them.
