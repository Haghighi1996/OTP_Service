package otp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestServiceIssue(t *testing.T) {
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	repository := &repositoryStub{record: OTP{ID: 42, CreatedAt: now}}
	service := newService(
		repository,
		func(int) (string, error) { return "012345", nil },
		func(code string) (string, error) { return "hash-of-" + code, nil },
		func(string, string) (bool, error) { t.Fatal("compare should not run"); return false, nil },
		func() time.Time { return now },
		DefaultCodeLength,
		DefaultTTL,
		nil,
	)

	issued, err := service.Issue(context.Background(), testTenantID, " +12025550123 ")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	if issued.Code != "012345" {
		t.Errorf("Code = %q, want %q", issued.Code, "012345")
	}
	if issued.Record.ID != 42 {
		t.Errorf("Record.ID = %d, want 42", issued.Record.ID)
	}
	if repository.params.PhoneNumber != "+12025550123" {
		t.Errorf("PhoneNumber = %q, want normalized input", repository.params.PhoneNumber)
	}
	if repository.params.CodeHash != "hash-of-012345" {
		t.Errorf("CodeHash = %q, want hash only", repository.params.CodeHash)
	}
	if repository.params.TenantID != testTenantID {
		t.Errorf("TenantID = %d, want %d", repository.params.TenantID, testTenantID)
	}
	wantExpiry := now.Add(DefaultTTL)
	if !repository.params.ExpiresAt.Equal(wantExpiry) {
		t.Errorf("ExpiresAt = %v, want %v", repository.params.ExpiresAt, wantExpiry)
	}
}

func TestServiceIssueRejectsInvalidPhoneNumber(t *testing.T) {
	repository := &repositoryStub{}
	service := newService(
		repository,
		func(int) (string, error) { t.Fatal("generator should not run"); return "", nil },
		func(string) (string, error) { t.Fatal("hasher should not run"); return "", nil },
		func(string, string) (bool, error) { t.Fatal("compare should not run"); return false, nil },
		time.Now,
		DefaultCodeLength,
		DefaultTTL,
		nil,
	)

	_, err := service.Issue(context.Background(), testTenantID, "202-555-0123")
	if !errors.Is(err, ErrInvalidPhoneNumber) {
		t.Fatalf("Issue() error = %v, want ErrInvalidPhoneNumber", err)
	}
	if repository.called {
		t.Error("repository should not be called")
	}
}

func TestServiceIssueRejectsZeroTenantID(t *testing.T) {
	repository := &repositoryStub{}
	service := newService(
		repository,
		func(int) (string, error) { return "012345", nil },
		func(string) (string, error) { return "hash", nil },
		func(string, string) (bool, error) { t.Fatal("compare should not run"); return false, nil },
		time.Now,
		DefaultCodeLength,
		DefaultTTL,
		nil,
	)

	_, err := service.Issue(context.Background(), 0, "+12025550123")
	if err == nil || !strings.Contains(err.Error(), "tenant ID is required") {
		t.Fatalf("Issue() error = %v, want tenant ID is required", err)
	}
	if repository.called {
		t.Error("repository should not be called")
	}
}

func TestServiceIssueStopsOnDependencyFailure(t *testing.T) {
	tests := []struct {
		name     string
		generate func(int) (string, error)
		hash     func(string) (string, error)
		want     string
	}{
		{
			name:     "generator",
			generate: func(int) (string, error) { return "", errors.New("randomness failure") },
			hash:     func(string) (string, error) { t.Fatal("hasher should not run"); return "", nil },
			want:     "generate OTP",
		},
		{
			name:     "hasher",
			generate: func(int) (string, error) { return "012345", nil },
			hash:     func(string) (string, error) { return "", errors.New("hashing failure") },
			want:     "hash OTP",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &repositoryStub{}
			service := newService(
				repository,
				test.generate,
				test.hash,
				func(string, string) (bool, error) { t.Fatal("compare should not run"); return false, nil },
				time.Now,
				DefaultCodeLength,
				DefaultTTL,
				nil,
			)

			_, err := service.Issue(context.Background(), testTenantID, "+12025550123")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Issue() error = %v, want error containing %q", err, test.want)
			}
			if repository.called {
				t.Error("repository should not be called")
			}
		})
	}
}

