package retry

import (
	"math/rand"
	"time"
)

// BackoffStrategy defines how to calculate backoff durations between retry attempts.
type BackoffStrategy interface {
	// Next returns the duration to wait before the next retry attempt.
	// attempt is the 0-based retry attempt number (0 = first retry, 1 = second retry, etc.)
	// maxBackoff is the maximum allowed backoff duration.
	Next(attempt int, maxBackoff time.Duration) time.Duration
}

// ExponentialBackoff implements exponential backoff with optional jitter.
// Formula: base * 2^attempt with optional jitter.
type ExponentialBackoff struct {
	Base       time.Duration
	Jitter     time.Duration // Max jitter to add (0 = no jitter)
	MaxBackoff time.Duration // Maximum allowed backoff duration
}

// Next calculates the backoff duration for the given attempt.
func (e *ExponentialBackoff) Next(attempt int, maxBackoff time.Duration) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	// Calculate exponential backoff: base * 2^attempt
	backoff := e.Base
	for i := 0; i < attempt; i++ {
		backoff *= 2
	}

	// Add jitter if configured
	if e.Jitter > 0 {
		jitter := time.Duration(rand.Int63n(int64(e.Jitter)))
		backoff += jitter
	}

	// Cap at maxBackoff
	if maxBackoff > 0 && backoff > maxBackoff {
		backoff = maxBackoff
	}

	// Cap at ExponentialBackoff.MaxBackoff if set
	if e.MaxBackoff > 0 && backoff > e.MaxBackoff {
		backoff = e.MaxBackoff
	}

	return backoff
}

// ConstantBackoff uses a constant backoff duration between retries.
type ConstantBackoff struct {
	Duration time.Duration
}

// Next returns the constant backoff duration.
func (c *ConstantBackoff) Next(attempt int, maxBackoff time.Duration) time.Duration {
	if maxBackoff > 0 && c.Duration > maxBackoff {
		return maxBackoff
	}
	return c.Duration
}

// LinearBackoff increases backoff linearly with each attempt.
// Formula: base * attempt
type LinearBackoff struct {
	Base       time.Duration
	MaxBackoff time.Duration
}

// Next calculates the linear backoff duration for the given attempt.
func (l *LinearBackoff) Next(attempt int, maxBackoff time.Duration) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	backoff := l.Base * time.Duration(attempt+1)

	if maxBackoff > 0 && backoff > maxBackoff {
		backoff = maxBackoff
	}
	if l.MaxBackoff > 0 && backoff > l.MaxBackoff {
		backoff = l.MaxBackoff
	}

	return backoff
}

// DefaultExponentialBackoff returns a default exponential backoff strategy.
func DefaultExponentialBackoff() *ExponentialBackoff {
	return &ExponentialBackoff{
		Base:       100 * time.Millisecond,
		Jitter:     50 * time.Millisecond,
		MaxBackoff: 30 * time.Second,
	}
}

// DefaultConstantBackoff returns a default constant backoff strategy.
func DefaultConstantBackoff() *ConstantBackoff {
	return &ConstantBackoff{Duration: 1 * time.Second}
}

// DefaultLinearBackoff returns a default linear backoff strategy.
func DefaultLinearBackoff() *LinearBackoff {
	return &LinearBackoff{
		Base:       100 * time.Millisecond,
		MaxBackoff: 30 * time.Second,
	}
}
