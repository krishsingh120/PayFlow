# Design Decisions

This document explains important architectural choices and the trade-offs behind them.
These are the answers you should give in a Pine Labs SDE interview.

---

## 1. ULIDs for Payment IDs

**Decision:** Use ULIDs (`pay_01J...`) instead of UUID v4 or auto-increment integers.

**Rationale:**
- ULIDs are monotonically sortable by creation time — `ORDER BY id` gives chronological results with no extra `created_at` index needed on small result sets.
- They are URL-safe (Crockford Base32) and have no hyphens, making them cleaner in API responses.
- Unlike sequential integers, they reveal no information about volume or insertion order.
- Unlike UUID v4, they don't require a random number generator to produce unique IDs at high throughput.

**Trade-off:** Slightly larger than integer IDs (26 chars vs ~10 chars). Negligible at our scale.

---

## 2. BIGINT Paise for Money

**Decision:** Store all monetary amounts as `BIGINT` representing paise (1 INR = 100 paise). Never use `FLOAT`, `DOUBLE`, or `DECIMAL`.

**Rationale:**
- Floating-point arithmetic is non-associative: `0.1 + 0.2 ≠ 0.3` in IEEE 754.
- Payment systems that use floats have caused real financial losses (rounding errors across millions of transactions compound).
- Integer arithmetic is exact, deterministic, and fast.
- `DECIMAL` is safe but slower than `BIGINT` and more verbose in Go.

**Trade-off:** All display formatting happens in the presentation layer. The API always returns paise; the frontend converts to rupees.

---

## 3. Chi Router (not Gin, not standard `net/http`)

**Decision:** Use `go-chi/chi` as the HTTP router.

**Rationale:**
- Chi is built on top of `net/http` — zero interface surprise, handlers are `http.HandlerFunc`.
- Middleware is composable using standard `http.Handler` chains.
- No magic: no struct tags for routing, no global state.
- Easy to explain line-by-line in an interview.

**Why not Gin?** Gin uses its own context type (`*gin.Context`), coupling all handlers to the framework. Chi keeps handlers idiomatic.

---

## 4. pgx/v5 Directly (not GORM or sqlx)

**Decision:** Use `jackc/pgx/v5` directly for all database access.

**Rationale:**
- Writing SQL explicitly demonstrates SQL knowledge to interviewers.
- GORM hides the SQL, making N+1 queries and missing indexes invisible.
- `pgx` gives direct access to PostgreSQL-specific features: advisory locks, LISTEN/NOTIFY, `RETURNING` clauses, and batch queries.
- Errors are explicit and typed — no silent query failures.

**Trade-off:** More boilerplate than an ORM. Acceptable for an interview showcase project.

---

## 5. Transactional Outbox Pattern

**Decision:** Write `outbox_events` rows in the same database transaction as the payment record, then relay them to RabbitMQ asynchronously.

**Rationale:**
- Without the outbox, we face a distributed transaction problem: we write to PostgreSQL *and* publish to RabbitMQ. If the app crashes between the two, the event is lost.
- The outbox guarantees at-least-once delivery without 2PC (Two-Phase Commit).
- The relay process reads unpublished outbox rows and publishes them; idempotency keys on the consumer side handle duplicates.

**Why not just publish directly to RabbitMQ in the HTTP handler?**  
Because the HTTP handler might succeed at writing to the DB but fail at publishing (network blip, broker restart). The payment would be in `CREATED` status with no job to process it.

---

## 6. SELECT FOR UPDATE on Refunds

**Decision:** When processing a refund request, acquire a row-level lock on the parent payment row using `SELECT ... FOR UPDATE`.

**Rationale:**
- Two concurrent refund requests for the same payment could both pass the over-refund check (they each see the same refunded total before the other commits).
- `SELECT FOR UPDATE` serialises concurrent access at the database level — no application-level mutex needed.
- This is a textbook solution to a classic concurrency interview question.

---

## 7. Thin API Gateway

**Decision:** The Node.js gateway does not contain payment logic. It only handles CORS, rate limiting, and proxying.

**Rationale:**
- Business logic should live in one place. If it existed in both Go and Node, they would drift.
- The gateway can be swapped, scaled independently, or replaced with nginx without touching business logic.
- Keeps Node.js easy to reason about: it's just an HTTP proxy with middleware.

---

## 8. Simulated Provider (No Real Cards)

**Decision:** Implement a fake payment provider that returns configurable outcomes (success, failure, pending, timeout) based on environment variables.

**Rationale:**
- No PCI DSS scope, no real money, no test credentials to manage.
- Configurable outcomes let us test every code path (including timeouts and UNKNOWN states) deterministically.
- Production integration with a real provider (Razorpay, Stripe) would require replacing only the `provider` package.

---

## 9. golang-migrate for Migrations

**Decision:** Use `golang-migrate` with numbered SQL files (`001_...up.sql`, `001_...down.sql`).

**Rationale:**
- File-based migrations are version-controlled alongside the code.
- Reversible migrations (`down.sql`) allow safe rollbacks.
- `golang-migrate` is well-maintained and CI-friendly.
- The migration state is tracked in a `schema_migrations` table.
