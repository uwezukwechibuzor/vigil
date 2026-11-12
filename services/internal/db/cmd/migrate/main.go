// Package main is the entrypoint for the vigil-migrate tool.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	db "github.com/uwezukwechibuzor/vigil/services/internal/db"
)

func main() {
	direction := flag.String("direction", "up", "up or down")
	pgDSN := flag.String("pg", os.Getenv("DATABASE_URL"), "Postgres DSN")
	chDSN := flag.String("ch", os.Getenv("CLICKHOUSE_URL"), "ClickHouse DSN")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()

	if *pgDSN == "" && *chDSN == "" {
		fmt.Fprintln(os.Stderr, "error: set DATABASE_URL and/or CLICKHOUSE_URL")
		os.Exit(1)
	}

	if *pgDSN != "" {
		var err error
		if *direction == "down" {
			err = db.MigratePostgresDown(ctx, *pgDSN, logger)
		} else {
			err = db.MigratePostgres(ctx, *pgDSN, logger)
		}
		if err != nil {
			logger.Error("postgres migration failed", "error", err)
			os.Exit(1)
		}
	}

	if *chDSN != "" {
		var err error
		if *direction == "down" {
			err = db.MigrateClickHouseDown(ctx, *chDSN, logger)
		} else {
			err = db.MigrateClickHouse(ctx, *chDSN, logger)
		}
		if err != nil {
			logger.Error("clickhouse migration failed", "error", err)
			os.Exit(1)
		}
	}
}
