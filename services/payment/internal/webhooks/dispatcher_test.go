package webhooks_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/krishsingh120/payflow/services/payment/internal/webhooks"
)

func TestWebhookSigner(t *testing.T) {
	secret := "whsec_test_secret_12345"
	payload := []byte(`{"id":"pay_01","amount":5000,"status":"SUCCEEDED"}`)
	now := time.Now().UTC()
	timestamp := now.Unix()

	t.Run("sign and verify successfully", func(t *testing.T) {
		header := webhooks.Sign(secret, timestamp, payload)
		assert.Contains(t, header, "t=")
		assert.Contains(t, header, "v1=")

		err := webhooks.Verify(secret, payload, header, 5*time.Minute)
		require.NoError(t, err)
	})

	t.Run("tampered payload fails signature verification", func(t *testing.T) {
		header := webhooks.Sign(secret, timestamp, payload)
		tampered := []byte(`{"id":"pay_01","amount":99999,"status":"SUCCEEDED"}`)

		err := webhooks.Verify(secret, tampered, header, 5*time.Minute)
		assert.ErrorIs(t, err, webhooks.ErrSignatureMismatch)
	})

	t.Run("wrong secret fails signature verification", func(t *testing.T) {
		header := webhooks.Sign(secret, timestamp, payload)
		err := webhooks.Verify("wrong_secret", payload, header, 5*time.Minute)
		assert.ErrorIs(t, err, webhooks.ErrSignatureMismatch)
	})

	t.Run("timestamp outside tolerance fails with replay error", func(t *testing.T) {
		oldTimestamp := time.Now().Add(-10 * time.Minute).Unix()
		header := webhooks.Sign(secret, oldTimestamp, payload)

		err := webhooks.Verify(secret, payload, header, 2*time.Minute)
		assert.ErrorIs(t, err, webhooks.ErrTimestampTolerance)
	})

	t.Run("malformed header fails verification", func(t *testing.T) {
		err := webhooks.Verify(secret, payload, "garbage-header", 5*time.Minute)
		assert.ErrorIs(t, err, webhooks.ErrInvalidSignatureHeader)
	})
}

func TestWebhookDispatcher_DeliverySuccess(t *testing.T) {
	secret := "whsec_live_mock_secret"
	payload := []byte(`{"event":"payment.succeeded","payment_id":"pay_123"}`)

	var receivedHeader string
	var receivedEvent string
	var receivedDelivery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("X-PayFlow-Signature")
		receivedEvent = r.Header.Get("X-PayFlow-Event")
		receivedDelivery = r.Header.Get("X-PayFlow-Delivery")

		body, _ := io.ReadAll(r.Body)
		err := webhooks.Verify(secret, body, receivedHeader, 1*time.Minute)
		if err != nil {
			http.Error(w, "invalid signature: "+err.Error(), http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"received":true}`))
	}))
	defer server.Close()

	repo := webhooks.NewMemoryRepository()
	disp := webhooks.NewDispatcher(repo, webhooks.Config{
		SigningSecret: secret,
		MaxAttempts:   3,
	}, zerolog.Nop())

	delivery := &webhooks.Delivery{
		ID:        webhooks.NewDeliveryID(),
		PaymentID: "pay_123",
		EventType: "payment.succeeded",
		URL:       server.URL,
		Payload:   payload,
		Status:    webhooks.StatusPending,
		Attempts:  0,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, repo.CreateDelivery(context.Background(), delivery))

	count, err := disp.ProcessPendingDeliveries(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	assert.NotEmpty(t, receivedHeader)
	assert.Equal(t, "payment.succeeded", receivedEvent)
	assert.Equal(t, delivery.ID, receivedDelivery)

	// Verify delivery is now marked DELIVERED
	pending, err := repo.GetPendingDeliveries(context.Background(), 10)
	require.NoError(t, err)
	assert.Empty(t, pending)
}

func TestWebhookDispatcher_RetryAndExhaustion(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	repo := webhooks.NewMemoryRepository()
	disp := webhooks.NewDispatcher(repo, webhooks.Config{
		SigningSecret: "whsec_secret",
		MaxAttempts:   2,
	}, zerolog.Nop())

	delivery := &webhooks.Delivery{
		ID:        webhooks.NewDeliveryID(),
		PaymentID: "pay_fail",
		EventType: "payment.failed",
		URL:       server.URL,
		Payload:   []byte(`{"status":"FAILED"}`),
		Status:    webhooks.StatusPending,
		Attempts:  0,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, repo.CreateDelivery(context.Background(), delivery))

	// Attempt 1 -> fails, status still PENDING with nextRetry scheduled
	err := disp.DispatchDelivery(context.Background(), delivery)
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

	// Re-fetch delivery from repo
	deliveries, _ := repo.ListByPaymentID(context.Background(), "pay_fail")
	require.Equal(t, 1, len(deliveries))
	assert.Equal(t, webhooks.StatusPending, deliveries[0].Status)
	assert.Equal(t, 1, deliveries[0].Attempts)
	assert.NotNil(t, deliveries[0].NextRetryAt)
	assert.NotNil(t, deliveries[0].LastError)

	// Attempt 2 (reaches maxAttempts = 2) -> marks terminal FAILED
	err = disp.DispatchDelivery(context.Background(), deliveries[0])
	require.NoError(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount))

	deliveriesAfter, _ := repo.ListByPaymentID(context.Background(), "pay_fail")
	assert.Equal(t, webhooks.StatusFailed, deliveriesAfter[0].Status)
	assert.Equal(t, 2, deliveriesAfter[0].Attempts)
	assert.Nil(t, deliveriesAfter[0].NextRetryAt)
}

func TestCalculateBackoff(t *testing.T) {
	assert.Equal(t, 5*time.Second, webhooks.CalculateBackoff(1))
	assert.Equal(t, 30*time.Second, webhooks.CalculateBackoff(2))
	assert.Equal(t, 2*time.Minute, webhooks.CalculateBackoff(3))
	assert.Equal(t, 10*time.Minute, webhooks.CalculateBackoff(4))
	assert.Equal(t, 30*time.Minute, webhooks.CalculateBackoff(5))
}
