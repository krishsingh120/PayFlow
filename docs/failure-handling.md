# Failure Handling

## 1. Idempotency

### Problem
A client may send the same payment request multiple times (network timeout, retry).

### Solution
Every payment request requires an `Idempotency-Key` header (UUID).
The key is stored in a `UNIQUE` column on the `payments` table.

| Scenario | Behaviour |
|---|---|
| First request | Create and return `201 Created` |
| Retry with same key + same payload | Return existing payment with `200 OK` |
| Retry with same key + different payload | Return `409 Conflict` |

The payload hash is stored alongside the idempotency key to detect payload mismatches.

---

## 2. Provider Timeouts and UNKNOWN State

### Problem
A payment provider call may time out. We do not know if the provider received and processed the request.

### Wrong approach
Marking a timed-out payment as `FAILED` risks double-charging or failing to settle.

### Solution
Timed-out payments are moved to `UNKNOWN` status. A reconciliation job periodically
queries the provider for the authoritative outcome and transitions to `SUCCEEDED` or `FAILED`.

```
PENDING → (provider timeout) → UNKNOWN → (reconciliation) → SUCCEEDED | FAILED
```

---

## 3. RabbitMQ Message Processing

### At-least-once delivery
- RabbitMQ may redeliver messages (broker restart, consumer crash, nack).
- The worker processes each message idempotently: it checks the current payment status
  before calling the provider. If the payment is already `PENDING` or beyond, it acks the message.

### Dead-letter handling
- Max delivery attempts: 3 (configurable via `x-max-retries` header).
- After exhaustion, the message is routed to `payflow.payments.dlq`.
- A separate DLQ monitor logs and alerts on dead-lettered messages.

### Duplicate delivery protection
- The worker uses the payment ID as a natural idempotency key.
- `UPDATE payments SET status='PENDING' WHERE id=? AND status='CREATED'` is atomic.
- If the status is already beyond `CREATED`, the worker acks without re-processing.

---

## 4. Over-Refund Prevention

### Problem
Two concurrent refund requests for the same payment may both pass the refund balance check
before either commits (TOCTOU race condition).

### Solution
1. `SELECT payments WHERE id=? FOR UPDATE` — acquires a row-level exclusive lock.
2. All other refund requests for the same payment block until the lock is released.
3. A database trigger additionally checks `SUM(refunds.amount) <= payments.amount`.

---

## 5. Transactional Outbox

### Problem
Publishing to RabbitMQ in the same HTTP handler as the DB write creates a failure window:
if the app crashes after the DB write but before the publish, the job is lost silently.

### Solution
The Transactional Outbox Pattern:
1. Write `outbox_events` row in the **same database transaction** as the payment record.
2. A relay goroutine polls `outbox_events WHERE published = FALSE`.
3. The relay publishes to RabbitMQ and marks the row `published = TRUE`.

At-least-once delivery is guaranteed. Consumer idempotency handles duplicates.

---

## 6. Webhook Delivery

### Problem
Webhook HTTP calls to external URLs may fail (network error, endpoint down).

### Solution
- Webhook deliveries are persisted in `webhook_deliveries`.
- Failed deliveries are retried with exponential backoff (delays: 1m, 5m, 30m, 2h, 8h).
- All webhook payloads are signed with HMAC-SHA256 using `WEBHOOK_SIGNING_SECRET`.
- Consumers should verify the `X-PayFlow-Signature` header before trusting the payload.

---

## 7. Database Unavailability

### HTTP API behaviour
- Handlers return `503 Service Unavailable` when the DB pool is exhausted or unreachable.
- The health endpoint distinguishes between liveness (process running) and readiness (DB reachable).

### Worker behaviour
- The worker uses exponential backoff with jitter when the DB is unreachable.
- Inflight RabbitMQ messages are nacked (not acked) so they are requeued.

---

## 8. Graceful Shutdown

Both the API and worker handle `SIGINT`/`SIGTERM`:

```
SIGTERM received
   → Stop accepting new HTTP requests (API) / Stop consuming new messages (Worker)
   → Wait up to 20s for in-flight requests to complete (API)
   → Wait up to 30s for in-flight jobs to complete (Worker)
   → Close DB pool and RabbitMQ connection
   → Exit 0
```
