package otp

import (
	"context"
	"errors"
	"time"
)

// ErrOTPNotFound is returned when no unused OTP exists for a phone number.
var ErrOTPNotFound = errors.New("OTP not found")

// ErrOTPExpired is returned when the OTP has expired.
var ErrOTPExpired = errors.New("OTP expired")

// OTP is a persisted one-time password record. CodeHash is always a hash; the
// plaintext code is intentionally not part of the domain model.
type OTP struct {
	ID          int64
	PhoneNumber string
	CodeHash    string
	ExpiresAt   time.Time
	UsedAt      *time.Time
	CreatedAt   time.Time
	TenantID    int64
}

// CreateParams contains the fields required to persist an OTP.
type CreateParams struct {
	PhoneNumber string
	CodeHash    string
	ExpiresAt   time.Time
	TenantID    int64
}

// GetOTPParams contains the fields required to look up an OTP.
type GetOTPParams struct {
	PhoneNumber string
	TenantID    int64
}

// MarkOTPParams contains the fields required to mark an OTP as used.
type MarkOTPParams struct {
	ID       int64
	TenantID int64
}

// Repository defines OTP persistence operations used by the service layer.
type Repository interface {
	Create(context.Context, CreateParams) (OTP, error)
	GetLatestUnusedByPhoneNumber(context.Context, GetOTPParams) (OTP, error)
	MarkAsUsed(context.Context, MarkOTPParams) error
}