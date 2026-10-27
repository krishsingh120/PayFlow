// Package ledger implements double-entry bookkeeping for PayFlow.
// Every financial event (payments, refunds) produces balanced debit and credit entries
// satisfying the fundamental accounting invariant: sum(DEBITS) == sum(CREDITS).
package ledger

import (
	"errors"
	"fmt"
	"time"
)

// Standard chart of accounts for PayFlow.
const (
	// AccountReceivable represents funds owed by payment providers/gateways (Asset).
	AccountReceivable = "receivable"

	// AccountRevenue represents gross recognized revenue (Income/Equity).
	AccountRevenue = "revenue"

	// AccountRefundLiability represents funds returned to customers (Contra-revenue/Liability).
	AccountRefundLiability = "refund_liability"
)

// Direction represents debit or credit accounting direction.
type Direction string

const (
	DirectionDebit  Direction = "DEBIT"
	DirectionCredit Direction = "CREDIT"
)

var (
	// ErrLedgerUnbalanced is returned when total debits do not equal total credits.
	ErrLedgerUnbalanced = errors.New("ledger transaction is unbalanced: sum(DEBIT) != sum(CREDIT)")

	// ErrInvalidAmount is returned when an entry amount is <= 0.
	ErrInvalidAmount = errors.New("ledger entry amount must be greater than 0")

	// ErrInvalidDirection is returned when the direction is neither DEBIT nor CREDIT.
	ErrInvalidDirection = errors.New("ledger entry direction must be DEBIT or CREDIT")
)

// Entry is an immutable financial record in the double-entry journal.
type Entry struct {
	ID        int64     `json:"id"`
	PaymentID string    `json:"payment_id"`
	Account   string    `json:"account"`
	Direction Direction `json:"direction"`
	Amount    int64     `json:"amount"` // in paise (never float)
	CreatedAt time.Time `json:"created_at"`
}

// Validate verifies standard domain constraints on a ledger entry.
func (e *Entry) Validate() error {
	if e.Amount <= 0 {
		return ErrInvalidAmount
	}
	if e.Direction != DirectionDebit && e.Direction != DirectionCredit {
		return ErrInvalidDirection
	}
	if e.Account == "" {
		return errors.New("ledger account cannot be empty")
	}
	return nil
}

// VerifyInvariant checks that a set of entries satisfies the double-entry invariant:
// SUM(DEBIT) == SUM(CREDIT).
func VerifyInvariant(entries []*Entry) error {
	if len(entries) == 0 {
		return errors.New("ledger transaction must contain at least two entries")
	}

	var totalDebit, totalCredit int64

	for _, e := range entries {
		if err := e.Validate(); err != nil {
			return err
		}
		switch e.Direction {
		case DirectionDebit:
			totalDebit += e.Amount
		case DirectionCredit:
			totalCredit += e.Amount
		}
	}

	if totalDebit != totalCredit {
		return fmt.Errorf("%w (total DEBIT: %d paise, total CREDIT: %d paise)",
			ErrLedgerUnbalanced, totalDebit, totalCredit)
	}

	return nil
}

// PaymentLedgerSummary aggregates all ledger entries for a payment with balance audit status.
type PaymentLedgerSummary struct {
	PaymentID    string   `json:"payment_id"`
	Entries      []*Entry `json:"entries"`
	TotalDebit   int64    `json:"total_debit"`
	TotalCredit  int64    `json:"total_credit"`
	IsBalanced   bool     `json:"is_balanced"`
}
