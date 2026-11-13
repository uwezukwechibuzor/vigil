// Package main is the entrypoint for the vigil-collector service.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	goredis "github.com/redis/go-redis/v9"
	"github.com/uwezukwechibuzor/vigil/services/collector/internal/collector"
	dbch "github.com/uwezukwechibuzor/vigil/services/internal/db/clickhouse"
	dbpg "github.com/uwezukwechibuzor/vigil/services/internal/db/postgres"
	"github.com/uwezukwechibuzor/vigil/services/internal/platform"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pgDSN := envOrDefault("POSTGRES_DSN", "pgx5://vigil:vigil@localhost:5432/vigil?sslmode=disable")
	chDSN := envOrDefault("CLICKHOUSE_DSN", "clickhouse://localhost:9000/vigil")
	redisDSN := envOrDefault("REDIS_URL", "redis://localhost:6379/0")

	pg, err := dbpg.New(ctx, pgDSN)
	if err != nil {
		logger.Error("postgres connect", "err", err)
		os.Exit(1)
	}

	ch, err := dbch.New(ctx, chDSN)
	if err != nil {
		logger.Error("clickhouse connect", "err", err)
		os.Exit(1)
	}
	defer func() { _ = ch.Close() }()

	redisOpts, err := goredis.ParseURL(redisDSN)
	if err != nil {
		logger.Error("redis parse url", "err", err)
		os.Exit(1)
	}
	rdb := goredis.NewClient(redisOpts)
	defer func() { _ = rdb.Close() }()

	coll, err := collector.New(pg, ch, rdb, logger)
	if err != nil {
		logger.Error("collector init", "err", err)
		os.Exit(1)
	}

	// Run collector in background; expose HTTP for health/metrics.
	go func() {
		if err := coll.Run(ctx); err != nil && err != context.Canceled {
			logger.Error("collector exited", "err", err)
		}
	}()

	platform.StartService("collector", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"service":"vigil-collector","status":"ok"}`))
		})
	})
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
