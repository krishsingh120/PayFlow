package ledger

import (
	"context"
	"sync"
	"time"
)

// MemoryRepository provides a thread-safe in-memory store for ledger entries.
type MemoryRepository struct {
	mu      sync.RWMutex
	nextID  int64
	entries []*Entry
}

// NewMemoryRepository constructs an empty in-memory ledger repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		nextID:  1,
		entries: make([]*Entry, 0),
	}
}

// RecordTransaction atomically appends a balanced transaction to the ledger.
func (m *MemoryRepository) RecordTransaction(_ context.Context, entries []*Entry) error {
	if err := VerifyInvariant(entries); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	for _, e := range entries {
		clone := &Entry{
			ID:        m.nextID,
			PaymentID: e.PaymentID,
			Account:   e.Account,
			Direction: e.Direction,
			Amount:    e.Amount,
			CreatedAt: now,
		}
		m.nextID++
		m.entries = append(m.entries, clone)
	}

	return nil
}

// GetEntriesByPaymentID returns all entries associated with a payment.
func (m *MemoryRepository) GetEntriesByPaymentID(_ context.Context, paymentID string) ([]*Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Entry
	for _, e := range m.entries {
		if e.PaymentID == paymentID {
			clone := *e
			result = append(result, &clone)
		}
	}
	return result, nil
}

// GetAccountBalance calculates the net balance for a specific account.
// Asset accounts (receivable) balance = DEBIT - CREDIT.
// Income accounts (revenue) balance = CREDIT - DEBIT.
func (m *MemoryRepository) GetAccountBalance(_ context.Context, account string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var debits, credits int64
	for _, e := range m.entries {
		if e.Account == account {
			switch e.Direction {
			case DirectionDebit:
				debits += e.Amount
			case DirectionCredit:
				credits += e.Amount
			}
		}
	}

	if account == AccountRevenue {
		return credits - debits, nil
	}
	return debits - credits, nil
}

// GetTotalDebitAndCredit sums all debits and credits in the entire journal.
func (m *MemoryRepository) GetTotalDebitAndCredit(_ context.Context) (int64, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalDebit, totalCredit int64
	for _, e := range m.entries {
		switch e.Direction {
		case DirectionDebit:
			totalDebit += e.Amount
		case DirectionCredit:
			totalCredit += e.Amount
		}
	}
	return totalDebit, totalCredit, nil
}

// ListAllEntries returns recent ledger entries up to limit.
func (m *MemoryRepository) ListAllEntries(_ context.Context, limit int) ([]*Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.entries) {
		limit = len(m.entries)
	}

	result := make([]*Entry, limit)
	for i := 0; i < limit; i++ {
		// Reverse order (most recent first)
		idx := len(m.entries) - 1 - i
		clone := *m.entries[idx]
		result[i] = &clone
	}
	return result, nil
}
