package webhooks

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// Dispatcher manages outbound webhook execution, signature generation, and exponential backoff retries.
type Dispatcher struct {
	repo          Repository
	httpClient    *http.Client
	signingSecret string
	maxAttempts   int
	interval      time.Duration
	log           zerolog.Logger
}

// Config holds Dispatcher settings.
type Config struct {
	SigningSecret string
	MaxAttempts   int
	PollInterval  time.Duration
	Timeout       time.Duration
}

// NewDispatcher constructs a new Dispatcher.
func NewDispatcher(repo Repository, cfg Config, log zerolog.Logger) *Dispatcher {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 4
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 1 * time.Second
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.SigningSecret == "" {
		cfg.SigningSecret = "whsec_test_secret_development_mode"
	}

	return &Dispatcher{
		repo: repo,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		signingSecret: cfg.SigningSecret,
		maxAttempts:   cfg.MaxAttempts,
		interval:      cfg.PollInterval,
		log:           log.With().Str("component", "webhook-dispatcher").Logger(),
	}
}

// CalculateBackoff returns the retry delay based on the attempt count.
func CalculateBackoff(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 5 * time.Second
	case 2:
		return 30 * time.Second
	case 3:
		return 2 * time.Minute
	case 4:
		return 10 * time.Minute
	default:
		return 30 * time.Minute
	}
}

// DispatchDelivery sends an HTTP POST request for a single webhook delivery record.
func (d *Dispatcher) DispatchDelivery(ctx context.Context, delivery *Delivery) error {
	now := time.Now().UTC()
	timestamp := now.Unix()
	sigHeader := Sign(d.signingSecret, timestamp, delivery.Payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, delivery.URL, bytes.NewReader(delivery.Payload))
	if err != nil {
		errMsg := fmt.Sprintf("failed to construct request: %v", err)
		return d.handleFailure(ctx, delivery, errMsg)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-PayFlow-Signature", sigHeader)
	req.Header.Set("X-PayFlow-Event", delivery.EventType)
	req.Header.Set("X-PayFlow-Delivery", delivery.ID)
	req.Header.Set("X-PayFlow-Timestamp", fmt.Sprintf("%d", timestamp))

	resp, err := d.httpClient.Do(req)
	if err != nil {
		errMsg := fmt.Sprintf("http transport error: %v", err)
		return d.handleFailure(ctx, delivery, errMsg)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		d.log.Info().
			Str("delivery_id", delivery.ID).
			Str("event_type", delivery.EventType).
			Int("status_code", resp.StatusCode).
			Msg("webhook delivered successfully")
		return d.repo.UpdateDelivery(ctx, delivery.ID, StatusDelivered, delivery.Attempts+1, nil, nil)
	}

	// Non-2xx response status
	bodySnippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
	errMsg := fmt.Sprintf("server responded with status %d: %s", resp.StatusCode, string(bodySnippet))
	return d.handleFailure(ctx, delivery, errMsg)
}

func (d *Dispatcher) handleFailure(ctx context.Context, delivery *Delivery, errMsg string) error {
	newAttempts := delivery.Attempts + 1
	d.log.Warn().
		Str("delivery_id", delivery.ID).
		Int("attempt", newAttempts).
		Str("error", errMsg).
		Msg("webhook delivery failed")

	if newAttempts >= d.maxAttempts {
		d.log.Error().
			Str("delivery_id", delivery.ID).
			Int("attempts", newAttempts).
			Msg("webhook delivery exhausted maximum retries — marked FAILED")
		return d.repo.UpdateDelivery(ctx, delivery.ID, StatusFailed, newAttempts, nil, &errMsg)
	}

	nextRetry := time.Now().UTC().Add(CalculateBackoff(newAttempts))
	return d.repo.UpdateDelivery(ctx, delivery.ID, StatusPending, newAttempts, &nextRetry, &errMsg)
}

// ProcessPendingDeliveries queries and dispatches pending deliveries.
func (d *Dispatcher) ProcessPendingDeliveries(ctx context.Context, limit int) (int, error) {
	deliveries, err := d.repo.GetPendingDeliveries(ctx, limit)
	if err != nil {
		d.log.Error().Err(err).Msg("failed to query pending webhook deliveries")
		return 0, err
	}

	count := 0
	for _, item := range deliveries {
		select {
		case <-ctx.Done():
			return count, ctx.Err()
		default:
		}

		if err := d.DispatchDelivery(ctx, item); err == nil {
			count++
		}
	}

	return count, nil
}

// Start runs the background dispatch polling loop.
func (d *Dispatcher) Start(ctx context.Context) {
	d.log.Info().
		Dur("interval", d.interval).
		Int("max_attempts", d.maxAttempts).
		Msg("starting webhook dispatcher loop")

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.log.Info().Msg("stopping webhook dispatcher loop")
			return
		case <-ticker.C:
			_, _ = d.ProcessPendingDeliveries(ctx, 20)
		}
	}
}
