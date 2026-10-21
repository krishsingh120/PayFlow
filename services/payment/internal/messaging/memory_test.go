package messaging_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/krishsingh120/payflow/services/payment/internal/messaging"
)

func TestMemoryBroker(t *testing.T) {
	ctx := context.Background()
	broker := messaging.NewMemoryBroker()

	t.Run("publishes and delivers to QueuePaymentProcessing", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(1)

		var receivedMsg []byte
		err := broker.Consume(ctx, messaging.QueuePaymentProcessing, func(_ context.Context, msg []byte) error {
			receivedMsg = msg
			wg.Done()
			return nil
		})
		require.NoError(t, err)

		// Publish event with routing key payment.created
		testPayload := []byte(`{"payment_id":"pay_123"}`)
		err = broker.Publish(ctx, messaging.RoutingKeyPaymentCreated, testPayload)
		require.NoError(t, err)

		// Wait for message delivery
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			assert.Equal(t, string(testPayload), string(receivedMsg))
		case <-time.After(1 * time.Second):
			t.Fatal("timed out waiting for in-memory message delivery")
		}
	})
}
