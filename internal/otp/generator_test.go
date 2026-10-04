package otp

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestGenerateCode(t *testing.T) {
	code, err := GenerateCode(DefaultCodeLength)
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}

	if len(code) != DefaultCodeLength {
		t.Errorf("code length = %d, want %d", len(code), DefaultCodeLength)
	}
	if strings.Trim(code, decimalDigits) != "" {
		t.Errorf("code = %q, want decimal digits only", code)
	}
}

func TestGenerateCodeRejectsInvalidLength(t *testing.T) {
	for _, length := range []int{0, minimumCodeLength - 1, maximumCodeLength + 1} {
		_, err := GenerateCode(length)
		if err == nil {
			t.Errorf("GenerateCode(%d) error = nil, want an error", length)
		}
	}
}

func TestGenerateCodePropagatesRandomnessFailure(t *testing.T) {
	_, err := generateCode(errorReader{}, DefaultCodeLength)
	if !errors.Is(err, errRandomnessUnavailable) {
		t.Fatalf("generateCode() error = %v, want %v", err, errRandomnessUnavailable)
	}
}

func TestGenerateCodePreservesLeadingZeros(t *testing.T) {
	code, err := generateCode(bytes.NewReader(make([]byte, DefaultCodeLength)), DefaultCodeLength)
	if err != nil {
		t.Fatalf("generateCode() error = %v", err)
	}

	if code != "000000" {
		t.Errorf("code = %q, want %q", code, "000000")
	}
}

var errRandomnessUnavailable = errors.New("randomness unavailable")

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errRandomnessUnavailable
}
