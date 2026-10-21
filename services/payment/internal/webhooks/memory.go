package webhooks

import (
	"context"
	"errors"
	"sync"
	"time"
)

// MemoryRepository provides a thread-safe in-memory store for webhook deliveries.
type MemoryRepository struct {
	mu         sync.RWMutex
	deliveries map[string]*Delivery
	ordered    []*Delivery
}

// NewMemoryRepository constructs an empty in-memory webhook repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		deliveries: make(map[string]*Delivery),
		ordered:    make([]*Delivery, 0),
	}
}

// CreateDelivery stores a new delivery record.
func (m *MemoryRepository) CreateDelivery(_ context.Context, d *Delivery) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	clone := cloneDelivery(d)
	m.deliveries[clone.ID] = clone
	m.ordered = append(m.ordered, clone)
	return nil
}

// GetPendingDeliveries retrieves pending deliveries whose next_retry_at is <= now.
func (m *MemoryRepository) GetPendingDeliveries(_ context.Context, limit int) ([]*Delivery, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now().UTC()
	var pending []*Delivery

	for _, d := range m.ordered {
		if d.Status == StatusPending {
			if d.NextRetryAt == nil || !d.NextRetryAt.After(now) {
				pending = append(pending, cloneDelivery(d))
				if limit > 0 && len(pending) >= limit {
					break
				}
			}
		}
	}

	return pending, nil
}

// UpdateDelivery updates the status, attempts, retry timestamp, and error string.
func (m *MemoryRepository) UpdateDelivery(_ context.Context, id string, status WebhookStatus, attempts int, nextRetry *time.Time, lastError *string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	d, exists := m.deliveries[id]
	if !exists {
		return errors.New("webhook delivery not found")
	}

	d.Status = status
	d.Attempts = attempts
	d.NextRetryAt = nextRetry
	d.LastError = lastError
	d.UpdatedAt = time.Now().UTC()

	return nil
}

// ListByPaymentID returns all webhook deliveries for a specific payment.
func (m *MemoryRepository) ListByPaymentID(_ context.Context, paymentID string) ([]*Delivery, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Delivery
	for _, d := range m.ordered {
		if d.PaymentID == paymentID {
			result = append(result, cloneDelivery(d))
		}
	}
	return result, nil
}

func cloneDelivery(src *Delivery) *Delivery {
	p := make([]byte, len(src.Payload))
	copy(p, src.Payload)
	dst := *src
	dst.Payload = p
	return &dst
}
