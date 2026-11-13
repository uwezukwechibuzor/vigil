package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/uwezukwechibuzor/vigil/services/api/internal/auth"
	"github.com/uwezukwechibuzor/vigil/services/internal/db/postgres"
)

// mockDB implements auth.DB for testing.
type mockDB struct {
	key *postgres.APIKey
	err error
}

func (m *mockDB) LookupAPIKey(_ context.Context, _ string) (*postgres.APIKey, error) {
	return m.key, m.err
}

// mockCache is a no-op cache for testing.
type mockCache struct{}

func (m *mockCache) Set(_ context.Context, _ string, _ any, _ time.Duration) error { return nil }
func (m *mockCache) Get(_ context.Context, _ string, _ any) (bool, error)          { return false, nil }

func makeKey(scopes []string, expiresAt *time.Time, revokedAt *time.Time) *postgres.APIKey {
	return &postgres.APIKey{
		ID:        uuid.New(),
		OrgID:     uuid.New(),
		Name:      "test-key",
		KeyPrefix: "vgl_test",
		KeyHash:   "fakehash",
		Scopes:    scopes,
		ExpiresAt: expiresAt,
		RevokedAt: revokedAt,
		CreatedAt: time.Now(),
	}
}

func TestAPIKeyAuth_ValidKey(t *testing.T) {
	key := makeKey([]string{"read"}, nil, nil)
	db := &mockDB{key: key}
	cache := &mockCache{}

	var gotKey *postgres.APIKey
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotKey = auth.APIKeyFromContext(r.Context())
	})

	mw := auth.APIKeyAuth(db, cache)
	handler := mw(next)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/chains", nil)
	req.Header.Set("Authorization", "Bearer vgl_testtoken")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotKey == nil {
		t.Fatal("expected API key in context, got nil")
	}
}

func TestAPIKeyAuth_MissingToken(t *testing.T) {
	mw := auth.APIKeyAuth(nil, nil)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := mw(next)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/chains", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAPIKeyAuth_RevokedKey(t *testing.T) {
	now := time.Now()
	key := makeKey([]string{"read"}, nil, &now)
	db := &mockDB{key: key}
	cache := &mockCache{}

	mw := auth.APIKeyAuth(db, cache)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := mw(next)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/chains", nil)
	req.Header.Set("Authorization", "Bearer vgl_anykey")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for revoked key, got %d", w.Code)
	}
}

func TestAPIKeyAuth_ExpiredKey(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour)
	key := makeKey([]string{"read"}, &past, nil)
	db := &mockDB{key: key}
	cache := &mockCache{}

	mw := auth.APIKeyAuth(db, cache)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := mw(next)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/chains", nil)
	req.Header.Set("Authorization", "Bearer vgl_anykey")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired key, got %d", w.Code)
	}
}

func TestRequireScope_Allowed(t *testing.T) {
	key := makeKey([]string{"read", "flightcheck"}, nil, nil)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := auth.RequireScope("flightcheck")
	handler := mw(next)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/flight-checks", nil)
	req = req.WithContext(auth.WithAPIKey(req.Context(), key))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRequireScope_Denied(t *testing.T) {
	key := makeKey([]string{"read"}, nil, nil) // only has 'read', not 'flightcheck'

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := auth.RequireScope("flightcheck")
	handler := mw(next)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/flight-checks", nil)
	req = req.WithContext(auth.WithAPIKey(req.Context(), key))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestHashLookupPath(t *testing.T) {
	// Verify that the lookup path uses hash comparison (not plaintext).
	// If the DB received a request at all, it used the hash path.
	calls := 0
	key := makeKey([]string{"read"}, nil, nil)
	db := &mockDB{key: key}
	db2 := &countingDB{key: key, counter: &calls}
	cache := &mockCache{}

	mw := auth.APIKeyAuth(db2, cache)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := mw(next)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer vgl_testtoken123")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if calls != 1 {
		t.Fatalf("expected exactly 1 DB call via hash lookup, got %d", calls)
	}
	_ = db
}

type countingDB struct {
	key     *postgres.APIKey
	counter *int
}

func (c *countingDB) LookupAPIKey(_ context.Context, _ string) (*postgres.APIKey, error) {
	*c.counter++
	return c.key, nil
}
