// Package main is the entrypoint for the vigil-seed tool.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/uwezukwechibuzor/vigil/services/internal/db/postgres"
	"github.com/uwezukwechibuzor/vigil/services/internal/db/seed"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	pool, err := postgres.New(ctx, dsn)
	if err != nil {
		logger.Error("connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := seed.Run(ctx, pool, logger); err != nil {
		logger.Error("seed failed", "error", err)
		os.Exit(1)
	}
}
