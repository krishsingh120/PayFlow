package payments_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/krishsingh120/payflow/services/payment/internal/payments"
)

func TestPaymentStatusValidation(t *testing.T) {
	tests := []struct {
		status payments.PaymentStatus
		valid  bool
	}{
		{payments.StatusCreated, true},
		{payments.StatusPending, true},
		{payments.StatusSucceeded, true},
		{payments.StatusFailed, true},
		{payments.StatusUnknown, true},
		{"INVALID", false},
		{"", false},
		{"REFUNDED", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			assert.Equal(t, tt.valid, payments.IsValidStatus(tt.status))
		})
	}
}

func TestPaymentStateMachineTransitions(t *testing.T) {
	cases := []struct {
		name    string
		from    payments.PaymentStatus
		to      payments.PaymentStatus
		allowed bool
	}{
		// Permitted transitions
		{"CREATED to PENDING", payments.StatusCreated, payments.StatusPending, true},
		{"PENDING to SUCCEEDED", payments.StatusPending, payments.StatusSucceeded, true},
		{"PENDING to FAILED", payments.StatusPending, payments.StatusFailed, true},
		{"PENDING to UNKNOWN", payments.StatusPending, payments.StatusUnknown, true},
		{"UNKNOWN to SUCCEEDED", payments.StatusUnknown, payments.StatusSucceeded, true},
		{"UNKNOWN to FAILED", payments.StatusUnknown, payments.StatusFailed, true},

		// Prohibited transitions
		{"CREATED to SUCCEEDED directly", payments.StatusCreated, payments.StatusSucceeded, false},
		{"CREATED to FAILED directly", payments.StatusCreated, payments.StatusFailed, false},
		{"CREATED to UNKNOWN directly", payments.StatusCreated, payments.StatusUnknown, false},
		{"UNKNOWN to PENDING backwards", payments.StatusUnknown, payments.StatusPending, false},

		// Terminal states (no transitions permitted)
		{"SUCCEEDED to PENDING", payments.StatusSucceeded, payments.StatusPending, false},
		{"SUCCEEDED to FAILED", payments.StatusSucceeded, payments.StatusFailed, false},
		{"SUCCEEDED to CREATED", payments.StatusSucceeded, payments.StatusCreated, false},
		{"FAILED to PENDING", payments.StatusFailed, payments.StatusPending, false},
		{"FAILED to SUCCEEDED", payments.StatusFailed, payments.StatusSucceeded, false},
		{"FAILED to CREATED", payments.StatusFailed, payments.StatusCreated, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			allowed := payments.CanTransition(tc.from, tc.to)
			assert.Equal(t, tc.allowed, allowed,
				"expected CanTransition(%s, %s) to be %v", tc.from, tc.to, tc.allowed)
		})
	}
}

func TestIsTerminal(t *testing.T) {
	assert.True(t, payments.IsTerminal(payments.StatusSucceeded))
	assert.True(t, payments.IsTerminal(payments.StatusFailed))
	assert.False(t, payments.IsTerminal(payments.StatusCreated))
	assert.False(t, payments.IsTerminal(payments.StatusPending))
	assert.False(t, payments.IsTerminal(payments.StatusUnknown))
}

func TestValidation(t *testing.T) {
	t.Run("valid request", func(t *testing.T) {
		req := payments.CreatePaymentRequest{
			Amount:      100000,
			Currency:    "INR",
			Description: "Order #123",
		}
		assert.NoError(t, req.Validate())
	})

	t.Run("zero amount", func(t *testing.T) {
		req := payments.CreatePaymentRequest{
			Amount:   0,
			Currency: "INR",
		}
		err := req.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "greater than 0")
	})

	t.Run("negative amount", func(t *testing.T) {
		req := payments.CreatePaymentRequest{
			Amount:   -500,
			Currency: "INR",
		}
		err := req.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "greater than 0")
	})

	t.Run("invalid currency length", func(t *testing.T) {
		req := payments.CreatePaymentRequest{
			Amount:   1000,
			Currency: "US",
		}
		assert.Error(t, req.Validate())
	})

	t.Run("lowercase currency normalized to uppercase", func(t *testing.T) {
		req := payments.CreatePaymentRequest{
			Amount:   1000,
			Currency: "inr",
		}
		assert.NoError(t, req.Validate())
		assert.Equal(t, "INR", req.Currency)
	})

	t.Run("description too long", func(t *testing.T) {
		longDesc := make([]byte, 501)
		for i := range longDesc {
			longDesc[i] = 'a'
		}
		req := payments.CreatePaymentRequest{
			Amount:      1000,
			Currency:    "INR",
			Description: string(longDesc),
		}
		assert.Error(t, req.Validate())
	})
}
