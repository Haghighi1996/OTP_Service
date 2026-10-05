package delivery

import (
	"context"
	"math/rand"
	"time"
)

// Real is a production SMS delivery adapter using Twilio-like API.
type Real struct {
	AccountSID string
	AuthToken  string
	From       string // E.164 sender number
}

// Send implements the Adapter interface.
func (r Real) Send(ctx context.Context, phoneNumber, code string) error {
	// In a real implementation, this would call the SMS provider API.
	// For now, we simulate a brief network delay and succeed.
	select {
	case <-time.After(50 * time.Millisecond):
		// Simulated successful delivery
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// Unreliable is an adapter that occasionally fails to simulate network issues.
type Unreliable struct {
	FailChance float32 // 0.0 to 1.0
}

// Send implements the Adapter interface with probabilistic failure.
func (u Unreliable) Send(ctx context.Context, phoneNumber, code string) error {
	select {
	case <-time.After(50 * time.Millisecond):
		if u.FailChance > 0 && rand.Float32() < u.FailChance {
			return ErrDeliveryFailed
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
