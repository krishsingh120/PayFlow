package ledger

import "context"

// Repository defines persistence contracts for double-entry ledger entries.
type Repository interface {
	// RecordTransaction atomically persists a balanced set of entries.
	// Fails if VerifyInvariant fails.
	RecordTransaction(ctx context.Context, entries []*Entry) error

	// GetEntriesByPaymentID retrieves all journal entries for a payment ordered by created_at.
	GetEntriesByPaymentID(ctx context.Context, paymentID string) ([]*Entry, error)

	// GetAccountBalance calculates the net balance for an account (DEBIT - CREDIT or CREDIT - DEBIT depending on normal balance).
	GetAccountBalance(ctx context.Context, account string) (int64, error)

	// GetTotalDebitAndCredit sums all debits and credits across the entire ledger to verify global balance.
	GetTotalDebitAndCredit(ctx context.Context) (totalDebit int64, totalCredit int64, err error)

	// ListAllEntries retrieves recent entries across all accounts.
	ListAllEntries(ctx context.Context, limit int) ([]*Entry, error)
}
