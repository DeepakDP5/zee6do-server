# Technical Guardrails

> Hard rules enforced across the Zee6do server codebase. These are non-negotiable unless explicitly overridden by an ADR.

---

## Go Version

- Always use the **latest stable Go release**.
- Pin the exact version via the `toolchain` directive in `go.mod`.
- Update with each minor release after CI passes.

---

## Linting & Static Analysis

**golangci-lint with strict linter set:**

Enabled linters (minimum):
- `govet` — correctness checks
- `errcheck` — unchecked error returns
- `staticcheck` — comprehensive static analysis
- `gosec` — security-focused analysis
- `revive` — flexible linter with configurable rules
- `exhaustive` — exhaustive switch statements on enums
- `gocritic` — opinionated code quality checks

All linters must pass in CI. No `//nolint` directives without an accompanying comment explaining why.

---

## Error Handling

### Pattern: Error Wrapping + Sentinel Errors

```go
// Sentinel errors for known conditions
var (
    ErrTaskNotFound    = errors.New("task not found")
    ErrUnauthorized    = errors.New("unauthorized")
    ErrConflict        = errors.New("conflict")
)

// Always wrap errors with context using %w
func (r *taskRepo) GetByID(ctx context.Context, id string) (*Task, error) {
    task, err := r.collection.FindOne(ctx, bson.M{"_id": id})
    if err != nil {
        if errors.Is(err, mongo.ErrNoDocuments) {
            return nil, ErrTaskNotFound
        }
        return nil, fmt.Errorf("taskRepo.GetByID(%s): %w", id, err)
    }
    return task, nil
}

// Callers check with errors.Is / errors.As
if errors.Is(err, ErrTaskNotFound) {
    return status.Error(codes.NotFound, "task not found")
}
```

**Rules:**
- Always wrap errors with `fmt.Errorf("context: %w", err)` — never return bare errors from lower layers.
- Use sentinel errors (`var ErrXxx = errors.New(...)`) for known, checkable conditions.
- Use `errors.Is()` and `errors.As()` for error checking — never string comparison.
- Never silently discard errors. If an error is intentionally ignored, add a comment explaining why.
- gRPC handlers map domain errors to gRPC status codes at the handler boundary, not deeper.

---

## Package Organization

### Domain-Driven Packages

```
internal/
    auth/           # Authentication domain
    tasks/          # Task management domain
    ai/             # AI orchestration domain
    scheduler/      # Scheduling domain
    connectors/     # External integrations
        google_calendar/
        slack/
        email/
        clickup/
    knowledge/      # Knowledge base domain
    ...
```

**Rules:**
- Packages mirror domain boundaries — not technical layers.
- Sub-packages are created only when genuinely needed (e.g., each connector is a sub-package of `connectors/`).
- No circular dependencies between packages. Dependencies flow inward: handlers -> services -> repositories.
- Each package owns its own types, interfaces, and repository definitions.
- Shared types that cross domain boundaries go in `pkg/` (middleware, crypto, config, errors).

---

## Dependency Injection

### Google Wire (Compile-Time DI)

```go
// wire.go — declares the dependency graph
func InitializeApp(cfg *config.Config) (*App, error) {
    wire.Build(
        NewMongoClient,
        NewTaskRepository,
        NewTaskService,
        NewTaskHandler,
        NewApp,
    )
    return nil, nil
}
```

**Rules:**
- All dependencies are wired via Google Wire — no manual `New()` chains in `main()`.
- Every service/handler accepts its dependencies as interfaces via its constructor.
- Wire provider functions live in the package they belong to (e.g., `tasks/wire.go`).
- The root `cmd/server/wire.go` composes all module providers.

---

## Concurrency

### Worker Pool + Errgroup

- **Worker pools** for background tasks: AI processing, connector sync jobs, notification delivery, data retention cleanup.
- **Errgroup** for request-scoped concurrency: parallel AI operations, concurrent connector queries.
- **SQS/Redis job queue** for heavy tasks that benefit from decoupling (dispatched in-process, processed by the same binary or a separate worker at scale).

