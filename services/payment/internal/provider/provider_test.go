package provider_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/krishsingh120/payflow/services/payment/internal/provider"
)

func TestSimulatedProvider(t *testing.T) {
	ctx := context.Background()
	p := provider.NewSimulatedProvider("success", 50)

	t.Run("standard amount returns OutcomeSuccess", func(t *testing.T) {
		res, err := p.ProcessPayment(ctx, provider.ProcessRequest{
			PaymentID: "pay_test_1",
			Amount:    50000, // ₹500.00
			Currency:  "INR",
		})
		require.NoError(t, err)
		assert.Equal(t, provider.OutcomeSuccess, res.Outcome)
		assert.NotEmpty(t, res.ProviderRef)
		assert.True(t, strings.HasPrefix(res.ProviderRef, "prov_sim_"))
		assert.Nil(t, res.FailureReason)
	})

	t.Run("amount ending in 99 returns OutcomeDecline (insufficient funds rule)", func(t *testing.T) {
		res, err := p.ProcessPayment(ctx, provider.ProcessRequest{
			PaymentID: "pay_test_2",
			Amount:    199, // ₹1.99
			Currency:  "INR",
		})
		require.NoError(t, err)
		assert.Equal(t, provider.OutcomeDecline, res.Outcome)
		assert.NotNil(t, res.FailureReason)
		assert.Contains(t, *res.FailureReason, "insufficient funds")
	})

	t.Run("amount ending in 98 returns OutcomeTimeout (bank switch timeout rule)", func(t *testing.T) {
		res, err := p.ProcessPayment(ctx, provider.ProcessRequest{
			PaymentID: "pay_test_3",
			Amount:    298, // ₹2.98
			Currency:  "INR",
		})
		require.NoError(t, err)
		assert.Equal(t, provider.OutcomeTimeout, res.Outcome)
		assert.NotNil(t, res.FailureReason)
		assert.Contains(t, *res.FailureReason, "timeout")
	})

	t.Run("description containing [force-fail] triggers decline", func(t *testing.T) {
		res, err := p.ProcessPayment(ctx, provider.ProcessRequest{
			PaymentID:   "pay_test_4",
			Amount:      10000,
			Currency:    "INR",
			Description: "Customer checkout [force-fail]",
		})
		require.NoError(t, err)
		assert.Equal(t, provider.OutcomeDecline, res.Outcome)
		assert.NotNil(t, res.FailureReason)
	})

	t.Run("description containing [force-timeout] triggers timeout", func(t *testing.T) {
		res, err := p.ProcessPayment(ctx, provider.ProcessRequest{
			PaymentID:   "pay_test_5",
			Amount:      10000,
			Currency:    "INR",
			Description: "Customer checkout [force-timeout]",
		})
		require.NoError(t, err)
		assert.Equal(t, provider.OutcomeTimeout, res.Outcome)
	})

	t.Run("provider initialized with default decline outcome", func(t *testing.T) {
		declineProvider := provider.NewSimulatedProvider("decline", 10)
		res, err := declineProvider.ProcessPayment(ctx, provider.ProcessRequest{
			PaymentID: "pay_test_6",
			Amount:    5000,
			Currency:  "INR",
		})
		require.NoError(t, err)
		assert.Equal(t, provider.OutcomeDecline, res.Outcome)
	})
}
