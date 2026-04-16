# Tech Stack

> Technology choices and version policies for the Zee6do Go backend server.

---

## Runtime & Language

| Technology | Purpose | Version Policy |
|---|---|---|
| **Go** | Server language | Latest stable release. Pinned via `go.toolchain` directive in `go.mod`. Updated with each minor release after testing. |
| **Protocol Buffers (proto3)** | API contract definition | Managed via Buf CLI + Buf Schema Registry (BSR) |
| **gRPC** | Client-server communication protocol | Latest stable `google.golang.org/grpc` |

## Database & Storage

| Technology | Purpose |
|---|---|
| **MongoDB** (Atlas on AWS) | Primary database — shared collections with field-level encryption (CSFLE) |
| **AWS S3** | File/attachment storage (photos, voice notes, documents) via pre-signed URLs |
| **Redis** (optional at scale) | Job queue backend, distributed rate limiting |

## AI / LLM

| Technology | Purpose |
|---|---|
| **Model-agnostic LLM interface** | Abstraction layer supporting OpenAI, Anthropic, Google, and self-hosted providers |
| **Vector embeddings** | Knowledge base semantic search (stored in MongoDB or dedicated vector store at scale) |

## Infrastructure

| Technology | Purpose |
|---|---|
| **AWS ECS Fargate / EC2** | Compute — Go binary in Docker container with auto-scaling |
| **AWS KMS** | Encryption key management for MongoDB CSFLE |
| **AWS Secrets Manager** | API keys, OAuth credentials, encryption keys |
| **AWS SQS** (optional) | Job queue for heavy background tasks at scale |
| **Docker** | Container runtime — multi-stage build, distroless base, non-root |

## Build & Deploy

| Technology | Purpose |
|---|---|
| **GitHub Actions** | CI/CD pipeline (lint, test, build, deploy) |
| **Docker** | Multi-stage build with distroless final image |
| **Buf CLI + BSR** | Proto linting, breaking change detection, code generation, schema distribution |

## Code Quality

| Tool | Purpose |
|---|---|
| **golangci-lint (strict)** | Static analysis — govet, errcheck, staticcheck, gosec, revive, exhaustive, gocritic |
| **Buf lint** | Proto file linting and breaking change detection |
| **go test** | Unit and integration testing with table-driven tests + testify mocks |
| **govulncheck** | Dependency vulnerability scanning |

## Observability

| Tool | Purpose |
|---|---|
| **Uber Zap** | Structured JSON logging with request-scoped loggers |
| **Sentry** | Error tracking and panic recovery reporting |
| **Datadog / AWS X-Ray** | APM — request tracing, latency analysis, dependency mapping |
| **Prometheus + Grafana** (or CloudWatch) | Metrics — request rates, error rates, AI latency, pool utilization |

## Key Libraries

| Library | Purpose |
|---|---|
| **google.golang.org/grpc** | gRPC server and interceptors |
| **go.mongodb.org/mongo-driver** | MongoDB Go driver |
| **github.com/google/wire** | Compile-time dependency injection |
| **github.com/spf13/viper** | Configuration loading (env vars + YAML + Secrets Manager) |
| **go.uber.org/zap** | Structured logging |
| **github.com/stretchr/testify** | Test assertions and mocking |
| **github.com/bufbuild/protovalidate-go** | Proto-based request validation |
| **golang.org/x/sync/errgroup** | Structured concurrency for request-scoped parallelism |

## Dependency Management

Go module dependencies are managed via `go.mod`. Review criteria for new dependencies:

1. **Maintenance status** — actively maintained, recent releases, responsive to issues
2. **License** — MIT, Apache 2.0, or BSD compatible
3. **Transitive dependencies** — minimize the dependency tree
4. **Security** — no known vulnerabilities (verified via `govulncheck`)
5. **Alternatives** — can this be done with the standard library?