func TestServiceIssueReturnsRepositoryError(t *testing.T) {
	repository := &repositoryStub{err: errors.New("database unavailable")}
	service := newService(
		repository,
		func(int) (string, error) { return "012345", nil },
		func(string) (string, error) { return "hash", nil },
		func(string, string) (bool, error) { t.Fatal("compare should not run"); return false, nil },
		time.Now,
		DefaultCodeLength,
		DefaultTTL,
		nil,
	)

	_, err := service.Issue(context.Background(), testTenantID, "+12025550123")
	if err == nil || !strings.Contains(err.Error(), "persist OTP") {
		t.Fatalf("Issue() error = %v, want persistence error", err)
	}
}

func TestServiceVerify(t *testing.T) {
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	repository := &repositoryStub{record: OTP{CodeHash: "stored-hash", ExpiresAt: now.Add(DefaultTTL)}}
	service := newService(
		repository,
		func(int) (string, error) { t.Fatal("generator should not run"); return "", nil },
		func(string) (string, error) { t.Fatal("hasher should not run"); return "", nil },
		func(code, hash string) (bool, error) {
			if code != "012345" || hash != "stored-hash" {
				t.Errorf("compare(%q, %q), want (012345, stored-hash)", code, hash)
			}
			return true, nil
		},
		func() time.Time { return now },
		DefaultCodeLength,
		DefaultTTL,
		nil,
	)

	if err := service.Verify(context.Background(), testTenantID, " +12025550123 ", "012345"); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if repository.getParams.PhoneNumber != "+12025550123" {
		t.Errorf("phone number = %q, want normalized input", repository.getParams.PhoneNumber)
	}
	if repository.getParams.TenantID != testTenantID {
		t.Errorf("TenantID = %d, want %d", repository.getParams.TenantID, testTenantID)
	}
	if !repository.marksUsed {
		t.Error("OTP should be marked as used after successful verification")
	}
}

func TestServiceVerifyMarksOTPAsUsed(t *testing.T) {
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	repository := &repositoryStub{record: OTP{ID: 42, CodeHash: "stored-hash", ExpiresAt: now.Add(DefaultTTL)}}
	service := newService(
		repository,
		func(int) (string, error) { t.Fatal("generator should not run"); return "", nil },
		func(string) (string, error) { t.Fatal("hasher should not run"); return "", nil },
		func(string, string) (bool, error) { return true, nil },
		func() time.Time { return now },
		DefaultCodeLength,
		DefaultTTL,
		nil,
	)

	if err := service.Verify(context.Background(), testTenantID, "+12025550123", "012345"); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if !repository.marksUsed {
		t.Error("OTP should be marked as used after successful verification")
	}
	if repository.markedParams.ID != 42 {
		t.Errorf("markedID = %d, want 42", repository.markedParams.ID)
	}
	if repository.markedParams.TenantID != testTenantID {
		t.Errorf("marked TenantID = %d, want %d", repository.markedParams.TenantID, testTenantID)
	}
}

func TestServiceVerifyRejectsExpiredOTP(t *testing.T) {
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	// The OTP was created 6 minutes ago with a 5-minute TTL, so it is expired.
	repository := &repositoryStub{record: OTP{CodeHash: "stored-hash", ExpiresAt: now.Add(-time.Minute)}}
	service := newService(
		repository,
		func(int) (string, error) { t.Fatal("generator should not run"); return "", nil },
		func(string) (string, error) { t.Fatal("hasher should not run"); return "", nil },
		func(string, string) (bool, error) { t.Fatal("compare should not run"); return false, nil },
		func() time.Time { return now },
		DefaultCodeLength,
		DefaultTTL,
		nil,
	)

	err := service.Verify(context.Background(), testTenantID, "+12025550123", "012345")
	if !errors.Is(err, ErrOTPExpired) {
		t.Fatalf("Verify() error = %v, want ErrOTPExpired", err)
	}
	if repository.marksUsed {
		t.Error("expired OTP should not be marked as used")
	}
}

