# Zee6do — Agent Instructions (Server)

> These instructions apply to all AI agent sessions working on the Zee6do Go backend (`zee6do-server` repository).
> For client-side instructions, see the `Agents.md` in the `vorflux` repository.

---

## Project Overview

**Zee6do** is an AI-powered personal productivity tool for individual professionals. This repository is the **Go backend server** — it handles gRPC API, AI orchestration, MongoDB persistence, connector integrations, real-time sync, notifications, and subscription management.

### Core Thesis

Task management should be invisible. The server's AI engine manages the entire lifecycle — prioritization, scheduling, status transitions, recurrence detection, and archival — while the user focuses on doing the work.

### Target Scale

- Launch: ~1,000 users (closed beta)
- Maturity: ~100,000 users
- Deployment: AWS (ECS Fargate/EC2 + MongoDB Atlas + S3)

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go (latest stable, pinned via `go.toolchain` in `go.mod`) |
| API | gRPC (Protocol Buffers, proto3) |
| Proto management | Buf CLI + Buf Schema Registry (BSR) |
| Database | MongoDB (Atlas on AWS), shared collections, field-level encryption (CSFLE) |
| File storage | AWS S3 (pre-signed URLs) |
| AI/LLM | Model-agnostic interface (OpenAI, Anthropic, Google, self-hosted) |
| DI | Google Wire (compile-time code generation) |
| Config | Viper (env vars + YAML + AWS Secrets Manager) |
| Logging | Uber Zap (structured JSON, request-scoped loggers) |
| Linting | golangci-lint (strict: govet, errcheck, staticcheck, gosec, revive, exhaustive, gocritic) |
| Testing | go test + testify (table-driven tests + mocks) |
| Container | Docker multi-stage build, distroless final image, non-root |
| CI/CD | GitHub Actions |
| Monitoring | Sentry, Datadog/X-Ray, Prometheus+Grafana, Zap structured logs |

---

## Architecture — Modular Monolith

Single Go binary with clean internal module boundaries. Modules communicate via Go interfaces, not internal HTTP/gRPC. Designed to split into microservices when scale demands.

### Module Map

| Module | Responsibility |
|---|---|
| `auth` | Phone OTP, social login (Google/Apple), JWT with device binding, session management |
| `tasks` | Task CRUD, subtasks, status transitions, priority, recurrence, search |
| `ai` | LLM provider abstraction, prompt management, AI context, batch ops, auto-apply, undo history |
| `scheduler` | Smart scheduling engine, "do date" computation, calendar conflict resolution |
| `connectors/google_calendar` | Two-way smart sync with AI conflict resolution |
| `connectors/slack` | Full integration: slash commands, notifications, AI conversation parsing |
| `connectors/email` | AI inbox parsing for action item extraction |
| `connectors/clickup` | Two-way sync (tasks, goals, docs) + knowledge base ingestion |
| `knowledge` | Personal knowledge base: ingestion, embeddings, semantic search |
| `notifications` | Push delivery (APNs/FCM), smart timing engine, morning briefing generation |
| `realtime` | WebSocket server, event broadcasting, connection management |
| `analytics` | Metrics computation, time tracking aggregation, AI narrative generation |
| `storage` | AWS S3 integration, pre-signed URLs, attachment metadata |
| `billing` | App Store / Play Store receipt verification, subscription state, trial logic |
| `sync` | WatermelonDB sync protocol, offline conflict resolution |
| `users` | Profile management, preferences, device registry, data export, account deletion |
| `flags` | Feature flag management |

### Package Layout

```
cmd/
  server/               # Main entry point, Wire injector
internal/
  auth/
  tasks/
  ai/
    providers/          # OpenAI, Anthropic, Google adapters
  scheduler/
  connectors/
    google_calendar/
    slack/
    email/
    clickup/
  knowledge/
  notifications/
  realtime/
  analytics/
  storage/
  billing/
  sync/
  users/
  flags/
pkg/
  middleware/           # Auth, logging, rate limiting, recovery
  crypto/              # Encryption, device fingerprint validation
  config/              # Viper config loading, typed structs
  errors/              # Sentinel errors, error utilities
proto/
  zee6do/              # All .proto files (versioned packages)
migrations/            # Auto-applied MongoDB migrations
```

---

## Coding Standards

### Error Handling — Wrapping + Sentinels

```go
// Sentinel errors for known conditions
var (
    ErrTaskNotFound = errors.New("task not found")
    ErrUnauthorized = errors.New("unauthorized")
    ErrConflict     = errors.New("conflict")
)

// Always wrap with context
return fmt.Errorf("taskRepo.GetByID(%s): %w", id, err)

// Check with errors.Is / errors.As
if errors.Is(err, ErrTaskNotFound) {
    return status.Error(codes.NotFound, "task not found")
}
```

