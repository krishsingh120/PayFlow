package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/krishsingh120/payflow/services/payment/internal/payments"
	"github.com/krishsingh120/payflow/services/payment/internal/webhooks"
)

// PaymentHandler handles HTTP requests for payment resources.
type PaymentHandler struct {
	svc payments.Service
}

// NewPaymentHandler returns a new PaymentHandler instance.
func NewPaymentHandler(svc payments.Service) *PaymentHandler {
	return &PaymentHandler{svc: svc}
}

// ErrorResponse represents the uniform API error schema.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// CreatePayment handles POST /v1/payments.
func (h *PaymentHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		renderError(w, http.StatusBadRequest, "BadRequest", "Idempotency-Key header is required")
		return
	}

	var req payments.CreatePaymentRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		renderError(w, http.StatusBadRequest, "BadRequest", "Invalid JSON request body: "+err.Error())
		return
	}

	res, isReplay, err := h.svc.CreatePayment(r.Context(), idempotencyKey, req)
	if err != nil {
		switch {
		case errors.Is(err, payments.ErrIdempotencyConflict):
			renderError(w, http.StatusConflict, "IdempotencyConflict", err.Error())
		case strings.Contains(err.Error(), "must be") || strings.Contains(err.Error(), "exceeds"):
			renderError(w, http.StatusUnprocessableEntity, "ValidationError", err.Error())
		default:
			renderError(w, http.StatusInternalServerError, "InternalError", "Failed to process payment creation")
		}
		return
	}

	if isReplay {
		w.Header().Set("Idempotent-Replay", "true")
		renderJSON(w, http.StatusOK, res)
		return
	}

	renderJSON(w, http.StatusCreated, res)
}

// GetPayment handles GET /v1/payments/{id}.
func (h *PaymentHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		renderError(w, http.StatusBadRequest, "BadRequest", "Payment ID is required")
		return
	}

	res, err := h.svc.GetPayment(r.Context(), id)
	if err != nil {
		if errors.Is(err, payments.ErrPaymentNotFound) {
			renderError(w, http.StatusNotFound, "NotFound", "payment not found")
			return
		}
		renderError(w, http.StatusInternalServerError, "InternalError", "Failed to retrieve payment")
		return
	}

	renderJSON(w, http.StatusOK, res)
}

// ListPayments handles GET /v1/payments.
func (h *PaymentHandler) ListPayments(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	limit := 20
	if l := query.Get("limit"); l != "" {
		parsed, err := strconv.Atoi(l)
		if err != nil || parsed <= 0 {
			renderError(w, http.StatusBadRequest, "BadRequest", "limit must be a positive integer")
			return
		}
		limit = parsed
	}

	cursor := query.Get("cursor")

	var statusFilter *payments.PaymentStatus
	if s := query.Get("status"); s != "" {
		status := payments.PaymentStatus(strings.ToUpper(strings.TrimSpace(s)))
		if !payments.IsValidStatus(status) {
			renderError(w, http.StatusBadRequest, "BadRequest", "invalid status filter: "+s)
			return
		}
		statusFilter = &status
	}

	filter := payments.ListFilter{
		Limit:  limit,
		Cursor: cursor,
		Status: statusFilter,
	}

	res, err := h.svc.ListPayments(r.Context(), filter)
	if err != nil {
		renderError(w, http.StatusInternalServerError, "InternalError", "Failed to list payments")
		return
	}

	renderJSON(w, http.StatusOK, res)
}

// CreateRefund handles POST /v1/payments/{id}/refunds.
func (h *PaymentHandler) CreateRefund(w http.ResponseWriter, r *http.Request) {
	paymentID := chi.URLParam(r, "id")
	if strings.TrimSpace(paymentID) == "" {
		renderError(w, http.StatusBadRequest, "BadRequest", "Payment ID is required")
		return
	}

	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))

	var req payments.CreateRefundRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		renderError(w, http.StatusBadRequest, "BadRequest", "Invalid JSON request body: "+err.Error())
		return
	}

	res, isReplay, err := h.svc.CreateRefund(r.Context(), paymentID, idempotencyKey, req)
	if err != nil {
		switch {
		case errors.Is(err, payments.ErrPaymentNotFound):
			renderError(w, http.StatusNotFound, "NotFound", "Payment not found")
		case errors.Is(err, payments.ErrRefundInvalidPaymentStatus):
			renderError(w, http.StatusConflict, "InvalidState", err.Error())
		case errors.Is(err, payments.ErrRefundAmountExceeded):
			renderError(w, http.StatusUnprocessableEntity, "OverRefundExceeded", err.Error())
		case errors.Is(err, payments.ErrIdempotencyConflict):
			renderError(w, http.StatusConflict, "IdempotencyConflict", err.Error())
		case strings.Contains(err.Error(), "must be") || strings.Contains(err.Error(), "exceeds"):
			renderError(w, http.StatusUnprocessableEntity, "ValidationError", err.Error())
		default:
			renderError(w, http.StatusInternalServerError, "InternalError", "Failed to process refund: "+err.Error())
		}
		return
	}

	if isReplay {
		w.Header().Set("Idempotent-Replay", "true")
		renderJSON(w, http.StatusOK, res)
		return
	}

	renderJSON(w, http.StatusCreated, res)
}

// ListRefunds handles GET /v1/payments/{id}/refunds.
func (h *PaymentHandler) ListRefunds(w http.ResponseWriter, r *http.Request) {
	paymentID := chi.URLParam(r, "id")
	if strings.TrimSpace(paymentID) == "" {
		renderError(w, http.StatusBadRequest, "BadRequest", "Payment ID is required")
		return
	}

	res, err := h.svc.ListRefunds(r.Context(), paymentID)
	if err != nil {
		renderError(w, http.StatusInternalServerError, "InternalError", "Failed to retrieve refunds")
		return
	}

	renderJSON(w, http.StatusOK, res)
}

// WebhookTestReceiver handles POST /v1/webhooks/test-receiver.
func (h *PaymentHandler) WebhookTestReceiver(w http.ResponseWriter, r *http.Request) {
	sigHeader := r.Header.Get("X-PayFlow-Signature")
	event := r.Header.Get("X-PayFlow-Event")
	deliveryID := r.Header.Get("X-PayFlow-Delivery")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		renderError(w, http.StatusBadRequest, "BadRequest", "Failed to read request body")
		return
	}

	secret := os.Getenv("WEBHOOK_SECRET")
	if secret == "" {
		secret = "whsec_test_secret_development_mode"
	}

	verified := false
	if sigHeader != "" {
		if err := webhooks.Verify(secret, body, sigHeader, 10*time.Minute); err != nil {
			renderError(w, http.StatusUnauthorized, "Unauthorized", "Webhook signature verification failed: "+err.Error())
			return
		}
		verified = true
	}

	renderJSON(w, http.StatusOK, map[string]any{
		"status":             "received",
		"signature_verified": verified,
		"event":              event,
		"delivery_id":        deliveryID,
		"payload_size":       len(body),
	})
}

func renderJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func renderError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error:   code,
		Message: message,
	})
}
