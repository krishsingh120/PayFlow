# PayFlow

> A payment processing and transaction management platform built to demonstrate backend engineering fundamentals — payment state machines, idempotency, async processing, failure handling, and double-entry bookkeeping.

[![CI](https://github.com/krishsingh120/PayFlow/actions/workflows/ci.yml/badge.svg)](https://github.com/krishsingh120/PayFlow/actions/workflows/ci.yml)

---

## Architecture Overview

### High-Level Design (HLD)

![PayFlow HLD Diagram](https://github.com/krishsingh120/ExecHub-HLD-Image/blob/main/PayFlow-HLD-Diagram.png?raw=true)

```
  Browser (React + Vite)
        |
  API Gateway  (Node.js · Express · TypeScript · port 3000)
        |
  Payment Service  (Go · Chi · port 8080)
       / \
  PostgreSQL   RabbitMQ
                  |
           Payment Worker  (Go — same codebase, different binary)
```

| Layer | Technology |
|---|---|
| Frontend | React, Vite, TypeScript |
| API Gateway | Node.js, Express, TypeScript |
| Payment Service | Go, Chi router |
| Background Worker | Go (shares Payment Service code) |
| Database | PostgreSQL 16 |
| Message Broker | RabbitMQ 3 |
| Infrastructure | Docker Compose |

---

## Quick Start

### Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) ≥ 24
- Go ≥ 1.21
- Node.js ≥ 20

### 1. Clone and configure

```bash
git clone https://github.com/krishsingh120/PayFlow.git
cd PayFlow
cp .env.example .env
# Edit .env if needed (defaults work with docker compose)
```

### 2. Start infrastructure + services

```bash
docker compose -f deployments/docker-compose.yml up -d
```

### 3. Run database migrations

```bash
bash scripts/migrate.sh up
```

### 4. Access the app

| Service | URL |
|---|---|
| React frontend | http://localhost:5173 |
| API Gateway | http://localhost:3000 |
| Payment Service | http://localhost:8080 |
| RabbitMQ Management | http://localhost:15672 (guest/guest) |

---

## Project Structure

```
PayFlow/
├── apps/web/                  # React + Vite + TypeScript frontend
├── services/
│   ├── api-gateway/           # Node.js thin proxy (CORS, rate-limit)
│   └── payment/               # Go payment service + worker
│       ├── cmd/api/           # HTTP server entry point
│       ├── cmd/worker/        # Background job worker entry point
│       └── internal/          # All business logic
├── migrations/                # SQL migrations (golang-migrate format)
├── deployments/               # Docker Compose
├── docs/                      # Architecture, design decisions, API reference
├── scripts/                   # Migration helpers
└── .github/workflows/         # CI pipeline
```

---

## API Reference

See [`docs/api-reference.md`](docs/api-reference.md) for full endpoint documentation.

### Core Endpoints

```
POST   /v1/payments                    Create payment
GET    /v1/payments/:id                Get payment status
GET    /v1/payments                    List payments (paginated)
POST   /v1/payments/:id/refunds        Initiate refund
GET    /v1/payments/:id/refunds        List refunds
GET    /v1/health                      Service health
```

### Create a Payment

```bash
curl -X POST http://localhost:3000/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $(uuidgen)" \
  -d '{"amount": 100000, "currency": "INR", "description": "Order #42"}'
```

---

## Implementation Phases

| Phase | Description | Status |
|---|---|---|
| 0 | Repository inspection, architecture, plan | ✅ Done |
| 1 | Monorepo skeleton, Docker Compose, health endpoints | ✅ Done |
| 2 | Payment Service, PostgreSQL schema, state machine, tests | ✅ Done |
| 3 | API Gateway, React frontend | ✅ Done |
| 4 | RabbitMQ, Go worker, simulated provider | ✅ Done |
| 5 | Idempotency hardening, refunds, webhooks, Outbox | ✅ Done |
| 6 | Double-entry ledger, automated reconciliation | ✅ Done |
| 7 | E2E verification, CI workflow, security review | ✅ Done |

---

## Documentation

| Document | Purpose |
|---|---|
| [Architecture](docs/architecture.md) | Component diagram and responsibilities |
| [Payment Lifecycle](docs/payment-lifecycle.md) | State machine and transitions |
| [API Reference](docs/api-reference.md) | Endpoints and examples |
| [Design Decisions](docs/design-decisions.md) | Key trade-offs |
| [Database Design](docs/database-design.md) | Schema and constraints |
| [Failure Handling](docs/failure-handling.md) | Retries, timeouts, idempotency |
| [Go Concurrency](docs/go-concurrency.md) | Patterns used in the codebase |
| [Implementation Log](docs/implementation-log.md) | Phase status and commits |
| [High-Level Design](docs/payflow-hld.md) | HLD with architecture diagram |

---

## API Reference

See [`docs/api-reference.md`](docs/api-reference.md) for full endpoint documentation.

### All Endpoints

```
POST   /v1/payments                        Create payment (idempotent)
GET    /v1/payments/:id                    Get payment status
GET    /v1/payments                        List payments (paginated, filterable)
POST   /v1/payments/:id/refunds            Initiate partial/full refund (idempotent)
GET    /v1/payments/:id/refunds            List refunds for payment
GET    /v1/ledger/payments/:id             Double-entry journal for payment
GET    /v1/ledger/accounts                 Account balances (receivable, revenue, etc.)
GET    /v1/ledger/entries                  All ledger entries
POST   /v1/reconciliation/run             Trigger automated reconciliation audit
GET    /v1/reconciliation/reports/latest  Latest reconciliation report
POST   /v1/webhooks/test-receiver         HMAC-SHA256 webhook test endpoint
GET    /v1/health                          Service health
```

### Create a Payment

```bash
curl -X POST http://localhost:3000/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $(uuidgen)" \
  -d '{"amount": 100000, "currency": "INR", "description": "Order #42"}'
```

### Run Reconciliation Audit

```bash
curl -X POST http://localhost:3000/api/v1/reconciliation/run
curl http://localhost:3000/api/v1/reconciliation/reports/latest
```

---

## E2E Verification

Run the complete invariants verification suite locally:

```powershell
# Windows (PowerShell)
powershell -ExecutionPolicy Bypass -File scripts\verify_e2e.ps1

# Linux / macOS
bash scripts/verify_e2e.sh
```

This verifies:
1. **Idempotency** — exactly-once guarantees, conflict detection
2. **State machine** — CREATED → PENDING → SUCCEEDED
3. **Double-entry ledger** — Sum(DEBIT) == Sum(CREDIT) at all times
4. **Refund engine** — partial/full refunds with over-refund protection
5. **Webhook security** — HMAC-SHA256 signing, constant-time verify, tamper rejection
6. **Reconciliation** — UNKNOWN state resolution, ledger self-healing

---

## Development Commands

```bash
# Go service — test and build
cd services/payment
go test ./...
go run ./cmd/e2e          # Full invariants verification
go build ./cmd/api
go build ./cmd/worker

# Node gateway
cd services/api-gateway
npm install && npm run dev
npm test

# Frontend
cd apps/web
npm install && npm run dev
npm run build

# Database migrations
bash scripts/migrate.sh up

# Docker compose
docker compose -f deployments/docker-compose.yml up -d
docker compose -f deployments/docker-compose.yml logs -f
docker compose -f deployments/docker-compose.yml down
```

---

## License

MIT
