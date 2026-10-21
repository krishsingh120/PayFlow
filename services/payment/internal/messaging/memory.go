package messaging

import (
	"context"
	"sync"
)

// MemoryBroker provides an in-memory, thread-safe implementation of Publisher and Consumer.
// It is ideal for local development without RabbitMQ, and enables sub-millisecond unit tests.
type MemoryBroker struct {
	mu          sync.RWMutex
	subscribers map[string][]func(ctx context.Context, msg []byte) error
}

// NewMemoryBroker returns an initialized in-memory broker.
func NewMemoryBroker() *MemoryBroker {
	return &MemoryBroker{
		subscribers: make(map[string][]func(ctx context.Context, msg []byte) error),
	}
}

// Publish delivers the payload to all subscribers registered for the routing key or queue.
func (m *MemoryBroker) Publish(ctx context.Context, routingKey string, payload []byte) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Clone payload to ensure isolation
	msgCopy := make([]byte, len(payload))
	copy(msgCopy, payload)

	// Check direct queue subscribers or routing key subscribers
	if handlers, exists := m.subscribers[routingKey]; exists {
		for _, h := range handlers {
			go func(handler func(context.Context, []byte) error) {
				_ = handler(ctx, msgCopy)
			}(h)
		}
	}

	// Also dispatch to QueuePaymentProcessing if routingKey is payment.created
	if routingKey == RoutingKeyPaymentCreated {
		if handlers, exists := m.subscribers[QueuePaymentProcessing]; exists {
			for _, h := range handlers {
				go func(handler func(context.Context, []byte) error) {
					_ = handler(ctx, msgCopy)
				}(h)
			}
		}
	}

	return nil
}

// Consume registers a handler for messages dispatched to the specified queue.
func (m *MemoryBroker) Consume(_ context.Context, queueName string, handler func(ctx context.Context, msg []byte) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.subscribers[queueName] = append(m.subscribers[queueName], handler)
	return nil
}

// Close is a no-op for the in-memory broker.
func (m *MemoryBroker) Close() error {
	return nil
}
