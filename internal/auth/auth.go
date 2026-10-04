package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// AuthContextKey is the context key used to store authenticated request context.
var AuthContextKey struct{}

// AuthenticatedRequest represents an HTTP request with authentication context.
type AuthenticatedRequest struct {
	*http.Request
	TenantID int64
}

// ErrInvalidAPIKey is returned when an API key is invalid or missing.
var ErrInvalidAPIKey = errors.New("invalid or missing API key")

// ErrInactiveAPIKey is returned when an API key exists but is inactive.
var ErrInactiveAPIKey = errors.New("API key is inactive")

// APIKey represents an API key record from the database.
type APIKey struct {
	ID        int64
	TenantID  int64
	KeyHash   string
	CreatedAt string // Time as string for JSON compatibility
	ExpiresAt *string
	IsActive  bool
}

// Store defines the interface for API key storage operations.
type Store interface {
	GetByKeyHash(context.Context, string) (APIKey, error)
}

// NewStore creates a new API key store backed by PostgreSQL.
func NewStore(pool *pgxpool.Pool) Store {
	return &pgxStore{pool: pool}
}

// pgxStore implements the Store interface using pgx.
type pgxStore struct {
	pool *pgxpool.Pool
}

// GetByKeyHash retrieves an API key by its bcrypt hash.
func (s *pgxStore) GetByKeyHash(ctx context.Context, keyHash string) (APIKey, error) {
	var ak APIKey
	var expiresAtPtr *string

	err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, key_hash, created_at, expires_at, is_active 
		 FROM api_keys 
		 WHERE key_hash = $1`, keyHash).Scan(
		&ak.ID,
		&ak.TenantID,
		&ak.KeyHash,
		&ak.CreatedAt,
		&expiresAtPtr,
		&ak.IsActive,
	)

	if err != nil {
		return APIKey{}, err
	}

	if expiresAtPtr != nil {
		ak.ExpiresAt = expiresAtPtr
	}

	return ak, nil
}

// HashAPIKey creates a bcrypt hash of an API key for secure storage.
func HashAPIKey(key string) (string, error) {
	if key == "" {
		return "", errors.New("API key cannot be empty")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

// CompareAPIKey compares an API key with its bcrypt hash.
func CompareAPIKey(key, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(key))
	return err == nil
}

// GetTenantID extracts the tenant ID from an authenticated request context.
func GetTenantID(ctx context.Context) (int64, bool) {
	authReq, ok := ctx.Value(AuthContextKey).(*AuthenticatedRequest)
	if !ok {
		return 0, false
	}
	return authReq.TenantID, true
}

// Middleware creates an HTTP middleware that validates API keys and adds tenant context.
func Middleware(store Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract API key from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, ErrInvalidAPIKey.Error(), http.StatusUnauthorized)
				return
			}

			// Expect format: "Bearer <key>"
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, ErrInvalidAPIKey.Error(), http.StatusUnauthorized)
				return
			}

			apiKey := parts[1]
			if apiKey == "" {
				http.Error(w, ErrInvalidAPIKey.Error(), http.StatusUnauthorized)
				return
			}

			// Hash the provided key to look it up
			keyHash, err := HashAPIKey(apiKey)
			if err != nil {
				http.Error(w, ErrInvalidAPIKey.Error(), http.StatusUnauthorized)
				return
			}

			// Look up the API key by hash
			ak, err := store.GetByKeyHash(r.Context(), keyHash)
			if err != nil {
				http.Error(w, ErrInvalidAPIKey.Error(), http.StatusUnauthorized)
				return
			}

			if !ak.IsActive {
				http.Error(w, ErrInactiveAPIKey.Error(), http.StatusUnauthorized)
				return
			}

			// Create authenticated request with tenant context
			authReq := &AuthenticatedRequest{
				Request:  r,
				TenantID: ak.TenantID,
			}

			// Add to context and call the next handler
			ctx := context.WithValue(r.Context(), AuthContextKey, authReq)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}