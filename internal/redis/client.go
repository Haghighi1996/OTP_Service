package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"otp-service/internal/config"
)

// Client wraps redis.Client with context-aware methods.
type Client struct {
	client *redis.Client
}

// NewClient creates a Redis client from configuration.
func NewClient(settings config.Config) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     settings.RedisURL,
		DB:       0,
		PoolSize: 10,
	})

	return &Client{client: rdb}
}

// Close closes the Redis client.
func (c *Client) Close() error {
	return c.client.Close()
}

// Ping checks the Redis connection.
func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Get retrieves a value by key.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

// Set sets a key-value pair with expiration.
func (c *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return c.client.Set(ctx, key, value, expiration).Err()
}

// Incr increments a key and returns the new value.
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	return c.client.Incr(ctx, key).Result()
}

// Expire sets expiration on a key.
func (c *Client) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return c.client.Expire(ctx, key, expiration).Result()
}

// Del deletes one or more keys.
func (c *Client) Del(ctx context.Context, keys ...string) (int64, error) {
	return c.client.Del(ctx, keys...).Result()
}

// Eval executes a Lua script.
func (c *Client) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	return c.client.Eval(ctx, script, keys, args...)
}
