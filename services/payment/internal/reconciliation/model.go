// Package reconciliation implements the automated transaction reconciliation
// and ledger audit engine for PayFlow.
package reconciliation

import (
	"time"
)

// DiscrepancyType categorizes reconciliation findings.
type DiscrepancyType string

const (
	DiscrepancyUnknownResolvedSuccess DiscrepancyType = "UNKNOWN_RESOLVED_SUCCESS"
	DiscrepancyUnknownResolvedFailed  DiscrepancyType = "UNKNOWN_RESOLVED_FAILED"
	DiscrepancyMissingLedgerHealed    DiscrepancyType = "MISSING_LEDGER_HEALED"
	DiscrepancyLedgerUnbalanced       DiscrepancyType = "LEDGER_UNBALANCED"
)

// Discrepancy details an individual inconsistency discovered during reconciliation.
type Discrepancy struct {
	PaymentID string          `json:"payment_id"`
	Type      DiscrepancyType `json:"type"`
	Details   string          `json:"details"`
}

// Report summarizes the findings and actions of a reconciliation run.
type Report struct {
	ID               string        `json:"id"`
	RunAt            time.Time     `json:"run_at"`
	PaymentsAudited  int           `json:"payments_audited"`
	UnknownsResolved int           `json:"unknowns_resolved"`
	LedgerBalanced   bool          `json:"ledger_balanced"`
	TotalDebitPaise  int64         `json:"total_debit_paise"`
	TotalCreditPaise int64         `json:"total_credit_paise"`
	Discrepancies    []Discrepancy `json:"discrepancies"`
	DurationMs       int64         `json:"duration_ms"`
}
