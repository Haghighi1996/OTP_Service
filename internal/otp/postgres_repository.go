package otp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const createQuery = `
	INSERT INTO otps (phone_number, otp_hash, expires_at)
	VALUES ($1, $2, $3)
	RETURNING id, phone_number, otp_hash, expires_at, used_at, created_at`

type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// PostgresRepository stores OTP records in PostgreSQL.
type PostgresRepository struct {
	queries rowQuerier
}

// NewPostgresRepository creates an OTP repository backed by a pgx pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return newPostgresRepository(pool)
}

func newPostgresRepository(queries rowQuerier) *PostgresRepository {
	return &PostgresRepository{queries: queries}
}

// Create persists an OTP and returns the database-generated record.
func (r *PostgresRepository) Create(ctx context.Context, params CreateParams) (OTP, error) {
	if err := validateCreateParams(params); err != nil {
		return OTP{}, err
	}

	var record OTP
	err := r.queries.QueryRow(ctx, createQuery, params.PhoneNumber, params.CodeHash, params.ExpiresAt).Scan(
		&record.ID,
		&record.PhoneNumber,
		&record.CodeHash,
		&record.ExpiresAt,
		&record.UsedAt,
		&record.CreatedAt,
	)
	if err != nil {
		return OTP{}, fmt.Errorf("create OTP: %w", err)
	}

	return record, nil
}

func validateCreateParams(params CreateParams) error {
	if strings.TrimSpace(params.PhoneNumber) == "" {
		return fmt.Errorf("phone number is required")
	}
	if strings.TrimSpace(params.CodeHash) == "" {
		return fmt.Errorf("OTP hash is required")
	}
	if params.ExpiresAt.IsZero() {
		return fmt.Errorf("OTP expiration is required")
	}
	if !params.ExpiresAt.After(time.Now()) {
		return fmt.Errorf("OTP expiration must be in the future")
	}
	return nil
}
