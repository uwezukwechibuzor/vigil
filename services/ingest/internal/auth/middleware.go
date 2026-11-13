// Package auth provides API key authentication middleware for the ingest service.
package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	dbpg "github.com/uwezukwechibuzor/vigil/services/internal/db/postgres"
)

// contextKey is an unexported type for context keys in this package.
type contextKey string

// projectIDKey is the context key for the authenticated project ID.
const projectIDKey contextKey = "project_id"

// APIKeyAuth returns a chi middleware that validates the X-Vigil-API-Key or Authorization header.
// Authenticates by SHA-256 hashing the key and looking it up in Postgres.
// If pg is nil, authentication is skipped (dev mode).
func APIKeyAuth(pg *dbpg.Pool, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if pg == nil {
				next.ServeHTTP(w, r)
				return
			}

			key := extractKey(r)
			if key == "" {
				logger.WarnContext(r.Context(), "auth: missing api key")
				http.Error(w, `{"error":"missing api key"}`, http.StatusUnauthorized)
				return
			}

			apiKey, err := pg.LookupAPIKey(r.Context(), key)
			if err != nil {
				logger.WarnContext(r.Context(), "auth: invalid api key", "error", err)
				http.Error(w, `{"error":"invalid api key"}`, http.StatusUnauthorized)
				return
			}

			projectID := ""
			if apiKey.ProjectID != nil {
				projectID = apiKey.ProjectID.String()
			}

			ctx := context.WithValue(r.Context(), projectIDKey, projectID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ProjectIDFromContext returns the project ID stored in the context, or empty string.
func ProjectIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(projectIDKey).(string)
	return v
}

// extractKey pulls the API key from X-Vigil-API-Key or Authorization: Bearer <key>.
func extractKey(r *http.Request) string {
	if k := r.Header.Get("X-Vigil-API-Key"); k != "" {
		return k
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}