- Always wrap errors with `fmt.Errorf("context: %w", err)`.
- Use sentinel errors for known, checkable conditions.
- gRPC status mapping happens at the handler boundary only.
- Never silently discard errors. Comment if intentionally ignoring.
- Errors are logged once at the top level, not at every layer.

### Dependency Injection — Google Wire

- All dependencies wired via Wire. No manual `New()` chains in `main()`.
- Every service/handler accepts dependencies as interfaces.
- Wire provider functions live in their owning package (e.g., `tasks/wire.go`).
- Root injector in `cmd/server/wire.go`.

### Concurrency

- **Worker pools** for background tasks (AI, sync, notifications, cleanup).
- **Errgroup** for request-scoped concurrency (parallel AI ops, concurrent queries).
- **SQS/Redis job queue** for heavy decoupled tasks.

**Hard rules:**
- No naked goroutines. Every goroutine launched via worker pool, errgroup, or managed wrapper.
- All goroutines must respect `context.Context` cancellation.
- All goroutines must recover panics.
- No unbounded goroutine creation — use pools with concurrency limits.

```go
// WRONG
go processTask(task)

// CORRECT
pool.Submit(func(ctx context.Context) error {
    return processTask(ctx, task)
})
```

### Database Access — Repository + Unit of Work

- Service code calls repository interfaces, never the MongoDB driver directly.
- No raw BSON in service code — BSON is confined to repository implementations.
- Unit of Work for operations spanning multiple collections (e.g., create task + log AI action).

### Logging — Uber Zap

- Structured JSON logging in all environments.
- gRPC interceptor injects request-scoped logger with `user_id`, `request_id`, `rpc_method`.
- Downstream code extracts logger from context — never creates its own.
- No `fmt.Println` or `log.Println` — only Zap.
- No sensitive data in logs (tokens, PII, passwords, raw user content).
- Log levels are meaningful: Debug (dev tracing), Info (operational), Warn (recoverable), Error (needs attention).

### Configuration — Viper

- Config from: env vars + YAML files + AWS Secrets Manager.
- Parsed into typed Go structs at startup with validation.
- Missing required values cause startup panic with clear error.
- Secrets in AWS Secrets Manager only (never env vars or files in production).
- Config structs injected via Wire, not global variables.

### Documentation

- **Self-documenting code** — clear names, comments only for non-obvious logic. No mandatory godoc.
- **Lightweight ADRs** in `docs/adr/` for major decisions.

---

## gRPC Conventions

### Interceptor Stack (configurable per-RPC)

| Order | Interceptor | Default | Skippable |
|---|---|---|---|
| 1 | Request ID injection | On | No |
| 2 | Logging (request-scoped zap) | On | No |
| 3 | Recovery (panic handler) | On | No |
| 4 | Metrics | On | No |
| 5 | Rate limiting (layered) | On | Per-RPC |
| 6 | Authentication (JWT) | On | Per-RPC |
| 7 | Validation (buf validate) | On | No |

### Rate Limiting — Layered

1. gRPC interceptor with per-RPC configurable limits
2. Redis-backed global limiter for distributed rate limiting
3. Hardened limits on auth endpoints (OTP brute force prevention)

### API Versioning

- Proto packages are versioned: `zee6do.v1.TaskService`.
- Breaking changes require a new package version.
- Additive changes (new fields, new RPCs) go to the current version.
- `buf breaking` checks enforced in CI.

### Request Validation

- Structural validation via `buf validate` proto annotations — automatic, no manual code.
- Business validation (authorization, ownership, state transitions) in the service layer.

---

## Security

- Input validation on all endpoints (proto + business validation).
- No raw string construction for queries — parameterized only.
- TLS 1.3 on all connections.
- JWT validated on every request, device fingerprint checked against claims.
- Rate limiting on all endpoints, hardened on auth.
- Secrets in AWS Secrets Manager. Field-level encryption (CSFLE + AWS KMS) for sensitive MongoDB fields.
- Error messages to client never leak internals (stack traces, DB errors, config).
- `gosec` and `govulncheck` run in CI. Container images scanned before ECR push.

---

## Testing

- **Table-driven tests** as the standard pattern.
- **Testify mocks** for dependency mocking.
- Test names describe behavior: `"returns not found when task does not exist"`.
- All new code must include tests. PRs without tests are rejected.

---

## Database — MongoDB

