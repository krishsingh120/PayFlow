package outbox_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/krishsingh120/payflow/services/payment/internal/outbox"
)

type mockPublisher struct {
	mu        sync.Mutex
	published []publishedMsg
	failNext  bool
}

type publishedMsg struct {
	routingKey string
	payload    []byte
}

func (m *mockPublisher) Publish(_ context.Context, routingKey string, payload []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failNext {
		return errors.New("simulated broker connection failure")
	}

	m.published = append(m.published, publishedMsg{
		routingKey: routingKey,
		payload:    payload,
	})
	return nil
}

func TestOutboxRelay_ProcessBatch(t *testing.T) {
	ctx := context.Background()
	repo := outbox.NewMemoryRepository()
	pub := &mockPublisher{}
	relay := outbox.NewRelay(repo, pub, 100*time.Millisecond, 10, zerolog.Nop())

	// 1. Store 2 events
	e1, err := repo.Store(ctx, "pay_01", "payment.created", []byte(`{"id":"pay_01"}`))
	require.NoError(t, err)
	assert.Equal(t, int64(1), e1.ID)

	e2, err := repo.Store(ctx, "pay_02", "payment.succeeded", []byte(`{"id":"pay_02"}`))
	require.NoError(t, err)
	assert.Equal(t, int64(2), e2.ID)

	// Verify both are unpublished
	unpub, err := repo.FetchUnpublished(ctx, 10)
	require.NoError(t, err)
	assert.Equal(t, 2, len(unpub))

	// 2. ProcessBatch
	count, err := relay.ProcessBatch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify published to broker
	pub.mu.Lock()
	assert.Equal(t, 2, len(pub.published))
	assert.Equal(t, "payment.created", pub.published[0].routingKey)
	assert.Equal(t, "payment.succeeded", pub.published[1].routingKey)
	pub.mu.Unlock()

	// Verify repository has 0 unpublished events left
	unpubAfter, err := repo.FetchUnpublished(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, unpubAfter)
}

func TestOutboxRelay_TransientFailureRetries(t *testing.T) {
	ctx := context.Background()
	repo := outbox.NewMemoryRepository()
	pub := &mockPublisher{failNext: true}
	relay := outbox.NewRelay(repo, pub, 100*time.Millisecond, 10, zerolog.Nop())

	_, err := repo.Store(ctx, "pay_03", "payment.created", []byte(`{"id":"pay_03"}`))
	require.NoError(t, err)

	// First attempt fails due to broker error
	count, err := relay.ProcessBatch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Event is still unpublished (not lost!)
	unpub, err := repo.FetchUnpublished(ctx, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, len(unpub))

	// Broker recovers
	pub.mu.Lock()
	pub.failNext = false
	pub.mu.Unlock()

	// Second attempt succeeds
	count2, err := relay.ProcessBatch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count2)

	unpubAfter, err := repo.FetchUnpublished(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, unpubAfter)
}

func TestOutboxRelay_StartStop(t *testing.T) {
	repo := outbox.NewMemoryRepository()
	pub := &mockPublisher{}
	relay := outbox.NewRelay(repo, pub, 20*time.Millisecond, 10, zerolog.Nop())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		relay.Start(ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Stopped cleanly
	case <-time.After(1 * time.Second):
		t.Fatal("relay did not stop cleanly on context cancellation")
	}
}
