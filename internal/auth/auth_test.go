package auth

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestAuthMiddleware(t *testing.T) {
	// This would require a test database setup
	// For now, we'll skip since this requires integration testing
	t.Skip("Integration test requires database setup")
}

func TestHashAndCompareAPIKey(t *testing.T) {
	key := "test-api-key-12345"
	hash, err := HashAPIKey(key)
	require.NoError(t, err)
	require.NotEmpty(t, hash)
	require.NotEqual(t, key, hash) // Should be hashed

	// Valid key should compare true
	ok := CompareAPIKey(key, hash)
	require.True(t, ok)

	// Invalid key should compare false
	ok = CompareAPIKey("wrong-key", hash)
	require.False(t, ok)
}

func TestHashAPIKeyRejectsEmpty(t *testing.T) {
	_, err := HashAPIKey("")
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be empty")
}