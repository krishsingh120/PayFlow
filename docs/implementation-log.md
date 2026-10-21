# Implementation Log

## Phase Status

| Phase | Description | Status | Commit |
|---|---|---|---|
| 0 | Repository inspection, architecture, plan | ✅ Done | — |
| 1 | Monorepo skeleton, Docker Compose, health endpoints | ✅ Done | `891b455` |
| 2 | Payment Service, PostgreSQL schema, state machine, tests | ✅ Done | `2316fc7` |
| 3 | API Gateway, React frontend | ✅ Done | `1698021` |
| 4 | RabbitMQ, Go worker, simulated provider | ✅ Done | `efcfde0` |
| 5 | Idempotency hardening, refunds, webhooks, Outbox | ✅ Done | `16b1f1b` |
| 6 | Double-entry ledger, reconciliation | ⏳ Pending | — |
| 7 | E2E, CI, security review, final cleanup | ⏳ Pending | — |

---

## Phase 0 — Repository Inspection & Plan

**Date:** 2026-10-20  
**Status:** ✅ Complete

### Findings
- Repository at `https://github.com/krishsingh120/PayFlow.git` was empty (no commits)
- Toolchain confirmed: Go 1.26.4, Node 20.19.0, npm 10.8.2, Docker 29.1.3, Docker Compose v2.40.3
- Git identity: Krish Singh / GitHub noreply address

### Deliverables
- Architecture document
- Payment state machine
- API contracts
- Database schema overview
- 8-phase implementation checklist

---

## Phase 1 — Monorepo Skeleton

**Date:** 2026-10-20  
**Status:** ✅ Complete

### Files Created

| File | Purpose |
|---|---|
| `.gitignore` | Go, Node, Docker, IDE artifact exclusions |
| `.env.example` | Safe configuration template |
| `README.md` | Project overview, quick start, architecture |
| `deployments/docker-compose.yml` | PostgreSQL 16, RabbitMQ 3, all services |
| `services/payment/go.mod` | Go module with chi, zerolog, godotenv, ulid |
| `services/payment/internal/config/config.go` | Centralised env-driven config |
| `services/payment/internal/http/router.go` | Chi router + middleware stack |
| `services/payment/internal/http/health.go` | `GET /v1/health` handler |
| `services/payment/cmd/api/main.go` | HTTP server with graceful shutdown |
| `services/payment/cmd/worker/main.go` | Worker entry point (loop wired Phase 4) |
| `services/payment/Dockerfile` | Multi-stage distroless image |
| `services/api-gateway/package.json` | Express, proxy, cors, rate-limit |
| `services/api-gateway/tsconfig.json` | TypeScript strict mode config |
| `services/api-gateway/src/index.ts` | Gateway: CORS, rate-limit, proxy |
| `services/api-gateway/Dockerfile` | Two-stage Node image |
| `docs/architecture.md` | System diagram and component table |
| `docs/payment-lifecycle.md` | State machine and transition table |
| `docs/design-decisions.md` | Key trade-offs and rationale |
| `docs/api-reference.md` | API contracts stub |
| `docs/implementation-log.md` | This file |
| `migrations/` | Directory created (migrations added Phase 2) |
| `scripts/migrate.sh` | Migration runner script |
| `.github/workflows/ci.yml` | CI pipeline |

### Verification
- `go mod tidy` — all dependencies resolved ✅
- `go build ./...` — both binaries compile ✅
- `npm install && npm run build` — TypeScript gateway builds ✅

### Commit
- **Hash:** `891b455`
- **Message:** `feat(skeleton): Phase 1 — monorepo skeleton, Docker Compose, health endpoints`
- **Push Status:** Pushed to `origin/main` ✅

---

## Phase 2 — Go Payment Service & Storage

**Date:** 2026-10-21  
**Status:** ✅ Complete

### Files Created & Modified

