package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"otp-service/internal/otp"
)

func TestOTPHandlerSend(t *testing.T) {
	expiresAt := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	issuer := &issuerStub{result: otp.IssuedOTP{
		Record: otp.OTP{ExpiresAt: expiresAt},
		Code:   "012345",
	}}
	handler := NewOTPHandler(issuer)

	request := httptest.NewRequest(http.MethodPost, "/v1/otp/send", strings.NewReader(`{"phone_number":"+12025550123"}`))
	response := httptest.NewRecorder()
	handler.Send(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusAccepted)
	}
	if issuer.phoneNumber != "+12025550123" {
		t.Errorf("phone number = %q, want request phone number", issuer.phoneNumber)
	}
	if response.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", response.Header().Get("Content-Type"))
	}
	if strings.Contains(response.Body.String(), "012345") {
		t.Fatal("response leaked the plaintext OTP")
	}

	var body sendOTPResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "accepted" || !body.ExpiresAt.Equal(expiresAt) {
		t.Errorf("response = %+v, want accepted response with expiry", body)
	}
}

func TestOTPHandlerSendRejectsInvalidRequest(t *testing.T) {
	tests := []struct {
		name   string
		method string
		body   string
		issuer *issuerStub
		want   int
	}{
		{
			name:   "unsupported method",
			method: http.MethodGet,
			issuer: &issuerStub{},
			want:   http.StatusMethodNotAllowed,
		},
		{
			name:   "unknown field",
			method: http.MethodPost,
			body:   `{"phone_number":"+12025550123","unexpected":true}`,
			issuer: &issuerStub{},
			want:   http.StatusBadRequest,
		},
		{
			name:   "invalid phone number",
			method: http.MethodPost,
			body:   `{"phone_number":"invalid"}`,
			issuer: &issuerStub{err: otp.ErrInvalidPhoneNumber},
			want:   http.StatusBadRequest,
		},
		{
			name:   "service failure",
			method: http.MethodPost,
			body:   `{"phone_number":"+12025550123"}`,
			issuer: &issuerStub{err: errors.New("database unavailable")},
			want:   http.StatusInternalServerError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewOTPHandler(test.issuer)
			request := httptest.NewRequest(test.method, "/v1/otp/send", strings.NewReader(test.body))
			response := httptest.NewRecorder()

			handler.Send(response, request)

			if response.Code != test.want {
				t.Errorf("status = %d, want %d", response.Code, test.want)
			}
			if test.method != http.MethodPost && response.Header().Get("Allow") != http.MethodPost {
				t.Errorf("Allow = %q, want %q", response.Header().Get("Allow"), http.MethodPost)
			}
		})
	}
}

type issuerStub struct {
	result      otp.IssuedOTP
	err         error
	phoneNumber string
}

func (s *issuerStub) Issue(_ context.Context, phoneNumber string) (otp.IssuedOTP, error) {
	s.phoneNumber = phoneNumber
	return s.result, s.err
}
