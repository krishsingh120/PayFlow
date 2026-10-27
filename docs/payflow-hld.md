# PayFlow — High-Level Design (HLD)

## 1. Overview

PayFlow is a payment-processing and transaction-management simulation platform built to demonstrate backend engineering fundamentals, including payment state transitions, idempotency, asynchronous processing, failure handling, and double-entry bookkeeping.

This document describes the proposed High-Level Design (HLD): the major components, their responsibilities, and the flow of a payment through the system.

> **Implementation note:** This document describes the target architecture. Treat a component or pattern as implemented only after verifying it in the codebase.

## 2. Architecture Diagram

![PayFlow HLD Diagram](https://github.com/krishsingh120/ExecHub-HLD-Image/blob/main/PayFlow-HLD-Diagram.png?raw=true)

### Components

| Component | Technology | Responsibility |
|---|---|---|
| Frontend | React, Vite, TypeScript | Lets users create payments and view their status and history |
| API Gateway | Node.js, Express, TypeScript | Entry point for client requests; handles routing and shared request-level concerns |
| Payment Service | Go, Chi | Validates payment requests and owns payment business logic and state transitions |
| Database | PostgreSQL 16 | Persists payment data and related records |
| Message broker | RabbitMQ 3 | Carries background payment-processing jobs |
| Background worker | Go; shares the Payment Service codebase | Consumes jobs, calls the simulated provider, and records outcomes |
| Mock payment provider | Simulated component | Returns configured test outcomes without moving real money |
| Outbox publisher | Component in the payment backend | Publishes committed outbox events to RabbitMQ |

The Go HTTP API and the Go worker are separate processes built from the same service codebase. They have different entry points and responsibilities.

## 3. Numbered Request Flow

### Step 1 — Client → API Gateway

The user submits a payment, for example ₹500, through the React frontend. The frontend sends an HTTP request such as `POST /api/v1/payments` to the Node.js API Gateway.

The gateway receives the request and forwards it to the appropriate backend service.

### Step 2 — API Gateway → Go Payment API

The gateway forwards the request to the Go Payment API. The payment service validates the amount, currency, request format, and other business rules.

Authentication and authorization must be enforced appropriately; the payment service should not blindly trust a request merely because it came through the gateway.

### Step 3 — Go Payment API → PostgreSQL

The payment service creates a payment with an initial status such as `PENDING`.

In the target design, one PostgreSQL transaction writes:
- The payment record.
- An outbox event indicating that the payment needs processing.

Both records commit together. This avoids committing a payment while failing to record the corresponding event that needs to be published.

Amounts should be stored as integer minor units—for INR, paise—rather than floating-point values. For example, ₹500.00 is stored as `50000` paise.

### Step 4 — PostgreSQL → Outbox Publisher

The outbox publisher reads events that have not yet been marked as published. It attempts to publish each event to RabbitMQ.

The publisher must handle temporary broker failures and safely retry publishing. A crash after publishing but before recording that publication succeeded can result in a duplicate event, so downstream processing must be idempotent.

### Step 5 — Outbox Publisher → RabbitMQ

The publisher sends the payment-processing event to RabbitMQ. The queue separates the initial HTTP request from background processing, allowing the worker to process the job independently.

A successful publish does not mean the payment itself succeeded; it only means the processing event was sent to the broker.

### Step 6 — RabbitMQ → Go Worker

The Go worker consumes the payment job from RabbitMQ. It runs as a separate background process, so the initial HTTP request does not need to wait for provider processing to finish.

The worker should acknowledge a message only after it has handled the job safely. Redelivery and duplicate delivery must be expected.

### Step 7 — Go Worker → Mock Payment Provider

The worker invokes the mock payment provider. The provider simulates an outcome such as:
- `SUCCESS`: the simulated provider approves the payment.
- `FAILURE`: the simulated provider definitively declines the payment.
- `TIMEOUT`: the result is unknown; the provider may have processed the request even though the response was not received.

This project simulates payment processing and does not transfer real money. Test scenarios must be controlled and validated by the backend rather than trusted directly from an untrusted client.

### Step 8 — Go Worker → PostgreSQL

After obtaining a definitive outcome, the worker updates the payment status in PostgreSQL:
- Approved payment → `SUCCEEDED`
- Definitively declined payment → `FAILED`

A timeout must not automatically be treated as a definitive failure. The system may need to keep the payment in a pending/unknown state and reconcile the outcome using a stable provider reference or a provider-supported idempotency mechanism.

The database update and message acknowledgement sequence must be designed for redelivery. If the worker crashes after the provider processes a payment but before the database update or acknowledgement, duplicate processing is possible. Database deduplication alone cannot prevent duplicate external effects if the provider offers neither idempotency nor status lookup.

### Step 9 — Client → Gateway → Payment API: Check Status

The frontend requests the current payment status through an endpoint such as `GET /api/v1/payments/:id`. The backend returns the latest persisted status.

The frontend can periodically repeat this request. This approach is called **polling**. While processing is ongoing, the status may remain `PENDING`.

## 4. Why Polling Instead of WebSockets?

The initial PayFlow version uses polling because it is straightforward to implement and sufficient for a project dashboard.

- **Polling:** The client periodically requests the latest status.
- **WebSockets:** The server can push updates over a persistent connection.

WebSockets may be useful when low-latency updates are important. However, they add connection lifecycle management, reconnection behavior, and coordination across multiple server instances. They can be considered a later enhancement rather than a requirement for the initial version.

## 5. Key Design Decisions

### Asynchronous payment processing

The API can persist a payment as `PENDING` and return without waiting for the simulated provider. A background worker handles processing. This separates request handling from potentially slow work.

### PostgreSQL for persistent state

PostgreSQL provides relational constraints and transactions that are useful for payment, refund, and ledger data. Database constraints and explicit state transitions help protect correctness.

### RabbitMQ for background jobs

RabbitMQ buffers processing jobs and decouples producers from consumers. It does not, by itself, guarantee that a payment is processed exactly once or that every payment succeeds.

### Transactional Outbox

The outbox pattern coordinates the payment database write with event publication. The payment and outbox event are committed in one database transaction; a separate publisher then sends the event to RabbitMQ. Duplicate publication remains possible, so consumers need idempotent behavior.

### Idempotency

Payment-creation requests should support an idempotency key so that a retry of the same logical request does not create a second payment. The system should reject reuse of the same key with a different request payload.

### Mock provider

A simulated provider allows success, definitive decline, and uncertain timeout scenarios to be tested without real payment processing.

## 6. Interview Explanation

> PayFlow is a payment-processing simulation platform designed around asynchronous processing. The React frontend sends requests through a Node.js API Gateway to a Go Payment API. The API validates the request and stores the payment in PostgreSQL with a `PENDING` status. In the target design, it also creates an outbox event in the same database transaction. An outbox publisher sends the event to RabbitMQ, where a Go worker consumes the payment job and invokes a mock payment provider. The worker then persists the result in PostgreSQL, and the client retrieves the latest status through a status API. The design also accounts for idempotency, duplicate message delivery, and uncertain provider outcomes.

## 7. Important Caveats

- `PENDING` means processing has not reached a confirmed terminal outcome; it does not mean the payment failed.
- A RabbitMQ publish or message acknowledgement is not proof of payment success.
- Retries should target transient technical failures, not definitive business declines.
- Exactly-once delivery should not be assumed. Design for at-least-once delivery and idempotent handling.
- Timeouts can leave the provider outcome uncertain; reconciliation or provider status lookup may be necessary.
- Only describe the outbox, refunds, ledger, reconciliation, or other features as implemented after verifying their code and tests.

## 8. Related Documentation

- [Payment Lifecycle](payment-lifecycle.md)
- [API Reference](api-reference.md)
- [Design Decisions](design-decisions.md)
- [Database Design](database-design.md)
- [Failure Handling](failure-handling.md)
- [Go Concurrency](go-concurrency.md)
- [Implementation Log](implementation-log.md)
