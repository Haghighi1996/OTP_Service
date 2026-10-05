package worker

import (
	"context"
	"errors"
	"sync"
	"time"

	"otp-service/internal/delivery"
	"otp-service/internal/retry"
)

// ErrQueueFull is returned when the job queue is full and backpressure is applied.
var ErrQueueFull = errors.New("job queue full")

// ErrWorkerPoolStopped is returned when submitting a job to a stopped worker pool.
var ErrWorkerPoolStopped = errors.New("worker pool stopped")

// DeliveryJob represents an async OTP delivery task.
type DeliveryJob struct {
	PhoneNumber string
	Code        string
	Adapter     delivery.Adapter
	MaxRetries  int

	// Result channel receives the error (or nil on success) after the job completes.
	// The channel is buffered with size 1 to avoid blocking the worker.
	Result chan error
}

// Do executes the delivery job with retries and exponential backoff.
func (j *DeliveryJob) Do(ctx context.Context) error {
	if j.Adapter == nil {
		return delivery.ErrDeliveryFailed
	}
	// Use retry package for better reliability
	retryOpts := &retry.RetryOptions{
		MaxRetries: j.MaxRetries,
		BackoffStrategy: &retry.ExponentialBackoff{
			Base:       100 * time.Millisecond,
			Jitter:     50 * time.Millisecond,
			MaxBackoff: 30 * time.Second,
		},
		AllowContextCancellation: true,
	}

	_, err := retry.Retry(ctx, retryOpts, func(ctx context.Context) (interface{}, error) {
		if err := j.Adapter.Send(ctx, j.PhoneNumber, j.Code); err != nil {
			return nil, err
		}
		return "delivered", nil
	})

	if err != nil {
		return err
	}
	return nil
}

// JobQueue is a bounded queue for delivery jobs.
// When full, Submit() returns ErrQueueFull (backpressure).
type JobQueue struct {
	jobs   chan *DeliveryJob
	closed bool
	mu     sync.Mutex
}

// NewJobQueue creates a job queue with the given capacity.
// Capacity must be > 0.
func NewJobQueue(capacity int) *JobQueue {
	if capacity <= 0 {
		capacity = 1
	}
	return &JobQueue{
		jobs: make(chan *DeliveryJob, capacity),
	}
}

// Submit adds a job to the queue.
// Returns ErrQueueFull if the queue is full.
// Returns ErrWorkerPoolStopped if the queue is closed.
func (q *JobQueue) Submit(job *DeliveryJob) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return ErrWorkerPoolStopped
	}

	select {
	case q.jobs <- job:
		return nil
	default:
		return ErrQueueFull
	}
}

// TrySubmit attempts to submit a job without blocking.
// Returns ErrQueueFull if the queue is full.
func (q *JobQueue) TrySubmit(job *DeliveryJob) error {
	return q.Submit(job)
}

// Jobs returns the channel of jobs for workers to consume.
func (q *JobQueue) Jobs() <-chan *DeliveryJob {
	return q.jobs
}

// Close closes the queue, preventing further submissions.
// Workers will drain remaining jobs before exiting.
func (q *JobQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !q.closed {
		q.closed = true
		close(q.jobs)
	}
}

// IsClosed returns true if the queue is closed.
func (q *JobQueue) IsClosed() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closed
}
