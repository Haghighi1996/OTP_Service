package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/otp_db?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("DB_MAX_CONNS", "")
	t.Setenv("DB_MIN_CONNS", "")
	t.Setenv("DB_MAX_CONN_LIFETIME", "")
	t.Setenv("DB_MAX_CONN_IDLE_TIME", "")
	t.Setenv("DB_HEALTH_CHECK_PERIOD", "")

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if config.HTTPAddr != defaultHTTPAddr {
		t.Errorf("HTTPAddr = %q, want %q", config.HTTPAddr, defaultHTTPAddr)
	}
	if config.DBMaxConns != defaultDBMaxConns || config.DBMinConns != defaultDBMinConns {
		t.Errorf("pool sizes = %d/%d, want %d/%d", config.DBMinConns, config.DBMaxConns, defaultDBMinConns, defaultDBMaxConns)
	}
	if config.DBMaxConnLifetime != time.Hour {
		t.Errorf("DBMaxConnLifetime = %v, want %v", config.DBMaxConnLifetime, time.Hour)
	}
	if config.RedisURL != "redis://localhost:6379" {
		t.Errorf("RedisURL = %q, want %q", config.RedisURL, "redis://localhost:6379")
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name string
		set    func(t *testing.T)
		want   string
	}{
		{
			name: "missing database URL",
			set: func(t *testing.T) {
				t.Setenv("DATABASE_URL", "")
			},
			want: "DATABASE_URL is required",
		},
		{
			name: "missing redis URL",
			set: func(t *testing.T) {
				t.Setenv("REDIS_URL", "")
			},
			want: "REDIS_URL is required",
		},
		{
			name: "minimum connections above maximum",
			set: func(t *testing.T) {
				t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/otp_db?sslmode=disable")
				t.Setenv("DB_MIN_CONNS", "11")
			},
			want: "DB_MIN_CONNS cannot be greater",
		},
		{
			name: "non-positive maximum connections",
			set: func(t *testing.T) {
				t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/otp_db?sslmode=disable")
				t.Setenv("DB_MAX_CONNS", "0")
			},
			want: "DB_MAX_CONNS must be a positive integer",
		},
		{
			name: "invalid duration",
			set: func(t *testing.T) {
				t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/otp_db?sslmode=disable")
				t.Setenv("DB_HEALTH_CHECK_PERIOD", "soon")
			},
			want: "DB_HEALTH_CHECK_PERIOD must be a positive duration",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "")
			t.Setenv("REDIS_URL", "")
			t.Setenv("DB_MAX_CONNS", "")
			t.Setenv("DB_MIN_CONNS", "")
			t.Setenv("DB_HEALTH_CHECK_PERIOD", "")
			test.set(t)
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %v, want error containing %q", err, test.want)
			}
		})
	}
}
