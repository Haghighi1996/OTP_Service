package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"otp-service/internal/auth"
	"otp-service/internal/otp"
)

const maxRequestBodyBytes = 1 << 20

type otpService interface {
	Issue(context.Context, int64, string) (otp.IssuedOTP, error)
	Verify(context.Context, int64, string, string) error
	VerifyAtomic(context.Context, int64, string, string) error
}

// OTPHandler exposes HTTP endpoints for OTP operations.
type OTPHandler struct {
	service otpService
}

// NewOTPHandler creates an HTTP handler backed by the OTP service.
func NewOTPHandler(service otpService) *OTPHandler {
	return &OTPHandler{service: service}
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

	tenantID, ok := auth.GetTenantID(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid or missing API key"})
		return
	}

	issued, err := h.service.Issue(r.Context(), tenantID, request.PhoneNumber)
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

type verifyOTPRequest struct {
	PhoneNumber string `json:"phone_number"`
	Code        string `json:"code"`
}

type verifyOTPResponse struct {
	Status string `json:"status"`
}

// Verify checks a submitted OTP for a phone number. Successful responses do
// not include the plaintext code or hash.
func (h *OTPHandler) Verify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	defer r.Body.Close()

	var request verifyOTPRequest
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

	tenantID, ok := auth.GetTenantID(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid or missing API key"})
		return
	}

	err := h.service.VerifyAtomic(r.Context(), tenantID, request.PhoneNumber, request.Code)
	if errors.Is(err, otp.ErrInvalidPhoneNumber) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid phone number"})
		return
	}
	if errors.Is(err, otp.ErrInvalidCode) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid OTP code"})
		return
	}
	if errors.Is(err, otp.ErrInvalidOTP) {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid OTP"})
		return
	}
	if errors.Is(err, otp.ErrOTPExpired) {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "OTP expired"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "unable to verify OTP"})
		return
	}

	writeJSON(w, http.StatusOK, verifyOTPResponse{Status: "verified"})
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