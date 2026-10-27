package ledger

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository provides a PostgreSQL-backed store for double-entry records.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository constructs a new PostgreSQL ledger repository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// RecordTransaction persists a balanced set of entries inside a single SQL transaction.
func (r *PostgresRepository) RecordTransaction(ctx context.Context, entries []*Entry) error {
	if err := VerifyInvariant(entries); err != nil {
		return err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin tx for ledger: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := `
		INSERT INTO ledger_entries (payment_id, account, direction, amount, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id, created_at
	`

	for _, e := range entries {
		err := tx.QueryRow(ctx, query, e.PaymentID, e.Account, string(e.Direction), e.Amount).
			Scan(&e.ID, &e.CreatedAt)
		if err != nil {
			return fmt.Errorf("postgres: insert ledger entry failed: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// GetEntriesByPaymentID returns all ledger records for a specific payment.
func (r *PostgresRepository) GetEntriesByPaymentID(ctx context.Context, paymentID string) ([]*Entry, error) {
	query := `
		SELECT id, payment_id, account, direction, amount, created_at
		FROM ledger_entries
		WHERE payment_id = $1
		ORDER BY id ASC
	`
	rows, err := r.pool.Query(ctx, query, paymentID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query ledger entries failed: %w", err)
	}
	defer rows.Close()

	var entries []*Entry
	for rows.Next() {
		var e Entry
		var dirStr string
		if err := rows.Scan(&e.ID, &e.PaymentID, &e.Account, &dirStr, &e.Amount, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("postgres: scan ledger entry failed: %w", err)
		}
		e.Direction = Direction(dirStr)
		entries = append(entries, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate ledger entries failed: %w", err)
	}

	return entries, nil
}

// GetAccountBalance calculates the balance of an account according to normal balance rules.
func (r *PostgresRepository) GetAccountBalance(ctx context.Context, account string) (int64, error) {
	query := `
		SELECT
			COALESCE(SUM(CASE WHEN direction = 'DEBIT' THEN amount ELSE 0 END), 0) AS total_debit,
			COALESCE(SUM(CASE WHEN direction = 'CREDIT' THEN amount ELSE 0 END), 0) AS total_credit
		FROM ledger_entries
		WHERE account = $1
	`
	var totalDebit, totalCredit int64
	err := r.pool.QueryRow(ctx, query, account).Scan(&totalDebit, &totalCredit)
	if err != nil {
		return 0, fmt.Errorf("postgres: calculate account balance failed: %w", err)
	}

	if account == AccountRevenue {
		return totalCredit - totalDebit, nil
	}
	return totalDebit - totalCredit, nil
}

// GetTotalDebitAndCredit returns global debit and credit sums across all accounts.
func (r *PostgresRepository) GetTotalDebitAndCredit(ctx context.Context) (int64, int64, error) {
	query := `
		SELECT
			COALESCE(SUM(CASE WHEN direction = 'DEBIT' THEN amount ELSE 0 END), 0) AS total_debit,
			COALESCE(SUM(CASE WHEN direction = 'CREDIT' THEN amount ELSE 0 END), 0) AS total_credit
		FROM ledger_entries
	`
	var totalDebit, totalCredit int64
	err := r.pool.QueryRow(ctx, query).Scan(&totalDebit, &totalCredit)
	if err != nil {
		return 0, 0, fmt.Errorf("postgres: calculate total debit and credit failed: %w", err)
	}
	return totalDebit, totalCredit, nil
}

// ListAllEntries retrieves recent entries across all accounts.
func (r *PostgresRepository) ListAllEntries(ctx context.Context, limit int) ([]*Entry, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT id, payment_id, account, direction, amount, created_at
		FROM ledger_entries
		ORDER BY id DESC
		LIMIT $1
	`
	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: list all ledger entries failed: %w", err)
	}
	defer rows.Close()

	var entries []*Entry
	for rows.Next() {
		var e Entry
		var dirStr string
		if err := rows.Scan(&e.ID, &e.PaymentID, &e.Account, &dirStr, &e.Amount, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("postgres: scan ledger entry row failed: %w", err)
		}
		e.Direction = Direction(dirStr)
		entries = append(entries, &e)
	}

	return entries, nil
}
