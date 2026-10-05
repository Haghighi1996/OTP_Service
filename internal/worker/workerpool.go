package worker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"otp-service/internal/delivery"
)

// ErrBackoffExceeded is returned when backoff duration exceeds max.
type ErrBackoffExceeded struct {
	MaxBackoff time.Duration
}

func (e *ErrBackoffExceeded) Error() string {
	return fmt.Sprintf("backoff exceeded %v", e.MaxBackoff)
}

// WorkerPool manages a pool of workers that process delivery jobs.
type WorkerPool struct {
	queue      *JobQueue
	workers    int32
	running    atomic.Bool
	wg         sync.WaitGroup
	stopChan   chan struct{}
	maxBackoff time.Duration
	delivery   delivery.Adapter
	maxRetries int
}

// WorkerPoolConfig holds configuration for a worker pool.
type WorkerPoolConfig struct {
	QueueCapacity int
	WorkerCount   int
	MaxBackoff    time.Duration
	MaxRetries    int
	Delivery      delivery.Adapter
}

// NewWorkerPool creates a new worker pool with the given configuration.
func NewWorkerPool(cfg WorkerPoolConfig) *WorkerPool {
	if cfg.QueueCapacity <= 0 {
		cfg.QueueCapacity = 100 // default queue size
	}
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 4 // default worker count
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 30 * time.Second
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 3 // default retries
	}

	queue := NewJobQueue(cfg.QueueCapacity)

	return &WorkerPool{
		queue:      queue,
		workers:    int32(cfg.WorkerCount),
		stopChan:   make(chan struct{}),
		maxBackoff: cfg.MaxBackoff,
		delivery:   cfg.Delivery,
		maxRetries: cfg.MaxRetries,
	}
}

// Queue returns the job queue for submitting jobs.
func (p *WorkerPool) Queue() *JobQueue {
	return p.queue
}

// Start begins processing jobs from the queue.
// Returns an error if the pool is already running.
func (p *WorkerPool) Start(ctx context.Context) error {
	if !p.running.CompareAndSwap(false, true) {
		return fmt.Errorf("worker pool already running")
	}

	for i := int32(0); i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i)
	}

	return nil
}

// worker is the main loop for a single worker goroutine.
func (p *WorkerPool) worker(ctx context.Context, id int32) {
	defer p.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopChan:
			return
		case job, ok := <-p.queue.Jobs():
			if !ok {
				// Queue closed, drain remaining jobs
				return
			}
			p.processJob(ctx, job)
		}
	}
}

// processJob handles a single delivery job.
func (p *WorkerPool) processJob(ctx context.Context, job *DeliveryJob) {
	if job.Adapter == nil {
		job.Adapter = p.delivery
	}
	if job.MaxRetries == 0 {
		job.MaxRetries = p.maxRetries
	}

	// Create context with timeout for this job
	jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result := job.Do(jobCtx)
	select {
	case job.Result <- result:
	default:
		// Result already filled or channel blocked
	}
}

// Stop signals all workers to stop.
// It returns immediately; use Wait() to wait for workers to finish.
func (p *WorkerPool) Stop() {
	if p.running.CompareAndSwap(true, false) {
		close(p.stopChan)
		p.queue.Close()
	}
}

// Wait blocks until all workers have finished and the queue is drained.
func (p *WorkerPool) Wait() {
	p.wg.Wait()
}

// Shutdown gracefully stops the worker pool.
// It signals workers to stop, drains the queue, and waits for completion.
func (p *WorkerPool) Shutdown(ctx context.Context) error {
	// Close queue to stop accepting new jobs
	p.queue.Close()

	// Signal workers to stop
	p.Stop()

	// Wait for workers with timeout
	done := make(chan struct{})
	go func() {
		p.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// EnqueueDelivery submits a delivery job and returns the result channel.
// If the queue is full, it returns ErrQueueFull.
// Implements delivery.Deliverer interface.
func (p *WorkerPool) EnqueueDelivery(phoneNumber, code string) (<-chan error, error) {
	result := make(chan error, 1)
	job := &DeliveryJob{
		PhoneNumber: phoneNumber,
		Code:        code,
		Adapter:     p.delivery,
		MaxRetries:  p.maxRetries,
		Result:      result,
	}
	return result, p.Submit(job)
}

// Submit attempts to submit a job to the pool.
// It returns ErrQueueFull if the queue is full (backpressure).
func (p *WorkerPool) Submit(job *DeliveryJob) error {
	if !p.running.Load() {
		return ErrWorkerPoolStopped
	}
	return p.queue.Submit(job)
}

// SubmitWithResult submits a job and returns the result channel.
// If the queue is full, it returns ErrQueueFull.
// Deprecated: Use EnqueueDelivery instead.
func (p *WorkerPool) SubmitWithResult(phoneNumber, code string) (<-chan error, error) {
	return p.EnqueueDelivery(phoneNumber, code)
}

// Stats returns current pool statistics.
type PoolStats struct {
	QueueSize     int
	Workers       int32
	Running       bool
	QueueCapacity int
}

func (p *WorkerPool) Stats() PoolStats {
	return PoolStats{
		QueueSize:     len(p.queue.jobs),
		Workers:       p.workers,
		Running:       p.running.Load(),
		QueueCapacity: cap(p.queue.jobs),
	}
}