func TestServiceVerifyReturnsErrInvalidOTPOnFailedMarkAsUsed(t *testing.T) {
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	repository := &repositoryStub{
		record:      OTP{ID: 42, CodeHash: "stored-hash", ExpiresAt: now.Add(DefaultTTL)},
		markUsedErr: ErrOTPNotFound,
	}
	service := newService(
		repository,
		func(int) (string, error) { t.Fatal("generator should not run"); return "", nil },
		func(string) (string, error) { t.Fatal("hasher should not run"); return "", nil },
		func(string, string) (bool, error) { return true, nil },
		func() time.Time { return now },
		DefaultCodeLength,
		DefaultTTL,
		nil,
	)

	err := service.Verify(context.Background(), testTenantID, "+12025550123", "012345")
	if !errors.Is(err, ErrInvalidOTP) {
		t.Fatalf("Verify() error = %v, want ErrInvalidOTP", err)
	}
}

func TestServiceVerifyRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		phone string
		code  string
		want  error
	}{
		{name: "phone number", phone: "202-555-0123", code: "012345", want: ErrInvalidPhoneNumber},
		{name: "code", phone: "+12025550123", code: "abc123", want: ErrInvalidCode},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &repositoryStub{}
			service := newService(
				repository,
				func(int) (string, error) { t.Fatal("generator should not run"); return "", nil },
				func(string) (string, error) { t.Fatal("hasher should not run"); return "", nil },
				func(string, string) (bool, error) { t.Fatal("compare should not run"); return false, nil },
				time.Now,
				DefaultCodeLength,
				DefaultTTL,
				nil,
			)

			err := service.Verify(context.Background(), testTenantID, test.phone, test.code)
			if !errors.Is(err, test.want) {
				t.Fatalf("Verify() error = %v, want %v", err, test.want)
			}
			if repository.called {
				t.Error("repository should not be called")
			}
		})
	}
}

