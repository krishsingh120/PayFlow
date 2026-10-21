// Package storage defines the persistence contracts and implementations for PayFlow.
package storage

import (
	"context"
	"errors"

	"github.com/krishsingh120/payflow/services/payment/internal/payments"
)

var (
	// ErrNotFound is returned when the requested record does not exist.
	ErrNotFound = errors.New("record not found")

	// ErrDuplicateKey is returned when a unique constraint is violated (e.g. idempotency key).
	ErrDuplicateKey = errors.New("duplicate key violation")

	// ErrConflict is returned when an optimistic lock or concurrent modification conflicts.
	ErrConflict = errors.New("resource conflict")

	// ErrInvalidTransition is returned when a requested state change violates the state machine.
	ErrInvalidTransition = errors.New("invalid status transition")
)

// PaymentRepository defines all database operations for payment lifecycle management.
type PaymentRepository interface {
	// Create stores a new payment record. Returns ErrDuplicateKey if idempotency_key already exists.
	Create(ctx context.Context, p *payments.Payment) error

	// GetByID retrieves a payment by its ULID primary key. Returns ErrNotFound if missing.
	GetByID(ctx context.Context, id string) (*payments.Payment, error)

	// GetByIdempotencyKey retrieves a payment by its unique idempotency key. Returns ErrNotFound if missing.
	GetByIdempotencyKey(ctx context.Context, key string) (*payments.Payment, error)

	// UpdateStatus updates the payment status using optimistic locking (matching expectedOld).
	// Returns ErrInvalidTransition or ErrConflict if expectedOld does not match the current DB state.
	UpdateStatus(ctx context.Context, id string, expectedOld payments.PaymentStatus, newStatus payments.PaymentStatus, providerRef, failureReason *string) (*payments.Payment, error)

	// List queries payments matching filter criteria with cursor pagination and total count.
	List(ctx context.Context, filter payments.ListFilter) ([]*payments.Payment, string, int64, error)
}

// RefundRepository defines database operations for refund management and over-refund prevention.
type RefundRepository interface {
	// CreateRefund stores a new refund with over-refund validation and status checks.
	CreateRefund(ctx context.Context, r *payments.Refund) error

	// GetRefundByID retrieves a refund by its ULID identifier.
	GetRefundByID(ctx context.Context, id string) (*payments.Refund, error)

	// ListRefundsByPaymentID retrieves all refunds for a specific payment.
	ListRefundsByPaymentID(ctx context.Context, paymentID string) ([]*payments.Refund, error)
}

// Repository combines PaymentRepository and RefundRepository contracts.
type Repository interface {
	PaymentRepository
	RefundRepository
}
