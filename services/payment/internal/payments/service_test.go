package payments_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/krishsingh120/payflow/services/payment/internal/payments"
	"github.com/krishsingh120/payflow/services/payment/internal/storage"
)

func setupTestService() (*payments.PaymentService, storage.PaymentRepository) {
	repo := storage.NewMemoryPaymentRepository()
	svc := payments.NewService(repo)
	return svc, repo
}

func TestCreatePayment(t *testing.T) {
	ctx := context.Background()
	svc, _ := setupTestService()

	t.Run("creates payment successfully", func(t *testing.T) {
		req := payments.CreatePaymentRequest{
			Amount:      100000,
			Currency:    "INR",
			Description: "Order #42",
			Metadata:    map[string]any{"order_id": "ord_123"},
		}

		res, isReplay, err := svc.CreatePayment(ctx, "idemp-key-1", req)
		require.NoError(t, err)
		assert.False(t, isReplay)
		assert.NotEmpty(t, res.ID)
		assert.True(t, strings.HasPrefix(res.ID, "pay_"), "expected pay_ prefix on ULID ID")
		assert.Equal(t, int64(100000), res.Amount)
		assert.Equal(t, "INR", res.Currency)
		assert.Equal(t, payments.StatusCreated, res.Status)
		assert.Equal(t, "idemp-key-1", res.IdempotencyKey)
	})

	t.Run("idempotent replay returns same payment", func(t *testing.T) {
		req := payments.CreatePaymentRequest{
			Amount:      50000,
			Currency:    "INR",
			Description: "Order #99",
		}

		res1, isReplay1, err := svc.CreatePayment(ctx, "idemp-key-replay", req)
		require.NoError(t, err)
		assert.False(t, isReplay1)

		// Second call with same key and same payload
		res2, isReplay2, err := svc.CreatePayment(ctx, "idemp-key-replay", req)
		require.NoError(t, err)
		assert.True(t, isReplay2, "expected second call to be marked as replay")
		assert.Equal(t, res1.ID, res2.ID)
		assert.Equal(t, res1.Amount, res2.Amount)
		assert.Equal(t, res1.Status, res2.Status)
	})

	t.Run("idempotency conflict with mismatched payload", func(t *testing.T) {
		req1 := payments.CreatePaymentRequest{
			Amount:   50000,
			Currency: "INR",
		}
		_, _, err := svc.CreatePayment(ctx, "idemp-key-conflict", req1)
		require.NoError(t, err)

		// Second call with same key but different amount
		req2 := payments.CreatePaymentRequest{
			Amount:   90000, // Different!
			Currency: "INR",
		}
		_, _, err = svc.CreatePayment(ctx, "idemp-key-conflict", req2)
		assert.ErrorIs(t, err, payments.ErrIdempotencyConflict)
	})

	t.Run("validation failure on negative amount", func(t *testing.T) {
		req := payments.CreatePaymentRequest{
			Amount:   -100,
			Currency: "INR",
		}
		_, _, err := svc.CreatePayment(ctx, "idemp-key-invalid", req)
		assert.Error(t, err)
	})

	t.Run("missing idempotency key", func(t *testing.T) {
		req := payments.CreatePaymentRequest{
			Amount:   100,
			Currency: "INR",
		}
		_, _, err := svc.CreatePayment(ctx, "  ", req)
		assert.Error(t, err)
	})
}

func TestGetPayment(t *testing.T) {
	ctx := context.Background()
	svc, _ := setupTestService()

	created, _, err := svc.CreatePayment(ctx, "get-test-key", payments.CreatePaymentRequest{
		Amount:   150000,
		Currency: "INR",
	})
	require.NoError(t, err)

	t.Run("found by ID", func(t *testing.T) {
		p, err := svc.GetPayment(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, p.ID)
		assert.Equal(t, created.Amount, p.Amount)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := svc.GetPayment(ctx, "pay_nonexistent")
		assert.ErrorIs(t, err, payments.ErrPaymentNotFound)
	})
}

func TestTransitionStatus(t *testing.T) {
	ctx := context.Background()
	svc, _ := setupTestService()

	created, _, err := svc.CreatePayment(ctx, "trans-key-1", payments.CreatePaymentRequest{
		Amount:   200000,
		Currency: "INR",
	})
	require.NoError(t, err)
	assert.Equal(t, payments.StatusCreated, created.Status)

	t.Run("transition CREATED to PENDING", func(t *testing.T) {
		res, err := svc.TransitionStatus(ctx, created.ID, payments.StatusPending, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, payments.StatusPending, res.Status)
	})

	t.Run("transition PENDING to SUCCEEDED", func(t *testing.T) {
		ref := "prov_ref_123"
		res, err := svc.TransitionStatus(ctx, created.ID, payments.StatusSucceeded, &ref, nil)
		require.NoError(t, err)
		assert.Equal(t, payments.StatusSucceeded, res.Status)
		assert.Equal(t, &ref, res.ProviderRef)
	})

	t.Run("transition from terminal state SUCCEEDED rejected", func(t *testing.T) {
		_, err := svc.TransitionStatus(ctx, created.ID, payments.StatusFailed, nil, nil)
		assert.ErrorIs(t, err, payments.ErrTerminalStatus)
	})
}

