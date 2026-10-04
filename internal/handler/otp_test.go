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
	issuer := &otpServiceStub{issued: otp.IssuedOTP{
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
		issuer *otpServiceStub
		want   int
	}{
		{
			name:   "unsupported method",
			method: http.MethodGet,
			issuer: &otpServiceStub{},
			want:   http.StatusMethodNotAllowed,
		},
		{
			name:   "unknown field",
			method: http.MethodPost,
			body:   `{"phone_number":"+12025550123","unexpected":true}`,
			issuer: &otpServiceStub{},
			want:   http.StatusBadRequest,
		},
		{
			name:   "invalid phone number",
			method: http.MethodPost,
			body:   `{"phone_number":"invalid"}`,
			issuer: &otpServiceStub{err: otp.ErrInvalidPhoneNumber},
			want:   http.StatusBadRequest,
		},
		{
			name:   "service failure",
			method: http.MethodPost,
			body:   `{"phone_number":"+12025550123"}`,
			issuer: &otpServiceStub{err: errors.New("database unavailable")},
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

func TestOTPHandlerVerify(t *testing.T) {
	service := &otpServiceStub{}
	otpHandler := NewOTPHandler(service)

	request := httptest.NewRequest(http.MethodPost, "/v1/otp/verify", strings.NewReader(`{"phone_number":"+12025550123","code":"012345"}`))
	response := httptest.NewRecorder()
	otpHandler.Verify(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if service.phoneNumber != "+12025550123" || service.code != "012345" {
		t.Errorf("verify args = (%q, %q), want request values", service.phoneNumber, service.code)
	}
	if response.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", response.Header().Get("Content-Type"))
	}

	var body verifyOTPResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "verified" {
		t.Errorf("response = %+v, want verified status", body)
	}
}

func TestOTPHandlerVerifyRejectsInvalidRequest(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		body    string
		service *otpServiceStub
		want    int
		error   string
	}{
		{
			name:    "unsupported method",
			method:  http.MethodGet,
			service: &otpServiceStub{},
			want:    http.StatusMethodNotAllowed,
			error:   "method not allowed",
		},
		{
			name:    "invalid JSON",
			method:  http.MethodPost,
			body:    `{"phone_number":`,
			service: &otpServiceStub{},
			want:    http.StatusBadRequest,
			error:   "invalid request body",
		},
		{
			name:    "unknown field",
			method:  http.MethodPost,
			body:    `{"phone_number":"+12025550123","code":"012345","unexpected":true}`,
			service: &otpServiceStub{},
			want:    http.StatusBadRequest,
			error:   "invalid request body",
		},
		{
			name:    "invalid phone number",
			method:  http.MethodPost,
			body:    `{"phone_number":"invalid","code":"012345"}`,
			service: &otpServiceStub{err: otp.ErrInvalidPhoneNumber},
			want:    http.StatusBadRequest,
			error:   "invalid phone number",
		},
		{
			name:    "invalid code format",
			method:  http.MethodPost,
			body:    `{"phone_number":"+12025550123","code":"abc"}`,
			service: &otpServiceStub{err: otp.ErrInvalidCode},
			want:    http.StatusBadRequest,
			error:   "invalid OTP code",
		},
		{
			name:    "invalid OTP",
			method:  http.MethodPost,
			body:    `{"phone_number":"+12025550123","code":"012345"}`,
			service: &otpServiceStub{err: otp.ErrInvalidOTP},
			want:    http.StatusUnauthorized,
			error:   "invalid OTP",
		},
		{
			name:    "expired OTP",
			method:  http.MethodPost,
			body:    `{"phone_number":"+12025550123","code":"012345"}`,
			service: &otpServiceStub{err: otp.ErrOTPExpired},
			want:    http.StatusUnauthorized,
			error:   "OTP expired",
		},
		{
			name:    "service failure",
			method:  http.MethodPost,
			body:    `{"phone_number":"+12025550123","code":"012345"}`,
			service: &otpServiceStub{err: errors.New("database unavailable")},
			want:    http.StatusInternalServerError,
			error:   "unable to verify OTP",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			otpHandler := NewOTPHandler(test.service)
			request := httptest.NewRequest(test.method, "/v1/otp/verify", strings.NewReader(test.body))
			response := httptest.NewRecorder()

			otpHandler.Verify(response, request)

			if response.Code != test.want {
				t.Errorf("status = %d, want %d", response.Code, test.want)
			}
			if test.method != http.MethodPost && response.Header().Get("Allow") != http.MethodPost {
				t.Errorf("Allow = %q, want %q", response.Header().Get("Allow"), http.MethodPost)
			}

			var body errorResponse
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Error != test.error {
				t.Errorf("error = %q, want %q", body.Error, test.error)
			}
		})
	}
}

type otpServiceStub struct {
	issued      otp.IssuedOTP
	err         error
	phoneNumber string
	code        string
}

func (s *otpServiceStub) Issue(_ context.Context, phoneNumber string) (otp.IssuedOTP, error) {
	s.phoneNumber = phoneNumber
	return s.issued, s.err
}

func (s *otpServiceStub) Verify(_ context.Context, phoneNumber, code string) error {
	s.phoneNumber = phoneNumber
	s.code = code
	return s.err
}
