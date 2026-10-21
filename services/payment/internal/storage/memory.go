package storage

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/krishsingh120/payflow/services/payment/internal/payments"
)

// MemoryPaymentRepository is an in-memory, thread-safe implementation of PaymentRepository.
// It is ideal for unit tests, contract tests, and offline local development.
type MemoryPaymentRepository struct {
	mu               sync.RWMutex
	byID             map[string]*payments.Payment
	byKey            map[string]string // idempotency_key -> id
	ordered          []*payments.Payment
	refundsByID      map[string]*payments.Refund
	refundsByPayment map[string][]*payments.Refund
}

// NewMemoryPaymentRepository constructs an empty in-memory repository.
func NewMemoryPaymentRepository() *MemoryPaymentRepository {
	return &MemoryPaymentRepository{
		byID:             make(map[string]*payments.Payment),
		byKey:            make(map[string]string),
		ordered:          make([]*payments.Payment, 0),
		refundsByID:      make(map[string]*payments.Refund),
		refundsByPayment: make(map[string][]*payments.Refund),
	}
}

// Create inserts a new payment record into the in-memory store.
// Returns ErrDuplicateKey if the idempotency_key has already been registered.
func (m *MemoryPaymentRepository) Create(_ context.Context, p *payments.Payment) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.byKey[p.IdempotencyKey]; exists {
		return ErrDuplicateKey
	}

	clone := clonePayment(p)
	m.byID[clone.ID] = clone
	m.byKey[clone.IdempotencyKey] = clone.ID
	m.ordered = append(m.ordered, clone)

	return nil
}

// GetByID looks up a payment by ID. Returns ErrNotFound if not found.
func (m *MemoryPaymentRepository) GetByID(_ context.Context, id string) (*payments.Payment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, exists := m.byID[id]
	if !exists {
		return nil, ErrNotFound
	}
	return clonePayment(p), nil
}

// GetByIdempotencyKey looks up a payment by its idempotency key.
func (m *MemoryPaymentRepository) GetByIdempotencyKey(_ context.Context, key string) (*payments.Payment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, exists := m.byKey[key]
	if !exists {
		return nil, ErrNotFound
	}
	return clonePayment(m.byID[id]), nil
}

// UpdateStatus atomically transitions payment status if current status matches expectedOld.
func (m *MemoryPaymentRepository) UpdateStatus(
	_ context.Context,
	id string,
	expectedOld payments.PaymentStatus,
	newStatus payments.PaymentStatus,
	providerRef, failureReason *string,
) (*payments.Payment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, exists := m.byID[id]
	if !exists {
		return nil, ErrNotFound
	}

	if p.Status != expectedOld {
		return nil, fmt.Errorf("%w: current status %s does not match expected %s",
			ErrConflict, p.Status, expectedOld)
	}

	p.Status = newStatus
	if providerRef != nil {
		p.ProviderRef = providerRef
	}
	p.FailureReason = failureReason
	p.UpdatedAt = time.Now().UTC()

	return clonePayment(p), nil
}

// List returns payments matching filter criteria with cursor pagination.
func (m *MemoryPaymentRepository) List(_ context.Context, filter payments.ListFilter) ([]*payments.Payment, string, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 1. Filter matching items
	var filtered []*payments.Payment
	for _, p := range m.ordered {
		if filter.Status != nil && p.Status != *filter.Status {
			continue
		}
		filtered = append(filtered, clonePayment(p))
	}

	// 2. Sort descending: created_at DESC, id DESC
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].CreatedAt.Equal(filtered[j].CreatedAt) {
			return filtered[i].ID > filtered[j].ID
		}
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	total := int64(len(filtered))

	// 3. Apply cursor pagination
	startIndex := 0
	if filter.Cursor != "" {
		for i, p := range filtered {
			if p.ID == filter.Cursor {
				startIndex = i + 1
				break
			}
		}
	}

	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	endIndex := startIndex + limit
	if startIndex >= len(filtered) {
		return []*payments.Payment{}, "", total, nil
	}
	if endIndex > len(filtered) {
		endIndex = len(filtered)
	}

	result := filtered[startIndex:endIndex]
	var nextCursor string
	if endIndex < len(filtered) && len(result) > 0 {
		nextCursor = result[len(result)-1].ID
	}

	return result, nextCursor, total, nil
}

// CreateRefund stores a refund verifying payment existence, SUCCEEDED status, and available amount.
func (m *MemoryPaymentRepository) CreateRefund(_ context.Context, r *payments.Refund) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, exists := m.byID[r.PaymentID]
	if !exists {
		return ErrNotFound
	}

	if p.Status != payments.StatusSucceeded {
		return payments.ErrRefundInvalidPaymentStatus
	}

	var totalRefunded int64
	for _, ref := range m.refundsByPayment[r.PaymentID] {
		if ref.Status != payments.StatusRefundFailed {
			totalRefunded += ref.Amount
		}
	}

	if totalRefunded+r.Amount > p.Amount {
		return payments.ErrRefundAmountExceeded
	}

	clone := &payments.Refund{
		ID:             r.ID,
		PaymentID:      r.PaymentID,
		Amount:         r.Amount,
		Status:         r.Status,
		Reason:         r.Reason,
		IdempotencyKey: r.IdempotencyKey,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}

	m.refundsByID[clone.ID] = clone
	m.refundsByPayment[clone.PaymentID] = append(m.refundsByPayment[clone.PaymentID], clone)

	return nil
}

// GetRefundByID retrieves a refund by its ID.
func (m *MemoryPaymentRepository) GetRefundByID(_ context.Context, id string) (*payments.Refund, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	r, exists := m.refundsByID[id]
	if !exists {
		return nil, ErrNotFound
	}
	clone := *r
	return &clone, nil
}

// ListRefundsByPaymentID retrieves all refunds for a specific payment.
func (m *MemoryPaymentRepository) ListRefundsByPaymentID(_ context.Context, paymentID string) ([]*payments.Refund, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list, exists := m.refundsByPayment[paymentID]
	if !exists {
		return []*payments.Refund{}, nil
	}

	res := make([]*payments.Refund, len(list))
	for i, r := range list {
		clone := *r
		res[i] = &clone
	}
	return res, nil
}

// Helper to deep copy a Payment struct to protect internal state.
func clonePayment(src *payments.Payment) *payments.Payment {
	dst := *src
	if src.Metadata != nil {
		metaCopy := make(map[string]any, len(src.Metadata))
		for k, v := range src.Metadata {
			metaCopy[k] = v
		}
		dst.Metadata = metaCopy
	}
	return &dst
}

