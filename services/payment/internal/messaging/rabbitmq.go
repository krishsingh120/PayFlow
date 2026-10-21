package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
)

// RabbitMQBroker implements Publisher and Consumer backed by RabbitMQ.
type RabbitMQBroker struct {
	url    string
	logger zerolog.Logger

	mu   sync.Mutex
	conn *amqp.Connection
	ch   *amqp.Channel
}

// NewRabbitMQBroker connects to RabbitMQ and declares the exchange and queues topology.
func NewRabbitMQBroker(url string, logger zerolog.Logger) (*RabbitMQBroker, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: dial failed: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("rabbitmq: open channel failed: %w", err)
	}

	b := &RabbitMQBroker{
		url:    url,
		logger: logger,
		conn:   conn,
		ch:     ch,
	}

	if err := b.setupTopology(); err != nil {
		_ = b.Close()
		return nil, fmt.Errorf("rabbitmq: setup topology failed: %w", err)
	}

	return b, nil
}

func (b *RabbitMQBroker) setupTopology() error {
	// 1. Declare Dead-Letter Exchange & Queue
	if err := b.ch.ExchangeDeclare(
		DLXExchange,
		"fanout",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,
	); err != nil {
		return fmt.Errorf("declare DLX exchange failed: %w", err)
	}

	if _, err := b.ch.QueueDeclare(
		DLXQueue,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	); err != nil {
		return fmt.Errorf("declare DLX queue failed: %w", err)
	}

	if err := b.ch.QueueBind(
		DLXQueue,
		"",
		DLXExchange,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("bind DLX queue failed: %w", err)
	}

	// 2. Declare Main Events Topic Exchange
	if err := b.ch.ExchangeDeclare(
		ExchangePayments,
		"topic",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,
	); err != nil {
		return fmt.Errorf("declare events exchange failed: %w", err)
	}

	// 3. Declare Payment Processing Queue with Dead-Letter configuration
	args := amqp.Table{
		"x-dead-letter-exchange": DLXExchange,
	}
	if _, err := b.ch.QueueDeclare(
		QueuePaymentProcessing,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		args,
	); err != nil {
		return fmt.Errorf("declare payment processing queue failed: %w", err)
	}

	// 4. Bind queue to topic exchange
	if err := b.ch.QueueBind(
		QueuePaymentProcessing,
		RoutingKeyPaymentCreated,
		ExchangePayments,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("bind processing queue failed: %w", err)
	}

	b.logger.Info().
		Str("exchange", ExchangePayments).
		Str("queue", QueuePaymentProcessing).
		Msg("rabbitmq topology initialized")

	return nil
}

// Publish sends a message to the payment events exchange with the specified routing key.
func (b *RabbitMQBroker) Publish(ctx context.Context, routingKey string, payload []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	msg := amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent, // persistent messages survive broker restarts
		Timestamp:    time.Now().UTC(),
		Body:         payload,
	}

	return b.ch.PublishWithContext(
		ctx,
		ExchangePayments,
		routingKey,
		false, // mandatory
		false, // immediate
		msg,
	)
}

// Consume starts consuming messages from the specified queue and passes them to the handler.
func (b *RabbitMQBroker) Consume(
	ctx context.Context,
	queueName string,
	handler func(ctx context.Context, msg []byte) error,
) error {
	msgs, err := b.ch.Consume(
		queueName,
		"",    // consumer tag (auto-generated)
		false, // auto-ack: false for reliable manual acknowledgment
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("rabbitmq: consume failed: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case delivery, ok := <-msgs:
				if !ok {
					b.logger.Warn().Msg("rabbitmq consumer delivery channel closed")
					return
				}

				if err := handler(ctx, delivery.Body); err != nil {
					b.logger.Error().Err(err).Msg("error processing message — routing to DLQ")
					// Nack with requeue=false sends directly to DLX
					_ = delivery.Nack(false, false)
				} else {
					_ = delivery.Ack(false)
				}
			}
		}
	}()

	return nil
}

// Close gracefully closes the channel and connection.
func (b *RabbitMQBroker) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	var firstErr error
	if b.ch != nil {
		if err := b.ch.Close(); err != nil {
			firstErr = err
		}
	}
	if b.conn != nil {
		if err := b.conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
