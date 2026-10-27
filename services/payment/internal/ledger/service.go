package ledger

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
)

// Service defines high-level double-entry operations.
type Service interface {
	RecordPaymentSucceeded(ctx context.Context, paymentID string, amount int64) error
	RecordRefundSucceeded(ctx context.Context, paymentID string, amount int64) error
	GetPaymentLedger(ctx context.Context, paymentID string) (*PaymentLedgerSummary, error)
	GetAccountBalances(ctx context.Context) (map[string]int64, error)
	AuditGlobalBalance(ctx context.Context) (bool, int64, int64, error)
	ListRecentEntries(ctx context.Context, limit int) ([]*Entry, error)
}

// LedgerService implements Service.
type LedgerService struct {
	repo Repository
	log  zerolog.Logger
}

// NewService constructs a new LedgerService.
func NewService(repo Repository, log zerolog.Logger) *LedgerService {
	return &LedgerService{
		repo: repo,
		log:  log.With().Str("component", "ledger-service").Logger(),
	}
}

// RecordPaymentSucceeded creates balanced double entries for a successful payment:
// DEBIT  receivable (Asset)  +amount
// CREDIT revenue    (Income) +amount
func (s *LedgerService) RecordPaymentSucceeded(ctx context.Context, paymentID string, amount int64) error {
	if strings.TrimSpace(paymentID) == "" {
		return fmt.Errorf("payment id is required")
	}
	if amount <= 0 {
		return ErrInvalidAmount
	}

	entries := []*Entry{
		{
			PaymentID: paymentID,
			Account:   AccountReceivable,
			Direction: DirectionDebit,
			Amount:    amount,
		},
		{
			PaymentID: paymentID,
			Account:   AccountRevenue,
			Direction: DirectionCredit,
			Amount:    amount,
		},
	}

	if err := s.repo.RecordTransaction(ctx, entries); err != nil {
		s.log.Error().Err(err).Str("payment_id", paymentID).Int64("amount", amount).
			Msg("failed to record payment succeeded ledger transaction")
		return err
	}

	s.log.Info().Str("payment_id", paymentID).Int64("amount", amount).
		Msg("recorded double-entry journal for payment success")
	return nil
}

// RecordRefundSucceeded creates balanced double entries for an approved refund:
// DEBIT  refund_liability (Contra/Liability) +amount
// CREDIT receivable       (Asset reduction)  +amount
func (s *LedgerService) RecordRefundSucceeded(ctx context.Context, paymentID string, amount int64) error {
	if strings.TrimSpace(paymentID) == "" {
		return fmt.Errorf("payment id is required")
	}
	if amount <= 0 {
		return ErrInvalidAmount
	}

	entries := []*Entry{
		{
			PaymentID: paymentID,
			Account:   AccountRefundLiability,
			Direction: DirectionDebit,
			Amount:    amount,
		},
		{
			PaymentID: paymentID,
			Account:   AccountReceivable,
			Direction: DirectionCredit,
			Amount:    amount,
		},
	}

	if err := s.repo.RecordTransaction(ctx, entries); err != nil {
		s.log.Error().Err(err).Str("payment_id", paymentID).Int64("amount", amount).
			Msg("failed to record refund succeeded ledger transaction")
		return err
	}

	s.log.Info().Str("payment_id", paymentID).Int64("amount", amount).
		Msg("recorded double-entry journal for refund success")
	return nil
}

// GetPaymentLedger retrieves all entries for a payment and audits invariant balance.
func (s *LedgerService) GetPaymentLedger(ctx context.Context, paymentID string) (*PaymentLedgerSummary, error) {
	if strings.TrimSpace(paymentID) == "" {
		return nil, fmt.Errorf("payment id is required")
	}

	entries, err := s.repo.GetEntriesByPaymentID(ctx, paymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve ledger entries: %w", err)
	}

	var totalDebit, totalCredit int64
	for _, e := range entries {
		switch e.Direction {
		case DirectionDebit:
			totalDebit += e.Amount
		case DirectionCredit:
			totalCredit += e.Amount
		}
	}

	return &PaymentLedgerSummary{
		PaymentID:   paymentID,
		Entries:     entries,
		TotalDebit:  totalDebit,
		TotalCredit: totalCredit,
		IsBalanced:  totalDebit == totalCredit,
	}, nil
}

// GetAccountBalances returns current net balances for standard accounts.
func (s *LedgerService) GetAccountBalances(ctx context.Context) (map[string]int64, error) {
	accounts := []string{AccountReceivable, AccountRevenue, AccountRefundLiability}
	balances := make(map[string]int64, len(accounts))

	for _, acc := range accounts {
		bal, err := s.repo.GetAccountBalance(ctx, acc)
		if err != nil {
			return nil, fmt.Errorf("failed to get balance for %s: %w", acc, err)
		}
		balances[acc] = bal
	}

	return balances, nil
}

// AuditGlobalBalance checks that the sum of all DEBITs equals the sum of all CREDITs globally.
func (s *LedgerService) AuditGlobalBalance(ctx context.Context) (bool, int64, int64, error) {
	totalDebit, totalCredit, err := s.repo.GetTotalDebitAndCredit(ctx)
	if err != nil {
		return false, 0, 0, err
	}
	return totalDebit == totalCredit, totalDebit, totalCredit, nil
}

// ListRecentEntries retrieves recent entries up to limit.
func (s *LedgerService) ListRecentEntries(ctx context.Context, limit int) ([]*Entry, error) {
	return s.repo.ListAllEntries(ctx, limit)
}
