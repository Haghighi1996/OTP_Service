package otp

import (
	"context"
	"time"
)

// OTP is a persisted one-time password record. CodeHash is always a hash; the
// plaintext code is intentionally not part of the domain model.
type OTP struct {
	ID          int64
	PhoneNumber string
	CodeHash    string
	ExpiresAt   time.Time
	UsedAt      *time.Time
	CreatedAt   time.Time
}

// CreateParams contains the fields required to persist an OTP.
type CreateParams struct {
	PhoneNumber string
	CodeHash    string
	ExpiresAt   time.Time
}

// Repository defines OTP persistence operations used by the service layer.
type Repository interface {
	Create(context.Context, CreateParams) (OTP, error)
}
