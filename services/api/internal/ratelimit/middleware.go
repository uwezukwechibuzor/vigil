// Package ratelimit provides sliding-window rate limiting middleware for the API.
package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	redisdb "github.com/uwezukwechibuzor/vigil/services/internal/db/redis"
)

// planLimits maps plan names to requests-per-minute limits.
var planLimits = map[string]int{
	"free":       100,
	"pro":        1000,
	"team":       5000,
	"enterprise": -1, // unlimited
}

// Limiter is the subset of Redis used for rate limiting and idempotency.
type Limiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (*redisdb.RateLimitResult, error)
	Set(ctx context.Context, key string, v any, ttl time.Duration) error
	Get(ctx context.Context, key string, dst any) (bool, error)
}

// cachedResponse stores an idempotent POST response in Redis.
type cachedResponse struct {
	Status int         `json:"status"`
	Header http.Header `json:"header"`
	Body   []byte      `json:"body"`
}

// Middleware returns a per-key sliding-window rate-limit handler.
// plan should be one of: free, pro, team, enterprise.
func Middleware(limiter Limiter, getKeyAndPlan func(r *http.Request) (key, plan string)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil {
				next.ServeHTTP(w, r)
				return
			}

			key, plan := getKeyAndPlan(r)
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			limit := planLimits[plan]
			if limit == -1 {
				// Enterprise: unlimited; check idempotency only.
				handleIdempotency(w, r, next, limiter, key)
				return
			}
			if limit == 0 {
				limit = planLimits["free"] // default
			}

			res, err := limiter.Allow(r.Context(), "rl:"+key, limit, time.Minute)
			if err != nil || res == nil {
				// Redis unavailable — degrade gracefully.
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(res.Remaining, 10))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(res.ResetAt.Unix(), 10))

			if !res.Allowed {
				writeTooManyRequests(w, res.ResetAt)
				return
			}

			handleIdempotency(w, r, next, limiter, key)
		})
	}
}

// handleIdempotency serves cached POST responses when Idempotency-Key header is set.
func handleIdempotency(w http.ResponseWriter, r *http.Request, next http.Handler, limiter Limiter, keyPrefix string) {
	idempKey := r.Header.Get("Idempotency-Key")
	if r.Method != http.MethodPost || idempKey == "" {
		next.ServeHTTP(w, r)
		return
	}

	cacheKey := fmt.Sprintf("idemp:%s:%s", keyPrefix, idempKey)

	var cached cachedResponse
	hit, _ := limiter.Get(r.Context(), cacheKey, &cached)
	if hit {
		for k, vals := range cached.Header {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		w.Header().Set("X-Idempotent-Replayed", "true")
		w.WriteHeader(cached.Status)
		_, _ = w.Write(cached.Body)
		return
	}

	rec := &responseRecorder{header: http.Header{}, code: http.StatusOK}
	next.ServeHTTP(rec, r)

	// Store response for 24 hours.
	resp := cachedResponse{
		Status: rec.code,
		Header: rec.header,
		Body:   rec.body,
	}
	_ = limiter.Set(r.Context(), cacheKey, resp, 24*time.Hour)

	for k, vals := range rec.header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(rec.code)
	_, _ = w.Write(rec.body)
}

// responseRecorder captures an HTTP response for idempotency caching.
type responseRecorder struct {
	header http.Header
	body   []byte
	code   int
}

func (r *responseRecorder) Header() http.Header  { return r.header }
func (r *responseRecorder) WriteHeader(code int) { r.code = code }
func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body = append(r.body, b...)
	return len(b), nil
}

// IPMiddleware limits unauthenticated requests by client IP.
// limit is requests per window duration. It degrades gracefully when Redis is unavailable.
func IPMiddleware(limiter Limiter, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil {
				next.ServeHTTP(w, r)
				return
			}
			ip := clientIP(r)
			res, err := limiter.Allow(r.Context(), "ip:"+ip, limit, window)
			if err != nil || res == nil {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(res.Remaining, 10))
			if !res.Allowed {
				writeTooManyRequests(w, res.ResetAt)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP extracts the real client IP, preferring X-Forwarded-For when trusted.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first (leftmost) address
		if idx := len(xff); idx > 0 {
			parts := xff[:idx]
			if comma := len(parts); comma > 0 {
				// simple split: take everything before the first comma
				for i, c := range parts {
					if c == ',' {
						return strings.TrimSpace(parts[:i])
					}
				}
				return strings.TrimSpace(parts)
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeTooManyRequests(w http.ResponseWriter, resetAt time.Time) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", strconv.FormatInt(resetAt.Unix(), 10))
	w.WriteHeader(http.StatusTooManyRequests)
	body := map[string]any{
		"error": map[string]string{
			"code":    "rate_limit_exceeded",
			"message": "rate limit exceeded, retry after " + resetAt.UTC().Format(time.RFC3339),
		},
	}
	_ = json.NewEncoder(w).Encode(body)
}
