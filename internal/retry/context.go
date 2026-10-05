package retry

import (
	"context"
	"time"
)

// WithContext returns a new RetryOptions with the given context set.
func WithContext(opts *RetryOptions, ctx context.Context) *RetryOptions {
	if opts == nil {
		opts = DefaultRetryOptions()
	}
	opts.Context = ctx
	return opts
}

// WithMaxRetries returns a new RetryOptions with the given max retries set.
func WithMaxRetries(opts *RetryOptions, maxRetries int) *RetryOptions {
	if opts == nil {
		opts = DefaultRetryOptions()
	}
	opts.MaxRetries = maxRetries
	return opts
}

// WithBackoffStrategy returns a new RetryOptions with the given backoff strategy set.
func WithBackoffStrategy(opts *RetryOptions, strategy BackoffStrategy) *RetryOptions {
	if opts == nil {
		opts = DefaultRetryOptions()
	}
	opts.BackoffStrategy = strategy
	return opts
}

// WithAllowContextCancellation returns a new RetryOptions with the given allow context cancellation flag set.
func WithAllowContextCancellation(opts *RetryOptions, allow bool) *RetryOptions {
	if opts == nil {
		opts = DefaultRetryOptions()
	}
	opts.AllowContextCancellation = allow
	return opts
}

// RetryContext is a helper that runs a function with context-aware retry logic.
// It wraps Retry and ensures the context is properly handled.
func RetryContext(ctx context.Context, opts *RetryOptions, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	if opts == nil {
		opts = DefaultRetryOptions()
	}
	
	if opts.Context != nil {
		ctx = opts.Context
	}
	
	return Retry(ctx, opts, fn)
}

// RetryContextVoid is a helper that runs a void function with context-aware retry logic.
func RetryContextVoid(ctx context.Context, opts *RetryOptions, fn func(context.Context) error) error {
	if opts == nil {
		opts = DefaultRetryOptions()
	}
	
	if opts.Context != nil {
		ctx = opts.Context
	}
	
	return RetryVoid(ctx, opts, fn)
}

// DoWithRetry is a convenience function that runs a function with default retry options.
// It's equivalent to calling RetryContext with DefaultRetryOptions().
func DoWithRetry(ctx context.Context, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	return RetryContext(ctx, DefaultRetryOptions(), fn)
}

// DoWithRetryVoid is a convenience function that runs a void function with default retry options.
// It's equivalent to calling RetryContextVoid with DefaultRetryOptions().
func DoWithRetryVoid(ctx context.Context, fn func(context.Context) error) error {
	return RetryContextVoid(ctx, DefaultRetryOptions(), fn)
}
