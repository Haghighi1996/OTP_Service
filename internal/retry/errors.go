package retry

import (
	"errors"
	"fmt"
)

// ErrMaxRetries is returned when the maximum number of retries has been exhausted.
var ErrMaxRetries = errors.New("maximum retries exceeded")

// ErrBackoffExceeded is returned when the backoff duration exceeds the maximum allowed.
type ErrBackoffExceeded struct {
	MaxBackoff  any
	ActualBackoff any
}

func (e *ErrBackoffExceeded) Error() string {
	return fmt.Sprintf("backoff %v exceeded max %v", e.ActualBackoff, e.MaxBackoff)
}

// ErrContextCanceled is returned when the context is canceled during a retry attempt.
var ErrContextCanceled = errors.New("retry context canceled")

// ErrContextDeadlineExceeded is returned when the context deadline is exceeded during a retry attempt.
var ErrContextDeadlineExceeded = errors.New("retry context deadline exceeded")

// IsRetryableError determines if an error should trigger a retry.
// By default, all errors are retryable unless they implement the NonRetryableError interface.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	var nr NonRetryableError
	if errors.As(err, &nr) {
		return false
	}
	return true
}

// NonRetryableError is an interface for errors that should not be retried.
type NonRetryableError interface {
	error
	IsNonRetryable() bool
}

// NonRetryableErrorImpl is a concrete implementation of NonRetryableError.
type NonRetryableErrorImpl struct {
	msg string
}

// Error returns the error message.
func (e *NonRetryableErrorImpl) Error() string {
	return e.msg
}

// IsNonRetryable returns true.
func (e *NonRetryableErrorImpl) IsNonRetryable() bool {
	return true
}

// NewNonRetryableError creates a new non-retryable error.
func NewNonRetryableError(msg string) *NonRetryableErrorImpl {
	return &NonRetryableErrorImpl{msg: msg}
}

// NewNonRetryableErrorf creates a new non-retryable error with formatted message.
func NewNonRetryableErrorf(format string, args ...interface{}) *NonRetryableErrorImpl {
	return &NonRetryableErrorImpl{msg: fmt.Sprintf(format, args...)}
}