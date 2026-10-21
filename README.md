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
| 2 | Payment Service, PostgreSQL schema, state machine, tests | ⏳ |
| 3 | API Gateway, React frontend | ⏳ |
| 4 | RabbitMQ, Go worker, simulated provider | ⏳ |
| 5 | Idempotency hardening, refunds, webhooks, Outbox | ⏳ |
| 6 | Double-entry ledger, reconciliation | ⏳ |
| 7 | E2E, CI, security review | ⏳ |

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

---

## Development Commands

```bash
# Go service
cd services/payment
go build ./...
go test ./...

# Node gateway
cd services/api-gateway
npm install && npm run dev

# Frontend
cd apps/web
npm install && npm run dev

# Docker compose
docker compose -f deployments/docker-compose.yml up -d
docker compose -f deployments/docker-compose.yml logs -f
docker compose -f deployments/docker-compose.yml down
```

---

## License

MIT
