package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPAddr         = ":8080"
	defaultDBMaxConns int32 = 10
	defaultDBMinConns int32 = 2

	defaultDBMaxConnLifetime = time.Hour
	defaultDBMaxConnIdleTime = 30 * time.Minute
	defaultDBHealthCheck     = time.Minute
)

// Config contains the runtime settings required by the HTTP server and
// PostgreSQL connection pool.
type Config struct {
	HTTPAddr string

	DatabaseURL       string
	RedisURL          string
	DBMaxConns        int32
	DBMinConns        int32
	DBMaxConnLifetime time.Duration
	DBMaxConnIdleTime time.Duration
	DBHealthCheck     time.Duration
}

// Load reads and validates process environment configuration.
func Load() (Config, error) {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	redisURL := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if redisURL == "" {
		return Config{}, fmt.Errorf("REDIS_URL is required")
	}

	maxConns, err := positiveInt32FromEnv("DB_MAX_CONNS", defaultDBMaxConns)
	if err != nil {
		return Config{}, err
	}
	minConns, err := int32FromEnv("DB_MIN_CONNS", defaultDBMinConns)
	if err != nil {
		return Config{}, err
	}
	if minConns > maxConns {
		return Config{}, fmt.Errorf("DB_MIN_CONNS cannot be greater than DB_MAX_CONNS")
	}

	maxConnLifetime, err := durationFromEnv("DB_MAX_CONN_LIFETIME", defaultDBMaxConnLifetime)
	if err != nil {
		return Config{}, err
	}
	maxConnIdleTime, err := durationFromEnv("DB_MAX_CONN_IDLE_TIME", defaultDBMaxConnIdleTime)
	if err != nil {
		return Config{}, err
	}
	healthCheck, err := durationFromEnv("DB_HEALTH_CHECK_PERIOD", defaultDBHealthCheck)
	if err != nil {
		return Config{}, err
	}

	httpAddr := strings.TrimSpace(os.Getenv("HTTP_ADDR"))
	if httpAddr == "" {
		httpAddr = defaultHTTPAddr
	}

	return Config{
		HTTPAddr:     httpAddr,
		DatabaseURL:  databaseURL,
		RedisURL:     redisURL,
		DBMaxConns:   maxConns,
		DBMinConns:   minConns,
		DBMaxConnLifetime: maxConnLifetime,
		DBMaxConnIdleTime: maxConnIdleTime,
		DBHealthCheck:  healthCheck,
	}, nil
}

func int32FromEnv(name string, fallback int32) (int32, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", name)
	}

	return int32(parsed), nil
}

func positiveInt32FromEnv(name string, fallback int32) (int32, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}

	return int32(parsed), nil
}

func durationFromEnv(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}

	return parsed, nil
}