func TestListPayments(t *testing.T) {
	ctx := context.Background()
	svc, _ := setupTestService()

	// Seed 5 payments
	for i := 1; i <= 5; i++ {
		_, _, err := svc.CreatePayment(ctx, string(rune('a'+i)), payments.CreatePaymentRequest{
			Amount:   int64(i * 1000),
			Currency: "INR",
		})
		require.NoError(t, err)
	}

	t.Run("list with pagination", func(t *testing.T) {
		res, err := svc.ListPayments(ctx, payments.ListFilter{Limit: 2})
		require.NoError(t, err)
		assert.Equal(t, 2, len(res.Data))
		assert.Equal(t, int64(5), res.Total)
		assert.NotEmpty(t, res.NextCursor)

		// Page 2 using cursor
		res2, err := svc.ListPayments(ctx, payments.ListFilter{Limit: 2, Cursor: res.NextCursor})
		require.NoError(t, err)
		assert.Equal(t, 2, len(res2.Data))
		assert.NotEqual(t, res.Data[0].ID, res2.Data[0].ID)
	})

	t.Run("filter by status", func(t *testing.T) {
		st := payments.StatusCreated
		res, err := svc.ListPayments(ctx, payments.ListFilter{Status: &st})
		require.NoError(t, err)
		assert.Equal(t, int64(5), res.Total)
	})
}

func TestRefunds(t *testing.T) {
	ctx := context.Background()
	svc, _ := setupTestService()

	// 1. Create and transition payment to SUCCEEDED
	p, _, err := svc.CreatePayment(ctx, "idemp-refund-base", payments.CreatePaymentRequest{
		Amount:   100000, // 1000 INR
		Currency: "INR",
	})
	require.NoError(t, err)

	_, err = svc.TransitionStatus(ctx, p.ID, payments.StatusPending, nil, nil)
	require.NoError(t, err)
	ref := "prov_123"
	_, err = svc.TransitionStatus(ctx, p.ID, payments.StatusSucceeded, &ref, nil)
	require.NoError(t, err)

	t.Run("creates partial refund successfully", func(t *testing.T) {
		res, isReplay, err := svc.CreateRefund(ctx, p.ID, "idemp-ref-1", payments.CreateRefundRequest{
			Amount: 60000, // 600 INR
			Reason: "Customer requested partial return",
		})
		require.NoError(t, err)
		assert.False(t, isReplay)
		assert.NotEmpty(t, res.ID)
		assert.True(t, strings.HasPrefix(res.ID, "ref_"))
		assert.Equal(t, int64(60000), res.Amount)
		assert.Equal(t, payments.StatusRefundSucceeded, res.Status)
	})

	t.Run("idempotent refund replay", func(t *testing.T) {
		res, isReplay, err := svc.CreateRefund(ctx, p.ID, "idemp-ref-1", payments.CreateRefundRequest{
			Amount: 60000,
			Reason: "Customer requested partial return",
		})
		require.NoError(t, err)
		assert.True(t, isReplay)
		assert.Equal(t, int64(60000), res.Amount)
	})

	t.Run("idempotent refund conflict with mismatched amount", func(t *testing.T) {
		_, _, err := svc.CreateRefund(ctx, p.ID, "idemp-ref-1", payments.CreateRefundRequest{
			Amount: 30000,
			Reason: "Different amount",
		})
		assert.ErrorIs(t, err, payments.ErrIdempotencyConflict)
	})

	t.Run("second partial refund reaching exact total", func(t *testing.T) {
		res, isReplay, err := svc.CreateRefund(ctx, p.ID, "idemp-ref-2", payments.CreateRefundRequest{
			Amount: 40000, // Remaining 400 INR
			Reason: "Remaining balance return",
		})
		require.NoError(t, err)
		assert.False(t, isReplay)
		assert.Equal(t, int64(40000), res.Amount)
	})

	t.Run("over-refund fails when total exceeded", func(t *testing.T) {
		_, _, err := svc.CreateRefund(ctx, p.ID, "idemp-ref-3", payments.CreateRefundRequest{
			Amount: 1, // 1 paise over limit
			Reason: "Extra refund",
		})
		assert.ErrorIs(t, err, payments.ErrRefundAmountExceeded)
	})

	t.Run("list refunds for payment", func(t *testing.T) {
		refunds, err := svc.ListRefunds(ctx, p.ID)
		require.NoError(t, err)
		assert.Equal(t, 2, len(refunds))
	})

	t.Run("refund non-succeeded payment fails", func(t *testing.T) {
		pendingPayment, _, err := svc.CreatePayment(ctx, "idemp-pending-refund", payments.CreatePaymentRequest{
			Amount:   50000,
			Currency: "INR",
		})
		require.NoError(t, err)

		_, _, err = svc.CreateRefund(ctx, pendingPayment.ID, "idemp-ref-invalid", payments.CreateRefundRequest{
			Amount: 10000,
		})
		assert.ErrorIs(t, err, payments.ErrRefundInvalidPaymentStatus)
	})
}

