package otp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const DefaultTTL = 5 * time.Minute

var ErrInvalidPhoneNumber = errors.New("phone number must use E.164 format")

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
	now        func() time.Time
	codeLength int
	ttl        time.Duration
}

// NewService creates an OTP service with production defaults.
func NewService(repository Repository) *Service {
	return newService(repository, GenerateCode, HashCode, time.Now, DefaultCodeLength, DefaultTTL)
}

func newService(
	repository Repository,
	generate func(int) (string, error),
	hash func(string) (string, error),
	now func() time.Time,
	codeLength int,
	ttl time.Duration,
) *Service {
	return &Service{
		repository: repository,
		generate:   generate,
		hash:       hash,
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
