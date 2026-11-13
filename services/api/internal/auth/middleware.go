// Package auth provides API key and session authentication middleware.
package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/uwezukwechibuzor/vigil/services/internal/db/postgres"
	redisdb "github.com/uwezukwechibuzor/vigil/services/internal/db/redis"
)

// contextKey is an unexported type for context keys in this package.
type contextKey int

const (
	apiKeyCtxKey contextKey = iota
	orgIDCtxKey
)

// DB is the subset of the postgres pool used by auth middleware.
type DB interface {
	LookupAPIKey(ctx context.Context, plaintext string) (*postgres.APIKey, error)
}

// Cache is the subset of the redis client used by auth middleware.
type Cache interface {
	Set(ctx context.Context, key string, v any, ttl time.Duration) error
	Get(ctx context.Context, key string, dst any) (bool, error)
}

// cachedKey holds the fields cached in Redis for quick scope lookups.
type cachedKey struct {
	ID        string     `json:"id"`
	OrgID     string     `json:"org_id"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

// cookieSession is the minimal parsed session payload from vigil_session cookie.
type cookieSession struct {
	OrgID string `json:"org_id"`
}

// APIKeyAuth returns middleware that authenticates Bearer tokens and attaches
// the resolved *postgres.APIKey to the request context.
func APIKeyAuth(db DB, cache Cache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if ok {
				key, err := lookupKey(r.Context(), token, db, cache)
				if err != nil {
					writeUnauthorized(w, "invalid or expired API key")
					return
				}
				r = r.WithContext(WithAPIKey(r.Context(), key))
				next.ServeHTTP(w, r)
				return
			}

			// Fall back to cookie auth (dashboard users).
			c, err := r.Cookie("vigil_session")
			if err == nil && c.Value != "" {
				orgID, err := parseCookieSession(c.Value)
				if err == nil {
					r = r.WithContext(context.WithValue(r.Context(), orgIDCtxKey, orgID))
					next.ServeHTTP(w, r)
					return
				}
			}

			writeUnauthorized(w, "authentication required")
		})
	}
}

// RequireScope returns middleware that checks whether the authenticated key
// has the required scope. Must be used after APIKeyAuth.
func RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := APIKeyFromContext(r.Context())
			if key == nil {
				// Cookie auth — scope checks not enforced for dashboard session.
				next.ServeHTTP(w, r)
				return
			}
			if !hasScope(key.Scopes, scope) {
				writeForbidden(w, "insufficient scope: "+scope)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// WithAPIKey attaches a resolved API key to a context.
func WithAPIKey(ctx context.Context, key *postgres.APIKey) context.Context {
	return context.WithValue(ctx, apiKeyCtxKey, key)
}

// APIKeyFromContext retrieves the API key from a context, or nil.
func APIKeyFromContext(ctx context.Context) *postgres.APIKey {
	k, _ := ctx.Value(apiKeyCtxKey).(*postgres.APIKey)
	return k
}

// OrgIDFromContext retrieves the org ID string from a context (cookie auth path).
func OrgIDFromContext(ctx context.Context) string {
	s, _ := ctx.Value(orgIDCtxKey).(string)
	return s
}

// lookupKey resolves a plaintext token to an APIKey, using a 7-day Redis cache.
func lookupKey(ctx context.Context, plaintext string, db DB, cache Cache) (*postgres.APIKey, error) {
	// Derive cache key from SHA-256 hash (never store plaintext in Redis).
	hash := sha256.Sum256([]byte(plaintext))
	hashHex := hex.EncodeToString(hash[:])
	cacheKey := "apikey:" + hashHex

	if cache != nil {
		var ck cachedKey
		hit, _ := cache.Get(ctx, cacheKey, &ck)
		if hit {
			if ck.RevokedAt != nil {
				return nil, errors.New("key revoked")
			}
			if ck.ExpiresAt != nil && ck.ExpiresAt.Before(time.Now()) {
				return nil, errors.New("key expired")
			}
			// Reconstruct minimal APIKey for scope checks.
			k := &postgres.APIKey{Scopes: ck.Scopes}
			return k, nil
		}
	}

	if db == nil {
		return nil, errors.New("auth unavailable")
	}

	key, err := db.LookupAPIKey(ctx, plaintext)
	if err != nil {
		return nil, err
	}

	// Validate expiry and revocation.
	if key.RevokedAt != nil {
		return nil, errors.New("key revoked")
	}
	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("key expired")
	}

	// Cache for 7 days (constant-time compare already done by hash lookup in DB).
	if cache != nil {
		ck := cachedKey{
			ID:        key.ID.String(),
			OrgID:     key.OrgID.String(),
			Scopes:    key.Scopes,
			ExpiresAt: key.ExpiresAt,
			RevokedAt: key.RevokedAt,
		}
		_ = cache.Set(ctx, cacheKey, ck, 7*24*time.Hour)
	}

	return key, nil
}

// parseCookieSession base64-decodes and parses the vigil_session cookie value.
func parseCookieSession(val string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(val)
	if err != nil {
		return "", err
	}
	var s cookieSession
	if err := json.Unmarshal(b, &s); err != nil {
		return "", err
	}
	if s.OrgID == "" {
		return "", errors.New("empty org_id in session")
	}
	return s.OrgID, nil
}

// bearerToken extracts a Bearer token from Authorization header.
func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return "", false
	}
	t := strings.TrimPrefix(h, "Bearer ")
	t = strings.TrimSpace(t)
	if t == "" {
		return "", false
	}
	return t, true
}

// hasScope reports whether scopes contains the required scope,
// using constant-time comparison to avoid timing side channels.
func hasScope(scopes []string, required string) bool {
	req := []byte(required)
	for _, s := range scopes {
		if subtle.ConstantTimeCompare([]byte(s), req) == 1 {
			return true
		}
	}
	return false
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusUnauthorized, "unauthorized", msg)
}

func writeForbidden(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusForbidden, "forbidden", msg)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": msg,
		},
	}
	_ = json.NewEncoder(w).Encode(body)
}

// Ensure redisdb import is used by the Cache interface.
var _ Cache = (*redisdb.Client)(nil)
