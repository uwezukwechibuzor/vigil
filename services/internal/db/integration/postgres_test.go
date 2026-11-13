package integration_test

import (
	"context"
	"testing"

	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/testcontainers/testcontainers-go"
	db "github.com/uwezukwechibuzor/vigil/services/internal/db"
	"github.com/uwezukwechibuzor/vigil/services/internal/db/postgres"
)

func startPostgres(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("vigil"),
		tcpostgres.WithUsername("vigil"),
		tcpostgres.WithPassword("vigil"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("postgres dsn: %v", err)
	}
	return dsn
}

func TestPostgresMigrations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	dsn := startPostgres(t)
	ctx := context.Background()
	logger := noopLogger()

	// golang-migrate expects pgx5 driver DSN with the pgx5:// scheme
	migDSN := "pgx5://" + dsn[len("postgres://"):]

	if err := db.MigratePostgres(ctx, migDSN, logger); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	// Verify by using the pool
	pool, err := postgres.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestOrgAndAPIKey(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	dsn := startPostgres(t)
	ctx := context.Background()
	logger := noopLogger()

	migDSN := "pgx5://" + dsn[len("postgres://"):]
	if err := db.MigratePostgres(ctx, migDSN, logger); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	pool, err := postgres.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	org, err := pool.CreateOrg(ctx, "test-org", "Test Org", "free")
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	if org.Slug != "test-org" {
		t.Errorf("slug = %q, want test-org", org.Slug)
	}

	plaintext, key, err := pool.GenerateAPIKey(ctx, org.ID, nil, "test-key", []string{"read"}, nil)
	if err != nil {
		t.Fatalf("generate api key: %v", err)
	}
	if plaintext == "" {
		t.Fatal("plaintext should not be empty")
	}

	// Lookup by plaintext
	found, err := pool.LookupAPIKey(ctx, plaintext)
	if err != nil {
		t.Fatalf("lookup api key: %v", err)
	}
	if found.ID != key.ID {
		t.Errorf("key id mismatch: got %v want %v", found.ID, key.ID)
	}

	// Migrate down — one step
	if err := db.MigratePostgresDown(ctx, migDSN, logger); err != nil {
		t.Fatalf("migrate down: %v", err)
	}
}
