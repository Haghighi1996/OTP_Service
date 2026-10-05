package otp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const createQuery = `
	INSERT INTO otps (phone_number, otp_hash, expires_at, tenant_id)
	VALUES ($1, $2, $3, $4)
	RETURNING id, phone_number, otp_hash, expires_at, used_at, created_at`

const getLatestUnusedByPhoneNumberQuery = `
	SELECT id, phone_number, otp_hash, expires_at, used_at, created_at
	FROM otps
	WHERE phone_number = $1 AND tenant_id = $2 AND used_at IS NULL
	ORDER BY created_at DESC, id DESC
	LIMIT 1`

const markAsUsedQuery = `
	UPDATE otps
	SET used_at = NOW()
	WHERE id = $1 AND tenant_id = $2 AND used_at IS NULL`

const atomicVerifyQuery = `
	SELECT id, phone_number, otp_hash, expires_at, used_at, created_at, tenant_id
	FROM otps
	WHERE phone_number = $1 AND tenant_id = $2 AND used_at IS NULL
	ORDER BY created_at DESC, id DESC
	FOR UPDATE SKIP LOCKED
	LIMIT 1`

const getLatestUnusedForUpdateQuery = `
	SELECT id, phone_number, otp_hash, expires_at, used_at, created_at, tenant_id
	FROM otps
	WHERE phone_number = $1 AND tenant_id = $2 AND used_at IS NULL
	ORDER BY created_at DESC, id DESC
	FOR UPDATE SKIP LOCKED
	LIMIT 1`

const markAsUsedInTxQuery = `
	UPDATE otps
	SET used_at = NOW()
	WHERE id = $1 AND tenant_id = $2 AND used_at IS NULL`

// PostgresRepository stores OTP records in PostgreSQL.
type PostgresRepository struct {
	queries rowQuerier
	pool    *pgxpool.Pool
}

// NewPostgresRepository creates an OTP repository backed by a pgx pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return newPostgresRepository(pool, pool)
}

func newPostgresRepository(queries rowQuerier, pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{queries: queries, pool: pool}
}

// Create persists an OTP and returns the database-generated record.
func (r *PostgresRepository) Create(ctx context.Context, params CreateParams) (OTP, error) {
	if err := validateCreateParams(params); err != nil {
		return OTP{}, err
	}

	var record OTP
	err := r.queries.QueryRow(ctx, createQuery, params.PhoneNumber, params.CodeHash, params.ExpiresAt, params.TenantID).Scan(
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

// GetLatestUnusedByPhoneNumber returns the newest unused OTP for a phone number.
func (r *PostgresRepository) GetLatestUnusedByPhoneNumber(ctx context.Context, params GetOTPParams) (OTP, error) {
	phoneNumber := strings.TrimSpace(params.PhoneNumber)
	if phoneNumber == "" {
		return OTP{}, fmt.Errorf("phone number is required")
	}

	var record OTP
	err := r.queries.QueryRow(ctx, getLatestUnusedByPhoneNumberQuery, phoneNumber, params.TenantID).Scan(
		&record.ID,
		&record.PhoneNumber,
		&record.CodeHash,
		&record.ExpiresAt,
		&record.UsedAt,
		&record.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return OTP{}, ErrOTPNotFound
	}
	if err != nil {
		return OTP{}, fmt.Errorf("get latest unused OTP: %w", err)
	}

	return record, nil
}

// MarkAsUsed marks the OTP with the given ID as used. Returns ErrOTPNotFound
// if the OTP was already used or does not exist.
func (r *PostgresRepository) MarkAsUsed(ctx context.Context, params MarkOTPParams) error {
	tag, err := r.queries.Exec(ctx, markAsUsedQuery, params.ID, params.TenantID)
	if err != nil {
		return fmt.Errorf("mark OTP as used: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrOTPNotFound
	}
	return nil
}

// BeginTx starts a new transaction.
func (r *PostgresRepository) BeginTx(ctx context.Context) (Tx, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	return tx, nil
}

// GetLatestUnusedForUpdate retrieves and locks the newest unused OTP within
// a transaction. Uses SELECT ... FOR UPDATE SKIP LOCKED.
func (r *PostgresRepository) GetLatestUnusedForUpdate(ctx context.Context, tx Tx, params GetOTPParams) (OTP, bool, error) {
	phoneNumber := strings.TrimSpace(params.PhoneNumber)
	if phoneNumber == "" {
		return OTP{}, false, fmt.Errorf("phone number is required")
	}
	if params.TenantID == 0 {
		return OTP{}, false, fmt.Errorf("tenant ID is required")
	}

	var record OTP
	err := tx.QueryRow(ctx, getLatestUnusedForUpdateQuery, phoneNumber, params.TenantID).Scan(
		&record.ID,
		&record.PhoneNumber,
		&record.CodeHash,
		&record.ExpiresAt,
		&record.UsedAt,
		&record.CreatedAt,
		&record.TenantID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return OTP{}, false, ErrOTPNotFound
	}
	if err != nil {
		return OTP{}, false, fmt.Errorf("get latest unused for update: %w", err)
	}

	return record, true, nil
}

// MarkAsUsedInTx marks an OTP as used within a transaction.
func (r *PostgresRepository) MarkAsUsedInTx(ctx context.Context, tx Tx, params MarkOTPParams) error {
	tag, err := tx.Exec(ctx, markAsUsedInTxQuery, params.ID, params.TenantID)
	if err != nil {
		return fmt.Errorf("mark OTP as used in tx: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrOTPNotFound
	}
	return nil
}

// AtomicVerify atomically retrieves the newest unused OTP for a phone number
// with row-level locking. It uses SELECT ... FOR UPDATE SKIP LOCKED to prevent
// race conditions during concurrent verification requests.
func (r *PostgresRepository) AtomicVerify(ctx context.Context, params AtomicVerifyParams) (OTP, bool, error) {
	phoneNumber := strings.TrimSpace(params.PhoneNumber)
	if phoneNumber == "" {
		return OTP{}, false, fmt.Errorf("phone number is required")
	}
	if params.TenantID == 0 {
		return OTP{}, false, fmt.Errorf("tenant ID is required")
	}

	var record OTP
	err := r.queries.QueryRow(ctx, atomicVerifyQuery, phoneNumber, params.TenantID).Scan(
		&record.ID,
		&record.PhoneNumber,
		&record.CodeHash,
		&record.ExpiresAt,
		&record.UsedAt,
		&record.CreatedAt,
		&record.TenantID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return OTP{}, false, ErrOTPNotFound
	}
	if err != nil {
		return OTP{}, false, fmt.Errorf("atomic verify: %w", err)
	}

	return record, true, nil
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
	if params.TenantID == 0 {
		return fmt.Errorf("tenant ID is required")
	}
	return nil
}
