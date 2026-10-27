package reconciliation

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/krishsingh120/payflow/services/payment/internal/ledger"
	"github.com/krishsingh120/payflow/services/payment/internal/payments"
	"github.com/krishsingh120/payflow/services/payment/internal/provider"
	"github.com/krishsingh120/payflow/services/payment/internal/storage"
)

// Engine performs automated reconciliation and double-entry consistency audits.
type Engine struct {
	repo      storage.PaymentRepository
	provider  provider.Provider
	ledgerSvc ledger.Service
	publisher payments.EventPublisher
	log       zerolog.Logger

	mu           sync.RWMutex
	latestReport *Report
}

// NewEngine constructs a new reconciliation engine.
func NewEngine(
	repo storage.PaymentRepository,
	prov provider.Provider,
	ledgerSvc ledger.Service,
	pub payments.EventPublisher,
	log zerolog.Logger,
) *Engine {
	return &Engine{
		repo:      repo,
		provider:  prov,
		ledgerSvc: ledgerSvc,
		publisher: pub,
		log:       log.With().Str("component", "reconciliation-engine").Logger(),
	}
}

// Run executes a complete reconciliation cycle:
// 1. Resolves UNKNOWN timeout transactions against external provider settlement records.
// 2. Backfills and verifies double-entry ledger journals for all successful transactions.
// 3. Audits the global debit = credit accounting invariant.
func (e *Engine) Run(ctx context.Context) (*Report, error) {
	start := time.Now()
	runID := fmt.Sprintf("rec_%s", payments.NewPaymentID()[4:])

	e.log.Info().Str("run_id", runID).Msg("starting reconciliation audit run")

	var (
		discrepancies    []Discrepancy
		unknownsResolved int
		auditedCount     int
	)

	// 1. Audit and resolve UNKNOWN status payments
	unknownStatus := payments.StatusUnknown
	unknownPayments, _, _, err := e.repo.List(ctx, payments.ListFilter{Status: &unknownStatus, Limit: 100})
	if err == nil {
		for _, p := range unknownPayments {
			auditedCount++
			res, provErr := e.provider.QueryPaymentStatus(ctx, p.ID)
			if provErr != nil {
				e.log.Warn().Err(provErr).Str("payment_id", p.ID).Msg("failed to query provider status")
				continue
			}

			switch res.Outcome {
			case provider.OutcomeSuccess:
				_, err = e.repo.UpdateStatus(ctx, p.ID, payments.StatusUnknown, payments.StatusSucceeded, &res.ProviderRef, nil)
				if err != nil {
					e.log.Error().Err(err).Str("payment_id", p.ID).Msg("failed to transition UNKNOWN to SUCCEEDED")
					continue
				}

				// Backfill double-entry ledger journal
				if e.ledgerSvc != nil {
					_ = e.ledgerSvc.RecordPaymentSucceeded(ctx, p.ID, p.Amount)
				}

				// Publish lifecycle event
				if e.publisher != nil {
					evtPayload, _ := json.Marshal(map[string]any{
						"payment_id":   p.ID,
						"amount":       p.Amount,
						"currency":     p.Currency,
						"status":       "SUCCEEDED",
						"provider_ref": res.ProviderRef,
						"reconciled":   true,
					})
					_ = e.publisher.Publish(ctx, "payment.succeeded", evtPayload)
				}

				discrepancies = append(discrepancies, Discrepancy{
					PaymentID: p.ID,
					Type:      DiscrepancyUnknownResolvedSuccess,
					Details:   fmt.Sprintf("Transaction resolved to SUCCEEDED (Provider Ref: %s)", res.ProviderRef),
				})
				unknownsResolved++

			case provider.OutcomeDecline:
				_, err = e.repo.UpdateStatus(ctx, p.ID, payments.StatusUnknown, payments.StatusFailed, &res.ProviderRef, res.FailureReason)
				if err != nil {
					e.log.Error().Err(err).Str("payment_id", p.ID).Msg("failed to transition UNKNOWN to FAILED")
					continue
				}

				if e.publisher != nil {
					evtPayload, _ := json.Marshal(map[string]any{
						"payment_id":     p.ID,
						"status":         "FAILED",
						"failure_reason": res.FailureReason,
						"reconciled":     true,
					})
					_ = e.publisher.Publish(ctx, "payment.failed", evtPayload)
				}

				discrepancies = append(discrepancies, Discrepancy{
					PaymentID: p.ID,
					Type:      DiscrepancyUnknownResolvedFailed,
					Details:   "Transaction confirmed as DECLINED by bank switch",
				})
				unknownsResolved++
			}
		}
	}

	// 2. Audit ledger consistency for all SUCCEEDED payments
	succeededStatus := payments.StatusSucceeded
	succeededPayments, _, _, err := e.repo.List(ctx, payments.ListFilter{Status: &succeededStatus, Limit: 200})
	if err == nil && e.ledgerSvc != nil {
		for _, p := range succeededPayments {
			auditedCount++
			summary, ledgerErr := e.ledgerSvc.GetPaymentLedger(ctx, p.ID)
			if ledgerErr != nil {
				continue
			}

			if len(summary.Entries) == 0 {
				// Missing ledger record — heal automatically
				if healErr := e.ledgerSvc.RecordPaymentSucceeded(ctx, p.ID, p.Amount); healErr == nil {
					discrepancies = append(discrepancies, Discrepancy{
						PaymentID: p.ID,
						Type:      DiscrepancyMissingLedgerHealed,
						Details:   fmt.Sprintf("Missing journal entries restored for ₹%.2f", float64(p.Amount)/100),
					})
				}
			} else if !summary.IsBalanced {
				discrepancies = append(discrepancies, Discrepancy{
					PaymentID: p.ID,
					Type:      DiscrepancyLedgerUnbalanced,
					Details:   fmt.Sprintf("Debit (%d paise) != Credit (%d paise)", summary.TotalDebit, summary.TotalCredit),
				})
			}
		}
	}

	// 3. Global balance audit
	var isBalanced bool = true
	var totalDebit, totalCredit int64
	if e.ledgerSvc != nil {
		var balErr error
		isBalanced, totalDebit, totalCredit, balErr = e.ledgerSvc.AuditGlobalBalance(ctx)
		if balErr != nil {
			isBalanced = false
		}
	}

	report := &Report{
		ID:               runID,
		RunAt:            time.Now().UTC(),
		PaymentsAudited:  auditedCount,
		UnknownsResolved: unknownsResolved,
		LedgerBalanced:   isBalanced,
		TotalDebitPaise:  totalDebit,
		TotalCreditPaise: totalCredit,
		Discrepancies:    discrepancies,
		DurationMs:       time.Since(start).Milliseconds(),
	}

	e.mu.Lock()
	e.latestReport = report
	e.mu.Unlock()

	e.log.Info().
		Str("run_id", runID).
		Int("audited", auditedCount).
		Int("resolved", unknownsResolved).
		Int("discrepancies", len(discrepancies)).
		Bool("ledger_balanced", isBalanced).
		Dur("duration", time.Since(start)).
		Msg("reconciliation audit run finished")

	return report, nil
}

// GetLatestReport returns the most recently generated reconciliation report.
func (e *Engine) GetLatestReport() *Report {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.latestReport
}
