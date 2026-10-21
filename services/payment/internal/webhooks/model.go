package webhooks

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// WebhookStatus represents delivery state.
type WebhookStatus string

const (
	StatusPending   WebhookStatus = "PENDING"
	StatusDelivered WebhookStatus = "DELIVERED"
	StatusFailed    WebhookStatus = "FAILED"
)

var (
	entropyLock sync.Mutex
	entropyPool = ulid.Monotonic(rand.Reader, 0)
)

// NewDeliveryID generates a monotonic ULID prefixed with "whd_".
func NewDeliveryID() string {
	entropyLock.Lock()
	defer entropyLock.Unlock()

	id := ulid.MustNew(ulid.Timestamp(time.Now()), entropyPool)
	return fmt.Sprintf("whd_%s", id.String())
}

// Delivery represents an outbound webhook delivery record.
type Delivery struct {
	ID          string        `json:"id"`
	PaymentID   string        `json:"payment_id"`
	EventType   string        `json:"event_type"`
	URL         string        `json:"url"`
	Payload     []byte        `json:"payload"`
	Status      WebhookStatus `json:"status"`
	Attempts    int           `json:"attempts"`
	NextRetryAt *time.Time    `json:"next_retry_at,omitempty"`
	LastError   *string       `json:"last_error,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}
