package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHashAndCompareAPIKey(t *testing.T) {
	key := "test-api-key-12345"
	hash, err := HashAPIKey(key)
	if err != nil {
		t.Fatalf("HashAPIKey() error = %v", err)
	}
	if hash == "" {
		t.Fatal("HashAPIKey() returned empty hash")
	}
	if hash == key {
		t.Fatal("HashAPIKey() should not return plaintext key")
	}

	// Valid key should compare true
	if !CompareAPIKey(key, hash) {
		t.Error("CompareAPIKey() should return true for valid key")
	}

	// Invalid key should compare false
	if CompareAPIKey("wrong-key", hash) {
		t.Error("CompareAPIKey() should return false for invalid key")
	}
}

func TestHashAPIKeyRejectsEmpty(t *testing.T) {
	_, err := HashAPIKey("")
	if err == nil {
		t.Fatal("HashAPIKey() should error on empty key")
	}
	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("HashAPIKey() error = %v, want error containing 'cannot be empty'", err)
	}
}

func TestMiddlewareRejectsMissingAuthHeader(t *testing.T) {
	store := &mockStore{}
	mw := Middleware(store)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := mw(next)

	req := httptest.NewRequest(http.MethodGet, "/v1/otp/send", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddlewareRejectsInvalidAuthFormat(t *testing.T) {
	store := &mockStore{}
	mw := Middleware(store)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := mw(next)

	req := httptest.NewRequest(http.MethodGet, "/v1/otp/send", nil)
	req.Header.Set("Authorization", "InvalidFormat")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddlewareRejectsEmptyAPIKey(t *testing.T) {
	store := &mockStore{}
	mw := Middleware(store)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := mw(next)

	req := httptest.NewRequest(http.MethodGet, "/v1/otp/send", nil)
	req.Header.Set("Authorization", "Bearer ")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddlewareRejectsUnknownAPIKey(t *testing.T) {
	store := &mockStore{err: ErrInvalidAPIKey}
	mw := Middleware(store)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := mw(next)

	req := httptest.NewRequest(http.MethodGet, "/v1/otp/send", nil)
	req.Header.Set("Authorization", "Bearer unknown-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddlewareRejectsInactiveAPIKey(t *testing.T) {
	store := &mockStore{err: ErrInactiveAPIKey}
	mw := Middleware(store)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := mw(next)

	req := httptest.NewRequest(http.MethodGet, "/v1/otp/send", nil)
	req.Header.Set("Authorization", "Bearer inactive-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddlewarePassesAuthenticatedRequest(t *testing.T) {
	store := &mockStore{key: APIKey{ID: 1, TenantID: 42, IsActive: true}}
	mw := Middleware(store)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := GetTenantID(r.Context())
		if !ok {
			t.Error("tenant ID not found in context")
			return
		}
		if tenantID != 42 {
			t.Errorf("TenantID = %d, want 42", tenantID)
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := mw(next)

	req := httptest.NewRequest(http.MethodGet, "/v1/otp/send", nil)
	req.Header.Set("Authorization", "Bearer valid-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

type mockStore struct {
	key APIKey
	err error
}

func (m *mockStore) GetByKeyHash(ctx context.Context, keyHash string) (APIKey, error) {
	return m.key, m.err
}

func TestAPIKeyStruct(t *testing.T) {
	ak := APIKey{
		ID:        1,
		TenantID:  42,
		KeyHash:   "hash",
		CreatedAt: "2024-01-01T00:00:00Z",
		ExpiresAt: nil,
		IsActive:  true,
	}

	if ak.ID != 1 {
		t.Errorf("ID = %d, want 1", ak.ID)
	}
	if ak.TenantID != 42 {
		t.Errorf("TenantID = %d, want 42", ak.TenantID)
	}
	if !ak.IsActive {
		t.Error("IsActive should be true")
	}
}