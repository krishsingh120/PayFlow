package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	svchttp "github.com/krishsingh120/payflow/services/payment/internal/http"
	"github.com/krishsingh120/payflow/services/payment/internal/payments"
	"github.com/krishsingh120/payflow/services/payment/internal/storage"
)

func setupTestServer() http.Handler {
	repo := storage.NewMemoryPaymentRepository()
	svc := payments.NewService(repo)
	logger := zerolog.Nop()
	return svchttp.NewRouter(logger, svc)
}

func TestHealthEndpoint(t *testing.T) {
	handler := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp["status"])
	assert.Equal(t, "payment-api", resp["service"])
}

func TestCreatePaymentEndpoint(t *testing.T) {
	handler := setupTestServer()

	t.Run("missing Idempotency-Key header returns 400", func(t *testing.T) {
		body := bytes.NewBufferString(`{"amount": 5000, "currency": "INR"}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/payments", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		var errResp svchttp.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
		assert.Equal(t, "BadRequest", errResp.Error)
		assert.Contains(t, errResp.Message, "Idempotency-Key header is required")
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		body := bytes.NewBufferString(`{invalid json}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/payments", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "key-bad-json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("validation failure returns 422", func(t *testing.T) {
		body := bytes.NewBufferString(`{"amount": -50, "currency": "INR"}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/payments", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "key-val-err")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		var errResp svchttp.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
		assert.Equal(t, "ValidationError", errResp.Error)
	})

	t.Run("valid payment returns 201 Created", func(t *testing.T) {
		body := bytes.NewBufferString(`{"amount": 100000, "currency": "INR", "description": "Order #1"}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/payments", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "key-success-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		var resp payments.PaymentResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.NotEmpty(t, resp.ID)
		assert.Equal(t, int64(100000), resp.Amount)
		assert.Equal(t, "INR", resp.Currency)
		assert.Equal(t, payments.StatusCreated, resp.Status)
		assert.Equal(t, "key-success-1", resp.IdempotencyKey)
	})

	t.Run("idempotent replay returns 200 OK with same payment", func(t *testing.T) {
		body := `{"amount": 75000, "currency": "INR", "description": "Order #2"}`

		// Call 1: 201 Created
		req1 := httptest.NewRequest(http.MethodPost, "/v1/payments", bytes.NewBufferString(body))
		req1.Header.Set("Content-Type", "application/json")
		req1.Header.Set("Idempotency-Key", "key-replay-test")
		rec1 := httptest.NewRecorder()
		handler.ServeHTTP(rec1, req1)
		assert.Equal(t, http.StatusCreated, rec1.Code)

		var resp1 payments.PaymentResponse
		require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &resp1))

		// Call 2: 200 OK replay
		req2 := httptest.NewRequest(http.MethodPost, "/v1/payments", bytes.NewBufferString(body))
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Idempotency-Key", "key-replay-test")
		rec2 := httptest.NewRecorder()
		handler.ServeHTTP(rec2, req2)
		assert.Equal(t, http.StatusOK, rec2.Code)

		var resp2 payments.PaymentResponse
		require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &resp2))
		assert.Equal(t, resp1.ID, resp2.ID)
		assert.Equal(t, resp1.Amount, resp2.Amount)
	})

	t.Run("idempotency conflict returns 409 Conflict", func(t *testing.T) {
		body1 := `{"amount": 20000, "currency": "INR"}`
		req1 := httptest.NewRequest(http.MethodPost, "/v1/payments", bytes.NewBufferString(body1))
		req1.Header.Set("Content-Type", "application/json")
		req1.Header.Set("Idempotency-Key", "key-conflict-test")
		rec1 := httptest.NewRecorder()
		handler.ServeHTTP(rec1, req1)
		assert.Equal(t, http.StatusCreated, rec1.Code)

		// Call with different amount using same key
		body2 := `{"amount": 99999, "currency": "INR"}`
		req2 := httptest.NewRequest(http.MethodPost, "/v1/payments", bytes.NewBufferString(body2))
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Idempotency-Key", "key-conflict-test")
		rec2 := httptest.NewRecorder()
		handler.ServeHTTP(rec2, req2)
		assert.Equal(t, http.StatusConflict, rec2.Code)

		var errResp svchttp.ErrorResponse
		require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &errResp))
		assert.Equal(t, "IdempotencyConflict", errResp.Error)
	})
}

func TestGetPaymentEndpoint(t *testing.T) {
	handler := setupTestServer()

	// Create payment first
	body := `{"amount": 42000, "currency": "INR"}`
	createReq := httptest.NewRequest(http.MethodPost, "/v1/payments", bytes.NewBufferString(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Idempotency-Key", "key-get-test")
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var created payments.PaymentResponse
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	t.Run("existing payment returns 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/payments/"+created.ID, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var got payments.PaymentResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		assert.Equal(t, created.ID, got.ID)
		assert.Equal(t, int64(42000), got.Amount)
	})

	t.Run("non-existent payment returns 404 Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/payments/pay_nonexistent123", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		var errResp svchttp.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
		assert.Equal(t, "NotFound", errResp.Error)
	})
}

func TestListPaymentsEndpoint(t *testing.T) {
	handler := setupTestServer()

	// Seed 3 payments
	for i := 1; i <= 3; i++ {
		b := `{"amount": 1000, "currency": "INR"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/payments", bytes.NewBufferString(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", string(rune('0'+i)))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)
	}

	t.Run("list payments returns 200 OK with data and total", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/payments?limit=2", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var listResp payments.ListPaymentsResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &listResp))
		assert.Equal(t, 2, len(listResp.Data))
		assert.Equal(t, int64(3), listResp.Total)
		assert.NotEmpty(t, listResp.NextCursor)
	})

	t.Run("invalid status filter returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/payments?status=INVALID_STATUS", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
