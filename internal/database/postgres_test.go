package database

import (
	"context"
	"testing"
	"time"

	"otp-service/internal/config"
)

func TestNewPoolRejectsInvalidDatabaseURL(t *testing.T) {
	settings := config.Config{DatabaseURL: "://invalid"}

	pool, err := NewPool(context.Background(), settings)
	if err == nil {
		pool.Close()
		t.Fatal("NewPool() error = nil, want an error")
	}
}

func TestNewPoolHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	settings := config.Config{
		DatabaseURL:       "postgres://user:password@localhost:5432/otp_db?sslmode=disable",
		DBMaxConns:        1,
		DBMinConns:        0,
		DBMaxConnLifetime: time.Hour,
		DBMaxConnIdleTime: time.Minute,
		DBHealthCheck:     time.Minute,
	}

	pool, err := NewPool(ctx, settings)
	if err == nil {
		pool.Close()
		t.Fatal("NewPool() error = nil, want an error for cancelled context")
	}
}
