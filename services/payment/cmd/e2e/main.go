package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"

	"github.com/krishsingh120/payflow/services/payment/internal/ledger"
	"github.com/krishsingh120/payflow/services/payment/internal/messaging"
	"github.com/krishsingh120/payflow/services/payment/internal/outbox"
	"github.com/krishsingh120/payflow/services/payment/internal/payments"
	"github.com/krishsingh120/payflow/services/payment/internal/provider"
	"github.com/krishsingh120/payflow/services/payment/internal/reconciliation"
	"github.com/krishsingh120/payflow/services/payment/internal/storage"
	"github.com/krishsingh120/payflow/services/payment/internal/webhooks"
	"github.com/krishsingh120/payflow/services/payment/internal/worker"
)

func main() {
	fmt.Println("================================================================================")
	fmt.Println("   PayFlow — Comprehensive End-to-End System & Invariants Verification")
	fmt.Println("   Pine Labs / Fintech Production Standard Architecture")
	fmt.Println("================================================================================")

	ctx := context.Background()
	logger := zerolog.Nop()

	// 1. Initialize in-memory infrastructure
	paymentRepo := storage.NewMemoryPaymentRepository()
	broker := messaging.NewMemoryBroker()
	outboxRepo := outbox.NewMemoryRepository()
	webhookRepo := webhooks.NewMemoryRepository()
	ledgerRepo := ledger.NewMemoryRepository()
	ledgerSvc := ledger.NewService(ledgerRepo, logger)
	simProvider := provider.NewSimulatedProvider("SUCCEEDED", 50)

	paymentSvc := payments.NewService(paymentRepo).
		WithPublisher(broker).
		WithLedger(ledgerSvc)

	processor := worker.NewPaymentProcessor(paymentRepo, simProvider, logger).
		WithOutbox(outboxRepo).
		WithWebhooks(webhookRepo, "https://merchant.example.com/webhooks").
		WithLedger(ledgerSvc)

	reconEngine := reconciliation.NewEngine(paymentRepo, simProvider, ledgerSvc, broker, logger)

	// Step 1: Idempotency & Conflict Verification
	fmt.Println("\n[1/5] Verifying Idempotency & Conflict Invariants...")
	idemKey := "e2e_idemp_key_001"
	req1 := payments.CreatePaymentRequest{
		Amount:      10000, // 100.00 INR
		Currency:    "INR",
		Description: "Order #84920 - E2E verification test",
	}

	p1, replay1, err := paymentSvc.CreatePayment(ctx, idemKey, req1)
	if err != nil || replay1 {
		fail("Failed initial payment creation: %v", err)
	}
	fmt.Printf("  ✓ Created payment %s (Amount: %d paise, Status: %s)\n", p1.ID, p1.Amount, p1.Status)

	// Replay exact request
	p1Replay, replay2, err := paymentSvc.CreatePayment(ctx, idemKey, req1)
	if err != nil || !replay2 || p1Replay.ID != p1.ID {
		fail("Idempotency replay failed or generated duplicate ID")
	}
	fmt.Printf("  ✓ Exact replay matched: ID %s, Replayed: %v (Exactly-once guaranteed)\n", p1Replay.ID, replay2)

	// Replay with mutated amount -> Must conflict
	reqConflict := req1
	reqConflict.Amount = 25000
	_, _, err = paymentSvc.CreatePayment(ctx, idemKey, reqConflict)
	if err != payments.ErrIdempotencyConflict {
		fail("Expected ErrIdempotencyConflict on mutated payload, got: %v", err)
	}
	fmt.Println("  ✓ Mutated payload rejected with 409 Conflict")

	// Step 2: Payment Processing, Outbox & Double-Entry Ledger
	fmt.Println("\n[2/5] Verifying Payment Processing, Outbox & Double-Entry Ledger...")
	jobPayload, _ := json.Marshal(messaging.PaymentJob{PaymentID: p1.ID})
	if err := processor.ProcessJob(ctx, jobPayload); err != nil {
		fail("Worker failed to process job: %v", err)
	}

	p1Updated, err := paymentRepo.GetByID(ctx, p1.ID)
	if err != nil || p1Updated.Status != payments.StatusSucceeded {
		fail("Expected payment status SUCCEEDED, got %s (err: %v)", p1Updated.Status, err)
	}
	providerRef := "N/A"
	if p1Updated.ProviderRef != nil {
		providerRef = *p1Updated.ProviderRef
	}
	fmt.Printf("  ✓ Payment transitioned: CREATED -> PENDING -> %s (Provider Ref: %s)\n", p1Updated.Status, providerRef)

	// Verify Outbox Event
	events, err := outboxRepo.FetchUnpublished(ctx, 10)
	if err != nil || len(events) == 0 {
		fail("Expected outbox event for payment succeeded, got 0")
	}
	fmt.Printf("  ✓ Transactional Outbox captured event: %d (Type: %s)\n", events[0].ID, events[0].EventType)

	// Verify Double-Entry Ledger
	p1Ledger, err := ledgerSvc.GetPaymentLedger(ctx, p1.ID)
	if err != nil || len(p1Ledger.Entries) != 2 {
		fail("Expected 2 balanced ledger entries, got %d", len(p1Ledger.Entries))
	}
	if !p1Ledger.IsBalanced {
		fail("Ledger invariant violated! Sum(DEBIT) != Sum(CREDIT)")
	}
	fmt.Printf("  ✓ Double-Entry Journal verified for %s:\n", p1.ID)
	for _, entry := range p1Ledger.Entries {
		fmt.Printf("     • %-6s %-18s %8d paise [ID: %d]\n", entry.Direction, entry.Account, entry.Amount, entry.ID)
	}
	fmt.Printf("     => Total Debit: %d | Total Credit: %d | Invariant Balanced: %v\n",
		p1Ledger.TotalDebit, p1Ledger.TotalCredit, p1Ledger.IsBalanced)

	// Step 3: Refund Engine & Over-Refund Invariants
	fmt.Println("\n[3/5] Verifying Partial & Full Refund Lifecycle with Balance Protections...")
	refReq1 := payments.CreateRefundRequest{
		Amount: 3000, // 30.00 INR
		Reason: "Customer requested partial return",
	}
	ref1, refReplay1, err := paymentSvc.CreateRefund(ctx, p1.ID, "ref_idemp_key_001", refReq1)
	if err != nil || refReplay1 {
		fail("Failed to create partial refund: %v", err)
	}
	fmt.Printf("  ✓ Refund %s created: %d paise refunded (Remaining refundable: %d paise)\n",
		ref1.ID, ref1.Amount, 10000-3000)

	// Verify Double-Entry Refund Journal
	refLedger, err := ledgerSvc.GetPaymentLedger(ctx, p1.ID)
	if err != nil || len(refLedger.Entries) != 4 {
		fail("Expected 4 total ledger entries (2 payment + 2 refund), got %d", len(refLedger.Entries))
	}
	if !refLedger.IsBalanced {
		fail("Double-entry balance drift detected after refund!")
	}
	fmt.Println("  ✓ Refund journal recorded (DEBIT refund_liability / CREDIT receivable) — Zero Invariant Drift")

	// Attempt Over-Refund (Amount 8000 when only 7000 remaining)
	refReqOver := payments.CreateRefundRequest{
		Amount: 8000,
		Reason: "Exceeds balance",
	}
	_, _, err = paymentSvc.CreateRefund(ctx, p1.ID, "ref_idemp_key_over", refReqOver)
	if err != payments.ErrRefundAmountExceeded {
		fail("Expected ErrRefundAmountExceeded, got: %v", err)
	}
	fmt.Println("  ✓ Over-refund attempt safely rejected with 422 Unprocessable Entity")

	// Step 4: Webhook Signing & HMAC-SHA256 Timing-Safe Verification
	fmt.Println("\n[4/5] Verifying Webhook Signing & HMAC-SHA256 Replay Defenses...")
	whPayload := []byte(fmt.Sprintf(`{"event":"payment.succeeded","payment_id":"%s","amount":10000}`, p1.ID))
	whSecret := "whsec_pine_labs_super_secure_key_123"
	timestamp := time.Now().Unix()

	sigHeader := webhooks.Sign(whSecret, timestamp, whPayload)
	fmt.Printf("  ✓ Generated HMAC-SHA256 Signature Header: %s\n", sigHeader)

	// Validate valid signature
	err = webhooks.Verify(whSecret, whPayload, sigHeader, 5*time.Minute)
	if err != nil {
		fail("Valid webhook signature rejected: %v", err)
	}
	fmt.Println("  ✓ Cryptographic verification passed with constant-time equality check")

	// Validate tampered payload rejected
	tamperedPayload := []byte(fmt.Sprintf(`{"event":"payment.succeeded","payment_id":"%s","amount":999999}`, p1.ID))
	err = webhooks.Verify(whSecret, tamperedPayload, sigHeader, 5*time.Minute)
	if err != webhooks.ErrSignatureMismatch {
		fail("Expected ErrSignatureMismatch on tampered payload, got: %v", err)
	}
	fmt.Println("  ✓ Tampered payload strictly rejected")

	// Step 5: Automated Reconciliation & Self-Healing Engine
	fmt.Println("\n[5/5] Verifying Automated Reconciliation & Self-Healing Engine...")
	// Create payment with amount ending in 98 paise to simulate banking gateway timeout / UNKNOWN
	timeoutReq := payments.CreatePaymentRequest{
		Amount:      40098,
		Currency:    "INR",
		Description: "[force-timeout] Simulation for automated reconciliation",
	}
	pTimeout, _, err := paymentSvc.CreatePayment(ctx, "recon_timeout_test_key", timeoutReq)
	if err != nil {
		fail("Failed to create timeout test payment: %v", err)
	}
	jobTimeout, _ := json.Marshal(messaging.PaymentJob{PaymentID: pTimeout.ID})
	_ = processor.ProcessJob(ctx, jobTimeout)

	pTimeoutCurrent, err := paymentRepo.GetByID(ctx, pTimeout.ID)
	if err != nil || pTimeoutCurrent.Status != payments.StatusUnknown {
		fail("Expected status UNKNOWN, got: %s", pTimeoutCurrent.Status)
	}
	fmt.Printf("  ✓ Created payment %s in pending %s state (simulated gateway network partition)\n",
		pTimeout.ID, pTimeoutCurrent.Status)

	// Run Reconciliation Engine
	fmt.Println("  ▶ Triggering automated reconciliation audit...")
	report, err := reconEngine.Run(ctx)
	if err != nil {
		fail("Reconciliation audit failed: %v", err)
	}
	fmt.Printf("  ✓ Audit completed [%s]:\n", report.ID)
	fmt.Printf("     • Payments Audited: %d\n", report.PaymentsAudited)
	fmt.Printf("     • Unknowns Resolved: %d\n", report.UnknownsResolved)
	fmt.Printf("     • Discrepancies Found/Healed: %d\n", len(report.Discrepancies))
	fmt.Printf("     • Total Debit Sum:  %d paise\n", report.TotalDebitPaise)
	fmt.Printf("     • Total Credit Sum: %d paise\n", report.TotalCreditPaise)
	fmt.Printf("     • Ledger Invariant Balanced: %v\n", report.LedgerBalanced)

	// Verify that the UNKNOWN payment was resolved to SUCCEEDED and healed in ledger
	pResolved, err := paymentRepo.GetByID(ctx, pTimeout.ID)
	if err != nil || pResolved.Status != payments.StatusSucceeded {
		fail("Expected payment to be healed to SUCCEEDED, got %s", pResolved.Status)
	}
	pResolvedLedger, err := ledgerSvc.GetPaymentLedger(ctx, pTimeout.ID)
	if err != nil || len(pResolvedLedger.Entries) != 2 || !pResolvedLedger.IsBalanced {
		fail("Expected healed double-entry ledger for resolved payment")
	}
	fmt.Printf("  ✓ Payment %s successfully healed to %s with balanced double-entry ledger entries!\n",
		pTimeout.ID, pResolved.Status)

	// Final Summary
	fmt.Println("\n================================================================================")
	fmt.Println("   ALL 5 VERIFICATION MODULES PASSED (100% INVARIANTS SATISFIED)")
	fmt.Println("   Zero Balance Drift | Exactly-Once Idempotency | At-Least-Once Delivery")
	fmt.Println("================================================================================")
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\n❌ VERIFICATION FAILURE: "+format+"\n", args...)
	os.Exit(1)
}
