package delivery

import (
	"context"
	"errors"
)

// ErrDeliveryFailed is returned when an OTP delivery cannot be completed.
var ErrDeliveryFailed = errors.New("delivery failed")

// Adapter is the interface for OTP delivery providers (SMS, email, etc.).
// Implementations must be safe for concurrent use.
type Adapter interface {
	// Send delivers the OTP code to the given phone number.
	// It must return ErrDeliveryFailed if the delivery cannot be completed.
	// Implementations should respect context cancellation.
	Send(ctx context.Context, phoneNumber, code string) error
}

// Deliverer is the interface for asynchronous delivery submission.
// It is used by the OTP service to enqueue delivery jobs.
type Deliverer interface {
	// EnqueueDelivery submits a delivery job asynchronously and returns a
	// channel that will receive the result (nil on success, error on failure).
	// It returns an error if the job cannot be submitted (e.g., queue full, stopped).
	EnqueueDelivery(phoneNumber, code string) (<-chan error, error)
}

// NoOp is a delivery adapter that does nothing. Useful for testing.
type NoOp struct{}

// Send implements the Adapter interface by returning nil.
func (NoOp) Send(ctx context.Context, phoneNumber, code string) error {
	return nil
}

// Failing is a delivery adapter that always fails. Useful for testing error paths.
type Failing struct{}

// Send implements the Adapter interface by returning ErrDeliveryFailed.
func (Failing) Send(ctx context.Context, phoneNumber, code string) error {
	return ErrDeliveryFailed
}
