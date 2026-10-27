package ledger_test

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/krishsingh120/payflow/services/payment/internal/ledger"
)

func TestLedgerInvariant(t *testing.T) {
	t.Run("balanced transaction passes", func(t *testing.T) {
		entries := []*ledger.Entry{
			{PaymentID: "pay_1", Account: ledger.AccountReceivable, Direction: ledger.DirectionDebit, Amount: 5000},
			{PaymentID: "pay_1", Account: ledger.AccountRevenue, Direction: ledger.DirectionCredit, Amount: 5000},
		}
		require.NoError(t, ledger.VerifyInvariant(entries))
	})

	t.Run("unbalanced transaction fails", func(t *testing.T) {
		entries := []*ledger.Entry{
			{PaymentID: "pay_1", Account: ledger.AccountReceivable, Direction: ledger.DirectionDebit, Amount: 5000},
			{PaymentID: "pay_1", Account: ledger.AccountRevenue, Direction: ledger.DirectionCredit, Amount: 4000},
		}
		err := ledger.VerifyInvariant(entries)
		require.Error(t, err)
		assert.ErrorIs(t, err, ledger.ErrLedgerUnbalanced)
	})

	t.Run("zero or negative amount fails", func(t *testing.T) {
		entries := []*ledger.Entry{
			{PaymentID: "pay_1", Account: ledger.AccountReceivable, Direction: ledger.DirectionDebit, Amount: 0},
			{PaymentID: "pay_1", Account: ledger.AccountRevenue, Direction: ledger.DirectionCredit, Amount: 0},
		}
		assert.ErrorIs(t, ledger.VerifyInvariant(entries), ledger.ErrInvalidAmount)
	})

	t.Run("invalid direction fails", func(t *testing.T) {
		entries := []*ledger.Entry{
			{PaymentID: "pay_1", Account: ledger.AccountReceivable, Direction: "INVALID", Amount: 5000},
			{PaymentID: "pay_1", Account: ledger.AccountRevenue, Direction: ledger.DirectionCredit, Amount: 5000},
		}
		assert.ErrorIs(t, ledger.VerifyInvariant(entries), ledger.ErrInvalidDirection)
	})
}

func TestLedgerService_PaymentAndRefundFlow(t *testing.T) {
	ctx := context.Background()
	repo := ledger.NewMemoryRepository()
	svc := ledger.NewService(repo, zerolog.Nop())

	paymentID := "pay_test_ledger_1"
	amount := int64(100000) // 1000 INR

	// 1. Record payment success
	err := svc.RecordPaymentSucceeded(ctx, paymentID, amount)
	require.NoError(t, err)

	summary, err := svc.GetPaymentLedger(ctx, paymentID)
	require.NoError(t, err)
	assert.True(t, summary.IsBalanced)
	assert.Equal(t, int64(100000), summary.TotalDebit)
	assert.Equal(t, int64(100000), summary.TotalCredit)
	assert.Equal(t, 2, len(summary.Entries))

	// 2. Record partial refund (400 INR)
	refundAmount := int64(40000)
	err = svc.RecordRefundSucceeded(ctx, paymentID, refundAmount)
	require.NoError(t, err)

	summaryAfterRefund, err := svc.GetPaymentLedger(ctx, paymentID)
	require.NoError(t, err)
	assert.True(t, summaryAfterRefund.IsBalanced)
	assert.Equal(t, int64(140000), summaryAfterRefund.TotalDebit)
	assert.Equal(t, int64(140000), summaryAfterRefund.TotalCredit)
	assert.Equal(t, 4, len(summaryAfterRefund.Entries))

	// 3. Check account balances
	balances, err := svc.GetAccountBalances(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(60000), balances[ledger.AccountReceivable])      // 1000 - 400 = 600 INR
	assert.Equal(t, int64(100000), balances[ledger.AccountRevenue])        // 1000 INR revenue
	assert.Equal(t, int64(40000), balances[ledger.AccountRefundLiability]) // 400 INR refunded

	// 4. Audit global invariant
	balanced, totalDebit, totalCredit, err := svc.AuditGlobalBalance(ctx)
	require.NoError(t, err)
	assert.True(t, balanced)
	assert.Equal(t, int64(140000), totalDebit)
	assert.Equal(t, int64(140000), totalCredit)
}