- **Shared collections** with `user_id` filtering. Field-level encryption (CSFLE) for sensitive fields (`phone`, `email`, `social_ids`, connector credentials).
- **Tuned connection pool**: explicit `maxPoolSize`, connect/socket timeouts, `serverSelectionTimeout`, retry writes.
- **Auto-migrations on startup**: each migration is a Go function with unique ID, tracked in `_migrations` collection, idempotent, run once.
- **Data retention**: 1-year rolling (detailed data), aggregated indefinitely, nightly purge job.

---

## Graceful Shutdown Sequence

1. Mark health check as unhealthy
2. Wait for load balancer drain interval
3. Drain WebSocket connections (close frames, graceful disconnect)
4. Stop worker pools (stop new jobs, wait for in-flight with timeout)
5. Stop gRPC server (`GracefulStop`)
6. Flush logs and metrics
7. Close database connections (MongoDB, Redis)
8. Exit

Each step has a timeout. Total max shutdown: ~30s, then force-exit.

---

## Git Workflow

- **Trunk-based**: short-lived feature branches off `main`, squash-merge via PR.
- **1 approval + CI green** to merge.
- **Deploy tags**: `deploy/YYYY-MM-DD-NNN` for each production deployment.
- CI: golangci-lint, go test, buf lint, buf breaking, Docker build, govulncheck.
- Branch naming: `feat/description`, `fix/description`, `chore/description`.
- Conventional commits encouraged: `feat:`, `fix:`, `chore:`, `docs:`.

---

## Key Product Context for Agents

When working on this codebase, keep these product decisions in mind:

### AI Behavior
- AI is **model-agnostic** — the `LLMProvider` interface abstracts all LLM calls. Switching providers is a config change.
- AI **auto-applies** changes (status transitions, rescheduling, priority changes). The client shows undo; the server executes immediately.
- AI manages the **full task lifecycle**: To Do -> In Progress -> Blocked -> Done -> Archived.
- AI **auto-assigns priority** (P0-P3) based on deadlines, dependencies, connector signals, and learned patterns. Users can override; the AI learns from overrides.
- AI determines optimal **"do dates"** — user sets due date, AI schedules when to work on it.
- AI **auto-categorizes and tags** tasks from content, context, and connector source.
- **Batch operations** supported: "reschedule all marketing tasks to next week."
- **AI undo** — the server maintains an action log; users can undo conversationally.
- AI uses **explicit + implicit signals** to learn: task patterns, overrides, usage times, connected app activity, and thumbs up/down on suggestions.
- **Smart throttling**: auto-batch rapid requests, detect abuse, track per-user LLM cost, degrade gracefully if costs spike.
- Target AI response time: **1-3 seconds**.

### Connectors
- **Google Calendar**: two-way smart sync. Calendar events inform scheduling. Scheduled tasks appear as calendar events. AI resolves conflicts when a new meeting overlaps a task.
- **Slack**: slash commands create tasks, AI parses conversations for action items, reminders sent to Slack.
- **Email**: AI scans inbox for action items, presents as suggestions (user approves/dismisses).
- **ClickUp**: two-way sync of tasks, goals, and docs. ClickUp content feeds the personal knowledge base.

### Knowledge Base
- Auto-ingested from connectors + manually added by user.
- Documents chunked, embedded, stored for semantic search.
- AI queries the knowledge base when scheduling, prioritizing, and generating templates.

### Notifications
- **Morning briefing**: AI sends interactive daily plan notification timed to when the user typically opens the app (AI-learned, not fixed).
- **Smart reminders**: timed based on user habits, not just due dates.
- **End of day**: AI reviews progress, moves incomplete tasks, pre-plans tomorrow.
- Notifications suppressed during Focus Mode.

### Sync
- WatermelonDB sync protocol (`PullChanges`, `PushChanges`).
- Field-level merge for non-conflicting changes.
- AI resolution for conflicting changes (user explicit changes win over AI changes).
- Timestamp-based fallback with conflict logged for review.

### Subscription
- Single plan: $14.99/month, ~20% yearly discount.
- 30-day trial (activated post-beta via server-side feature flag).
- Native App Store (StoreKit 2) / Play Store (Play Billing) receipt verification.
- Expired subscription -> read-only mode (view tasks, no create/edit/AI).

### Data Privacy
- GDPR-compliant: full data export (JSON/CSV), account deletion within 30 days.
- Email content processed for task extraction but not stored permanently.
- Field-level encryption for PII (phone, email, social IDs, connector credentials).
- 1-year data retention, then aggregate and purge.

---

## Related Repository

- **Client**: [vorflux](https://github.com/DeepakDP5/vorflux) — Expo React Native mobile app (iOS + Android).