**Rules:**
- **No naked goroutines.** Every goroutine must be launched via a worker pool, errgroup, or a managed wrapper.
- **All goroutines must respect `context.Context` cancellation.** If the context is cancelled, the goroutine must exit promptly.
- **Panics must be recovered.** Worker pools and goroutine wrappers must include panic recovery with logging.
- **No unbounded goroutine creation.** Use worker pools with configured concurrency limits.

```go
// WRONG — naked goroutine, no context, no recovery
go processTask(task)

// CORRECT — worker pool with context and recovery
pool.Submit(func(ctx context.Context) error {
    return processTask(ctx, task)
})
```

---

## Database Access

### Repository + Unit of Work Pattern

```go
// Repository interface — defined in the domain package
type TaskRepository interface {
    GetByID(ctx context.Context, id string) (*Task, error)
    Create(ctx context.Context, task *Task) error
    Update(ctx context.Context, task *Task) error
    Delete(ctx context.Context, id string) error
    ListByUser(ctx context.Context, userID string, opts ListOpts) ([]*Task, error)
}

// Unit of Work — for transactional consistency across collections
type UnitOfWork interface {
    Begin(ctx context.Context) (Transaction, error)
}

type Transaction interface {
    TaskRepo() TaskRepository
    AIActionRepo() AIActionRepository
    Commit(ctx context.Context) error
    Rollback(ctx context.Context) error
}
```

**Rules:**
- Service code calls repository interfaces, never the MongoDB driver directly.
- Repository implementations live alongside their domain package (e.g., `tasks/mongo_repo.go`).
- Unit of Work is used when an operation must atomically modify multiple collections (e.g., creating a task + logging an AI action).
- No raw BSON construction in service code. BSON is confined to repository implementations.

---

## gRPC Conventions

### Request Validation
- Structural validation is defined in `.proto` files using `buf validate` annotations.
- Validation is applied automatically — no manual validation code in handlers for structural rules.
- Business validation (e.g., "user owns this task") lives in the service layer.

### API Versioning
- Proto packages are versioned: `zee6do.v1.TaskService`, `zee6do.v2.TaskService`.
- Breaking changes require a new package version.
- Additive changes (new fields, new RPCs) are made to the current version.

### Interceptor Stack
Configurable per-RPC with these defaults:

| Order | Interceptor | Skippable? |
|---|---|---|
| 1 | Request ID injection | No |
| 2 | Logging (request-scoped zap logger) | No |
| 3 | Recovery (panic handler) | No |
| 4 | Metrics (request count, latency) | No |
| 5 | Rate limiting (layered) | Per-RPC (e.g., skip for health check) |
| 6 | Authentication (JWT validation) | Per-RPC (e.g., skip for health check, auth RPCs) |
| 7 | Validation (buf validate) | No |

### Rate Limiting (Layered)
1. **gRPC interceptor** — per-RPC configurable limits
2. **Redis-backed global limiter** — distributed rate limiting across server instances
3. **Auth endpoint hardening** — stricter limits on `SendOTP`, `VerifyOTP` to prevent brute force

---

## Configuration

### Viper

- Configuration loaded from: environment variables, YAML config files, and AWS Secrets Manager.
- All config is parsed into **typed Go structs** at startup.
- Missing required config values cause the server to fail fast with a clear error message at startup.
- Secrets (API keys, OAuth credentials, encryption keys) are loaded from AWS Secrets Manager, never from env vars or config files in production.
- Config structs are defined in `pkg/config/` and passed to modules via Wire.

---

## Logging

### Uber Zap

- **Structured JSON logging** in all environments.
- **Request-scoped loggers**: gRPC interceptor injects a zap logger into context with pre-populated fields (`user_id`, `request_id`, `rpc_method`).
- Downstream code extracts the logger from context — never creates its own.

**Rules:**
- No `fmt.Println` or `log.Println` anywhere — only zap.
- No sensitive data in logs: no tokens, no PII, no passwords, no raw request bodies containing user content.
- Log levels are meaningful: `Debug` for development tracing, `Info` for operational events, `Warn` for recoverable issues, `Error` for failures requiring attention.
- Every error log must include the `error` field and sufficient context to diagnose without reproducing.

---

## Graceful Shutdown

Full lifecycle shutdown sequence when SIGTERM/SIGINT is received:

