package otp

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

// rowQuerier abstracts pgxpool.Pool and pgx.Tx for query execution.
type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Tx defines a database transaction interface.
type Tx interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// Repository defines OTP persistence operations used by the service layer.
type Repository interface {
	Create(context.Context, CreateParams) (OTP, error)
	GetLatestUnusedByPhoneNumber(context.Context, GetOTPParams) (OTP, error)
	MarkAsUsed(context.Context, MarkOTPParams) error

	// AtomicVerify atomically retrieves and locks the newest unused OTP for a
	// phone number. It uses SELECT ... FOR UPDATE SKIP LOCKED to prevent race
	// conditions during concurrent verification requests. The caller is
	// responsible for validating expiration, comparing the code, and marking
	// the OTP as used.
	AtomicVerify(context.Context, AtomicVerifyParams) (OTP, bool, error)

	// BeginTx starts a new transaction.
	BeginTx(context.Context) (Tx, error)

	// GetLatestUnusedForUpdate retrieves and locks the newest unused OTP within
	// a transaction.
	GetLatestUnusedForUpdate(context.Context, Tx, GetOTPParams) (OTP, bool, error)

	// MarkAsUsedInTx marks an OTP as used within a transaction.
	MarkAsUsedInTx(context.Context, Tx, MarkOTPParams) error
}

// AtomicVerifyParams contains the fields required for atomic verify.
type AtomicVerifyParams struct {
	PhoneNumber string
	TenantID    int64
}

// VerifyOTPParams contains the fields required to verify an OTP.
type VerifyOTPParams struct {
	PhoneNumber string
	Code        string
	TenantID    int64
}
