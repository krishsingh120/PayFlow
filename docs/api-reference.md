# API Reference

Base URL: `http://localhost:3000/api` (via gateway) or `http://localhost:8080` (direct)

All endpoints return `Content-Type: application/json`.
All timestamps are RFC 3339 / ISO 8601 UTC.
All monetary amounts are **integers representing paise** (1 INR = 100 paise).

---

## Health

### GET /v1/health

Returns service health status.

**Response 200**
```json
{
  "status": "ok",
  "service": "payment-api",
  "timestamp": "2026-10-20T09:36:00Z"
}
```

---

## Payments

### POST /v1/payments

Create a new payment. Requires the `Idempotency-Key` header.

**Headers**
```
Content-Type:    application/json
Idempotency-Key: <uuid>         (required, must be unique per payment attempt)
```

**Request Body**
```json
{
  "amount":      100000,
  "currency":    "INR",
  "description": "Order #42",
  "metadata":    { "order_id": "ord_abc123" }
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `amount` | integer | Yes | Paise (≥ 1) |
| `currency` | string | Yes | 3-letter ISO code (e.g. `"INR"`) |
| `description` | string | No | |
| `metadata` | object | No | Arbitrary key-value pairs |

**Response 201 Created**
```json
{
  "id":              "pay_01J9ABCDEFGHJKMNPQRSTVWX",
  "amount":          100000,
  "currency":        "INR",
  "status":          "CREATED",
  "description":     "Order #42",
  "idempotency_key": "550e8400-e29b-41d4-a716-446655440000",
  "metadata":        { "order_id": "ord_abc123" },
  "created_at":      "2026-10-20T09:36:00Z",
  "updated_at":      "2026-10-20T09:36:00Z"
}
```

**Response 200 OK** _(idempotent replay — same key, same payload)_  
Returns the existing payment record unchanged.

**Response 409 Conflict** _(same key, different payload)_
```json
{
  "error": "IdempotencyConflict",
  "message": "Idempotency key already used with a different request payload"
}
```

**Response 422 Unprocessable Entity** _(validation failure)_
```json
{
  "error": "ValidationError",
  "message": "amount must be greater than 0"
}
```

---

### GET /v1/payments/:id

Retrieve a payment by its ID.

**Response 200 OK**
```json
{
  "id":              "pay_01J9ABCDEFGHJKMNPQRSTVWX",
  "amount":          100000,
  "currency":        "INR",
  "status":          "SUCCEEDED",
  "description":     "Order #42",
  "idempotency_key": "550e8400-e29b-41d4-a716-446655440000",
  "provider_ref":    "prov_xyz789",
  "failure_reason":  null,
  "metadata":        { "order_id": "ord_abc123" },
  "created_at":      "2026-10-20T09:36:00Z",
  "updated_at":      "2026-10-20T09:36:05Z"
}
```

**Response 404 Not Found**
```json
{ "error": "NotFound", "message": "payment not found" }
```

---

### GET /v1/payments

List payments (paginated, newest first).

**Query Parameters**

| Param | Default | Notes |
|---|---|---|
| `limit` | 20 | Max 100 |
| `cursor` | — | Opaque pagination cursor (ULID of last item) |
| `status` | — | Filter by status |

**Response 200 OK**
```json
{
  "data": [ /* array of payment objects */ ],
  "next_cursor": "pay_01J9...",
  "total": 42
}
```

---

## Refunds

### POST /v1/payments/:id/refunds

Initiate a refund for a SUCCEEDED payment.

**Request Body**
```json
{
  "amount": 50000,
  "reason": "Customer request"
}
```

**Response 201 Created**
```json
{
  "id":         "ref_01J9...",
  "payment_id": "pay_01J9...",
  "amount":     50000,
  "status":     "PENDING",
  "reason":     "Customer request",
  "created_at": "2026-10-20T10:00:00Z"
}
```

**Response 422** _(over-refund attempt)_
```json
{
  "error":   "OverRefund",
  "message": "refund amount exceeds remaining refundable amount"
}
```

---

### GET /v1/payments/:id/refunds

List all refunds for a payment.

**Response 200 OK**
```json
{
  "data": [ /* array of refund objects */ ]
}
```

---

## Error Format

All error responses follow this shape:

```json
{
  "error":   "ErrorCode",
  "message": "Human-readable description"
}
```

| HTTP Status | Error Code | Cause |
|---|---|---|
| 400 | `BadRequest` | Malformed JSON |
| 404 | `NotFound` | Resource not found |
| 409 | `IdempotencyConflict` | Key reused with different payload |
| 422 | `ValidationError` | Invalid field values |
| 422 | `OverRefund` | Refund exceeds available amount |
| 429 | `RateLimitExceeded` | Too many requests |
| 500 | `InternalError` | Unexpected server error |
| 502 | `BadGateway` | Payment service unreachable |
