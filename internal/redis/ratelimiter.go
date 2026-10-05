package redis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"otp-service/internal/auth"
)

// ErrRateLimitExceeded is returned when a rate limit is exceeded.
var ErrRateLimitExceeded = fmt.Errorf("rate limit exceeded")

// ErrRedisUnavailable is returned when Redis is unavailable.
var ErrRedisUnavailable = fmt.Errorf("redis unavailable")

// ErrInvalidAPIKey is returned when the API key is missing or invalid.
var ErrInvalidAPIKey = fmt.Errorf("invalid or missing API key")

const (
	// defaultWindowSeconds is the sliding window duration in seconds.
	defaultWindowSeconds = 60
	// defaultMaxRequests is the maximum number of requests allowed in a window.
	defaultMaxRequests = 20
)

// LimiterConfig holds the configuration for rate limiter limits.
type LimiterConfig struct {
	// MaxRequests is the maximum number of requests allowed within the window.
	MaxRequests int
	// WindowSeconds is the duration of the rate-limiting window in seconds.
	WindowSeconds int
}

// DefaultConfig returns the default rate limiter configuration.
func DefaultConfig() LimiterConfig {
	return LimiterConfig{
		MaxRequests:   defaultMaxRequests,
		WindowSeconds: defaultWindowSeconds,
	}
}

// Limiter is a Redis-backed rate limiter using the fixed window algorithm.
// It supports tenant-aware, API-key-aware, and phone-number-aware limits.
type Limiter struct {
	client *Client
	config LimiterConfig
}

// NewLimiter creates a new Redis-backed rate limiter.
func NewLimiter(client *Client, config LimiterConfig) *Limiter {
	return &Limiter{
		client: client,
		config: config,
	}
}

// keyForPhone generates a rate limit key for a phone number.
func (rl *Limiter) keyForPhone(phoneNumber string) string {
	return fmt.Sprintf("ratelimit:phone:%s", phoneNumber)
}

// keyForTenant generates a rate limit key for a tenant.
func (rl *Limiter) keyForTenant(tenantID int64) string {
	return fmt.Sprintf("ratelimit:tenant:%d", tenantID)
}

// keyForAPIKey generates a rate limit key for an API key.
func (rl *Limiter) keyForAPIKey(apiKey string) string {
	return fmt.Sprintf("ratelimit:apikey:%s", apiKey)
}

// CheckPhone determines whether the given phone number can make a request.
// It atomically increments the counter and rejects requests that exceed the limit.
func (rl *Limiter) CheckPhone(ctx context.Context, phoneNumber string) (int64, error) {
	key := rl.keyForPhone(phoneNumber)
	count, err := rl.client.Incr(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("increment phone rate limit: %w", err)
	}
	if count == 1 {
		_, _ = rl.client.Expire(ctx, key, time.Duration(rl.config.WindowSeconds)*time.Second)
	}
	if count > int64(rl.config.MaxRequests) {
		return count, ErrRateLimitExceeded
	}
	return count, nil
}

// CheckTenant determines whether the given tenant can make a request.
// It atomically increments the counter and rejects requests that exceed the limit.
func (rl *Limiter) CheckTenant(ctx context.Context, tenantID int64) (int64, error) {
	key := rl.keyForTenant(tenantID)
	count, err := rl.client.Incr(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("increment tenant rate limit: %w", err)
	}
	if count == 1 {
		_, _ = rl.client.Expire(ctx, key, time.Duration(rl.config.WindowSeconds)*time.Second)
	}
	if count > int64(rl.config.MaxRequests) {
		return count, ErrRateLimitExceeded
	}
	return count, nil
}

// CheckAPIKey determines whether the given API key can make a request.
// It atomically increments the counter and rejects requests that exceed the limit.
func (rl *Limiter) CheckAPIKey(ctx context.Context, apiKey string) (int64, error) {
	key := rl.keyForAPIKey(apiKey)
	count, err := rl.client.Incr(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("increment api key rate limit: %w", err)
	}
	if count == 1 {
		_, _ = rl.client.Expire(ctx, key, time.Duration(rl.config.WindowSeconds)*time.Second)
	}
	if count > int64(rl.config.MaxRequests) {
		return count, ErrRateLimitExceeded
	}
	return count, nil
}

// CheckCombined applies both a phone-number limit and a tenant-wide limit.
// This provides defense in depth: a single tenant cannot exhaust another tenant's quota.
func (rl *Limiter) CheckCombined(ctx context.Context, tenantID int64, phoneNumber string) (bool, int64, error) {
	tenantCount, err := rl.CheckTenant(ctx, tenantID)
	if err != nil {
		return false, 0, err
	}
	phoneCount, err := rl.CheckPhone(ctx, phoneNumber)
	if err != nil {
		return false, 0, err
	}
	return tenantCount <= int64(rl.config.MaxRequests) && phoneCount <= int64(rl.config.MaxRequests), phoneCount, nil
}

// Middleware is HTTP middleware that applies rate limiting using phone number and tenant dimensions.
// It must be placed after the auth middleware so the tenant ID is available in the request context.
func (rl *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := auth.GetTenantID(r.Context())
		if !ok {
			http.Error(w, ErrInvalidAPIKey.Error(), http.StatusUnauthorized)
			return
		}

		phoneNumber, err := extractPhoneNumber(r)
		if err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		allowed, _, err := rl.CheckCombined(r.Context(), tenantID, phoneNumber)
		if err != nil {
			http.Error(w, "rate limiting unavailable", http.StatusServiceUnavailable)
			return
		}
		if !allowed {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// extractPhoneNumber reads the request body, extracts the phone_number field,
// and resets the body so the handler can read it again.
func extractPhoneNumber(r *http.Request) (string, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()

	var req struct {
		PhoneNumber string `json:"phone_number"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return "", err
	}

	r.Body = io.NopCloser(bytes.NewReader(body))
	return req.PhoneNumber, nil
}