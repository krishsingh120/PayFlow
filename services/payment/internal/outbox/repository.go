package outbox

import "context"

// Repository defines operations for persisting and querying outbox events.
type Repository interface {
	// Store persists an outbox event.
	Store(ctx context.Context, aggregateID, eventType string, payload []byte) (*Event, error)

	// FetchUnpublished retrieves unpublished events up to limit ordered by creation time.
	FetchUnpublished(ctx context.Context, limit int) ([]*Event, error)

	// MarkPublished updates the event to published = true.
	MarkPublished(ctx context.Context, id int64) error
}
