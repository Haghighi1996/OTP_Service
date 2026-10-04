package redis

import (
	"context"
	"fmt"
)

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
		// First request: set expiration so the counter resets after the window.
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
	// Apply the stricter tenant-wide limit first, then the phone-number limit.
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