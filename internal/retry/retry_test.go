package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetryWithMaxRetries(t *testing.T) {
	opts := DefaultRetryOptions()
	opts.MaxRetries = 2
	opts.BackoffStrategy = &ExponentialBackoff{Base: 10 * time.Millisecond}

	attempt := 0
	fn := func(ctx context.Context) (interface{}, error) {
		attempt++
		return nil, errors.New("failure")
	}

	_, err := Retry(context.Background(), opts, fn)
	assert.Error(t, err)
	assert.Equal(t, 3, attempt) // Should try initial + 2 retries
}

func TestRetrySuccessAfterRetries(t *testing.T) {
	opts := DefaultRetryOptions()
	opts.MaxRetries = 5
	opts.BackoffStrategy = &ExponentialBackoff{Base: 10 * time.Millisecond}

	attempt := 0
	fn := func(ctx context.Context) (interface{}, error) {
		attempt++
		if attempt < 3 {
			return nil, errors.New("failure")
		}
		return "success", nil
	}

	result, err := Retry(context.Background(), opts, fn)
	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, 3, attempt)
}

func TestRetryNonRetryableError(t *testing.T) {
	opts := DefaultRetryOptions()
	opts.MaxRetries = 5

	attempt := 0
	fn := func(ctx context.Context) (interface{}, error) {
		attempt++
		if attempt == 1 {
			return nil, NewNonRetryableError("non-retryable")
		}
		return nil, errors.New("another failure")
	}

	_, err := Retry(context.Background(), opts, fn)
	assert.Error(t, err)
	var nr NonRetryableError
	assert.True(t, errors.As(err, &nr))
	assert.Equal(t, 1, attempt) // Should not retry
}

func TestRetryBackoffStrategies(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tests := []struct {
		name     string
		strategy BackoffStrategy
	}{
		{"Exponential", &ExponentialBackoff{Base: 10 * time.Millisecond}},
		{"Constant", &ConstantBackoff{Duration: 10 * time.Millisecond}},
		{"Linear", &LinearBackoff{Base: 10 * time.Millisecond}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			attempt := 0
			fn := func(ctx context.Context) (interface{}, error) {
				attempt++
				return nil, errors.New("failure")
			}

			opts := DefaultRetryOptions()
			opts.MaxRetries = 1
			opts.BackoffStrategy = test.strategy

			_, err := Retry(ctx, opts, fn)
			assert.Error(t, err)
		})
	}
}

func TestRetryContextCancellation(t *testing.T) {
	opts := DefaultRetryOptions()
	opts.MaxRetries = 10
	opts.AllowContextCancellation = true
	opts.BackoffStrategy = &ExponentialBackoff{Base: 1 * time.Second}

	ctx, cancel := context.WithCancel(context.Background())

	attempt := 0
	fn := func(ctx context.Context) (interface{}, error) {
		attempt++
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		return nil, errors.New("failure")
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := Retry(ctx, opts, fn)
	assert.Error(t, err)
	assert.Equal(t, 1, attempt)
}

func TestRetryVoid(t *testing.T) {
	opts := DefaultRetryOptions()
	opts.MaxRetries = 3
	opts.BackoffStrategy = &ExponentialBackoff{Base: 10 * time.Millisecond}

	attempt := 0
	fn := func(ctx context.Context) error {
		attempt++
		if attempt < 3 {
			return errors.New("failure")
		}
		return nil
	}

	err := RetryVoid(context.Background(), opts, fn)
	assert.NoError(t, err)
	assert.Equal(t, 3, attempt)
}

func TestRetryStatistics(t *testing.T) {
	opts := DefaultRetryOptions()
	opts.MaxRetries = 1
	opts.BackoffStrategy = &ExponentialBackoff{Base: 10 * time.Millisecond}

	stats := NewStats()
	fn := func(ctx context.Context) (interface{}, error) {
		stats.Attempts++
		if stats.Attempts < 2 {
			return nil, errors.New("failure")
		}
		return "success", nil
	}

	result, err := Retry(context.Background(), opts, fn)
	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, 2, stats.Attempts)
}

func TestDefaultRetryOptions(t *testing.T) {
	opts := DefaultRetryOptions()
	assert.NotNil(t, opts)
	assert.Equal(t, 3, opts.MaxRetries)
	assert.NotNil(t, opts.BackoffStrategy)
	assert.True(t, opts.AllowContextCancellation)
}

func TestRetryHelpers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	attempt := 0
	fn := func(ctx context.Context) (interface{}, error) {
		attempt++
		return nil, errors.New("failure")
	}

	opts := DefaultRetryOptions()
	opts.MaxRetries = 1
	opts = WithMaxRetries(opts, 1)
	opts = WithAllowContextCancellation(opts, true)

	_, err := RetryContext(ctx, opts, fn)
	assert.Error(t, err)
	assert.Equal(t, 2, attempt)
}

func TestIsRetryableError(t *testing.T) {
	assert.False(t, IsRetryableError(nil))
	assert.True(t, IsRetryableError(errors.New("some error")))
	assert.False(t, IsRetryableError(NewNonRetryableError("non-retryable")))
}

func TestStatsRecordAndReset(t *testing.T) {
	stats := NewStats()

	stats.Record(nil, errors.New("test"), false)
	assert.Equal(t, 1, stats.Attempts)
	assert.Equal(t, 1, stats.TotalRetries)
	assert.NotNil(t, stats.LastError)

	stats.Record("ok", nil, true)
	assert.True(t, stats.Successful)

	stats.Reset()
	assert.Equal(t, 0, stats.Attempts)
	assert.False(t, stats.Successful)
	assert.Nil(t, stats.LastError)
}

func TestStatsMerge(t *testing.T) {
	stats1 := NewStats()
	stats1.Attempts = 5
	stats1.TotalRetries = 3
	stats1.Successful = true
	stats1.LastError = errors.New("err1")
	stats1.ErrorTypes = map[error]int{stats1.LastError: 3}

	stats2 := NewStats()
	stats2.Attempts = 3
	stats2.TotalRetries = 2
	stats2.LastError = errors.New("err2")
	stats2.ErrorTypes = map[error]int{stats2.LastError: 2}

	stats1.Merge(stats2)

	assert.Equal(t, 8, stats1.Attempts)
	assert.Equal(t, 5, stats1.TotalRetries)
	assert.True(t, stats1.Successful)
}
