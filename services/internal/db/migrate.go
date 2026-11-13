// Package db provides database access, migrations and helpers for all Vigil services.
package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // register pgx5 driver for golang-migrate
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/postgres/*.sql
var postgresMigFS embed.FS

//go:embed migrations/clickhouse/*.sql
var clickhouseMigFS embed.FS

// MigratePostgres applies all pending Postgres migrations.
func MigratePostgres(_ context.Context, dsn string, logger *slog.Logger) error {
	return pgMigrate(dsn, logger, true)
}

// MigratePostgresDown rolls back one Postgres migration.
func MigratePostgresDown(_ context.Context, dsn string, logger *slog.Logger) error {
	return pgMigrate(dsn, logger, false)
}

func pgMigrate(dsn string, logger *slog.Logger, up bool) error {
	// golang-migrate's pgx/v5 driver is registered under the "pgx5://" scheme.
	// Accept the standard "postgres://" or "postgresql://" prefix and convert.
	migDSN := strings.NewReplacer(
		"postgres://", "pgx5://",
		"postgresql://", "pgx5://",
	).Replace(dsn)

	subFS, err := fs.Sub(postgresMigFS, "migrations/postgres")
	if err != nil {
		return fmt.Errorf("postgres migration fs: %w", err)
	}
	src, err := iofs.New(subFS, ".")
	if err != nil {
		return fmt.Errorf("postgres migration source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, migDSN)
	if err != nil {
		return fmt.Errorf("postgres migrate init: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	var migrateErr error
	if up {
		migrateErr = m.Up()
	} else {
		migrateErr = m.Steps(-1)
	}
	if migrateErr != nil && migrateErr != migrate.ErrNoChange {
		return fmt.Errorf("postgres migrate: %w", migrateErr)
	}

	v, _, _ := m.Version()
	logger.Info("postgres migrations done", "version", v, "direction", map[bool]string{true: "up", false: "down"}[up])
	return nil
}

// MigrateClickHouse runs versioned ClickHouse *.up.sql files in order.
func MigrateClickHouse(ctx context.Context, dsn string, logger *slog.Logger) error {
	return chMigrate(ctx, dsn, logger, "up")
}

// MigrateClickHouseDown rolls back ClickHouse *.down.sql files in reverse order.
func MigrateClickHouseDown(ctx context.Context, dsn string, logger *slog.Logger) error {
	return chMigrate(ctx, dsn, logger, "down")
}

func chMigrate(ctx context.Context, dsn string, logger *slog.Logger, direction string) error {
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return fmt.Errorf("clickhouse parse dsn: %w", err)
	}
	conn, err := clickhouse.Open(opts)
	if err != nil {
		return fmt.Errorf("clickhouse open: %w", err)
	}
	defer func() { _ = conn.Close() }()

	subFS, err := fs.Sub(clickhouseMigFS, "migrations/clickhouse")
	if err != nil {
		return fmt.Errorf("clickhouse migration fs: %w", err)
	}

	entries, err := fs.ReadDir(subFS, ".")
	if err != nil {
		return fmt.Errorf("clickhouse readdir: %w", err)
	}

	suffix := "." + direction + ".sql"
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), suffix) {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	if direction == "down" {
		sort.Sort(sort.Reverse(sort.StringSlice(files)))
	}

	for _, f := range files {
		data, err := fs.ReadFile(subFS, f)
		if err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}
		for _, stmt := range splitSQL(string(data)) {
			if err := conn.Exec(ctx, stmt); err != nil {
				return fmt.Errorf("clickhouse exec %s: %w", f, err)
			}
		}
		logger.Info("clickhouse migration applied", "file", f, "direction", direction)
	}
	return nil
}

func splitSQL(sql string) []string {
	var out []string
	for _, part := range strings.Split(sql, ";") {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}
