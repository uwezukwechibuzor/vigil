// Package postgres provides typed pgx-based repositories for Vigil's relational data.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool wraps a pgxpool and is the entry point for all Postgres operations.
type Pool struct {
	db *pgxpool.Pool
}

// New creates a Pool from a DSN string.
func New(ctx context.Context, dsn string) (*Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgx pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("pgx ping: %w", err)
	}
	return &Pool{db: pool}, nil
}

// Close releases all pool connections.
func (p *Pool) Close() {
	p.db.Close()
}

// Ping checks the database connection.
func (p *Pool) Ping(ctx context.Context) error {
	return p.db.Ping(ctx)
}

// QueryRow executes a query that is expected to return at most one row.
// It is a thin pass-through to the underlying pgxpool.
func (p *Pool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return p.db.QueryRow(ctx, sql, args...)
}
