// Package provider provides a simulated external payment acquirer / banking gateway.
// In real-world payment companies like Pine Labs or Razorpay, transactions are submitted
// to banking switches / networks which respond synchronously or asynchronously.
package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/krishsingh120/payflow/services/payment/internal/payments"
)

// Outcome represents the simulated transaction outcome from the provider.
type Outcome string

const (
	OutcomeSuccess Outcome = "SUCCESS"
	OutcomeDecline Outcome = "DECLINE"
	OutcomeTimeout Outcome = "TIMEOUT"
)

// ProcessRequest contains the details passed to the external payment provider.
type ProcessRequest struct {
	PaymentID   string
	Amount      int64 // paise
	Currency    string
	Description string
}

// ProcessResult contains the provider's transaction authorization result.
type ProcessResult struct {
	Outcome       Outcome
	ProviderRef   string
	FailureReason *string
	Latency       time.Duration
}

// Provider defines the contract for processing payments with external acquirers.
type Provider interface {
	ProcessPayment(ctx context.Context, req ProcessRequest) (*ProcessResult, error)
}

// SimulatedProvider is an in-process mock simulating a bank switch with deterministic test rules.
type SimulatedProvider struct {
	defaultOutcome Outcome
	timeoutMs      int
}

// NewSimulatedProvider creates a provider simulator with the configured default outcome and latency.
func NewSimulatedProvider(defaultOutcome string, timeoutMs int) *SimulatedProvider {
	outcome := OutcomeSuccess
	switch strings.ToLower(strings.TrimSpace(defaultOutcome)) {
	case "decline", "failed":
		outcome = OutcomeDecline
	case "timeout", "unknown":
		outcome = OutcomeTimeout
	}

	if timeoutMs <= 0 {
		timeoutMs = 100 // fast default for tests
	}

	return &SimulatedProvider{
		defaultOutcome: outcome,
		timeoutMs:      timeoutMs,
	}
}

// ProcessPayment simulates submitting a transaction to a payment network.
// Deterministic rules for testing & interview demonstrations:
// - Amount ending in 99 paise (e.g. ₹9.99): Declines with "INSUFFICIENT_FUNDS"
// - Amount ending in 98 paise (e.g. ₹9.98): Simulates network timeout -> OutcomeTimeout
// - Description containing "[force-fail]": Declines with "CARD_RESTRICTED"
// - Description containing "[force-timeout]": Simulates gateway timeout -> OutcomeTimeout
// - All other requests resolve according to defaultOutcome (default: SUCCESS)
func (p *SimulatedProvider) ProcessPayment(ctx context.Context, req ProcessRequest) (*ProcessResult, error) {
	start := time.Now()

	// 1. Check deterministic triggers based on amount
	if req.Amount%100 == 99 {
		reason := "card declined: insufficient funds (rule: amount ends in 99)"
		return &ProcessResult{
			Outcome:       OutcomeDecline,
			ProviderRef:   generateProviderRef(),
			FailureReason: &reason,
			Latency:       time.Since(start),
		}, nil
	}

	if req.Amount%100 == 98 {
		reason := "gateway timeout: bank switch did not respond in time (rule: amount ends in 98)"
		return &ProcessResult{
			Outcome:       OutcomeTimeout,
			ProviderRef:   generateProviderRef(),
			FailureReason: &reason,
			Latency:       time.Since(start),
		}, nil
	}

	// 2. Check deterministic triggers based on description tags
	desc := strings.ToLower(req.Description)
	if strings.Contains(desc, "[force-fail]") {
		reason := "card declined: restricted payment instrument"
		return &ProcessResult{
			Outcome:       OutcomeDecline,
			ProviderRef:   generateProviderRef(),
			FailureReason: &reason,
			Latency:       time.Since(start),
		}, nil
	}

	if strings.Contains(desc, "[force-timeout]") {
		reason := "provider communication timeout"
		return &ProcessResult{
			Outcome:       OutcomeTimeout,
			ProviderRef:   generateProviderRef(),
			FailureReason: &reason,
			Latency:       time.Since(start),
		}, nil
	}

	// 3. Fallback to configured default outcome
	switch p.defaultOutcome {
	case OutcomeDecline:
		reason := "card declined by issuer"
		return &ProcessResult{
			Outcome:       OutcomeDecline,
			ProviderRef:   generateProviderRef(),
			FailureReason: &reason,
			Latency:       time.Since(start),
		}, nil

	case OutcomeTimeout:
		reason := "provider response timed out"
		return &ProcessResult{
			Outcome:       OutcomeTimeout,
			ProviderRef:   generateProviderRef(),
			FailureReason: &reason,
			Latency:       time.Since(start),
		}, nil

	default:
		return &ProcessResult{
			Outcome:     OutcomeSuccess,
			ProviderRef: generateProviderRef(),
			Latency:     time.Since(start),
		}, nil
	}
}

func generateProviderRef() string {
	return fmt.Sprintf("prov_sim_%s", payments.NewPaymentID()[4:])
}
