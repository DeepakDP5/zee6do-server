# Zee6do Server

**Go backend server for Zee6do -- the AI-powered personal productivity tool.**

This repository contains the server-side infrastructure that powers Zee6do: a gRPC API server, AI orchestration engine, connector integrations, real-time sync, and notification services. It is designed as a modular monolith that can be split into microservices as scale demands.

---

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
  - [Modular Monolith](#modular-monolith)
  - [Module Breakdown](#module-breakdown)
  - [System Diagram](#system-diagram)
- [API Layer (gRPC)](#api-layer-grpc)
- [Authentication & Security](#authentication--security)
- [AI Orchestration Engine](#ai-orchestration-engine)
  - [Model-Agnostic Design](#model-agnostic-design)
  - [AI Capabilities Served](#ai-capabilities-served)
  - [Smart Throttling](#smart-throttling)
  - [AI Response Performance](#ai-response-performance)
- [Database (MongoDB)](#database-mongodb)
  - [Schema Design](#schema-design)
  - [Collections](#collections)
  - [Field-Level Encryption](#field-level-encryption)
  - [Data Retention](#data-retention)
- [Real-Time Communication](#real-time-communication)
  - [WebSocket Server](#websocket-server)
  - [Push Notifications](#push-notifications)
  - [Smart Notification Timing](#smart-notification-timing)
- [Connector Integrations](#connector-integrations)
  - [Google Calendar (Smart Sync)](#google-calendar-smart-sync)
  - [Slack (Full Integration)](#slack-full-integration)
  - [Email (AI Parsing)](#email-ai-parsing)
  - [ClickUp (Two-Way Sync + Knowledge Base)](#clickup-two-way-sync--knowledge-base)
- [Personal Knowledge Base](#personal-knowledge-base)
- [Offline Sync & Conflict Resolution](#offline-sync--conflict-resolution)
- [File Storage (AWS S3)](#file-storage-aws-s3)
- [Subscription & Billing](#subscription--billing)
- [Voice Processing](#voice-processing)
- [Feature Flags](#feature-flags)
- [Observability & Monitoring](#observability--monitoring)
- [Deployment & Infrastructure](#deployment--infrastructure)
- [Testing Strategy](#testing-strategy)
- [CI/CD Pipeline](#cicd-pipeline)
- [Data Privacy & GDPR](#data-privacy--gdpr)
- [Repository Structure](#repository-structure)

---

## Overview

The Zee6do server is the brain behind the app. It handles:

- **All task CRUD** via gRPC endpoints consumed by the mobile client
- **AI orchestration** -- routing requests to LLM providers, maintaining per-user AI context, executing batch operations, and auto-managing the task lifecycle
- **Connector integrations** -- two-way sync with Google Calendar, Slack, Email, and ClickUp
- **Real-time sync** -- WebSocket connections for live in-app updates and push notification delivery
- **Personal knowledge base** -- ingesting, indexing, and querying user knowledge for AI context
- **Authentication** -- Phone OTP, social login (Google, Apple), JWT with device binding
- **Subscription management** -- Verifying App Store/Play Store receipts and enforcing subscription state
- **File storage** -- Managing uploads/downloads to AWS S3 via pre-signed URLs

**Target scale:** ~1,000 users at launch (closed beta), growing to ~100,000 users at maturity.

---

## Architecture

### Modular Monolith

The server starts as a **modular monolith** -- a single Go binary with clean internal module boundaries. Each module has its own package, data access layer, and service interfaces. Modules communicate via Go interfaces (not HTTP/gRPC internally), making it straightforward to extract any module into a separate microservice when scale demands.

**Why modular monolith:**
- Simpler deployment and operations for a small team at launch
- No distributed system complexity (no service discovery, no distributed tracing needed day one)
- Clean module boundaries mean extraction to microservices is a deployment change, not a rewrite
- Single binary is easier to debug, profile, and reason about

### Module Breakdown

| Module | Responsibility |
|---|---|
| **auth** | Phone OTP verification, social login (Google/Apple OAuth), JWT issuance with device binding, token refresh, session management |
| **tasks** | Task CRUD, subtasks, status transitions, priority management, recurrence handling, task search |
| **ai** | LLM provider abstraction, prompt engineering, AI context management, batch operations, auto-apply logic, undo history |
| **scheduler** | Smart scheduling engine -- determines optimal "do dates," resolves calendar conflicts, manages the AI agenda view |
| **connectors** | Integration adapters for Google Calendar, Slack, Email, ClickUp -- each connector is a sub-module with its own sync logic |
| **knowledge** | Personal knowledge base -- ingestion, indexing, vector storage, semantic search for AI context retrieval |
| **notifications** | Push notification delivery (APNs/FCM), smart timing engine, notification preferences, morning briefing generation |
| **realtime** | WebSocket server for live in-app updates, connection management, event broadcasting |
| **analytics** | Productivity metrics computation, time tracking aggregation, AI insight generation |
| **storage** | AWS S3 integration -- pre-signed URL generation for upload/download, attachment metadata management |
| **billing** | App Store / Play Store receipt verification, subscription state management, trial/expiry logic |
| **sync** | Offline sync protocol, conflict resolution, WatermelonDB sync endpoint |
| **users** | User profile management, preferences, device registry, data export, account deletion |
| **flags** | Feature flag management -- read flags from config/remote, expose to client |

### System Diagram

```
                          +---------------------+
                          |   Mobile Client      |
                          |   (React Native /    |
                          |    Expo)             |
                          +----------+----------+
                                     |
                          gRPC  /  WebSocket  /  Push
                                     |
                          +----------v----------+
                          |                      |
                          |   Zee6do Server      |
                          |   (Go Monolith)      |
                          |                      |
                          |  +------+ +-------+  |
                          |  | auth | | tasks |  |
                          |  +------+ +-------+  |
                          |  +------+ +-------+  |
                          |  |  ai  | |sched- |  |
                          |  |      | |  uler |  |
                          |  +------+ +-------+  |
                          |  +------+ +-------+  |
                          |  |conn- | |knowl- |  |
                          |  |ectors| | edge  |  |
                          |  +------+ +-------+  |
                          |  +------+ +-------+  |
                          |  |notif-| |real-  |  |
                          |  |icate | | time  |  |
                          |  +------+ +-------+  |
                          |  +------+ +-------+  |
                          |  |anal- | |billing|  |
                          |  |ytics | |       |  |
                          |  +------+ +-------+  |
                          |  +------+ +-------+  |
                          |  |sync  | |storage|  |
                          |  +------+ +-------+  |
                          +----------+----------+
                                     |
              +----------------------+----------------------+
              |                      |                      |
     +--------v--------+   +--------v--------+   +---------v-------+
     |    MongoDB       |   |    AWS S3       |   |  LLM Providers  |
     |  (Shared DB +    |   |  (Attachments)  |   | (OpenAI, Claude,|
     |  Field-Level     |   |                 |   |  Gemini, etc.)  |
     |  Encryption)     |   +-----------------+   +-----------------+
     +---------+--------+
               |
     +---------v---------+
     | External Services  |
     | - Google Calendar   |
     | - Slack             |
     | - Email (IMAP/API)  |
     | - ClickUp           |
     | - APNs / FCM        |
     +--------------------+
```

---

## API Layer (gRPC)

All client-server communication uses **gRPC** with Protocol Buffers:

- **High performance** -- Binary serialization is significantly more efficient than JSON over REST
- **Strongly typed** -- Proto definitions serve as the contract between client and server
- **Streaming** -- Server-side streaming for AI responses and real-time updates
- **Code generation** -- Auto-generated Go server stubs and TypeScript client code

**Key service definitions:**

| Service | Key RPCs |
|---|---|
| `AuthService` | `SendOTP`, `VerifyOTP`, `SocialLogin`, `RefreshToken`, `ListDevices`, `RevokeDevice` |
| `TaskService` | `CreateTask`, `UpdateTask`, `DeleteTask`, `GetTask`, `ListTasks`, `BatchUpdate`, `SearchTasks` |
| `AIService` | `Chat`, `ProcessNaturalLanguage`, `GetSuggestions`, `UndoAction`, `GenerateTemplate`, `GetDailyPlan` |
| `SchedulerService` | `GetAgendaView`, `RescheduleTask`, `ResolveConflict` |
| `ConnectorService` | `LinkConnector`, `UnlinkConnector`, `SyncNow`, `GetSyncStatus` |
| `KnowledgeService` | `AddDocument`, `SearchKnowledge`, `ListDocuments`, `DeleteDocument` |
| `AnalyticsService` | `GetMetrics`, `GetTimeBreakdown`, `GetAINarrative` |
| `NotificationService` | `GetPreferences`, `UpdatePreferences`, `RegisterDevice` |
| `SyncService` | `PullChanges`, `PushChanges` (WatermelonDB sync protocol) |
| `UserService` | `GetProfile`, `UpdateProfile`, `ExportData`, `DeleteAccount` |
| `BillingService` | `VerifyReceipt`, `GetSubscriptionStatus`, `RestorePurchases` |
| `StorageService` | `GetUploadURL`, `GetDownloadURL`, `DeleteAttachment` |

---

## Authentication & Security

### Authentication Flow

1. **Phone OTP** -- User enters phone number -> server sends OTP via SMS provider -> user submits OTP -> server verifies and issues JWT
2. **Social Login** -- User authenticates via Google/Apple on device -> server receives OAuth token -> verifies with provider -> issues JWT
3. **JWT with Device Binding** -- JWT access tokens are bound to the device fingerprint. A token used from a different device is rejected, requiring re-authentication. This prevents token theft.
4. **Token Refresh** -- Short-lived access tokens (e.g., 15 minutes) with longer-lived refresh tokens (e.g., 30 days). Refresh tokens rotate on use.

### Security Measures

| Measure | Implementation |
|---|---|
| **Transport** | TLS 1.3 for all connections |
| **Token binding** | JWT claims include device fingerprint hash |
| **Rate limiting** | Per-IP and per-user rate limiting on auth endpoints |
| **OTP security** | Time-limited OTPs, brute-force protection (max attempts) |
| **Device management** | Users can view all active sessions and revoke any device |
| **Biometric** | Handled client-side; server validates JWT regardless of how user authenticated locally |

---

## AI Orchestration Engine

### Model-Agnostic Design

The AI module abstracts LLM interactions behind a provider interface:

```go
type LLMProvider interface {
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
    Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
}
```

Supported providers (swappable via configuration):
- OpenAI (GPT-4, GPT-4o)
- Anthropic (Claude)
- Google (Gemini)
- Self-hosted models (via OpenAI-compatible API)

Switching providers requires zero client changes -- the mobile app sends the same gRPC requests regardless of which LLM processes them.

### AI Capabilities Served

The server's AI module handles all AI-driven features:

| Capability | Server Responsibility |
|---|---|
| **Task lifecycle management** | Execute status transitions, update priorities, reschedule tasks based on AI decisions |
| **Natural language parsing** | Parse free-form text/voice transcriptions into structured task objects |
| **Smart scheduling** | Compute optimal "do dates" using calendar, workload, and user patterns |
| **Priority assignment** | Calculate P0-P3 priority based on multi-signal analysis |
| **Auto-categorization** | Assign categories and tags using content analysis and learned patterns |
| **Batch operations** | Execute multi-task commands ("reschedule all marketing tasks") |
| **Template generation** | Create structured project templates from user descriptions and knowledge base |
| **Daily planning** | Generate the morning briefing and interactive daily plan |
| **End-of-day review** | Analyze day's progress, move incomplete tasks, pre-plan tomorrow |
| **Analytics narratives** | Generate natural language summaries for any time period |
| **Undo history** | Maintain action log and execute reversals on request |
| **Knowledge-informed decisions** | Query the personal knowledge base to inform AI decisions |

### Smart Throttling

No hard rate limit on AI interactions. Instead:

- **Auto-batching** -- Rapid sequential requests are batched into a single LLM call
- **Abuse detection** -- Abnormal usage patterns (e.g., 1000 requests in a minute) trigger temporary throttling
- **Cost controls** -- Internal per-user cost tracking with configurable ceilings; if LLM costs spike, the server degrades gracefully (e.g., falls back to simpler models or cached responses)
- **Queue management** -- Non-urgent AI operations (analytics generation, end-of-day review) are queued and processed during off-peak hours

### AI Response Performance

Target: **1-3 seconds** for standard AI operations with a loading indicator on the client.

Optimizations:
- **Prompt caching** -- Frequently used prompt templates are pre-compiled
- **Context windowing** -- AI context is efficiently managed to minimize token usage
- **Parallel processing** -- Independent AI operations (e.g., categorization + scheduling) run concurrently
- **Response caching** -- Identical queries within a short window return cached responses

---

## Database (MongoDB)

### Schema Design

**Shared database with field-level encryption** -- all users share the same collections, with `user_id` field for filtering and sensitive fields encrypted at the document level.

### Collections

| Collection | Purpose | Key Fields |
|---|---|---|
| `users` | User profiles and preferences | `phone`, `email`, `social_ids`, `preferences`, `work_hours`, `timezone` |
| `tasks` | All task data | `user_id`, `title`, `description`, `status`, `priority`, `due_date`, `scheduled_date`, `category`, `tags`, `attachments`, `subtasks`, `recurrence`, `ai_notes`, `source`, `connector_ref` |
| `sessions` | Active login sessions | `user_id`, `device_fingerprint`, `jwt_id`, `created_at`, `last_active` |
| `ai_context` | Per-user AI learning data | `user_id`, `completion_patterns`, `override_history`, `usage_patterns`, `explicit_feedback` |
| `ai_actions` | Undo history for AI actions | `user_id`, `action_type`, `before_state`, `after_state`, `timestamp`, `undone` |
| `knowledge_docs` | Personal knowledge base documents | `user_id`, `title`, `content`, `source` (manual/clickup/slack/email), `embedding_ref`, `created_at` |
| `knowledge_embeddings` | Vector embeddings for semantic search | `doc_id`, `user_id`, `embedding`, `chunk_text` |
| `connectors` | User connector configurations | `user_id`, `type` (google_cal/slack/email/clickup), `credentials` (encrypted), `sync_state`, `last_sync` |
| `sync_log` | Offline sync tracking | `user_id`, `device_id`, `last_pulled_at`, `last_pushed_at`, `pending_changes` |
| `notifications` | Notification history and preferences | `user_id`, `type`, `payload`, `sent_at`, `read_at`, `preferences` |
| `analytics_daily` | Daily aggregated analytics | `user_id`, `date`, `tasks_created`, `tasks_completed`, `time_by_category`, `focus_minutes` |
| `subscriptions` | Subscription state | `user_id`, `store` (apple/google), `receipt`, `status`, `trial_start`, `trial_end`, `expires_at` |
| `attachments` | File attachment metadata | `user_id`, `task_id`, `s3_key`, `filename`, `mime_type`, `size_bytes` |

### Field-Level Encryption

MongoDB Client-Side Field Level Encryption (CSFLE) is used for sensitive data:

- **Encrypted fields:** `phone`, `email`, `social_ids`, connector `credentials`, knowledge base `content` (when sourced from email/Slack)
- **Encryption keys:** Managed via AWS KMS
- **Queryable encryption:** Where supported, encrypted fields can still be queried without decrypting the entire document

### Data Retention

- **Detailed activity data:** Retained for 1 year
- **Aggregated analytics:** Retained indefinitely (rolling aggregation after 1 year)
- **Raw data purge:** Detailed event logs and AI action history older than 1 year are automatically purged
- **User-requested deletion:** Full account deletion permanently removes all data within 30 days (GDPR compliance)

---

## Real-Time Communication

### WebSocket Server

A WebSocket server provides real-time in-app updates:

- **Task sync events** -- When a task is modified (by AI, by a connector, or by another device), all connected clients receive the update
- **AI action notifications** -- When the AI auto-applies a change, the client receives the event to show the undo toast
- **Connector events** -- New tasks from Slack, calendar conflicts, email action items
- **Sync completion** -- Notify client when a background sync cycle completes

**Connection management:**
- Authenticated WebSocket connections (JWT in handshake)
- Automatic reconnection with exponential backoff on the client
- Heartbeat ping/pong to detect stale connections
- Per-user connection registry supporting multiple devices

### Push Notifications

| Provider | Platform | Usage |
|---|---|---|
| APNs | iOS | Smart reminders, morning briefing, daily summary |
| FCM | Android | Smart reminders, morning briefing, daily summary |

Push notifications are sent when the user's app is backgrounded or closed. When the app is in the foreground, the WebSocket connection delivers events instead (no duplicate notifications).

### Smart Notification Timing

The notification module includes a timing engine:

- **Usage pattern analysis** -- Tracks when the user typically opens the app each day
- **Morning briefing timing** -- Sent just before the user's predicted app-open time (no fixed schedule)
- **Reminder optimization** -- Reminders are timed based on when the user typically handles similar tasks, not just the due date
- **Quiet hours** -- Automatically inferred from usage patterns (no notifications during sleeping hours)

---

## Connector Integrations

### Google Calendar (Smart Sync)

**Two-way sync with AI conflict resolution:**

- **Inbound:** Calendar events are pulled and used by the scheduler to find free time blocks, avoid conflicts, and inform smart context switching
- **Outbound:** Tasks with scheduled times are pushed as calendar events (separate "Zee6do" calendar)
- **Conflict resolution:** When a new calendar event overlaps with a scheduled task, the AI automatically reschedules the task to the next optimal slot
- **Sync frequency:** Webhook-based (Google Calendar push notifications) for near-real-time sync
- **OAuth2:** Google OAuth2 for calendar access with minimal required scopes

### Slack (Full Integration)

- **Task creation:** Slash commands (`/zee6do add ...`) and emoji reactions to save messages as tasks
- **Notification delivery:** Reminders and daily summaries posted to a configured Slack channel or DM
- **AI conversation parsing:** When enabled, the AI periodically scans designated Slack channels for messages containing action items and suggests tasks
- **OAuth2:** Slack OAuth for workspace access
- **Events API:** Real-time event subscription for message monitoring

### Email (AI Parsing)

- **Inbox scanning:** Server connects to the user's email via OAuth (Gmail API / Microsoft Graph) and periodically scans for emails containing action items
- **AI extraction:** LLM analyzes email content and extracts potential tasks with context (sender, subject, deadline mentions)
- **Suggestion queue:** Extracted tasks are presented as suggestions in the app -- user approves or dismisses
- **Privacy:** Email content is processed server-side but not stored permanently. Only the extracted task data is saved.

### ClickUp (Two-Way Sync + Knowledge Base)

- **Two-way task sync:** ClickUp tasks sync bidirectionally with Zee6do tasks. Changes in either system propagate to the other.
- **Goals sync:** ClickUp Goals are synced and used by the AI to align task priorities with higher-level objectives
- **Docs sync:** ClickUp Docs are ingested into the personal knowledge base, giving the AI full project context
- **Conflict handling:** ClickUp is treated as the source of truth for tasks that originated there. Zee6do-originated tasks are source of truth in Zee6do.
- **API:** ClickUp API v2 with OAuth2 authentication
- **Webhook:** ClickUp webhooks for real-time change detection

---

## Personal Knowledge Base

The knowledge base module provides AI-enhanced document storage and retrieval:

- **Ingestion sources:** ClickUp Docs, ClickUp Goals, user-uploaded documents (via the app), manually entered notes
- **Processing pipeline:** Documents are chunked, embedded (via LLM embedding models), and stored for semantic search
- **Vector storage:** Embeddings stored in MongoDB (or a dedicated vector store if scale demands)
- **Semantic search:** AI queries the knowledge base when making decisions (scheduling, prioritizing, generating templates). For example, if a user asks "create a project plan for client onboarding," the AI searches the knowledge base for existing onboarding SOPs.
- **User-facing search:** Users can browse and search their knowledge base directly from the app
- **Auto-refresh:** Connected source documents are re-synced periodically to keep the knowledge base current

---

## Offline Sync & Conflict Resolution

The sync module implements the WatermelonDB sync protocol:

- **Pull endpoint:** Client sends last pull timestamp, server returns all changes since then
- **Push endpoint:** Client sends locally queued changes, server applies them
- **Conflict resolution:** When the same task is modified on multiple devices (or by the AI) while offline:
  - **Field-level merge:** Non-conflicting field changes are merged automatically
  - **AI resolution:** When the same field was changed differently, the AI determines the best resolution based on context (e.g., if the user changed priority to P0 on one device and the AI changed it to P1, the user's explicit change wins)
  - **Timestamp-based fallback:** If AI resolution is ambiguous, last-write-wins with the conflict logged for user review

---

## File Storage (AWS S3)

- **Bucket:** Dedicated S3 bucket for user attachments, organized by `user_id/task_id/`
- **Upload flow:** Client requests a pre-signed upload URL from the server -> uploads directly to S3 -> confirms upload to server -> server stores metadata in MongoDB
- **Download flow:** Client requests a pre-signed download URL -> downloads directly from S3
- **Security:** Pre-signed URLs expire after a short window (e.g., 15 minutes). Bucket policy enforces that only pre-signed URLs can access objects.
- **Lifecycle:** When a task is permanently deleted, associated S3 objects are cleaned up via a background job
- **Size limits:** Per-file and per-user storage limits enforced server-side

---

## Subscription & Billing

### Receipt Verification

- **Apple App Store:** Server-side receipt verification via the App Store Server API (StoreKit 2). Verifies purchase receipts, handles subscription renewals, and detects cancellations.
- **Google Play Store:** Server-side receipt verification via the Google Play Developer API. Real-time developer notifications (RTDN) for subscription events.

### Subscription States

| State | User Experience |
|---|---|
| **Active** | Full access to all features |
| **Trial** | Full access for 30 days (post-beta; during beta, all features free) |
| **Expired** | Read-only mode -- can view tasks but cannot create, edit, or use AI |
| **Grace period** | Store-managed grace period during payment retry |

### Beta Flag

A server-side feature flag controls when the subscription paywall activates. During beta, this flag is off and all features are free. When the product is ready for public launch, the flag is flipped to enable the 30-day trial and subscription requirement.

---

## Voice Processing

**Hybrid speech-to-text architecture:**

- **Primary:** Client uses on-device STT (iOS Speech Framework / Android SpeechRecognizer) and sends the transcription text to the server for AI parsing
- **Fallback:** When the client indicates low STT confidence, raw audio is uploaded to the server, which processes it through a cloud STT service (AWS Transcribe, Google Cloud Speech, or OpenAI Whisper) before AI parsing
- **Server-side AI parsing:** Regardless of STT source, the transcribed text goes through the same NLP pipeline to extract structured task data

---

## Feature Flags

Server-side feature flag management:

- **Flag storage:** Configuration file or remote config service (AWS AppConfig, LaunchDarkly, or custom MongoDB-backed solution)
- **Client exposure:** Flags are served to the mobile client via a dedicated gRPC endpoint, cached locally
- **Use cases:**
  - Gradual feature rollout (percentage-based or user-segment-based)
  - A/B testing new AI behaviors
  - Kill switches for problematic features
  - Beta paywall flag (controls trial activation)
  - Connector enable/disable per connector

---

## Observability & Monitoring

**Full observability stack:**

| Layer | Tool | Purpose |
|---|---|---|
| **Crash reporting** | Sentry | Server panic recovery and error tracking |
| **APM** | Datadog or AWS X-Ray | Request tracing, latency analysis, dependency mapping |
| **Structured logging** | zerolog or zap | JSON-structured logs with correlation IDs |
| **Metrics** | Prometheus + Grafana (or CloudWatch) | Request rates, error rates, AI latency, sync throughput, WebSocket connections |
| **Uptime monitoring** | AWS CloudWatch Alarms or Pingdom | Health check endpoints, alerting on downtime |
| **AI cost tracking** | Custom metrics | Per-user LLM token usage, cost per AI operation, throttling trigger rates |

**Key metrics monitored:**

- gRPC request latency (p50, p95, p99)
- AI operation latency by type
- LLM provider error rates and fallback frequency
- WebSocket connection count and message throughput
- Sync conflict rate and resolution outcomes
- Connector sync lag and failure rates
- MongoDB query performance
- S3 upload/download latency

---

## Deployment & Infrastructure

**AWS-hosted:**

| Component | AWS Service | Notes |
|---|---|---|
| **Compute** | ECS Fargate or EC2 | Go binary in Docker container, auto-scaling |
| **Database** | MongoDB Atlas (on AWS) | Managed MongoDB with encryption at rest, automated backups |
| **Object Storage** | S3 | User attachments with lifecycle policies |
| **Secrets** | AWS Secrets Manager | API keys, OAuth credentials, encryption keys |
| **KMS** | AWS KMS | MongoDB field-level encryption key management |
| **CDN** | CloudFront (optional) | For any static assets or frequently accessed attachments |
| **DNS** | Route 53 | Domain management and health checks |
| **Networking** | VPC + ALB | Private subnets for compute/database, ALB for gRPC termination |

**Environment tiers:**
- **Development** -- Local Docker Compose with MongoDB, mock connectors
- **Staging** -- AWS environment mirroring production at smaller scale
- **Production** -- Full AWS deployment with auto-scaling, monitoring, and alerting

---

## Testing Strategy

**Full testing pyramid:**

| Level | Tools | Scope |
|---|---|---|
| **Unit tests** | Go testing + testify | Individual functions, AI prompt parsing, business logic, data transformations |
| **Integration tests** | Go testing + testcontainers | MongoDB queries, S3 operations, gRPC endpoint tests with real database |
| **API tests** | grpcurl + custom test harness | Full gRPC API contract testing, auth flows, error handling |
| **Connector tests** | Mock servers + integration tests | Each connector's sync logic with mocked external APIs |
| **Load tests** | k6 or vegeta | gRPC throughput, WebSocket connection scaling, AI response latency under load |

---

## CI/CD Pipeline

**GitHub Actions:**

```
Push to branch
  -> Lint (golangci-lint)
  -> Unit tests
  -> Integration tests (testcontainers)
  -> Build Docker image
  -> Push to ECR (on main branch)
  -> Deploy to staging (automatic)
  -> Deploy to production (manual approval)
```

- **Branch protection:** Main branch requires passing CI and code review
- **Docker:** Multi-stage build for minimal production image
- **Staging deployment:** Automatic on merge to main
- **Production deployment:** Manual promotion from staging after validation
- **Rollback:** Instant rollback via ECS task definition revision

---

## Data Privacy & GDPR

| Requirement | Implementation |
|---|---|
| **Data export** | Full export of all user data (tasks, notes, attachments, analytics, knowledge base) as JSON/CSV via `UserService.ExportData` |
| **Account deletion** | `UserService.DeleteAccount` triggers a cascade deletion job: all MongoDB documents, S3 objects, connector tokens, and AI context permanently removed within 30 days |
| **Encryption at rest** | MongoDB Atlas encryption at rest + field-level encryption for sensitive fields |
| **Encryption in transit** | TLS 1.3 on all connections |
| **Data minimization** | Email content processed for task extraction but not stored permanently |
| **Retention policy** | Detailed data: 1 year. Aggregated data: indefinite. Automated purge job runs nightly. |
| **Consent** | Users explicitly grant access to each connector; permissions are revocable at any time |

---

## Repository Structure

```
zee6do-server/
  cmd/
    server/               # Main entry point, server bootstrap
  internal/
    auth/                 # Authentication module (OTP, social, JWT, device binding)
    tasks/                # Task CRUD, status management, search
    ai/                   # AI orchestration, LLM provider abstraction, prompt management
      providers/          # OpenAI, Anthropic, Google adapters
    scheduler/            # Smart scheduling engine, conflict resolution
    connectors/           # Connector integrations
      google_calendar/
      slack/
      email/
      clickup/
    knowledge/            # Personal knowledge base, embeddings, semantic search
    notifications/        # Push notification delivery, smart timing
    realtime/             # WebSocket server, event broadcasting
    analytics/            # Metrics computation, AI narrative generation
    storage/              # AWS S3 integration, pre-signed URLs
    billing/              # Receipt verification, subscription management
    sync/                 # Offline sync protocol, conflict resolution
    users/                # Profile management, data export, account deletion
    flags/                # Feature flag management
  pkg/
    middleware/           # Auth middleware, logging, rate limiting
    crypto/               # Encryption utilities, device fingerprint validation
    config/               # Configuration loading (env, files, secrets)
    errors/               # Error types and handling
  proto/
    zee6do/               # Protocol Buffer definitions for all gRPC services
  migrations/             # MongoDB index and schema migration scripts
  deployments/
    docker/               # Dockerfile, docker-compose for local dev
    k8s/ or ecs/          # Deployment manifests
  scripts/                # Build, test, deployment helper scripts
  docs/                   # API documentation, architecture decision records
  go.mod
  go.sum
  Makefile
  README.md
```

**Related repository:** [vorflux](https://github.com/DeepakDP5/vorflux) -- Expo React Native mobile client (iOS & Android).
