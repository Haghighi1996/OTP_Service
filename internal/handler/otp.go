package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"otp-service/internal/otp"
)

const maxRequestBodyBytes = 1 << 20

type otpIssuer interface {
	Issue(context.Context, string) (otp.IssuedOTP, error)
}

// OTPHandler exposes HTTP endpoints for OTP operations.
type OTPHandler struct {
	issuer otpIssuer
}

// NewOTPHandler creates an HTTP handler backed by the OTP service.
func NewOTPHandler(issuer otpIssuer) *OTPHandler {
	return &OTPHandler{issuer: issuer}
}

type sendOTPRequest struct {
	PhoneNumber string `json:"phone_number"`
}

type sendOTPResponse struct {
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expires_at"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// Send issues an OTP for a phone number. The plaintext OTP is intentionally
// omitted from the response and is reserved for a future delivery adapter.
func (h *OTPHandler) Send(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	defer r.Body.Close()

	var request sendOTPRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}
	if err := ensureSingleJSONValue(decoder); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	issued, err := h.issuer.Issue(r.Context(), request.PhoneNumber)
	if errors.Is(err, otp.ErrInvalidPhoneNumber) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid phone number"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "unable to issue OTP"})
		return
	}

	writeJSON(w, http.StatusAccepted, sendOTPResponse{
		Status:    "accepted",
		ExpiresAt: issued.Record.ExpiresAt,
	})
}

func ensureSingleJSONValue(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
