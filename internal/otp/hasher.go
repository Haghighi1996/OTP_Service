package otp

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCode = errors.New("OTP code must contain only decimal digits and use a valid length")

// HashCode creates a one-way bcrypt hash for an OTP. Plaintext OTPs must only
// exist in memory long enough to deliver or verify them.
func HashCode(code string) (string, error) {
	if err := validateCode(code); err != nil {
		return "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash OTP: %w", err)
	}

	return string(hash), nil
}

// VerifyCode compares an OTP with its bcrypt hash. A non-matching code is not
// an error; malformed persisted hashes are reported to the caller.
func VerifyCode(code, codeHash string) (bool, error) {
	if err := validateCode(code); err != nil {
		return false, err
	}

	err := bcrypt.CompareHashAndPassword([]byte(codeHash), []byte(code))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("compare OTP hash: %w", err)
	}

	return true, nil
}

func validateCode(code string) error {
	if len(code) < minimumCodeLength || len(code) > maximumCodeLength {
		return ErrInvalidCode
	}

	for _, character := range code {
		if character < '0' || character > '9' {
			return ErrInvalidCode
		}
	}

	return nil
}
