// Package payments defines the payment domain model, state machine, and service logic.
package payments

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// PaymentStatus represents the lifecycle state of a payment.
type PaymentStatus string

const (
	StatusCreated   PaymentStatus = "CREATED"
	StatusPending   PaymentStatus = "PENDING"
	StatusSucceeded PaymentStatus = "SUCCEEDED"
	StatusFailed    PaymentStatus = "FAILED"
	StatusUnknown   PaymentStatus = "UNKNOWN"
)

// Valid payment statuses.
var validStatuses = map[PaymentStatus]bool{
	StatusCreated:   true,
	StatusPending:   true,
	StatusSucceeded: true,
	StatusFailed:    true,
	StatusUnknown:   true,
}

// AllowedTransitions maps each status to its permitted next states.
// Invariants:
// - CREATED can only move to PENDING (when worker begins processing)
// - PENDING can resolve to SUCCEEDED, FAILED, or UNKNOWN (provider timeout)
// - UNKNOWN can resolve to SUCCEEDED or FAILED via reconciliation
// - SUCCEEDED and FAILED are terminal states (no further transitions permitted)
var AllowedTransitions = map[PaymentStatus][]PaymentStatus{
	StatusCreated:   {StatusPending},
	StatusPending:   {StatusSucceeded, StatusFailed, StatusUnknown},
	StatusUnknown:   {StatusSucceeded, StatusFailed},
	StatusSucceeded: {},
	StatusFailed:    {},
}

// IsValidStatus returns true if the status is recognised.
func IsValidStatus(s PaymentStatus) bool {
	return validStatuses[s]
}

// IsTerminal returns true if the status is an end-state where no more transitions are allowed.
func IsTerminal(s PaymentStatus) bool {
	return s == StatusSucceeded || s == StatusFailed
}

// CanTransition returns true if moving from `from` to `to` is permitted by the state machine.
func CanTransition(from, to PaymentStatus) bool {
	allowed, ok := AllowedTransitions[from]
	if !ok {
		return false
	}
	for _, target := range allowed {
		if target == to {
			return true
		}
	}
	return false
}

// Payment is the core domain entity representing a financial transaction.
// All amounts are represented as int64 in paise (1 INR = 100 paise) to avoid floating point errors.
type Payment struct {
	ID             string         `json:"id"`
	IdempotencyKey string         `json:"idempotency_key"`
	Amount         int64          `json:"amount"`
	Currency       string         `json:"currency"`
	Status         PaymentStatus  `json:"status"`
	Description    string         `json:"description,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	ProviderRef    *string        `json:"provider_ref,omitempty"`
	FailureReason  *string        `json:"failure_reason,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// CreatePaymentRequest represents the payload required to create a new payment.
type CreatePaymentRequest struct {
	Amount      int64          `json:"amount"`
	Currency    string         `json:"currency"`
	Description string         `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

var currencyRegex = regexp.MustCompile(`^[A-Z]{3}$`)

// Validate checks the request fields according to domain rules.
func (r *CreatePaymentRequest) Validate() error {
	if r.Amount <= 0 {
		return errors.New("amount must be greater than 0")
	}

	curr := strings.ToUpper(strings.TrimSpace(r.Currency))
	if !currencyRegex.MatchString(curr) {
		return errors.New("currency must be a valid 3-letter ISO code (e.g. INR, USD)")
	}
	r.Currency = curr

	if len(r.Description) > 500 {
		return errors.New("description cannot exceed 500 characters")
	}

	if r.Metadata != nil {
		// Ensure metadata is serialisable to JSON and within reasonable size
		data, err := json.Marshal(r.Metadata)
		if err != nil {
			return fmt.Errorf("metadata must be valid JSON: %w", err)
		}
		if len(data) > 65536 {
			return errors.New("metadata exceeds maximum allowed size (64KB)")
		}
	}

	return nil
}

// PaymentResponse is the external representation returned to API consumers.
type PaymentResponse struct {
	ID             string         `json:"id"`
	Amount         int64          `json:"amount"`
	Currency       string         `json:"currency"`
	Status         PaymentStatus  `json:"status"`
	Description    string         `json:"description,omitempty"`
	IdempotencyKey string         `json:"idempotency_key"`
	ProviderRef    *string        `json:"provider_ref,omitempty"`
	FailureReason  *string        `json:"failure_reason,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// ToResponse converts an internal Payment entity to its API response shape.
func (p *Payment) ToResponse() *PaymentResponse {
	return &PaymentResponse{
		ID:             p.ID,
		Amount:         p.Amount,
		Currency:       p.Currency,
		Status:         p.Status,
		Description:    p.Description,
		IdempotencyKey: p.IdempotencyKey,
		ProviderRef:    p.ProviderRef,
		FailureReason:  p.FailureReason,
		Metadata:       p.Metadata,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

// ListFilter specifies filtering and pagination criteria for listing payments.
type ListFilter struct {
	Limit  int            `json:"limit"`
	Cursor string         `json:"cursor"`
	Status *PaymentStatus `json:"status"`
}

// ListPaymentsResponse wraps the paginated response for payment listings.
type ListPaymentsResponse struct {
	Data       []*PaymentResponse `json:"data"`
	NextCursor string             `json:"next_cursor,omitempty"`
	Total      int64              `json:"total"`
}
