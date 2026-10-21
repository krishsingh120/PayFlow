package worker_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/krishsingh120/payflow/services/payment/internal/messaging"
	"github.com/krishsingh120/payflow/services/payment/internal/payments"
	"github.com/krishsingh120/payflow/services/payment/internal/provider"
	"github.com/krishsingh120/payflow/services/payment/internal/storage"
	"github.com/krishsingh120/payflow/services/payment/internal/worker"
)

func setupTestProcessor() (*worker.PaymentProcessor, storage.PaymentRepository) {
	repo := storage.NewMemoryPaymentRepository()
	simProvider := provider.NewSimulatedProvider("success", 10)
	logger := zerolog.Nop()
	proc := worker.NewPaymentProcessor(repo, simProvider, logger)
	return proc, repo
}

func TestProcessJob_Success(t *testing.T) {
	ctx := context.Background()
	proc, repo := setupTestProcessor()

	// Seed CREATED payment
	now := time.Now().UTC()
	payment := &payments.Payment{
		ID:             "pay_worker_1",
		IdempotencyKey: "idem_worker_1",
		Amount:         50000, // ₹500.00
		Currency:       "INR",
		Status:         payments.StatusCreated,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	require.NoError(t, repo.Create(ctx, payment))

	// Execute job
	job := messaging.PaymentJob{PaymentID: payment.ID, CreatedAt: now}
	payload, _ := json.Marshal(job)

	err := proc.ProcessJob(ctx, payload)
	require.NoError(t, err)

	// Verify payment is now SUCCEEDED with provider_ref
	updated, err := repo.GetByID(ctx, payment.ID)
	require.NoError(t, err)
	assert.Equal(t, payments.StatusSucceeded, updated.Status)
	assert.NotNil(t, updated.ProviderRef)
	assert.Nil(t, updated.FailureReason)
}

func TestProcessJob_Decline(t *testing.T) {
	ctx := context.Background()
	proc, repo := setupTestProcessor()

	// Seed payment with amount ending in 99 (triggers decline rule)
	now := time.Now().UTC()
	payment := &payments.Payment{
		ID:             "pay_worker_decline",
		IdempotencyKey: "idem_worker_decline",
		Amount:         999, // ₹9.99
		Currency:       "INR",
		Status:         payments.StatusCreated,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	require.NoError(t, repo.Create(ctx, payment))

	job := messaging.PaymentJob{PaymentID: payment.ID, CreatedAt: now}
	payload, _ := json.Marshal(job)

	err := proc.ProcessJob(ctx, payload)
	require.NoError(t, err)

	// Verify payment is FAILED with failure reason
	updated, err := repo.GetByID(ctx, payment.ID)
	require.NoError(t, err)
	assert.Equal(t, payments.StatusFailed, updated.Status)
	assert.NotNil(t, updated.FailureReason)
	assert.Contains(t, *updated.FailureReason, "insufficient funds")
}

func TestProcessJob_Timeout(t *testing.T) {
	ctx := context.Background()
	proc, repo := setupTestProcessor()

	// Seed payment with amount ending in 98 (triggers timeout rule)
	now := time.Now().UTC()
	payment := &payments.Payment{
		ID:             "pay_worker_timeout",
		IdempotencyKey: "idem_worker_timeout",
		Amount:         198, // ₹1.98
		Currency:       "INR",
		Status:         payments.StatusCreated,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	require.NoError(t, repo.Create(ctx, payment))

	job := messaging.PaymentJob{PaymentID: payment.ID, CreatedAt: now}
	payload, _ := json.Marshal(job)

	err := proc.ProcessJob(ctx, payload)
	require.NoError(t, err)

	// Verify payment is UNKNOWN with failure reason
	updated, err := repo.GetByID(ctx, payment.ID)
	require.NoError(t, err)
	assert.Equal(t, payments.StatusUnknown, updated.Status)
	assert.NotNil(t, updated.FailureReason)
	assert.Contains(t, *updated.FailureReason, "timeout")
}

func TestProcessJob_AlreadyTerminal(t *testing.T) {
	ctx := context.Background()
	proc, repo := setupTestProcessor()

	// Seed payment already in SUCCEEDED status
	now := time.Now().UTC()
	ref := "prov_existing"
	payment := &payments.Payment{
		ID:             "pay_worker_done",
		IdempotencyKey: "idem_worker_done",
		Amount:         20000,
		Currency:       "INR",
		Status:         payments.StatusSucceeded,
		ProviderRef:    &ref,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	require.NoError(t, repo.Create(ctx, payment))

	job := messaging.PaymentJob{PaymentID: payment.ID, CreatedAt: now}
	payload, _ := json.Marshal(job)

	// Must succeed without attempting invalid state transition
	err := proc.ProcessJob(ctx, payload)
	require.NoError(t, err)

	unchanged, err := repo.GetByID(ctx, payment.ID)
	require.NoError(t, err)
	assert.Equal(t, payments.StatusSucceeded, unchanged.Status)
}

func TestProcessJob_MalformedPayload(t *testing.T) {
	ctx := context.Background()
	proc, _ := setupTestProcessor()

	err := proc.ProcessJob(ctx, []byte(`not json`))
	assert.Error(t, err)

	err = proc.ProcessJob(ctx, []byte(`{}`))
	assert.Error(t, err)
}
