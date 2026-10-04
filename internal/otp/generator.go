package otp

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
)

const (
	DefaultCodeLength = 6
	minimumCodeLength = 4
	maximumCodeLength = 10
)

const decimalDigits = "0123456789"

// GenerateCode creates a cryptographically secure numeric OTP.
func GenerateCode(length int) (string, error) {
	return generateCode(rand.Reader, length)
}

func generateCode(reader io.Reader, length int) (string, error) {
	if length < minimumCodeLength || length > maximumCodeLength {
		return "", fmt.Errorf("OTP length must be between %d and %d digits", minimumCodeLength, maximumCodeLength)
	}

	code := make([]byte, length)
	digitCount := big.NewInt(int64(len(decimalDigits)))
	for index := range code {
		digit, err := rand.Int(reader, digitCount)
		if err != nil {
			return "", fmt.Errorf("generate OTP digit: %w", err)
		}
		code[index] = decimalDigits[digit.Int64()]
	}

	return string(code), nil
}
