package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/krishsingh120/payflow/services/payment/internal/payments"
	"github.com/krishsingh120/payflow/services/payment/internal/storage"
)

func TestMemoryPaymentRepository(t *testing.T) {
	ctx := context.Background()
	repo := storage.NewMemoryPaymentRepository()

	now := time.Now().UTC()
	p1 := &payments.Payment{
		ID:             "pay_01",
		IdempotencyKey: "idem_01",
		Amount:         1000,
		Currency:       "INR",
		Status:         payments.StatusCreated,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	t.Run("Create and GetByID", func(t *testing.T) {
		err := repo.Create(ctx, p1)
		require.NoError(t, err)

		got, err := repo.GetByID(ctx, p1.ID)
		require.NoError(t, err)
		assert.Equal(t, p1.ID, got.ID)
		assert.Equal(t, p1.IdempotencyKey, got.IdempotencyKey)
		assert.Equal(t, p1.Amount, got.Amount)
	})

	t.Run("Create duplicate idempotency key returns ErrDuplicateKey", func(t *testing.T) {
		dup := &payments.Payment{
			ID:             "pay_02",
			IdempotencyKey: "idem_01", // duplicate!
			Amount:         2000,
			Currency:       "INR",
			Status:         payments.StatusCreated,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		err := repo.Create(ctx, dup)
		assert.ErrorIs(t, err, storage.ErrDuplicateKey)
	})

	t.Run("GetByIdempotencyKey", func(t *testing.T) {
		got, err := repo.GetByIdempotencyKey(ctx, "idem_01")
		require.NoError(t, err)
		assert.Equal(t, p1.ID, got.ID)

		_, err = repo.GetByIdempotencyKey(ctx, "nonexistent")
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})

	t.Run("UpdateStatus with matching expectedOld succeeds", func(t *testing.T) {
		ref := "prov_ref_01"
		updated, err := repo.UpdateStatus(ctx, p1.ID, payments.StatusCreated, payments.StatusPending, &ref, nil)
		require.NoError(t, err)
		assert.Equal(t, payments.StatusPending, updated.Status)
		assert.Equal(t, &ref, updated.ProviderRef)
	})

	t.Run("UpdateStatus with mismatched expectedOld returns ErrConflict", func(t *testing.T) {
		_, err := repo.UpdateStatus(ctx, p1.ID, payments.StatusCreated, payments.StatusSucceeded, nil, nil)
		assert.ErrorIs(t, err, storage.ErrConflict)
	})

	t.Run("List with status filter and cursor", func(t *testing.T) {
		p2 := &payments.Payment{
			ID:             "pay_02",
			IdempotencyKey: "idem_02",
			Amount:         5000,
			Currency:       "INR",
			Status:         payments.StatusCreated,
			CreatedAt:      now.Add(time.Minute),
			UpdatedAt:      now.Add(time.Minute),
		}
		require.NoError(t, repo.Create(ctx, p2))

		// List all
		items, _, total, err := repo.List(ctx, payments.ListFilter{Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Equal(t, 2, len(items))

		// List filtered by PENDING
		st := payments.StatusPending
		pendingItems, _, totalPending, err := repo.List(ctx, payments.ListFilter{Status: &st})
		require.NoError(t, err)
		assert.Equal(t, int64(1), totalPending)
		assert.Equal(t, 1, len(pendingItems))
		assert.Equal(t, p1.ID, pendingItems[0].ID)
	})
}
