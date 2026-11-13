package ratelimit_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/uwezukwechibuzor/vigil/services/api/internal/ratelimit"
	redisdb "github.com/uwezukwechibuzor/vigil/services/internal/db/redis"
)

// mockLimiter simulates the Redis rate limiter.
type mockLimiter struct {
	allowed   bool
	remaining int64
	resetAt   time.Time
	cache     map[string][]byte
}

func newMockLimiter(allowed bool) *mockLimiter {
	return &mockLimiter{
		allowed:   allowed,
		remaining: 99,
		resetAt:   time.Now().Add(time.Minute),
		cache:     make(map[string][]byte),
	}
}

func (m *mockLimiter) Allow(_ context.Context, _ string, _ int, _ time.Duration) (*redisdb.RateLimitResult, error) {
	return &redisdb.RateLimitResult{
		Allowed:   m.allowed,
		Remaining: m.remaining,
		ResetAt:   m.resetAt,
	}, nil
}

func (m *mockLimiter) Set(_ context.Context, key string, v any, _ time.Duration) error {
	b, _ := json.Marshal(v)
	m.cache[key] = b
	return nil
}

func (m *mockLimiter) Get(_ context.Context, key string, dst any) (bool, error) {
	b, ok := m.cache[key]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(b, dst)
}

func TestRateLimit_HeadersPresent(t *testing.T) {
	lim := newMockLimiter(true)
	mw := ratelimit.Middleware(lim, func(_ *http.Request) (string, string) {
		return "test-key", "pro"
	})

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/chains", nil)
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("X-RateLimit-Limit") == "" {
		t.Fatal("expected X-RateLimit-Limit header")
	}
	if w.Header().Get("X-RateLimit-Remaining") == "" {
		t.Fatal("expected X-RateLimit-Remaining header")
	}
	if w.Header().Get("X-RateLimit-Reset") == "" {
		t.Fatal("expected X-RateLimit-Reset header")
	}
}

func TestRateLimit_429WhenExceeded(t *testing.T) {
	lim := newMockLimiter(false) // not allowed
	mw := ratelimit.Middleware(lim, func(_ *http.Request) (string, string) {
		return "test-key", "free"
	})

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/chains", nil)
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}

func TestRateLimit_IdempotencyReplay(t *testing.T) {
	lim := newMockLimiter(true)
	mw := ratelimit.Middleware(lim, func(_ *http.Request) (string, string) {
		return "test-key", "pro"
	})

	callCount := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"abc"}`))
	})

	// First request.
	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/api-keys", nil)
	req1.Header.Set("Idempotency-Key", "unique-key-1")
	w1 := httptest.NewRecorder()
	mw(next).ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("first request: expected 201, got %d", w1.Code)
	}

	// Replay with same idempotency key — should return same response without calling next.
	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/api-keys", nil)
	req2.Header.Set("Idempotency-Key", "unique-key-1")
	w2 := httptest.NewRecorder()
	mw(next).ServeHTTP(w2, req2)

	if w2.Code != http.StatusCreated {
		t.Fatalf("replay: expected 201, got %d", w2.Code)
	}
	if w2.Header().Get("X-Idempotent-Replayed") != "true" {
		t.Fatal("expected X-Idempotent-Replayed header on replay")
	}
	// Handler should only be called once.
	if callCount != 1 {
		t.Fatalf("expected handler to be called once, got %d", callCount)
	}
}

func TestRateLimit_NilLimiter(t *testing.T) {
	// nil limiter should degrade gracefully and pass through.
	mw := ratelimit.Middleware(nil, func(_ *http.Request) (string, string) {
		return "test-key", "free"
	})

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/chains", nil)
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with nil limiter, got %d", w.Code)
	}
}
