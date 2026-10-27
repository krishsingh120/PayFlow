// Package worker provides asynchronous transaction processing logic for background workers.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/krishsingh120/payflow/services/payment/internal/ledger"
	"github.com/krishsingh120/payflow/services/payment/internal/messaging"
	"github.com/krishsingh120/payflow/services/payment/internal/outbox"
	"github.com/krishsingh120/payflow/services/payment/internal/payments"
	"github.com/krishsingh120/payflow/services/payment/internal/provider"
	"github.com/krishsingh120/payflow/services/payment/internal/storage"
	"github.com/krishsingh120/payflow/services/payment/internal/webhooks"
)

// PaymentProcessor handles the execution lifecycle of payment processing jobs.
type PaymentProcessor struct {
	repo        storage.PaymentRepository
	provider    provider.Provider
	logger      zerolog.Logger
	outboxRepo  outbox.Repository
	webhookRepo webhooks.Repository
	webhookURL  string
	ledgerSvc   ledger.Service
}

// NewPaymentProcessor creates a new processor with the supplied dependencies.
func NewPaymentProcessor(
	repo storage.PaymentRepository,
	provider provider.Provider,
	logger zerolog.Logger,
) *PaymentProcessor {
	return &PaymentProcessor{
		repo:     repo,
		provider: provider,
		logger:   logger,
	}
}

// WithOutbox attaches an Outbox repository for event persistence.
func (p *PaymentProcessor) WithOutbox(repo outbox.Repository) *PaymentProcessor {
	p.outboxRepo = repo
	return p
}

// WithWebhooks attaches a Webhook repository and destination URL.
func (p *PaymentProcessor) WithWebhooks(repo webhooks.Repository, webhookURL string) *PaymentProcessor {
	p.webhookRepo = repo
	p.webhookURL = webhookURL
	return p
}

// WithLedger attaches a Ledger service to record double-entry journals on payment success.
func (p *PaymentProcessor) WithLedger(svc ledger.Service) *PaymentProcessor {
	p.ledgerSvc = svc
	return p
}

