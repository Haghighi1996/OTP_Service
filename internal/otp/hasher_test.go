package otp

import (
	"errors"
	"strings"
	"testing"
)

func TestHashCodeAndVerifyCode(t *testing.T) {
	code := "012345"
	hash, err := HashCode(code)
	if err != nil {
		t.Fatalf("HashCode() error = %v", err)
	}

	if hash == code {
		t.Fatal("HashCode() returned plaintext code")
	}
	if !strings.HasPrefix(hash, "$2") {
		t.Errorf("HashCode() returned unexpected bcrypt format: %q", hash)
	}

	matches, err := VerifyCode(code, hash)
	if err != nil {
		t.Fatalf("VerifyCode() error = %v", err)
	}
	if !matches {
		t.Error("VerifyCode() = false, want true for matching code")
	}

	matches, err = VerifyCode("012346", hash)
	if err != nil {
		t.Fatalf("VerifyCode() wrong code error = %v", err)
	}
	if matches {
		t.Error("VerifyCode() = true, want false for non-matching code")
	}
}

func TestHashCodeRejectsInvalidCode(t *testing.T) {
	for _, code := range []string{"123", "12345678901", "12a456", " 12345"} {
		_, err := HashCode(code)
		if !errors.Is(err, ErrInvalidCode) {
			t.Errorf("HashCode(%q) error = %v, want ErrInvalidCode", code, err)
		}
	}
}

func TestVerifyCodeRejectsInvalidInputAndMalformedHash(t *testing.T) {
	_, err := VerifyCode("abc123", "not-a-bcrypt-hash")
	if !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("VerifyCode() invalid code error = %v, want ErrInvalidCode", err)
	}

	_, err = VerifyCode("123456", "not-a-bcrypt-hash")
	if err == nil || !strings.Contains(err.Error(), "compare OTP hash") {
		t.Fatalf("VerifyCode() malformed hash error = %v, want comparison error", err)
	}
}
