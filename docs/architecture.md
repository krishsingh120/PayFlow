# Architecture

## System Diagram

```
 ┌─────────────────────────────────────────────────────────────────┐
 │                        Client Browser                          │
 │                    React + Vite + TypeScript                    │
 │                          :5173                                  │
 └──────────────────────────────┬──────────────────────────────────┘
                                │ HTTP
                                ▼
 ┌─────────────────────────────────────────────────────────────────┐
 │                        API Gateway                              │
 │               Node.js · Express · TypeScript                    │
 │                          :3000                                  │
 │  • CORS          • Rate limiting        • Request logging       │
 │  • Transparent proxy → /api/* → Payment Service                 │
 └──────────────────────────────┬──────────────────────────────────┘
                                │ HTTP (internal)
                                ▼
 ┌─────────────────────────────────────────────────────────────────┐
 │                      Payment Service                            │
 │                      Go · Chi router                            │
 │                          :8080                                  │
 │  • Payment creation & idempotency                               │
 │  • State machine enforcement                                    │
 │  • Refund processing                                            │
 │  • Webhook delivery                                             │
 │  • Transactional Outbox relay                                   │
 └──────────┬───────────────────────────────────┬─────────────────┘
            │                                   │
            ▼                                   ▼
 ┌──────────────────────┐           ┌───────────────────────┐
 │     PostgreSQL 16     │           │      RabbitMQ 3        │
 │                      │           │                        │
 │  payments            │           │  payflow.payments      │
 │  refunds             │◄──────────│  (job queue)           │
 │  outbox_events       │           │                        │
 │  webhook_deliveries  │           │  payflow.payments.dlq  │
 │  ledger_entries      │           │  (dead-letter queue)   │
 └──────────────────────┘           └──────────┬─────────────┘
                                               │ AMQP consume
                                               ▼
 ┌─────────────────────────────────────────────────────────────────┐
 │                      Payment Worker                             │
 │              Go · same codebase, separate binary                │
 │                                                                 │
 │  • Consumes jobs from RabbitMQ                                  │
 │  • Calls simulated payment provider                             │
 │  • Transitions payment state (PENDING → SUCCEEDED/FAILED)       │
 │  • Bounded retries + dead-letter handling                       │
 └─────────────────────────────────────────────────────────────────┘
```

## Component Responsibilities

### React Frontend (`apps/web/`)
- Create payments via form
- Display transaction list with live status polling
- Initiate refunds
- Show payment details and refund history

### API Gateway (`services/api-gateway/`)
- CORS headers for browser clients
- Rate limiting (200 req/min per IP)
- HTTP access logging
- Transparent proxy: strips `/api` prefix, forwards to Payment Service
- **Does NOT contain payment business logic**

### Payment Service (`services/payment/`)
- All payment domain logic
- Idempotent payment creation
- State machine transitions
- Refund processing with over-refund protection
- Webhook delivery with HMAC signing
- Transactional Outbox Pattern relay
- Double-entry ledger entries

### Payment Worker (`services/payment/cmd/worker/`)
- Separate process, same Go codebase
- Consumes from `payflow.payments` RabbitMQ queue
- Calls simulated provider with configurable outcomes
- Handles retries, exponential backoff, dead-letter

### PostgreSQL
- Primary data store for all payment and financial data
- Enforces critical invariants via CHECK constraints and triggers
- Uses BIGINT for all monetary amounts (paise)

### RabbitMQ
- Decouples HTTP request handling from asynchronous payment processing
- Dead-letter queue for exhausted retries

## Key Design Principles

1. **Money is always BIGINT paise** — no floating-point arithmetic anywhere
2. **Idempotency via database UNIQUE constraint** — not application memory
3. **State machine enforced by CHECK constraint + application logic**
4. **Thin gateway** — payment logic lives exclusively in Go
5. **Transactional Outbox** — guarantees broker delivery without distributed transactions
6. **`SELECT FOR UPDATE`** on concurrent refund requests — DB-level race prevention