// ProcessJob decodes a PaymentJob message and advances the payment state machine
// through authorization with the simulated provider.
func (p *PaymentProcessor) ProcessJob(ctx context.Context, payload []byte) error {
	var job messaging.PaymentJob
	if err := json.Unmarshal(payload, &job); err != nil {
		p.logger.Error().Err(err).Msg("failed to unmarshal payment job payload")
		return fmt.Errorf("unmarshal job error: %w", err)
	}

	if job.PaymentID == "" {
		return errors.New("invalid job: missing payment_id")
	}

	start := time.Now()
	p.logger.Info().
		Str("payment_id", job.PaymentID).
		Msg("starting background payment processing")

	// 1. Fetch current payment from storage
	payment, err := p.repo.GetByID(ctx, job.PaymentID)
	if err != nil {
		p.logger.Error().Err(err).Str("payment_id", job.PaymentID).Msg("payment not found")
		return fmt.Errorf("payment %s not found: %w", job.PaymentID, err)
	}

	// 2. Idempotency check: if already in a terminal state, nothing to do
	if payments.IsTerminal(payment.Status) {
		p.logger.Info().
			Str("payment_id", payment.ID).
			Str("status", string(payment.Status)).
			Msg("payment already in terminal state — skipping duplicate processing")
		return nil
	}

	// 3. Transition from CREATED to PENDING (if currently CREATED)
	if payment.Status == payments.StatusCreated {
		updated, err := p.repo.UpdateStatus(
			ctx,
			payment.ID,
			payments.StatusCreated,
			payments.StatusPending,
			nil,
			nil,
		)
		if err != nil {
			p.logger.Error().Err(err).Str("payment_id", payment.ID).Msg("failed to transition to PENDING")
			return fmt.Errorf("failed to transition to PENDING: %w", err)
		}
		payment = updated
	}

	// 4. Submit transaction to provider
	req := provider.ProcessRequest{
		PaymentID:   payment.ID,
		Amount:      payment.Amount,
		Currency:    payment.Currency,
		Description: payment.Description,
	}

	result, err := p.provider.ProcessPayment(ctx, req)
	if err != nil {
		p.logger.Error().Err(err).Str("payment_id", payment.ID).Msg("provider processing error")
		return fmt.Errorf("provider processing error: %w", err)
	}

	// 5. Update state machine based on provider outcome
	switch result.Outcome {
	case provider.OutcomeSuccess:
		_, err = p.repo.UpdateStatus(
			ctx,
			payment.ID,
			payments.StatusPending,
			payments.StatusSucceeded,
			&result.ProviderRef,
			nil,
		)
		if err != nil {
			p.logger.Error().Err(err).Str("payment_id", payment.ID).Msg("failed to transition to SUCCEEDED")
			return err
		}
		p.logger.Info().
			Str("payment_id", payment.ID).
			Str("provider_ref", result.ProviderRef).
			Dur("duration", time.Since(start)).
			Msg("payment successfully processed")

		if p.ledgerSvc != nil {
			if lErr := p.ledgerSvc.RecordPaymentSucceeded(ctx, payment.ID, payment.Amount); lErr != nil {
				p.logger.Error().Err(lErr).Str("payment_id", payment.ID).
					Msg("failed to record ledger entries for payment")
			}
		}

		p.emitOutcomeEvents(ctx, payment.ID, "payment.succeeded", payment.Amount, payment.Currency, &result.ProviderRef, nil)

	case provider.OutcomeDecline:
		_, err = p.repo.UpdateStatus(
			ctx,
			payment.ID,
			payments.StatusPending,
			payments.StatusFailed,
			&result.ProviderRef,
			result.FailureReason,
		)
		if err != nil {
			p.logger.Error().Err(err).Str("payment_id", payment.ID).Msg("failed to transition to FAILED")
			return err
		}
		p.logger.Warn().
			Str("payment_id", payment.ID).
			Str("reason", *result.FailureReason).
			Msg("payment declined by provider")

		p.emitOutcomeEvents(ctx, payment.ID, "payment.failed", payment.Amount, payment.Currency, &result.ProviderRef, result.FailureReason)

	case provider.OutcomeTimeout:
		_, err = p.repo.UpdateStatus(
			ctx,
			payment.ID,
			payments.StatusPending,
			payments.StatusUnknown,
			&result.ProviderRef,
			result.FailureReason,
		)
		if err != nil {
			p.logger.Error().Err(err).Str("payment_id", payment.ID).Msg("failed to transition to UNKNOWN")
			return err
		}
		p.logger.Warn().
			Str("payment_id", payment.ID).
			Msg("payment timed out at provider — marked as UNKNOWN for reconciliation")

		p.emitOutcomeEvents(ctx, payment.ID, "payment.timeout", payment.Amount, payment.Currency, &result.ProviderRef, result.FailureReason)
	}

	return nil
}

func (p *PaymentProcessor) emitOutcomeEvents(
	ctx context.Context,
	paymentID, eventType string,
	amount int64,
	currency string,
	providerRef, failureReason *string,
) {
	data := map[string]any{
		"event":          eventType,
		"payment_id":     paymentID,
		"amount":         amount,
		"currency":       currency,
		"provider_ref":   providerRef,
		"failure_reason": failureReason,
		"timestamp":      time.Now().UTC(),
	}
	payload, _ := json.Marshal(data)

	if p.outboxRepo != nil {
		if _, err := p.outboxRepo.Store(ctx, paymentID, eventType, payload); err != nil {
			p.logger.Error().Err(err).Str("payment_id", paymentID).Msg("failed to store outbox event")
		}
	}

	if p.webhookRepo != nil && p.webhookURL != "" {
		delivery := &webhooks.Delivery{
			ID:        webhooks.NewDeliveryID(),
			PaymentID: paymentID,
			EventType: eventType,
			URL:       p.webhookURL,
			Payload:   payload,
			Status:    webhooks.StatusPending,
			Attempts:  0,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := p.webhookRepo.CreateDelivery(ctx, delivery); err != nil {
			p.logger.Error().Err(err).Str("payment_id", paymentID).Msg("failed to enqueue webhook delivery")
		}
	}
}
