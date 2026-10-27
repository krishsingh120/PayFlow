package http

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/krishsingh120/payflow/services/payment/internal/ledger"
	"github.com/krishsingh120/payflow/services/payment/internal/reconciliation"
)

// LedgerHandler handles HTTP endpoints for double-entry bookkeeping inspections.
type LedgerHandler struct {
	svc ledger.Service
}

// NewLedgerHandler constructs a new LedgerHandler.
func NewLedgerHandler(svc ledger.Service) *LedgerHandler {
	return &LedgerHandler{svc: svc}
}

// GetPaymentLedger handles GET /v1/ledger/payments/{id}.
func (h *LedgerHandler) GetPaymentLedger(w http.ResponseWriter, r *http.Request) {
	paymentID := chi.URLParam(r, "id")
	if strings.TrimSpace(paymentID) == "" {
		renderError(w, http.StatusBadRequest, "BadRequest", "Payment ID is required")
		return
	}

	summary, err := h.svc.GetPaymentLedger(r.Context(), paymentID)
	if err != nil {
		renderError(w, http.StatusInternalServerError, "InternalError", "Failed to retrieve ledger entries: "+err.Error())
		return
	}

	renderJSON(w, http.StatusOK, summary)
}

// GetAccountBalances handles GET /v1/ledger/accounts.
func (h *LedgerHandler) GetAccountBalances(w http.ResponseWriter, r *http.Request) {
	balances, err := h.svc.GetAccountBalances(r.Context())
	if err != nil {
		renderError(w, http.StatusInternalServerError, "InternalError", "Failed to retrieve account balances")
		return
	}

	renderJSON(w, http.StatusOK, map[string]any{
		"balances": balances,
	})
}

// ListEntries handles GET /v1/ledger/entries.
func (h *LedgerHandler) ListEntries(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	entries, err := h.svc.ListRecentEntries(r.Context(), limit)
	if err != nil {
		renderError(w, http.StatusInternalServerError, "InternalError", "Failed to list ledger entries")
		return
	}

	renderJSON(w, http.StatusOK, map[string]any{
		"entries": entries,
		"count":   len(entries),
	})
}

// ReconciliationHandler handles HTTP endpoints for triggering audits and viewing reports.
type ReconciliationHandler struct {
	engine *reconciliation.Engine
}

// NewReconciliationHandler constructs a new ReconciliationHandler.
func NewReconciliationHandler(engine *reconciliation.Engine) *ReconciliationHandler {
	return &ReconciliationHandler{engine: engine}
}

// Run handles POST /v1/reconciliation/run.
func (h *ReconciliationHandler) Run(w http.ResponseWriter, r *http.Request) {
	report, err := h.engine.Run(r.Context())
	if err != nil {
		renderError(w, http.StatusInternalServerError, "InternalError", "Reconciliation run failed: "+err.Error())
		return
	}

	renderJSON(w, http.StatusOK, report)
}

// GetLatestReport handles GET /v1/reconciliation/reports/latest.
func (h *ReconciliationHandler) GetLatestReport(w http.ResponseWriter, _ *http.Request) {
	report := h.engine.GetLatestReport()
	if report == nil {
		renderError(w, http.StatusNotFound, "NotFound", "No reconciliation report has been generated yet")
		return
	}

	renderJSON(w, http.StatusOK, report)
}
