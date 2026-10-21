// Package payments defines the payment and refund domain model and business rules.
package payments

import (
	"errors"
	"time"
)

// RefundStatus represents the lifecycle state of a refund.
type RefundStatus string

const (
	StatusRefundPending   RefundStatus = "PENDING"
	StatusRefundSucceeded RefundStatus = "SUCCEEDED"
	StatusRefundFailed    RefundStatus = "FAILED"
)

var (
	// ErrRefundInvalidPaymentStatus is returned when attempting to refund a non-SUCCEEDED payment.
	ErrRefundInvalidPaymentStatus = errors.New("payment must be in SUCCEEDED status to be refunded")

	// ErrRefundAmountExceeded is returned when the refund amount exceeds the remaining refundable amount.
	ErrRefundAmountExceeded = errors.New("refund amount exceeds available refundable balance")

	// ErrRefundNotFound is returned when the requested refund does not exist.
	ErrRefundNotFound = errors.New("refund not found")
)

// Refund represents a monetary refund associated with a payment.
type Refund struct {
	ID             string       `json:"id"`
	PaymentID      string       `json:"payment_id"`
	Amount         int64        `json:"amount"` // in paise
	Status         RefundStatus `json:"status"`
	Reason         string       `json:"reason,omitempty"`
	IdempotencyKey string       `json:"idempotency_key,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// CreateRefundRequest is the input payload for creating a refund.
type CreateRefundRequest struct {
	Amount int64  `json:"amount"`
	Reason string `json:"reason,omitempty"`
}

// Validate checks request validity.
func (r *CreateRefundRequest) Validate() error {
	if r.Amount <= 0 {
		return errors.New("refund amount must be greater than 0")
	}
	if len(r.Reason) > 500 {
		return errors.New("reason cannot exceed 500 characters")
	}
	return nil
}

// RefundResponse is the external API representation of a refund.
type RefundResponse struct {
	ID             string       `json:"id"`
	PaymentID      string       `json:"payment_id"`
	Amount         int64        `json:"amount"`
	Status         RefundStatus `json:"status"`
	Reason         string       `json:"reason,omitempty"`
	IdempotencyKey string       `json:"idempotency_key,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// ToResponse converts a Refund entity to its API response shape.
func (r *Refund) ToResponse() *RefundResponse {
	return &RefundResponse{
		ID:             r.ID,
		PaymentID:      r.PaymentID,
		Amount:         r.Amount,
		Status:         r.Status,
		Reason:         r.Reason,
		IdempotencyKey: r.IdempotencyKey,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}
