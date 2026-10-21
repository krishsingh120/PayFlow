// Package outbox implements the Transactional Outbox pattern for PayFlow.
// It ensures reliable, at-least-once message delivery to message brokers
// without requiring distributed two-phase commits.
package outbox

import (
	"time"
)

// Event represents an outbox event record.
type Event struct {
	ID          int64     `json:"id"`
	AggregateID string    `json:"aggregate_id"`
	EventType   string    `json:"event_type"`
	Payload     []byte    `json:"payload"`
	Published   bool      `json:"published"`
	CreatedAt   time.Time `json:"created_at"`
}
