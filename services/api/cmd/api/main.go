// Package main is the entrypoint for the vigil-api service.
//
//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest --config ../../oapi-codegen.yaml ../../openapi.yaml
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	api "github.com/uwezukwechibuzor/vigil/services/api"
	"github.com/uwezukwechibuzor/vigil/services/api/gen"
	"github.com/uwezukwechibuzor/vigil/services/api/internal/analytics"
	"github.com/uwezukwechibuzor/vigil/services/api/internal/auth"
	"github.com/uwezukwechibuzor/vigil/services/api/internal/docs"
	"github.com/uwezukwechibuzor/vigil/services/api/internal/handler"
	"github.com/uwezukwechibuzor/vigil/services/api/internal/metering"
	"github.com/uwezukwechibuzor/vigil/services/api/internal/ratelimit"
	"github.com/uwezukwechibuzor/vigil/services/internal/db/clickhouse"
	"github.com/uwezukwechibuzor/vigil/services/internal/db/postgres"
	redisdb "github.com/uwezukwechibuzor/vigil/services/internal/db/redis"
	"github.com/uwezukwechibuzor/vigil/services/internal/platform"
)

func main() {
	platform.StartService("api", func(r chi.Router) {
		cfg := platform.MustLoadConfig("api")
		logger := platform.NewLogger(cfg.LogLevel)

		// --- connect optional backing services ---
		var pg *postgres.Pool
		if cfg.DatabaseURL != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			var err error
			pg, err = postgres.New(ctx, cfg.DatabaseURL)
			if err != nil {
				logger.Warn("postgres unavailable, running degraded", "error", err)
			}
		} else {
			logger.Warn("DATABASE_URL not set; Postgres features disabled")
		}

		var rc *redisdb.Client
		if cfg.RedisURL != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var err error
			rc, err = redisdb.New(ctx, cfg.RedisURL)
			if err != nil {
				logger.Warn("redis unavailable, rate limiting disabled", "error", err)
			}
		}

		var ch *clickhouse.Client
		if cfg.ClickHouseURL != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			var err error
			ch, err = clickhouse.New(ctx, cfg.ClickHouseURL)
			if err != nil {
				logger.Warn("clickhouse unavailable, telemetry features disabled", "error", err)
			}
		} else {
			logger.Warn("CLICKHOUSE_URL not set; ClickHouse features disabled")
		}

		// --- build handler tree ---
		meter := metering.New(pg, logger)
		bgCtx := context.Background()
		meter.StartRollup(bgCtx)

		ac := analytics.New(logger)

		h := handler.New(pg, ch, rc, meter, logger, ac)
		strictH := gen.NewStrictHandler(h, nil)

		// --- CORS ---
		corsOrigins := getEnv("CORS_ALLOWED_ORIGINS", "*")
		origins := strings.Split(corsOrigins, ",")
		for i, o := range origins {
			origins[i] = strings.TrimSpace(o)
		}

		corsMiddleware := cors.Handler(cors.Options{
			AllowedOrigins:   origins,
			AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Authorization", "Content-Type", "Idempotency-Key"},
			ExposedHeaders:   []string{"X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"},
			AllowCredentials: true,
			MaxAge:           300,
		})
		r.Use(corsMiddleware)

		// --- public routes (no auth required) ---
		secure := getEnv("APP_ENV", "local") == "production"
		authH := handler.NewAuthHandler(pg, secure)
		r.Post("/auth/signup", authH.Signup)
		r.Post("/auth/login", authH.Login)
		r.Post("/auth/logout", authH.Logout)

		docsHandler := docs.Handler(api.OpenAPISpec)
		r.Handle("/docs", docsHandler)
		r.Handle("/openapi.yaml", docsHandler)

		// --- protected routes (API key or session required) ---
		r.Group(func(r chi.Router) {
			r.Use(auth.APIKeyAuth(pg, rc))
			r.Use(ratelimit.Middleware(rc, func(req *http.Request) (key, plan string) {
				k := auth.APIKeyFromContext(req.Context())
				if k != nil {
					return k.ID.String(), "free"
				}
				return "", ""
			}))

			r.Get("/v1/me", h.Me)
			r.Mount("/v1", http.MaxBytesHandler(
				gen.HandlerFromMux(strictH, chi.NewRouter()),
				1<<20,
			))
			r.Get("/v1/data/export", h.DataExportHandler)
			r.Post("/v1/data/delete", h.DataDeleteHandler)
		})

		logger.Info("api service routes registered")
	})
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// unused slog import guard.
var _ = slog.Default
