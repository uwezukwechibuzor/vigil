package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EndpointKind matches the endpoint_kind enum in Postgres.
type EndpointKind string

// Endpoint kind values matching the endpoint_kind Postgres enum.
const (
	EndpointKindRPC     EndpointKind = "rpc"
	EndpointKindIndexer EndpointKind = "indexer"
	EndpointKindOracle  EndpointKind = "oracle"
	EndpointKindCustom  EndpointKind = "custom"
)

// Endpoint is one monitored endpoint from the endpoints table.
type Endpoint struct {
	ID        uuid.UUID
	ProjectID uuid.UUID
	ChainID   *uuid.UUID
	Kind      EndpointKind
	Name      string
	URL       string
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ListEnabledEndpoints returns all enabled endpoints, ordered by ID for
// stable sharding.
func (p *Pool) ListEnabledEndpoints(ctx context.Context) ([]Endpoint, error) {
	rows, err := p.db.Query(ctx, `
		SELECT id, project_id, chain_id, kind, name, url, enabled, created_at, updated_at
		FROM endpoints
		WHERE enabled = true
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("list endpoints: %w", err)
	}
	defer rows.Close()

	var out []Endpoint
	for rows.Next() {
		var e Endpoint
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.ChainID, &e.Kind,
			&e.Name, &e.URL, &e.Enabled, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan endpoint: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
