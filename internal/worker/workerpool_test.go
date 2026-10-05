package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"otp-service/internal/delivery"
)

func TestJobQueueSubmit(t *testing.T) {
	queue := NewJobQueue(2)

	job1 := &DeliveryJob{Result: make(chan error, 1)}
	job2 := &DeliveryJob{Result: make(chan error, 1)}

	if err := queue.Submit(job1); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if err := queue.Submit(job2); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if len(queue.jobs) != 2 {
		t.Errorf("queue length = %d, want 2", len(queue.jobs))
	}
}

func TestJobQueueSubmitWhenFull(t *testing.T) {
	queue := NewJobQueue(1)

	job := &DeliveryJob{Result: make(chan error, 1)}
	if err := queue.Submit(job); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	job2 := &DeliveryJob{Result: make(chan error, 1)}
	if err := queue.Submit(job2); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("Submit() error = %v, want ErrQueueFull", err)
	}
}

func TestJobQueueClose(t *testing.T) {
	queue := NewJobQueue(2)
	queue.Close()

	if !queue.IsClosed() {
		t.Error("queue should be closed")
	}

	job := &DeliveryJob{Result: make(chan error, 1)}
	if err := queue.Submit(job); !errors.Is(err, ErrWorkerPoolStopped) {
		t.Fatalf("Submit() error = %v, want ErrWorkerPoolStopped", err)
	}
}

func TestWorkerPoolSubmitBeforeStart(t *testing.T) {
	pool := NewWorkerPool(WorkerPoolConfig{WorkerCount: 1})
	job := &DeliveryJob{Result: make(chan error, 1)}

	if err := pool.Submit(job); !errors.Is(err, ErrWorkerPoolStopped) {
		t.Fatalf("Submit() error = %v, want ErrWorkerPoolStopped", err)
	}
}

func TestWorkerPoolStartAndStop(t *testing.T) {
	pool := NewWorkerPool(WorkerPoolConfig{WorkerCount: 2, QueueCapacity: 10})
	ctx := context.Background()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if stats := pool.Stats(); !stats.Running {
		t.Error("pool should be running")
	}

	pool.Stop()
	pool.Wait()
}

func TestWorkerPoolShutdown(t *testing.T) {
	pool := NewWorkerPool(WorkerPoolConfig{WorkerCount: 2, QueueCapacity: 10})
	ctx := context.Background()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	done := make(chan struct{})
	go func() {
		pool.Wait()
		close(done)
	}()

	pool.Shutdown(ctx)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown timed out")
	}
}

func TestWorkerPoolBackpressure(t *testing.T) {
	pool := NewWorkerPool(WorkerPoolConfig{WorkerCount: 1, QueueCapacity: 1})
	ctx := context.Background()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer pool.Stop()
	defer pool.Wait()

	job1 := &DeliveryJob{Result: make(chan error, 1)}
	job2 := &DeliveryJob{Result: make(chan error, 1)}

	if err := pool.Submit(job1); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if err := pool.Submit(job2); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	job3 := &DeliveryJob{Result: make(chan error, 1)}
	if err := pool.Submit(job3); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("Submit() error = %v, want ErrQueueFull", err)
	}
}

func TestWorkerPoolDelivery(t *testing.T) {
	pool := NewWorkerPool(WorkerPoolConfig{
		WorkerCount:  1,
		QueueCapacity: 10,
		Delivery:     delivery.NoOp{},
		MaxRetries:   0,
	})
	ctx := context.Background()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer pool.Shutdown(ctx)

	resultChan, err := pool.EnqueueDelivery("+12025550123", "123456")
	if err != nil {
		t.Fatalf("EnqueueDelivery() error = %v", err)
	}

	select {
	case result := <-resultChan:
		if result != nil {
			t.Errorf("delivery result = %v, want nil", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("delivery timed out")
	}
}

func TestWorkerPoolDeliveryFailure(t *testing.T) {
	pool := NewWorkerPool(WorkerPoolConfig{
		WorkerCount:  1,
		QueueCapacity: 10,
		Delivery:     delivery.Failing{},
		MaxRetries:   0,
	})
	ctx := context.Background()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer pool.Shutdown(ctx)

	resultChan, err := pool.EnqueueDelivery("+12025550123", "123456")
	if err != nil {
		t.Fatalf("EnqueueDelivery() error = %v", err)
	}

	select {
	case result := <-resultChan:
		if result == nil {
			t.Error("delivery result = nil, want error")
		}
		if !errors.Is(result, delivery.ErrDeliveryFailed) {
			t.Errorf("delivery result = %v, want ErrDeliveryFailed", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("delivery timed out")
	}
}

func TestJobQueueIsClosed(t *testing.T) {
	queue := NewJobQueue(10)
	if queue.IsClosed() {
		t.Error("queue should not be closed initially")
	}

	queue.Close()
	if !queue.IsClosed() {
		t.Error("queue should be closed after Close()")
	}
}

func TestWorkerPoolStats(t *testing.T) {
	pool := NewWorkerPool(WorkerPoolConfig{WorkerCount: 2, QueueCapacity: 10})
	stats := pool.Stats()

	if stats.Workers != 2 {
		t.Errorf("Stats().Workers = %d, want 2", stats.Workers)
	}
	if stats.QueueCapacity != 10 {
		t.Errorf("Stats().QueueCapacity = %d, want 10", stats.QueueCapacity)
	}
	if stats.Running {
		t.Error("Stats().Running = true, want false before Start()")
	}
	if stats.QueueSize != 0 {
		t.Errorf("Stats().QueueSize = %d, want 0", stats.QueueSize)
	}
}

func TestJobQueueCloseIdempotent(t *testing.T) {
	queue := NewJobQueue(10)
	queue.Close()
	queue.Close() // should not panic
}

func TestWorkerPoolSubmitAfterStop(t *testing.T) {
	pool := NewWorkerPool(WorkerPoolConfig{WorkerCount: 1, QueueCapacity: 10})
	ctx := context.Background()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	pool.Stop()

	job := &DeliveryJob{Result: make(chan error, 1)}
	if err := pool.Submit(job); !errors.Is(err, ErrWorkerPoolStopped) {
		t.Fatalf("Submit() error = %v, want ErrWorkerPoolStopped", err)
	}
}