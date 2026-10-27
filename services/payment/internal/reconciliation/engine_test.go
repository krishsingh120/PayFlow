package reconciliation_test

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/krishsingh120/payflow/services/payment/internal/ledger"
	"github.com/krishsingh120/payflow/services/payment/internal/payments"
	"github.com/krishsingh120/payflow/services/payment/internal/provider"
	"github.com/krishsingh120/payflow/services/payment/internal/reconciliation"
	"github.com/krishsingh120/payflow/services/payment/internal/storage"
)

func TestReconciliationEngine_Run(t *testing.T) {
	ctx := context.Background()

	// Set up dependencies
	paymentsRepo := storage.NewMemoryPaymentRepository()
	simProvider := provider.NewSimulatedProvider("success", 10)
	ledgerRepo := ledger.NewMemoryRepository()
	ledgerSvc := ledger.NewService(ledgerRepo, zerolog.Nop())

	engine := reconciliation.NewEngine(paymentsRepo, simProvider, ledgerSvc, nil, zerolog.Nop())

	// 1. Seed an UNKNOWN payment (e.g. from a previous timeout)
	now := time.Now().UTC()
	unknownPayment := &payments.Payment{
		ID:             "pay_rec_unknown_1",
		IdempotencyKey: "idem_rec_1",
		Amount:         75000, // ₹750.00
		Currency:       "INR",
		Status:         payments.StatusUnknown,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	require.NoError(t, paymentsRepo.Create(ctx, unknownPayment))

	// 2. Seed another UNKNOWN payment that provider confirms as failed (ID contains 'fail')
	failPayment := &payments.Payment{
		ID:             "pay_rec_fail_2",
		IdempotencyKey: "idem_rec_2",
		Amount:         25000,
		Currency:       "INR",
		Status:         payments.StatusUnknown,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	require.NoError(t, paymentsRepo.Create(ctx, failPayment))

	// 3. Seed a SUCCEEDED payment missing ledger entries (simulating an unexpected crash before ledger write)
	provRef := "prov_unhealed"
	orphanSucceeded := &payments.Payment{
		ID:             "pay_rec_orphan_3",
		IdempotencyKey: "idem_rec_3",
		Amount:         100000, // ₹1000.00
		Currency:       "INR",
		Status:         payments.StatusSucceeded,
		ProviderRef:    &provRef,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	require.NoError(t, paymentsRepo.Create(ctx, orphanSucceeded))

	// Run reconciliation engine
	report, err := engine.Run(ctx)
	require.NoError(t, err)

	// Assertions
	assert.NotEmpty(t, report.ID)
	assert.Equal(t, 2, report.UnknownsResolved)
	assert.True(t, report.LedgerBalanced)
	assert.True(t, len(report.Discrepancies) >= 3)

	// Verify unknownPayment transitioned to SUCCEEDED and ledger recorded
	p1, err := paymentsRepo.GetByID(ctx, unknownPayment.ID)
	require.NoError(t, err)
	assert.Equal(t, payments.StatusSucceeded, p1.Status)
	assert.NotNil(t, p1.ProviderRef)

	summary1, err := ledgerSvc.GetPaymentLedger(ctx, unknownPayment.ID)
	require.NoError(t, err)
	assert.True(t, summary1.IsBalanced)
	assert.Equal(t, int64(75000), summary1.TotalDebit)

	// Verify failPayment transitioned to FAILED
	p2, err := paymentsRepo.GetByID(ctx, failPayment.ID)
	require.NoError(t, err)
	assert.Equal(t, payments.StatusFailed, p2.Status)

	// Verify orphanSucceeded had its ledger healed
	summary3, err := ledgerSvc.GetPaymentLedger(ctx, orphanSucceeded.ID)
	require.NoError(t, err)
	assert.True(t, summary3.IsBalanced)
	assert.Equal(t, int64(100000), summary3.TotalDebit)

	// Check latest report cached
	latest := engine.GetLatestReport()
	require.NotNil(t, latest)
	assert.Equal(t, report.ID, latest.ID)
}
