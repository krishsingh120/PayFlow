package webhooks

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository provides a PostgreSQL-backed store for webhook deliveries.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository constructs a new Postgres-backed webhook repository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// CreateDelivery inserts a new delivery record.
func (r *PostgresRepository) CreateDelivery(ctx context.Context, d *Delivery) error {
	query := `
		INSERT INTO webhook_deliveries (
			id, payment_id, event_type, url, payload, status,
			attempts, next_retry_at, last_error, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.pool.Exec(ctx, query,
		d.ID,
		d.PaymentID,
		d.EventType,
		d.URL,
		d.Payload,
		string(d.Status),
		d.Attempts,
		d.NextRetryAt,
		d.LastError,
		d.CreatedAt,
		d.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: insert webhook delivery failed: %w", err)
	}
	return nil
}

// GetPendingDeliveries retrieves pending deliveries whose next_retry_at is <= now or null.
func (r *PostgresRepository) GetPendingDeliveries(ctx context.Context, limit int) ([]*Delivery, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT
			id, payment_id, event_type, url, payload, status,
			attempts, next_retry_at, last_error, created_at, updated_at
		FROM webhook_deliveries
		WHERE status = 'PENDING'
		  AND (next_retry_at IS NULL OR next_retry_at <= NOW())
		ORDER BY created_at ASC
		LIMIT $1
	`
	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: query pending webhooks failed: %w", err)
	}
	defer rows.Close()

	var deliveries []*Delivery
	for rows.Next() {
		var d Delivery
		var statusStr string
		if err := rows.Scan(
			&d.ID,
			&d.PaymentID,
			&d.EventType,
			&d.URL,
			&d.Payload,
			&statusStr,
			&d.Attempts,
			&d.NextRetryAt,
			&d.LastError,
			&d.CreatedAt,
			&d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres: scan webhook row failed: %w", err)
		}
		d.Status = WebhookStatus(statusStr)
		deliveries = append(deliveries, &d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate webhook rows failed: %w", err)
	}

	return deliveries, nil
}

// UpdateDelivery updates the status, retry timestamp, and attempts of a delivery.
func (r *PostgresRepository) UpdateDelivery(ctx context.Context, id string, status WebhookStatus, attempts int, nextRetry *time.Time, lastError *string) error {
	query := `
		UPDATE webhook_deliveries
		SET status = $1, attempts = $2, next_retry_at = $3, last_error = $4, updated_at = NOW()
		WHERE id = $5
	`
	_, err := r.pool.Exec(ctx, query, string(status), attempts, nextRetry, lastError, id)
	if err != nil {
		return fmt.Errorf("postgres: update webhook delivery failed: %w", err)
	}
	return nil
}

// ListByPaymentID queries all deliveries associated with a payment.
func (r *PostgresRepository) ListByPaymentID(ctx context.Context, paymentID string) ([]*Delivery, error) {
	query := `
		SELECT
			id, payment_id, event_type, url, payload, status,
			attempts, next_retry_at, last_error, created_at, updated_at
		FROM webhook_deliveries
		WHERE payment_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, paymentID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list webhooks by payment failed: %w", err)
	}
	defer rows.Close()

	var deliveries []*Delivery
	for rows.Next() {
		var d Delivery
		var statusStr string
		if err := rows.Scan(
			&d.ID,
			&d.PaymentID,
			&d.EventType,
			&d.URL,
			&d.Payload,
			&statusStr,
			&d.Attempts,
			&d.NextRetryAt,
			&d.LastError,
			&d.CreatedAt,
			&d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres: scan webhook row failed: %w", err)
		}
		d.Status = WebhookStatus(statusStr)
		deliveries = append(deliveries, &d)
	}

	return deliveries, nil
}
