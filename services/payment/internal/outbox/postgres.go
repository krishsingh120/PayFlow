package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository provides a PostgreSQL-backed implementation of outbox.Repository.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a new outbox repository backed by Postgres.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Store inserts a new outbox event.
func (r *PostgresRepository) Store(ctx context.Context, aggregateID, eventType string, payload []byte) (*Event, error) {
	query := `
		INSERT INTO outbox_events (
			aggregate_id, event_type, payload, published, created_at
		) VALUES ($1, $2, $3, false, $4)
		RETURNING id
	`
	now := time.Now().UTC()
	var id int64
	err := r.pool.QueryRow(ctx, query, aggregateID, eventType, payload, now).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("postgres: store outbox event failed: %w", err)
	}

	return &Event{
		ID:          id,
		AggregateID: aggregateID,
		EventType:   eventType,
		Payload:     payload,
		Published:   false,
		CreatedAt:   now,
	}, nil
}

// FetchUnpublished queries up to limit unpublished events.
func (r *PostgresRepository) FetchUnpublished(ctx context.Context, limit int) ([]*Event, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT id, aggregate_id, event_type, payload, published, created_at
		FROM outbox_events
		WHERE published = FALSE
		ORDER BY created_at ASC, id ASC
		LIMIT $1
	`
	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: fetch unpublished outbox events failed: %w", err)
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(
			&e.ID,
			&e.AggregateID,
			&e.EventType,
			&e.Payload,
			&e.Published,
			&e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres: scan outbox row failed: %w", err)
		}
		events = append(events, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate outbox rows failed: %w", err)
	}

	return events, nil
}

// MarkPublished updates the outbox event to published = TRUE.
func (r *PostgresRepository) MarkPublished(ctx context.Context, id int64) error {
	query := `UPDATE outbox_events SET published = TRUE WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("postgres: mark outbox event published failed: %w", err)
	}
	return nil
}
