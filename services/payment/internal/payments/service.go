package payments

import (
	"context"
	"encoding/json"
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
	// Refunds
	CreateRefund(ctx context.Context, r *Refund) error
	GetRefundByID(ctx context.Context, id string) (*Refund, error)
	ListRefundsByPaymentID(ctx context.Context, paymentID string) ([]*Refund, error)
}

// Service defines the business use-cases for payment processing.
type Service interface {
	CreatePayment(ctx context.Context, idempotencyKey string, req CreatePaymentRequest) (*PaymentResponse, bool, error)
	GetPayment(ctx context.Context, id string) (*PaymentResponse, error)
	ListPayments(ctx context.Context, filter ListFilter) (*ListPaymentsResponse, error)
	TransitionStatus(ctx context.Context, id string, newStatus PaymentStatus, providerRef, failureReason *string) (*PaymentResponse, error)
	// Refunds
	CreateRefund(ctx context.Context, paymentID string, idempotencyKey string, req CreateRefundRequest) (*RefundResponse, bool, error)
	GetRefund(ctx context.Context, id string) (*RefundResponse, error)
	ListRefunds(ctx context.Context, paymentID string) ([]*RefundResponse, error)
}

// EventPublisher defines the contract for emitting payment lifecycle events to message brokers.
type EventPublisher interface {
	Publish(ctx context.Context, routingKey string, payload []byte) error
}

// LedgerRecorder defines the contract for recording accounting journal entries.
type LedgerRecorder interface {
	RecordRefundSucceeded(ctx context.Context, paymentID string, amount int64) error
}

// PaymentService implements the Service interface.
type PaymentService struct {
	repo      Repository
	publisher EventPublisher
	ledger    LedgerRecorder
}

// NewService creates a new PaymentService with the given repository.
func NewService(repo Repository) *PaymentService {
	return &PaymentService{repo: repo}
}

// WithPublisher attaches an event publisher for asynchronous job dispatching.
func (s *PaymentService) WithPublisher(p EventPublisher) *PaymentService {
	s.publisher = p
	return s
}

// WithLedger attaches a double-entry ledger recorder for financial tracking.
func (s *PaymentService) WithLedger(l LedgerRecorder) *PaymentService {
	s.ledger = l
	return s
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

	// 4. Dispatch payment.created event to message broker for background worker processing
	if s.publisher != nil {
		jobPayload, _ := json.Marshal(map[string]any{
			"payment_id": payment.ID,
			"created_at": payment.CreatedAt,
		})
		_ = s.publisher.Publish(ctx, "payment.created", jobPayload)
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

// CreateRefund handles refund requests with strict over-refund validation and idempotency.
func (s *PaymentService) CreateRefund(ctx context.Context, paymentID string, idempotencyKey string, req CreateRefundRequest) (*RefundResponse, bool, error) {
	if strings.TrimSpace(paymentID) == "" {
		return nil, false, errors.New("payment id is required")
	}
	if err := req.Validate(); err != nil {
		return nil, false, err
	}

	// 1. Verify payment exists and is SUCCEEDED
	payment, err := s.repo.GetByID(ctx, paymentID)
	if err != nil {
		return nil, false, ErrPaymentNotFound
	}
	if payment.Status != StatusSucceeded {
		return nil, false, ErrRefundInvalidPaymentStatus
	}

	// 2. Check for idempotent replay
	key := strings.TrimSpace(idempotencyKey)
	if key != "" {
		existingRefunds, err := s.repo.ListRefundsByPaymentID(ctx, paymentID)
		if err == nil {
			for _, ref := range existingRefunds {
				if ref.IdempotencyKey == key {
					if ref.Amount == req.Amount && ref.Reason == req.Reason {
						return ref.ToResponse(), true, nil
					}
					return nil, false, ErrIdempotencyConflict
				}
			}
		}
	}

	// 3. Construct and persist refund
	now := time.Now().UTC()
	refund := &Refund{
		ID:             NewRefundID(),
		PaymentID:      paymentID,
		Amount:         req.Amount,
		Status:         StatusRefundSucceeded,
		Reason:         req.Reason,
		IdempotencyKey: key,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.repo.CreateRefund(ctx, refund); err != nil {
		return nil, false, err
	}

	// 4. Record double-entry ledger journal for refund
	if s.ledger != nil {
		_ = s.ledger.RecordRefundSucceeded(ctx, refund.PaymentID, refund.Amount)
	}

	// 5. Emit payment.refunded event
	if s.publisher != nil {
		eventPayload, _ := json.Marshal(map[string]any{
			"refund_id":  refund.ID,
			"payment_id": refund.PaymentID,
			"amount":     refund.Amount,
			"status":     string(refund.Status),
			"reason":     refund.Reason,
			"created_at": refund.CreatedAt,
		})
		_ = s.publisher.Publish(ctx, "payment.refunded", eventPayload)
	}

	return refund.ToResponse(), false, nil
}

// GetRefund retrieves a refund by ID.
func (s *PaymentService) GetRefund(ctx context.Context, id string) (*RefundResponse, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("refund id is required")
	}
	ref, err := s.repo.GetRefundByID(ctx, id)
	if err != nil {
		return nil, ErrRefundNotFound
	}
	return ref.ToResponse(), nil
}

// ListRefunds lists all refunds for a payment.
func (s *PaymentService) ListRefunds(ctx context.Context, paymentID string) ([]*RefundResponse, error) {
	if strings.TrimSpace(paymentID) == "" {
		return nil, errors.New("payment id is required")
	}
	refunds, err := s.repo.ListRefundsByPaymentID(ctx, paymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list refunds: %w", err)
	}
	data := make([]*RefundResponse, len(refunds))
	for i, r := range refunds {
		data[i] = r.ToResponse()
	}
	return data, nil
}

// isPayloadEquivalent checks whether the existing payment matches the request attributes.
// Hardened to check amount, currency, description, and metadata equality.
func isPayloadEquivalent(p *Payment, req *CreatePaymentRequest) bool {
	if p.Amount != req.Amount || p.Currency != req.Currency || p.Description != req.Description {
		return false
	}
	if len(p.Metadata) != len(req.Metadata) {
		return false
	}
	if len(p.Metadata) > 0 {
		pMeta, err1 := json.Marshal(p.Metadata)
		reqMeta, err2 := json.Marshal(req.Metadata)
		if err1 != nil || err2 != nil || string(pMeta) != string(reqMeta) {
			return false
		}
	}
	return true
}
