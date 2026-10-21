package outbox

import (
	"context"
	"errors"
	"sync"
	"time"
)

// MemoryRepository provides a thread-safe in-memory implementation of outbox.Repository.
type MemoryRepository struct {
	mu     sync.RWMutex
	nextID int64
	events []*Event
}

// NewMemoryRepository returns a new in-memory outbox repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		nextID: 1,
		events: make([]*Event, 0),
	}
}

// Store records an event in the outbox.
func (m *MemoryRepository) Store(_ context.Context, aggregateID, eventType string, payload []byte) (*Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pCopy := make([]byte, len(payload))
	copy(pCopy, payload)

	evt := &Event{
		ID:          m.nextID,
		AggregateID: aggregateID,
		EventType:   eventType,
		Payload:     pCopy,
		Published:   false,
		CreatedAt:   time.Now().UTC(),
	}
	m.nextID++
	m.events = append(m.events, evt)

	return cloneEvent(evt), nil
}

// FetchUnpublished returns up to limit unpublished events.
func (m *MemoryRepository) FetchUnpublished(_ context.Context, limit int) ([]*Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Event
	for _, e := range m.events {
		if !e.Published {
			result = append(result, cloneEvent(e))
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

// MarkPublished marks an event as published.
func (m *MemoryRepository) MarkPublished(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, e := range m.events {
		if e.ID == id {
			e.Published = true
			return nil
		}
	}
	return errors.New("outbox event not found")
}

func cloneEvent(src *Event) *Event {
	p := make([]byte, len(src.Payload))
	copy(p, src.Payload)
	return &Event{
		ID:          src.ID,
		AggregateID: src.AggregateID,
		EventType:   src.EventType,
		Payload:     p,
		Published:   src.Published,
		CreatedAt:   src.CreatedAt,
	}
}