| File | Purpose |
|---|---|
| `migrations/000001_create_payments_table.up.sql` | Payments table schema, constraints, indexes |
| `migrations/000001_create_payments_table.down.sql` | Rollback payments table |
| `migrations/000002_create_refunds_table.up.sql` | Refunds table schema with over-refund trigger |
| `migrations/000002_create_refunds_table.down.sql` | Rollback refunds table and trigger |
| `services/payment/internal/payments/model.go` | Payment entity, status enum, state machine rules, validation |
| `services/payment/internal/payments/ulid.go` | Monotonic ULID generator with `pay_` / `ref_` prefix |
| `services/payment/internal/payments/service.go` | Business service: idempotency, state transitions, listing |
| `services/payment/internal/storage/repository.go` | Storage contracts and domain errors |
| `services/payment/internal/storage/postgres.go` | Production PostgreSQL repository using `pgx/v5` |
| `services/payment/internal/storage/memory.go` | Thread-safe in-memory repository for testing/standalone |
| `services/payment/internal/http/payments.go` | HTTP handlers for `POST /v1/payments`, `GET /v1/payments/{id}`, `GET /v1/payments` |
| `services/payment/internal/http/router.go` | Chi routing for payments endpoints |
| `services/payment/cmd/api/main.go` | Server startup wiring pool/repository/service |
| `services/payment/internal/payments/state_machine_test.go` | State machine transition & validation tests |
| `services/payment/internal/payments/service_test.go` | Payment service business logic tests |
| `services/payment/internal/storage/memory_test.go` | Storage repository tests |
| `services/payment/internal/http/payments_test.go` | HTTP integration tests using `httptest` |

### Verification
- `go test ./...` — 100% passed across all packages (`internal/http`, `internal/payments`, `internal/storage`) ✅
- `go build ./cmd/api; go build ./cmd/worker` — binaries build cleanly with zero warnings ✅
- `npm run build` in `services/api-gateway` — compiles cleanly ✅

---

## Phase 3 — React Frontend & Idempotency Lab

**Date:** 2026-10-21  
**Status:** ✅ Complete

### Files Created & Modified

| File | Purpose |
|---|---|
| `apps/web/package.json` | React 19 + TypeScript + Vite + Lucide icons |
| `apps/web/vite.config.ts` | Vite dev server + proxy configuration to API Gateway (`:3000`) |
| `apps/web/index.html` | Google Fonts (Outfit, JetBrains Mono), SEO tags, SVG icon |
| `apps/web/src/index.css` | Vanilla CSS design system: curated dark theme, emerald accents, glassmorphic panels |
| `apps/web/src/lib/api.ts` | Type-safe frontend client for payments, listing, and health checks |
| `apps/web/src/components/Header.tsx` | Brand header with live gateway/engine ping indicators and action triggers |
| `apps/web/src/components/MetricsGrid.tsx` | Stats cards for volume (₹), transaction counts, and success rates |
| `apps/web/src/components/TransactionList.tsx` | Live feed with status filtering, search, auto-polling (3s), inspect drawer |
| `apps/web/src/components/CreatePaymentModal.tsx` | Payment creation form with live paise conversion and UUID generator |
| `apps/web/src/components/IdempotencyTesterModal.tsx` | Interactive Pine Labs interview lab testing 201 Created, 200 Replay, and 409 Conflict |
| `apps/web/src/components/PaymentDetailsModal.tsx` | Deep transaction inspector with ULID breakdown and copyable raw JSON |
| `apps/web/src/App.tsx` | Main application assembly and state coordination |
| `apps/web/Dockerfile` | Container configuration for web preview |

### Verification
- `npm run build` in `apps/web` — built cleanly in 423ms (`dist/` generated with zero type errors) ✅
- `npm run build` in `services/api-gateway` — compiles cleanly ✅
- `go test ./...` in `services/payment` — 100% test pass rate across all packages ✅

---

## Phase 4 — RabbitMQ Integration, Go Worker, Simulated Provider

**Date:** 2026-10-21  
**Status:** ✅ Complete

### Files Created & Modified

| File | Purpose |
|---|---|
| `services/payment/internal/provider/provider.go` | SimulatedProvider: deterministic test rules (amount-based decline/timeout triggers) |
| `services/payment/internal/provider/provider_test.go` | Provider outcome rules tests |
| `services/payment/internal/messaging/broker.go` | Messaging interfaces + constants (queue names, routing keys, DLX) |
| `services/payment/internal/messaging/rabbitmq.go` | Production RabbitMQ broker: durable topic exchange, DLQ, manual ack/nack |
| `services/payment/internal/messaging/memory.go` | In-memory broker (dev/test fallback) |
| `services/payment/internal/messaging/memory_test.go` | Message delivery unit tests |
| `services/payment/internal/worker/processor.go` | PaymentProcessor: CREATED→PENDING→SUCCEEDED/FAILED/UNKNOWN transitions |
| `services/payment/internal/worker/processor_test.go` | Worker tests: success, decline, timeout, terminal idempotency, malformed payload |
| `services/payment/internal/payments/service.go` | Added EventPublisher interface + publish `payment.created` on create |
| `services/payment/cmd/api/main.go` | Wire RabbitMQ publisher (fallback to memory), publish on payment creation |
| `services/payment/cmd/worker/main.go` | Wire storage, provider, processor, and consumer for job processing loop |

