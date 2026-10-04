package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"otp-service/internal/config"
)

// NewPool creates a configured PostgreSQL connection pool and verifies that it
// can reach the database before returning it.
func NewPool(ctx context.Context, settings config.Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(settings.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}

	poolConfig.MaxConns = settings.DBMaxConns
	poolConfig.MinConns = settings.DBMinConns
	poolConfig.MaxConnLifetime = settings.DBMaxConnLifetime
	poolConfig.MaxConnIdleTime = settings.DBMaxConnIdleTime
	poolConfig.HealthCheckPeriod = settings.DBHealthCheck

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}

	return pool, nil
}
