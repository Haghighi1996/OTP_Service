package otp

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresRepositoryCreateValidation(t *testing.T) {
	repository := newPostgresRepository(nil)

	tests := []struct {
		name   string
		params CreateParams
		want   string
	}{
		{
			name:   "missing phone number",
			params: CreateParams{CodeHash: "hash", ExpiresAt: time.Now().Add(time.Minute)},
			want:   "phone number is required",
		},
		{
			name:   "missing hash",
			params: CreateParams{PhoneNumber: "+12025550123", ExpiresAt: time.Now().Add(time.Minute)},
			want:   "OTP hash is required",
		},
		{
			name:   "expired OTP",
			params: CreateParams{PhoneNumber: "+12025550123", CodeHash: "hash", ExpiresAt: time.Now().Add(-time.Minute)},
			want:   "OTP expiration must be in the future",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := repository.Create(context.Background(), test.params)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Create() error = %v, want error containing %q", err, test.want)
			}
		})
	}
}

func TestPostgresRepositoryCreate(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	repository := newPostgresRepository(tx)
	expiresAt := time.Now().UTC().Add(5 * time.Minute).Truncate(time.Microsecond)
	record, err := repository.Create(ctx, CreateParams{
		PhoneNumber: "+12025550123",
		CodeHash:    "stored-hash-only",
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if record.ID == 0 {
		t.Error("Create() returned no ID")
	}
	if record.PhoneNumber != "+12025550123" || record.CodeHash != "stored-hash-only" {
		errorf := "Create() returned unexpected record: %+v"
		t.Errorf(errorf, record)
	}
	if !record.ExpiresAt.Equal(expiresAt) {
		t.Errorf("ExpiresAt = %v, want %v", record.ExpiresAt, expiresAt)
	}
	if record.UsedAt != nil {
		t.Errorf("UsedAt = %v, want nil", record.UsedAt)
	}
	if record.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
}

func TestPostgresRepositoryGetLatestUnusedByPhoneNumberValidation(t *testing.T) {
	repository := newPostgresRepository(nil)

	_, err := repository.GetLatestUnusedByPhoneNumber(context.Background(), "  ")
	if err == nil || !strings.Contains(err.Error(), "phone number is required") {
		t.Fatalf("GetLatestUnusedByPhoneNumber() error = %v, want missing phone number", err)
	}
}

func TestPostgresRepositoryGetLatestUnusedByPhoneNumber(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	repository := newPostgresRepository(tx)
	phoneNumber := "+12025550999"

	_, err = repository.GetLatestUnusedByPhoneNumber(ctx, phoneNumber)
	if !errors.Is(err, ErrOTPNotFound) {
		t.Fatalf("GetLatestUnusedByPhoneNumber() error = %v, want ErrOTPNotFound", err)
	}

	older, err := repository.Create(ctx, CreateParams{
		PhoneNumber: phoneNumber,
		CodeHash:    "older-hash",
		ExpiresAt:   time.Now().UTC().Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Create() older OTP error = %v", err)
	}
	newer, err := repository.Create(ctx, CreateParams{
		PhoneNumber: phoneNumber,
		CodeHash:    "newer-hash",
		ExpiresAt:   time.Now().UTC().Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Create() newer OTP error = %v", err)
	}
	if older.ID == newer.ID {
		t.Fatal("expected distinct OTP records")
	}

	record, err := repository.GetLatestUnusedByPhoneNumber(ctx, phoneNumber)
	if err != nil {
		t.Fatalf("GetLatestUnusedByPhoneNumber() error = %v", err)
	}
	if record.ID != newer.ID || record.CodeHash != "newer-hash" {
		t.Errorf("record = %+v, want newest unused OTP", record)
	}

	_ = repository.MarkAsUsed(ctx, newer.ID)
	record, err = repository.GetLatestUnusedByPhoneNumber(ctx, phoneNumber)
	if err != nil {
		t.Fatalf("GetLatestUnusedByPhoneNumber() after mark: %v", err)
	}
	if record.ID != older.ID || record.CodeHash != "older-hash" {
		t.Errorf("record = %+v, want older unused OTP after newer consumed", record)
	}
}

func TestPostgresRepositoryMarkAsUsed(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	repository := newPostgresRepository(tx)

	t.Run("returns ErrOTPNotFound for nonexistent OTP", func(t *testing.T) {
		err := repository.MarkAsUsed(ctx, 999999)
		if !errors.Is(err, ErrOTPNotFound) {
			t.Fatalf("MarkAsUsed() error = %v, want ErrOTPNotFound", err)
		}
	})

	record, err := repository.Create(ctx, CreateParams{
		PhoneNumber: "+12025550999",
		CodeHash:    "stored-hash",
		ExpiresAt:   time.Now().UTC().Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	t.Run("marks OTP as used", func(t *testing.T) {
		if err := repository.MarkAsUsed(ctx, record.ID); err != nil {
			t.Fatalf("MarkAsUsed() error = %v", err)
		}

		var usedAt *time.Time
		err := tx.QueryRow(ctx, "SELECT used_at FROM otps WHERE id = $1", record.ID).Scan(&usedAt)
		if err != nil {
			t.Fatalf("query used_at: %v", err)
		}
		if usedAt == nil {
			t.Error("used_at was not set")
		}
	})

	t.Run("returns ErrOTPNotFound when already used", func(t *testing.T) {
		err := repository.MarkAsUsed(ctx, record.ID)
		if !errors.Is(err, ErrOTPNotFound) {
			t.Fatalf("MarkAsUsed() error = %v, want ErrOTPNotFound", err)
		}
	})
}
