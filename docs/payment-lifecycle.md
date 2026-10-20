# Payment Lifecycle

## State Machine

```
                    ┌──────────┐
                    │ CREATED  │  ← payment record written to DB
                    └────┬─────┘    job published to RabbitMQ
                         │
                         │  Worker dequeues job
                         ▼
                    ┌──────────┐
                    │ PENDING  │  ← worker calls simulated provider
                    └────┬─────┘
               ┌─────────┼──────────┐
               │         │          │
               ▼         ▼          ▼
         ┌─────────┐ ┌────────┐ ┌──────────┐
         │SUCCEEDED│ │ FAILED │ │ UNKNOWN  │  ← provider timed out
         └─────────┘ └────────┘ └──────────┘
               │                    │
               │                    │  Reconciliation job queries provider
               │                    │  for true outcome
               │                    ▼
               │          SUCCEEDED or FAILED
               │
               └──→ Refund possible (separate record, state PENDING→SUCCEEDED/FAILED)
```

## States

| State | Description | Terminal? |
|---|---|---|
| `CREATED` | Payment record written, job enqueued | No |
| `PENDING` | Worker picked up job, provider call in flight | No |
| `SUCCEEDED` | Provider confirmed payment succeeded | **Yes** |
| `FAILED` | Provider declined or hard-failed | **Yes** |
| `UNKNOWN` | Provider timed out — outcome genuinely uncertain | No |

## Permitted Transitions

| From | To | Trigger | Notes |
|---|---|---|---|
| `CREATED` | `PENDING` | Worker dequeues job | |
| `PENDING` | `SUCCEEDED` | Provider returns success | Ledger entries written in same transaction |
| `PENDING` | `FAILED` | Provider returns decline | failure_reason recorded |
| `PENDING` | `UNKNOWN` | Provider times out | Do NOT mark as FAILED |
| `UNKNOWN` | `SUCCEEDED` | Reconciliation confirms | |
| `UNKNOWN` | `FAILED` | Reconciliation confirms | |

**Invalid transitions are rejected at the application level and enforced by the DB CHECK constraint.**

## Why UNKNOWN Matters

A timeout does not mean the payment failed. The provider may have received the
request and processed it. Marking a timed-out payment as FAILED risks:
- Double-charging the customer on retry
- Failing to settle a payment that actually succeeded

PayFlow keeps timed-out payments in `UNKNOWN` state and resolves them via a
periodic reconciliation job that queries the provider for the authoritative outcome.

## Refund Lifecycle

Refunds are separate records and do not change the parent payment's status.

```
REFUND_PENDING → REFUND_SUCCEEDED | REFUND_FAILED
```

Protection against over-refunds is enforced at the database level using
`SELECT FOR UPDATE` on the payment row combined with a trigger that prevents
`SUM(refund amounts) > payment.amount`.