1. **Mark health check as unhealthy** — load balancer stops sending new requests
2. **Wait for drain interval** — allow load balancer to complete drain (configurable, e.g., 10s)
3. **Drain WebSocket connections** — send close frames, wait for graceful disconnect
4. **Stop worker pools** — stop accepting new jobs, wait for in-flight jobs to complete (with timeout)
5. **Stop gRPC server** — `GracefulStop()` waits for in-flight RPCs to finish
6. **Flush logs and metrics** — ensure all buffered logs and metrics are sent
7. **Close database connections** — close MongoDB, Redis connections
8. **Exit**

Each step has a timeout. If the total shutdown exceeds the maximum (e.g., 30s), force-exit.

---

## Security

### Standard Practices
- Input validation on all endpoints (proto validation + business validation)
- Parameterized queries only — no string concatenation for BSON/queries
- Rate limiting on all endpoints, hardened on auth endpoints
- TLS 1.3 for all connections
- No secrets in logs or error messages
- No secrets in environment variables in production — use AWS Secrets Manager
- JWT tokens validated on every request (except explicitly skipped RPCs)
- Device fingerprint validated against JWT claims

---

## Testing

### Table-Driven Tests + Testify Mocks

```go
func TestTaskService_Create(t *testing.T) {
    tests := []struct {
        name    string
        input   *CreateTaskRequest
        mockFn  func(repo *MockTaskRepository)
        want    *Task
        wantErr error
    }{
        {
            name:  "creates task successfully",
            input: &CreateTaskRequest{Title: "Buy groceries"},
            mockFn: func(repo *MockTaskRepository) {
                repo.On("Create", mock.Anything, mock.Anything).Return(nil)
            },
            want: &Task{Title: "Buy groceries"},
        },
        // ... more cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            repo := new(MockTaskRepository)
            tt.mockFn(repo)
            svc := NewTaskService(repo)
            got, err := svc.Create(context.Background(), tt.input)
            // assertions...
        })
    }
}
```

**Rules:**
- Table-driven tests are the standard pattern for all unit tests.
- Dependencies are mocked using testify/mock.
- Test names describe behavior: `"returns not found when task does not exist"`, not `"TestGetByID_Error"`.
- All new code must include tests. PRs adding functionality without tests will be rejected.
- Coverage target: tracked but not hard-gated in CI (yet).

---

## Proto Management (Buf + BSR)

- All `.proto` files live in the `proto/` directory.
- **Buf CLI** handles linting, breaking change detection, and code generation.
- **Buf Schema Registry (BSR)** is used for proto distribution — the mobile client consumes generated TypeScript stubs from BSR.
- `buf lint` and `buf breaking` run in CI on every PR.
- Proto packages are versioned: `zee6do.v1`, `zee6do.v2`, etc.

---

## Database Migrations

- Migrations are applied **automatically on server startup**.
- Each migration is a Go function with a unique ID, registered in a migration registry.
- Applied migrations are tracked in a `_migrations` collection in MongoDB.
- Migrations run once and are idempotent.
- Migration functions can create indexes, modify collection schemas, and transform data.
- Migrations are tested in CI with a real MongoDB instance (testcontainers or equivalent).

---

## Docker Image

- **Multi-stage build**: builder stage compiles the Go binary, final stage uses `gcr.io/distroless/static` (or `scratch`).
- **Non-root user**: the container runs as a non-root user.
- **No shell**: distroless/scratch images have no shell — reduces attack surface.
- **Vulnerability scanning**: container images are scanned in CI before push to ECR.
- **Minimal image size**: target < 20MB for the final image.

---

## Git & Code Review

### Branching
- **Trunk-based development**: short-lived feature branches off `main`, squash-merged via PR.
- No `develop` branch. No long-lived feature branches.
- Branch naming: `feat/description`, `fix/description`, `chore/description`.
- **Deploy tags**: Git tags are created for each production deployment (e.g., `deploy/2024-01-15-001`) for easy rollback reference.

### PRs
- **1 approval + CI green** required to merge.
- CI checks: golangci-lint, go test, buf lint, buf breaking, Docker build.
- PRs should be small and focused.

### Commits
- Conventional commit format is encouraged: `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`.
