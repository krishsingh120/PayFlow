// Package messaging provides asynchronous message publishing and consumption abstractions.
package messaging

import (
	"context"
	"time"
)

const (
	// ExchangePayments is the topic exchange for domain payment events.
	ExchangePayments = "payflow.events"

	// QueuePaymentProcessing is the queue consumed by workers for payment authorization.
	QueuePaymentProcessing = "payment.processing.queue"

	// RoutingKeyPaymentCreated routes new payment creation jobs to the processing queue.
	RoutingKeyPaymentCreated = "payment.created"

	// DLXExchange is the dead-letter exchange for unprocessable or failed messages.
	DLXExchange = "payflow.dlx"

	// DLXQueue is the dead-letter queue.
	DLXQueue = "payment.dlq"
)

// PaymentJob represents the message payload sent to the worker to process a payment.
type PaymentJob struct {
	PaymentID string    `json:"payment_id"`
	CreatedAt time.Time `json:"created_at"`
}

// Publisher defines message publishing capability.
type Publisher interface {
	Publish(ctx context.Context, routingKey string, payload []byte) error
}

// Consumer defines message consumption capability.
type Consumer interface {
	Consume(ctx context.Context, queueName string, handler func(ctx context.Context, msg []byte) error) error
	Close() error
}
