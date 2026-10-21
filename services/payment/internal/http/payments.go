package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/krishsingh120/payflow/services/payment/internal/payments"
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
