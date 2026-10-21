package webhooks

import (
	"context"
	"time"
)

// Repository defines storage contracts for webhook delivery lifecycle.
type Repository interface {
	CreateDelivery(ctx context.Context, d *Delivery) error
	GetPendingDeliveries(ctx context.Context, limit int) ([]*Delivery, error)
	UpdateDelivery(ctx context.Context, id string, status WebhookStatus, attempts int, nextRetry *time.Time, lastError *string) error
	ListByPaymentID(ctx context.Context, paymentID string) ([]*Delivery, error)
}
