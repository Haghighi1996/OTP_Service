package delivery

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdapterNoOp(t *testing.T) {
	adapter := NoOp{}
	err := adapter.Send(context.Background(), "+1234567890", "123456")
	assert.NoError(t, err)
}

func TestAdapterFailing(t *testing.T) {
	adapter := Failing{}
	err := adapter.Send(context.Background(), "+1234567890", "123456")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrDeliveryFailed))
}
