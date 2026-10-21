package payments

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrIdempotencyConflict is returned when an idempotency key is reused with mismatched parameters.
	ErrIdempotencyConflict = errors.New("idempotency key already used with a different request payload")

	// ErrPaymentNotFound is returned when a requested payment does not exist.
	ErrPaymentNotFound = errors.New("payment not found")

	// ErrInvalidStatusTransition is returned when a status transition violates the state machine.
	ErrInvalidStatusTransition = errors.New("invalid status transition")

	// ErrTerminalStatus is returned when attempting to mutate a payment in a terminal state.
	ErrTerminalStatus = errors.New("cannot transition from a terminal status")
)

// Repository is the minimal interface needed by the PaymentService.
type Repository interface {
	Create(ctx context.Context, p *Payment) error
	GetByID(ctx context.Context, id string) (*Payment, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*Payment, error)
	UpdateStatus(ctx context.Context, id string, expectedOld PaymentStatus, newStatus PaymentStatus, providerRef, failureReason *string) (*Payment, error)
	List(ctx context.Context, filter ListFilter) ([]*Payment, string, int64, error)
}

// Service defines the business use-cases for payment processing.
type Service interface {
	CreatePayment(ctx context.Context, idempotencyKey string, req CreatePaymentRequest) (*PaymentResponse, bool, error)
	GetPayment(ctx context.Context, id string) (*PaymentResponse, error)
	ListPayments(ctx context.Context, filter ListFilter) (*ListPaymentsResponse, error)
	TransitionStatus(ctx context.Context, id string, newStatus PaymentStatus, providerRef, failureReason *string) (*PaymentResponse, error)
}

// PaymentService implements the Service interface.
type PaymentService struct {
	repo Repository
}

// NewService creates a new PaymentService with the given repository.
func NewService(repo Repository) *PaymentService {
	return &PaymentService{repo: repo}
}

// CreatePayment handles payment creation with strict idempotency and validation.
// Returns:
// - response: the created or existing payment
// - isReplay: true if this was an idempotent replay of an existing payment
// - error: validation error, idempotency conflict, or internal failure
func (s *PaymentService) CreatePayment(ctx context.Context, idempotencyKey string, req CreatePaymentRequest) (*PaymentResponse, bool, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return nil, false, errors.New("idempotency key is required")
	}

	if err := req.Validate(); err != nil {
		return nil, false, err
	}

	// 1. Check if a payment with this idempotency key already exists
	existing, err := s.repo.GetByIdempotencyKey(ctx, key)
	if err == nil && existing != nil {
		// Idempotency key already registered: verify payload matching
		if isPayloadEquivalent(existing, &req) {
			return existing.ToResponse(), true, nil
		}
		return nil, false, ErrIdempotencyConflict
	}

	// 2. Prepare new Payment entity
	now := time.Now().UTC()
	payment := &Payment{
		ID:             NewPaymentID(),
		IdempotencyKey: key,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Status:         StatusCreated,
		Description:    req.Description,
		Metadata:       req.Metadata,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// 3. Persist to repository
	if err := s.repo.Create(ctx, payment); err != nil {
		// Handle potential race condition if two requests with the same key execute concurrently
		existingAfterRace, checkErr := s.repo.GetByIdempotencyKey(ctx, key)
		if checkErr == nil && existingAfterRace != nil {
			if isPayloadEquivalent(existingAfterRace, &req) {
				return existingAfterRace.ToResponse(), true, nil
			}
			return nil, false, ErrIdempotencyConflict
		}
		return nil, false, fmt.Errorf("failed to create payment: %w", err)
	}

	return payment.ToResponse(), false, nil
}

// GetPayment retrieves a payment by its unique ID.
func (s *PaymentService) GetPayment(ctx context.Context, id string) (*PaymentResponse, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("payment id is required")
	}

	payment, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrPaymentNotFound
	}

	return payment.ToResponse(), nil
}

// ListPayments returns a paginated list of payments.
func (s *PaymentService) ListPayments(ctx context.Context, filter ListFilter) (*ListPaymentsResponse, error) {
	if filter.Status != nil && !IsValidStatus(*filter.Status) {
		return nil, fmt.Errorf("invalid status filter: %s", *filter.Status)
	}

	items, nextCursor, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list payments: %w", err)
	}

	data := make([]*PaymentResponse, len(items))
	for i, item := range items {
		data[i] = item.ToResponse()
	}

	return &ListPaymentsResponse{
		Data:       data,
		NextCursor: nextCursor,
		Total:      total,
	}, nil
}

// TransitionStatus transitions a payment to a new state according to the state machine.
func (s *PaymentService) TransitionStatus(
	ctx context.Context,
	id string,
	newStatus PaymentStatus,
	providerRef, failureReason *string,
) (*PaymentResponse, error) {
	if !IsValidStatus(newStatus) {
		return nil, fmt.Errorf("invalid target status: %s", newStatus)
	}

	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrPaymentNotFound
	}

	if IsTerminal(current.Status) {
		return nil, fmt.Errorf("%w: current status %s is terminal", ErrTerminalStatus, current.Status)
	}

	if !CanTransition(current.Status, newStatus) {
		return nil, fmt.Errorf("%w: transition from %s to %s is not permitted",
			ErrInvalidStatusTransition, current.Status, newStatus)
	}

	updated, err := s.repo.UpdateStatus(ctx, id, current.Status, newStatus, providerRef, failureReason)
	if err != nil {
		return nil, fmt.Errorf("failed to update payment status: %w", err)
	}

	return updated.ToResponse(), nil
}

// isPayloadEquivalent checks whether the existing payment matches the request attributes.
func isPayloadEquivalent(p *Payment, req *CreatePaymentRequest) bool {
	return p.Amount == req.Amount &&
		p.Currency == req.Currency &&
		p.Description == req.Description
}
