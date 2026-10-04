package otp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestServiceIssue(t *testing.T) {
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	repository := &repositoryStub{record: OTP{ID: 42, CreatedAt: now}}
	service := newService(
		repository,
		func(int) (string, error) { return "012345", nil },
		func(code string) (string, error) { return "hash-of-" + code, nil },
		func() time.Time { return now },
		DefaultCodeLength,
		DefaultTTL,
	)

	issued, err := service.Issue(context.Background(), " +12025550123 ")
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
		time.Now,
		DefaultCodeLength,
		DefaultTTL,
	)

	_, err := service.Issue(context.Background(), "202-555-0123")
	if !errors.Is(err, ErrInvalidPhoneNumber) {
		t.Fatalf("Issue() error = %v, want ErrInvalidPhoneNumber", err)
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
			service := newService(repository, test.generate, test.hash, time.Now, DefaultCodeLength, DefaultTTL)

			_, err := service.Issue(context.Background(), "+12025550123")
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
		time.Now,
		DefaultCodeLength,
		DefaultTTL,
	)

	_, err := service.Issue(context.Background(), "+12025550123")
	if err == nil || !strings.Contains(err.Error(), "persist OTP") {
		t.Fatalf("Issue() error = %v, want persistence error", err)
	}
}

type repositoryStub struct {
	params CreateParams
	record OTP
	err    error
	called bool
}

func (r *repositoryStub) Create(_ context.Context, params CreateParams) (OTP, error) {
	r.called = true
	r.params = params
	return r.record, r.err
}
