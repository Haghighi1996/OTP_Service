package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"otp-service/internal/delivery"
	"otp-service/internal/otp"
	"otp-service/internal/worker"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOTPVerifyHTTPFlow(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	repository := otp.NewPostgresRepository(pool)
	workerPool := worker.NewWorkerPool(worker.WorkerPoolConfig{
		QueueCapacity: 10,
		WorkerCount:   1,
		Delivery:      delivery.NoOp{},
	})
	_ = workerPool.Start(context.Background())
	defer workerPool.Shutdown(context.Background())
	service := otp.NewService(repository, workerPool)
	otpHandler := NewOTPHandler(service)

	phoneNumber := "+12025550177"
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM otps WHERE phone_number = $1", phoneNumber)
	})

	codeHash, err := otp.HashCode("012345")
	if err != nil {
		t.Fatalf("HashCode() error = %v", err)
	}
	_, err = repository.Create(ctx, otp.CreateParams{
		PhoneNumber: phoneNumber,
		CodeHash:    codeHash,
		ExpiresAt:   time.Now().UTC().Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	success := httptest.NewRequest(http.MethodPost, "/v1/otp/verify", bytes.NewBufferString(`{"phone_number":"+12025550177","code":"012345"}`))
	success.Header.Set("Content-Type", "application/json")
	successResponse := httptest.NewRecorder()
	otpHandler.Verify(successResponse, success)
	if successResponse.Code != http.StatusOK {
		t.Fatalf("valid OTP status = %d, want %d", successResponse.Code, http.StatusOK)
	}
	var verified verifyOTPResponse
	if err := json.NewDecoder(successResponse.Body).Decode(&verified); err != nil {
		t.Fatalf("decode success response: %v", err)
	}
	if verified.Status != "verified" {
		t.Errorf("status = %q, want verified", verified.Status)
	}

	invalid := httptest.NewRequest(http.MethodPost, "/v1/otp/verify", bytes.NewBufferString(`{"phone_number":"+12025550177","code":"999999"}`))
	invalidResponse := httptest.NewRecorder()
	otpHandler.Verify(invalidResponse, invalid)
	if invalidResponse.Code != http.StatusUnauthorized {
		t.Fatalf("invalid OTP status = %d, want %d", invalidResponse.Code, http.StatusUnauthorized)
	}

	missing := httptest.NewRequest(http.MethodPost, "/v1/otp/verify", bytes.NewBufferString(`{"phone_number":"+10999999999","code":"012345"}`))
	missingResponse := httptest.NewRecorder()
	otpHandler.Verify(missingResponse, missing)
	if missingResponse.Code != http.StatusUnauthorized {
		t.Fatalf("missing OTP status = %d, want %d", missingResponse.Code, http.StatusUnauthorized)
	}
}

func TestOTPVerifyHTTPExpired(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	repository := otp.NewPostgresRepository(pool)
	workerPool := worker.NewWorkerPool(worker.WorkerPoolConfig{
		QueueCapacity: 10,
		WorkerCount:   1,
		Delivery:      delivery.NoOp{},
	})
	_ = workerPool.Start(context.Background())
	defer workerPool.Shutdown(context.Background())
	service := otp.NewService(repository, workerPool)
	otpHandler := NewOTPHandler(service)

	phoneNumber := "+12025550178"
	_, _ = pool.Exec(ctx, "DELETE FROM otps WHERE phone_number = $1", phoneNumber)
	defer pool.Exec(ctx, "DELETE FROM otps WHERE phone_number = $1", phoneNumber)

	codeHash, err := otp.HashCode("012345")
	if err != nil {
		t.Fatalf("HashCode() error = %v", err)
	}
	// Insert directly with a past expiration to bypass Create's validation.
	expiredAt := time.Now().UTC().Add(-10 * time.Minute)
	_, err = pool.Exec(ctx, `
		INSERT INTO otps (phone_number, otp_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4)`, phoneNumber, codeHash, expiredAt, expiredAt.Add(-time.Minute))
	if err != nil {
		t.Fatalf("insert expired OTP: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/otp/verify", bytes.NewBufferString(`{"phone_number":"+12025550178","code":"012345"}`))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	otpHandler.Verify(response, req)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expired OTP status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	var body errorResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error != "OTP expired" {
		t.Errorf("error = %q, want %q", body.Error, "OTP expired")
	}
}
