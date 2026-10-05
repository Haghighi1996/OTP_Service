package retry

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// RetryOptions configures retry behavior.
type RetryOptions struct {
	MaxRetries             int
	BackoffStrategy        BackoffStrategy
	RetryableErrors        []error
	NonRetryableErrors     []error
	AllowContextCancellation bool
	Context                context.Context
}

// DefaultRetryOptions returns sensible defaults for retry configuration.
func DefaultRetryOptions() *RetryOptions {
	return &RetryOptions{
		MaxRetries:             3,
		BackoffStrategy:        DefaultExponentialBackoff(),
		AllowContextCancellation: true,
	}
}

// RetryFunc is a function that performs an operation and may return an error.
type RetryFunc func(ctx context.Context) (interface{}, error)

// Retry executes a function with retry logic.
func Retry(ctx context.Context, opts *RetryOptions, fn RetryFunc) (interface{}, error) {
	if opts == nil {
		opts = DefaultRetryOptions()
	}

	if opts.BackoffStrategy == nil {
		opts.BackoffStrategy = DefaultExponentialBackoff()
	}

	retryCtx := ctx
	if opts.Context != nil {
		retryCtx = opts.Context
	}

	var lastErr error

	for attempt := 0; ; attempt++ {
		result, err := fn(retryCtx)
		if err == nil {
			return result, nil
		}

		if opts.MaxRetries >= 0 && attempt >= opts.MaxRetries {
			return result, fmt.Errorf("%w: %v", ErrMaxRetries, lastErr)
		}

		var nr NonRetryableError
		if errors.As(err, &nr) && nr.IsNonRetryable() {
			return result, err
		}

		if opts.AllowContextCancellation {
			select {
			case <-retryCtx.Done():
				return result, fmt.Errorf("%w: %v", ctx.Err(), lastErr)
			default:
			}
		}

		backoff := opts.BackoffStrategy.Next(attempt, 0)

		select {
		case <-time.After(backoff):
		case <-retryCtx.Done():
			return result, fmt.Errorf("%w: %v", ctx.Err(), lastErr)
		}

		lastErr = err
	}
}

// RetryVoid executes a void function (no return value) with retry logic.
func RetryVoid(ctx context.Context, opts *RetryOptions, fn func(ctx context.Context) error) error {
	if opts == nil {
		opts = DefaultRetryOptions()
	}

	var lastErr error

	for attempt := 0; ; attempt++ {
		err := fn(ctx)
		if err == nil {
			return nil
		}

		if opts.MaxRetries >= 0 && attempt >= opts.MaxRetries {
			return fmt.Errorf("%w: %v", ErrMaxRetries, lastErr)
		}

		var nr NonRetryableError
		if errors.As(err, &nr) && nr.IsNonRetryable() {
			return err
		}

		if opts.AllowContextCancellation {
			select {
			case <-ctx.Done():
				return fmt.Errorf("%w: %v", ctx.Err(), lastErr)
			default:
			}
		}

		backoff := opts.BackoffStrategy.Next(attempt, 0)

		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return fmt.Errorf("%w: %v", ctx.Err(), lastErr)
		}

		lastErr = err
	}
}

// Stats records retry statistics.
type Stats struct {
	Attempts            int
	Successful          bool
	TotalRetries        int
	TotalBackoff        time.Duration
	LastError           error
	ErrorTypes          map[error]int
	ContextCancels      int
	MaxRetriesExceeded  bool
}

// NewStats creates a new Stats instance.
func NewStats() *Stats {
	return &Stats{
		ErrorTypes: make(map[error]int),
	}
}

// Record records a retry attempt result.
func (s *Stats) Record(result interface{}, err error, isSuccess bool) {
	s.Attempts++
	if isSuccess {
		s.Successful = true
	} else {
		s.TotalRetries++
		s.LastError = err
		if err != nil {
			s.ErrorTypes[err]++
		}
	}
}

// Reset resets all statistics.
func (s *Stats) Reset() {
	s.Attempts = 0
	s.Successful = false
	s.TotalRetries = 0
	s.TotalBackoff = 0
	s.LastError = nil
	s.ErrorTypes = make(map[error]int)
	s.ContextCancels = 0
	s.MaxRetriesExceeded = false
}

// Merge merges another Stats instance into this one.
func (s *Stats) Merge(other *Stats) {
	s.Attempts += other.Attempts
	s.Successful = s.Successful || other.Successful
	s.TotalRetries += other.TotalRetries
	s.TotalBackoff += other.TotalBackoff
	if s.LastError == nil && other.LastError != nil {
		s.LastError = other.LastError
	}
	for err, count := range other.ErrorTypes {
		s.ErrorTypes[err] += count
	}
	s.ContextCancels += other.ContextCancels
	s.MaxRetriesExceeded = s.MaxRetriesExceeded || other.MaxRetriesExceeded
}
