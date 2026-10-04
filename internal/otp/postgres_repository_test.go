package otp

import (
	"context"
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
