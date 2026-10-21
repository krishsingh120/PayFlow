package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/krishsingh120/payflow/services/payment/internal/payments"
)

// PostgresPaymentRepository implements PaymentRepository using PostgreSQL with pgx/v5.
type PostgresPaymentRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresPaymentRepository creates a new repository using the supplied connection pool.
func NewPostgresPaymentRepository(pool *pgxpool.Pool) *PostgresPaymentRepository {
	return &PostgresPaymentRepository{pool: pool}
}

// Create inserts a new payment record into PostgreSQL.
// Returns ErrDuplicateKey if the idempotency_key violates the unique constraint.
func (r *PostgresPaymentRepository) Create(ctx context.Context, p *payments.Payment) error {
	query := `
		INSERT INTO payments (
			id, idempotency_key, amount, currency, status,
			description, metadata, provider_ref, failure_reason,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	var metaBytes []byte
	if p.Metadata != nil {
		var err error
		metaBytes, err = json.Marshal(p.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	_, err := r.pool.Exec(ctx, query,
		p.ID,
		p.IdempotencyKey,
		p.Amount,
		p.Currency,
		string(p.Status),
		p.Description,
		metaBytes,
		p.ProviderRef,
		p.FailureReason,
		p.CreatedAt,
		p.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateKey
		}
		return fmt.Errorf("postgres: insert payment failed: %w", err)
	}

	return nil
}

// GetByID retrieves a payment by its ULID identifier.
func (r *PostgresPaymentRepository) GetByID(ctx context.Context, id string) (*payments.Payment, error) {
	query := `
		SELECT
			id, idempotency_key, amount, currency, status,
			description, metadata, provider_ref, failure_reason,
			created_at, updated_at
		FROM payments
		WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, query, id)
	return scanPayment(row)
}

// GetByIdempotencyKey retrieves a payment by its unique idempotency key.
func (r *PostgresPaymentRepository) GetByIdempotencyKey(ctx context.Context, key string) (*payments.Payment, error) {
	query := `
		SELECT
			id, idempotency_key, amount, currency, status,
			description, metadata, provider_ref, failure_reason,
			created_at, updated_at
		FROM payments
		WHERE idempotency_key = $1
	`
	row := r.pool.QueryRow(ctx, query, key)
	return scanPayment(row)
}

// UpdateStatus performs an atomic status transition using optimistic concurrency.
func (r *PostgresPaymentRepository) UpdateStatus(
	ctx context.Context,
	id string,
	expectedOld payments.PaymentStatus,
	newStatus payments.PaymentStatus,
	providerRef, failureReason *string,
) (*payments.Payment, error) {
	query := `
		UPDATE payments
		SET status = $1,
		    provider_ref = COALESCE($2, provider_ref),
		    failure_reason = $3,
		    updated_at = NOW()
		WHERE id = $4 AND status = $5
		RETURNING
			id, idempotency_key, amount, currency, status,
			description, metadata, provider_ref, failure_reason,
			created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query, string(newStatus), providerRef, failureReason, id, string(expectedOld))
	updated, err := scanPayment(row)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Check whether the payment exists at all
			existing, checkErr := r.GetByID(ctx, id)
			if checkErr != nil {
				return nil, checkErr
			}
			return nil, fmt.Errorf("%w: current status %s does not match expected %s",
				ErrConflict, existing.Status, expectedOld)
		}
		return nil, err
	}

	return updated, nil
}

// List returns a paginated list of payments, optionally filtered by status.
func (r *PostgresPaymentRepository) List(ctx context.Context, filter payments.ListFilter) ([]*payments.Payment, string, int64, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// 1. Fetch total count matching criteria
	countQuery := `SELECT COUNT(*) FROM payments WHERE ($1::text IS NULL OR status = $1)`
	var statusParam *string
	if filter.Status != nil {
		s := string(*filter.Status)
		statusParam = &s
	}

	var total int64
	if err := r.pool.QueryRow(ctx, countQuery, statusParam).Scan(&total); err != nil {
		return nil, "", 0, fmt.Errorf("postgres: count payments failed: %w", err)
	}

	// 2. Fetch records with cursor pagination
	query := `
		SELECT
			id, idempotency_key, amount, currency, status,
			description, metadata, provider_ref, failure_reason,
			created_at, updated_at
		FROM payments
		WHERE ($1::text IS NULL OR status = $1)
		  AND ($2::text IS NULL OR created_at < (SELECT created_at FROM payments WHERE id = $2))
		ORDER BY created_at DESC, id DESC
		LIMIT $3
	`

	var cursorParam *string
	if filter.Cursor != "" {
		cursorParam = &filter.Cursor
	}

	rows, err := r.pool.Query(ctx, query, statusParam, cursorParam, limit+1)
	if err != nil {
		return nil, "", 0, fmt.Errorf("postgres: list payments query failed: %w", err)
	}
	defer rows.Close()

	items := make([]*payments.Payment, 0, limit)
	for rows.Next() {
		p, err := scanPaymentRow(rows)
		if err != nil {
			return nil, "", 0, err
		}
		items = append(items, p)
	}

	var nextCursor string
	if len(items) > limit {
		nextCursor = items[limit-1].ID
		items = items[:limit]
	}

	return items, nextCursor, total, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanPayment(row scannable) (*payments.Payment, error) {
	var (
		p         payments.Payment
		statusStr string
		metaBytes []byte
	)

	err := row.Scan(
		&p.ID,
		&p.IdempotencyKey,
		&p.Amount,
		&p.Currency,
		&statusStr,
		&p.Description,
		&metaBytes,
		&p.ProviderRef,
		&p.FailureReason,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan payment failed: %w", err)
	}

	p.Status = payments.PaymentStatus(statusStr)

	if len(metaBytes) > 0 {
		var meta map[string]any
		if err := json.Unmarshal(metaBytes, &meta); err == nil {
			p.Metadata = meta
		}
	}

	return &p, nil
}

func scanPaymentRow(rows pgx.Rows) (*payments.Payment, error) {
	var (
		p         payments.Payment
		statusStr string
		metaBytes []byte
	)

	err := rows.Scan(
		&p.ID,
		&p.IdempotencyKey,
		&p.Amount,
		&p.Currency,
		&statusStr,
		&p.Description,
		&metaBytes,
		&p.ProviderRef,
		&p.FailureReason,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan row failed: %w", err)
	}

	p.Status = payments.PaymentStatus(statusStr)

	if len(metaBytes) > 0 {
		var meta map[string]any
		if err := json.Unmarshal(metaBytes, &meta); err == nil {
			p.Metadata = meta
		}
	}

	return &p, nil
}