### Architecture
- `POST /v1/payments` → Go API persists payment (CREATED) → publishes `payment.created` to RabbitMQ (or in-memory fallback)
- Worker consumes `payment.processing.queue` → PENDING → calls SimulatedProvider → resolves SUCCEEDED / FAILED / UNKNOWN
- Failed messages routed to DLX / DLQ (no data loss)
- Worker runs standalone without RabbitMQ/Postgres (memory fallback for local dev)

### Verification
- `go test ./...` — 100% pass (7 packages: messaging, provider, worker + all previous) ✅
- `go build ./cmd/api; go build ./cmd/worker` — builds cleanly ✅

---

## Phase 5 — Idempotency Hardening, Refunds, Webhooks, Transactional Outbox

**Date:** 2026-10-21  
**Status:** ✅ Complete

### Files Created & Modified

| File | Purpose |
|---|---|
| `migrations/000003_create_outbox_events_table.up.sql` | Transactional outbox table for reliable event dispatch |
| `migrations/000003_create_outbox_events_table.down.sql` | Revert outbox_events table |
| `migrations/000004_create_webhook_deliveries_table.up.sql` | Webhook delivery log and retry schedule table |
| `migrations/000004_create_webhook_deliveries_table.down.sql` | Revert webhook_deliveries table |
| `services/payment/internal/payments/refund.go` | Refund model, validation, and status definitions |
| `services/payment/internal/payments/service.go` | Added `CreateRefund`, `GetRefund`, `ListRefunds`, and hardened deep JSON `isPayloadEquivalent` |
| `services/payment/internal/payments/service_test.go` | Comprehensive refund tests (partial refunds, over-refund prevention, replay conflict) |
| `services/payment/internal/storage/repository.go` | Added `RefundRepository` interface |
| `services/payment/internal/storage/memory.go` | In-memory `RefundRepository` with concurrency locks and amount checks |
| `services/payment/internal/storage/postgres.go` | PostgreSQL `RefundRepository` with `SELECT ... FOR UPDATE` row locking |
| `services/payment/internal/outbox/model.go` | Outbox event domain entity |
| `services/payment/internal/outbox/repository.go` | Outbox repository interface |
| `services/payment/internal/outbox/memory.go` | Thread-safe in-memory outbox repository |
| `services/payment/internal/outbox/postgres.go` | PostgreSQL outbox repository |
| `services/payment/internal/outbox/relay.go` | Background Outbox Relay worker for at-least-once publishing |
| `services/payment/internal/outbox/relay_test.go` | Relay unit tests (batching, recovery on transient failure) |
| `services/payment/internal/webhooks/model.go` | Webhook delivery models and `whd_` ULID generator |
| `services/payment/internal/webhooks/signer.go` | HMAC-SHA256 signature generator and timing-safe validator with timestamp tolerance |
| `services/payment/internal/webhooks/repository.go` | Webhook repository contract |
| `services/payment/internal/webhooks/memory.go` | In-memory webhooks repository |
| `services/payment/internal/webhooks/postgres.go` | PostgreSQL webhooks repository |
| `services/payment/internal/webhooks/dispatcher.go` | Webhook dispatcher with exponential backoff retries |
| `services/payment/internal/webhooks/dispatcher_test.go` | Signer and dispatcher unit tests (replay prevention, retry escalation) |
| `services/payment/internal/http/payments.go` | `CreateRefund`, `ListRefunds`, `WebhookTestReceiver` handlers + `Idempotent-Replay` header |
| `services/payment/internal/http/payments_test.go` | HTTP tests for refunds, replay headers, and webhook receiver |
| `services/payment/internal/http/router.go` | Route wiring for `/v1/payments/{id}/refunds` and `/v1/webhooks/test-receiver` |
| `services/payment/internal/worker/processor.go` | Optional Outbox and Webhooks integration on payment resolution |
| `apps/web/src/lib/api.ts` | Added `Refund` interface, `createRefund`, and `listRefunds` API client |
| `apps/web/src/components/PaymentDetailsModal.tsx` | Added Refund Management UI, balance tracker, and refund history |
| `apps/web/src/App.tsx` | Wired live refresh on refund submission |

### Verification
- `go test ./...` in `services/payment` — 100% pass across all 11 packages ✅
- `go build ./cmd/api; go build ./cmd/worker` — binaries build cleanly ✅
- `npm run build` in `apps/web` — builds cleanly with zero TypeScript errors ✅
- `npm run build` in `services/api-gateway` — builds cleanly ✅

