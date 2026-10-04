package otp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const DefaultTTL = 5 * time.Minute

var (
	ErrInvalidPhoneNumber = errors.New("phone number must use E.164 format")
	ErrInvalidOTP         = errors.New("invalid OTP")
)

// IssuedOTP is the result of issuing an OTP. Code is plaintext only so a later
// delivery adapter can send it; it must never be persisted or returned by the
// public HTTP API.
type IssuedOTP struct {
	Record OTP
	Code   string
}

// Service coordinates OTP generation, hashing, expiry, and persistence.
type Service struct {
	repository Repository
	generate   func(int) (string, error)
	hash       func(string) (string, error)
	compare    func(string, string) (bool, error)
	now        func() time.Time
	codeLength int
	ttl        time.Duration
}

// NewService creates an OTP service with production defaults.
func NewService(repository Repository) *Service {
	return newService(repository, GenerateCode, HashCode, VerifyCode, time.Now, DefaultCodeLength, DefaultTTL)
}

func newService(
	repository Repository,
	generate func(int) (string, error),
	hash func(string) (string, error),
	compare func(string, string) (bool, error),
	now func() time.Time,
	codeLength int,
	ttl time.Duration,
) *Service {
	return &Service{
		repository: repository,
		generate:   generate,
		hash:       hash,
		compare:    compare,
		now:        now,
		codeLength: codeLength,
		ttl:        ttl,
	}
}

// Issue creates and persists a new OTP for a phone number.
func (s *Service) Issue(ctx context.Context, phoneNumber string) (IssuedOTP, error) {
	phoneNumber = strings.TrimSpace(phoneNumber)
	if !isE164PhoneNumber(phoneNumber) {
		return IssuedOTP{}, ErrInvalidPhoneNumber
	}

	code, err := s.generate(s.codeLength)
	if err != nil {
		return IssuedOTP{}, fmt.Errorf("generate OTP: %w", err)
	}

	codeHash, err := s.hash(code)
	if err != nil {
		return IssuedOTP{}, fmt.Errorf("hash OTP: %w", err)
	}

	record, err := s.repository.Create(ctx, CreateParams{
		PhoneNumber: phoneNumber,
		CodeHash:    codeHash,
		ExpiresAt:   s.now().UTC().Add(s.ttl),
	})
	if err != nil {
		return IssuedOTP{}, fmt.Errorf("persist OTP: %w", err)
	}

	return IssuedOTP{Record: record, Code: code}, nil
}

// Verify checks a submitted OTP against the newest unused record for a phone
// number. Expiration and one-time consumption are enforced in this method.
func (s *Service) Verify(ctx context.Context, phoneNumber, code string) error {
	phoneNumber = strings.TrimSpace(phoneNumber)
	if !isE164PhoneNumber(phoneNumber) {
		return ErrInvalidPhoneNumber
	}

	code = strings.TrimSpace(code)
	if err := validateCode(code); err != nil {
		return ErrInvalidCode
	}

	record, err := s.repository.GetLatestUnusedByPhoneNumber(ctx, phoneNumber)
	if errors.Is(err, ErrOTPNotFound) {
		return ErrInvalidOTP
	}
	if err != nil {
		return fmt.Errorf("load OTP: %w", err)
	}

	if record.ExpiresAt.Before(s.now()) {
		return ErrOTPExpired
	}

	matches, err := s.compare(code, record.CodeHash)
	if err != nil {
		if errors.Is(err, ErrInvalidCode) {
			return ErrInvalidCode
		}
		return fmt.Errorf("compare OTP: %w", err)
	}
	if !matches {
		return ErrInvalidOTP
	}

	if err := s.repository.MarkAsUsed(ctx, record.ID); err != nil {
		if errors.Is(err, ErrOTPNotFound) {
			// This can happen in a race condition where the OTP was used by
			// another request after we loaded it but before we marked it.
			return ErrInvalidOTP
		}
		return fmt.Errorf("mark OTP as used: %w", err)
	}

	return nil
}

func isE164PhoneNumber(phoneNumber string) bool {
	if len(phoneNumber) < 9 || len(phoneNumber) > 16 || phoneNumber[0] != '+' {
		return false
	}
	if phoneNumber[1] == '0' {
		return false
	}

	for _, character := range phoneNumber[1:] {
		if character < '0' || character > '9' {
			return false
		}
	}

	return true
}