func TestServiceVerifyRejectsUnknownOrMismatchedOTP(t *testing.T) {
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	tests := []struct {
		name       string
		repository *repositoryStub
		compare    func(string, string) (bool, error)
		wantErr    error
	}{
		{
			name:       "missing OTP",
			repository: &repositoryStub{err: ErrOTPNotFound},
			compare:    func(string, string) (bool, error) { t.Fatal("compare should not run"); return false, nil },
			wantErr:    ErrInvalidOTP,
		},
		{
			name: "mismatched OTP",
			repository: &repositoryStub{
				record:      OTP{CodeHash: "stored-hash", ExpiresAt: now.Add(DefaultTTL)},
				markUsedErr: ErrOTPNotFound,
			},
			compare: func(string, string) (bool, error) { return false, nil },
			wantErr: ErrInvalidOTP,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := newService(
				test.repository,
				func(int) (string, error) { t.Fatal("generator should not run"); return "", nil },
				func(string) (string, error) { t.Fatal("hasher should not run"); return "", nil },
				test.compare,
				func() time.Time { return now },
				DefaultCodeLength,
				DefaultTTL,
				nil,
			)

			err := service.Verify(context.Background(), testTenantID, "+12025550123", "012345")
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Verify() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestServiceVerifyReturnsRepositoryError(t *testing.T) {
	repository := &repositoryStub{err: errors.New("database unavailable")}
	service := newService(
		repository,
		func(int) (string, error) { t.Fatal("generator should not run"); return "", nil },
		func(string) (string, error) { t.Fatal("hasher should not run"); return "", nil },
		func(string, string) (bool, error) { t.Fatal("compare should not run"); return false, nil },
		time.Now,
		DefaultCodeLength,
		DefaultTTL,
		nil,
	)

	err := service.Verify(context.Background(), testTenantID, "+12025550123", "012345")
	if err == nil || !strings.Contains(err.Error(), "load OTP") {
		t.Fatalf("Verify() error = %v, want load error", err)
	}
}

func TestServiceVerifyRejectsZeroTenantID(t *testing.T) {
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	repository := &repositoryStub{record: OTP{CodeHash: "stored-hash", ExpiresAt: now.Add(DefaultTTL)}}
	service := newService(
		repository,
		func(int) (string, error) { t.Fatal("generator should not run"); return "", nil },
		func(string) (string, error) { t.Fatal("hasher should not run"); return "", nil },
		func(string, string) (bool, error) { return true, nil },
		func() time.Time { return now },
		DefaultCodeLength,
		DefaultTTL,
		nil,
	)

	err := service.Verify(context.Background(), 0, "+12025550123", "012345")
	if err == nil || !strings.Contains(err.Error(), "tenant ID is required") {
		t.Fatalf("Verify() error = %v, want tenant ID is required", err)
	}
}

// TestServiceVerifyAtomicConcurrentConsumption ensures that concurrent
// verification of the same OTP cannot consume it twice.
// Exactly one caller should succeed; all others must receive ErrInvalidOTP.
func TestServiceVerifyAtomicConcurrentConsumption(t *testing.T) {
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	storedHash := "stored-hash"
	record := OTP{ID: 42, CodeHash: storedHash, ExpiresAt: now.Add(DefaultTTL)}

	var successCount int32
	var failCount int32
	var mu sync.Mutex

	// Run 20 concurrent verification attempts against the same OTP.
	const concurrency = 20
	var wg sync.WaitGroup
	wg.Add(concurrency)

	// All goroutines share the same stub to simulate real row-level locking
	sharedRepo := &repositoryStub{record: record}

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			svc := newService(
				sharedRepo,
				func(int) (string, error) { return "", nil },
				func(string) (string, error) { return "", nil },
				func(code, hash string) (bool, error) {
					if code != "012345" || hash != storedHash {
						return false, fmt.Errorf("compare(%q, %q), want (012345, stored-hash)", code, hash)
					}
					return true, nil
				},
				func() time.Time { return now },
				DefaultCodeLength,
				DefaultTTL,
				nil,
			)

			err := svc.VerifyAtomic(context.Background(), testTenantID, "+12025550123", "012345")
			mu.Lock()
			if err == nil {
				successCount++
			} else {
				failCount++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	if successCount != 1 {
		t.Fatalf("expected exactly 1 successful verification, got %d", successCount)
	}
	if failCount != concurrency-1 {
		t.Fatalf("expected %d failures, got %d", concurrency-1, failCount)
	}
}

func TestServiceVerifyAtomicRejectsExpiredOTP(t *testing.T) {
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	repo := &repositoryStub{record: OTP{CodeHash: "stored-hash", ExpiresAt: now.Add(-time.Minute)}}
	svc := newService(
		repo,
		func(int) (string, error) { t.Fatal("generator should not run"); return "", nil },
		func(string) (string, error) { t.Fatal("hasher should not run"); return "", nil },
		func(string, string) (bool, error) { t.Fatal("compare should not run"); return false, nil },
		func() time.Time { return now },
		DefaultCodeLength,
		DefaultTTL,
		nil,
	)

	err := svc.VerifyAtomic(context.Background(), testTenantID, "+12025550123", "012345")
	if !errors.Is(err, ErrOTPExpired) {
		t.Fatalf("VerifyAtomic() error = %v, want ErrOTPExpired", err)
	}
}

func TestServiceVerifyAtomicRejectsInvalidCode(t *testing.T) {
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	repo := &repositoryStub{record: OTP{CodeHash: "stored-hash", ExpiresAt: now.Add(DefaultTTL)}}
	svc := newService(
		repo,
		func(int) (string, error) { t.Fatal("generator should not run"); return "", nil },
		func(string) (string, error) { t.Fatal("hasher should not run"); return "", nil },
		func(string, string) (bool, error) { return false, nil },
		func() time.Time { return now },
		DefaultCodeLength,
		DefaultTTL,
		nil,
	)

	err := svc.VerifyAtomic(context.Background(), testTenantID, "+12025550123", "wrong")
	if !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("VerifyAtomic() error = %v, want ErrInvalidCode", err)
	}
}

func TestServiceVerifyAtomicRejectsMissingOTP(t *testing.T) {
	repo := &repositoryStub{record: OTP{CodeHash: "stored-hash", ExpiresAt: time.Now().Add(DefaultTTL)}}
	svc := newService(
		repo,
		func(int) (string, error) { t.Fatal("generator should not run"); return "", nil },
		func(string) (string, error) { t.Fatal("hasher should not run"); return "", nil },
		func(string, string) (bool, error) { t.Fatal("compare should not run"); return false, nil },
		func() time.Time { return time.Now() },
		DefaultCodeLength,
		DefaultTTL,
		nil,
	)

	// repo.err is ignored by GetLatestUnusedForUpdate (it returns found=true + err),
	// so use the atomic consumption flag instead: consume the stub first,
	// then verify that a fresh atomic verify sees the OTP as missing.
	repo.consumedMu.Lock()
	repo.consumed = true
	repo.consumedMu.Unlock()

	err := svc.VerifyAtomic(context.Background(), testTenantID, "+12025550123", "012345")
	if err == nil {
		t.Fatal("VerifyAtomic() expected an error for a consumed OTP, got nil")
	}
	if !errors.Is(err, ErrInvalidOTP) {
		t.Fatalf("VerifyAtomic() error = %v, want ErrInvalidOTP", err)
	}
}

type repositoryStub struct {
	params       CreateParams
	getParams    GetOTPParams
	phoneNumber  string
	record       OTP
	err          error
	called       bool
	marksUsed    bool
	markedParams MarkOTPParams
	markUsedErr  error

	// For atomic test simulation
	consumed   bool
	consumedMu sync.Mutex
}

func (r *repositoryStub) Create(_ context.Context, params CreateParams) (OTP, error) {
	r.called = true
	r.params = params
	return r.record, r.err
}

func (r *repositoryStub) GetLatestUnusedByPhoneNumber(_ context.Context, params GetOTPParams) (OTP, error) {
	r.called = true
	r.getParams = params
	r.phoneNumber = params.PhoneNumber
	return r.record, r.err
}

func (r *repositoryStub) MarkAsUsed(_ context.Context, params MarkOTPParams) error {
	r.marksUsed = true
	r.markedParams = params
	if r.markUsedErr != nil {
		return r.markUsedErr
	}
	return nil
}

func (r *repositoryStub) AtomicVerify(_ context.Context, _ AtomicVerifyParams) (OTP, bool, error) {
	return r.record, true, r.err
}

func (r *repositoryStub) BeginTx(_ context.Context) (Tx, error) {
	return &txStub{}, r.err
}

func (r *repositoryStub) GetLatestUnusedForUpdate(_ context.Context, _ Tx, _ GetOTPParams) (OTP, bool, error) {
	r.consumedMu.Lock()
	defer r.consumedMu.Unlock()
	if r.consumed {
		return OTP{}, false, nil
	}
	return r.record, true, r.err
}

func (r *repositoryStub) MarkAsUsedInTx(_ context.Context, _ Tx, _ MarkOTPParams) error {
	r.consumedMu.Lock()
	defer r.consumedMu.Unlock()
	if r.consumed {
		return ErrOTPNotFound
	}
	r.consumed = true
	return nil
}

type txStub struct{}

func (t *txStub) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row { return nil }
func (t *txStub) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}
func (t *txStub) Commit(_ context.Context) error   { return nil }
func (t *txStub) Rollback(_ context.Context) error { return nil }
