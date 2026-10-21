package outbox

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// Publisher emits events to message brokers.
type Publisher interface {
	Publish(ctx context.Context, routingKey string, payload []byte) error
}

// Relay periodically sweeps unpublished outbox events and dispatches them to the broker.
// This guarantees at-least-once message delivery without distributed two-phase commits.
type Relay struct {
	repo      Repository
	publisher Publisher
	interval  time.Duration
	batchSize int
	log       zerolog.Logger
}

// NewRelay constructs an Outbox Relay instance.
func NewRelay(repo Repository, pub Publisher, interval time.Duration, batchSize int, log zerolog.Logger) *Relay {
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	return &Relay{
		repo:      repo,
		publisher: pub,
		interval:  interval,
		batchSize: batchSize,
		log:       log.With().Str("component", "outbox-relay").Logger(),
	}
}

// ProcessBatch fetches and publishes a single batch of unpublished events.
func (r *Relay) ProcessBatch(ctx context.Context) (int, error) {
	events, err := r.repo.FetchUnpublished(ctx, r.batchSize)
	if err != nil {
		r.log.Error().Err(err).Msg("failed to fetch unpublished outbox events")
		return 0, err
	}

	if len(events) == 0 {
		return 0, nil
	}

	publishedCount := 0
	for _, event := range events {
		select {
		case <-ctx.Done():
			return publishedCount, ctx.Err()
		default:
		}

		if err := r.publisher.Publish(ctx, event.EventType, event.Payload); err != nil {
			r.log.Warn().
				Err(err).
				Int64("event_id", event.ID).
				Str("event_type", event.EventType).
				Msg("failed to publish outbox event to broker, will retry next tick")
			break
		}

		if err := r.repo.MarkPublished(ctx, event.ID); err != nil {
			r.log.Error().
				Err(err).
				Int64("event_id", event.ID).
				Msg("failed to mark outbox event as published")
			continue
		}

		publishedCount++
		r.log.Debug().
			Int64("event_id", event.ID).
			Str("event_type", event.EventType).
			Msg("outbox event published successfully")
	}

	return publishedCount, nil
}

// Start runs the relay polling loop until the context is cancelled.
func (r *Relay) Start(ctx context.Context) {
	r.log.Info().
		Dur("interval", r.interval).
		Int("batch_size", r.batchSize).
		Msg("starting outbox relay loop")

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.log.Info().Msg("stopping outbox relay loop")
			return
		case <-ticker.C:
			_, _ = r.ProcessBatch(ctx)
		}
	}
}
