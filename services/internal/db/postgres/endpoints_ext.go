package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// ListEndpoints returns all endpoints for a given project, ordered by created_at.
func (p *Pool) ListEndpoints(ctx context.Context, projectID uuid.UUID) ([]Endpoint, error) {
	rows, err := p.db.Query(ctx, `
		SELECT id, project_id, chain_id, kind, name, url, enabled, created_at, updated_at
		FROM endpoints
		WHERE project_id = $1
		ORDER BY created_at DESC
	`, projectID)
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

// CreateEndpoint inserts a new endpoint and returns it.
func (p *Pool) CreateEndpoint(ctx context.Context, projectID, chainID *uuid.UUID, kind EndpointKind, name, url string) (*Endpoint, error) {
	var e Endpoint
	err := p.db.QueryRow(ctx, `
		INSERT INTO endpoints (project_id, chain_id, kind, name, url)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, project_id, chain_id, kind, name, url, enabled, created_at, updated_at
	`, projectID, chainID, kind, name, url).
		Scan(&e.ID, &e.ProjectID, &e.ChainID, &e.Kind, &e.Name, &e.URL, &e.Enabled, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create endpoint: %w", err)
	}
	return &e, nil
}

// UpdateEndpoint applies partial updates to an endpoint.
func (p *Pool) UpdateEndpoint(ctx context.Context, id uuid.UUID, name, url *string, enabled *bool) (*Endpoint, error) {
	var e Endpoint
	err := p.db.QueryRow(ctx, `
		UPDATE endpoints
		SET
			name    = COALESCE($2, name),
			url     = COALESCE($3, url),
			enabled = COALESCE($4, enabled),
			updated_at = now()
		WHERE id = $1
		RETURNING id, project_id, chain_id, kind, name, url, enabled, created_at, updated_at
	`, id, name, url, enabled).
		Scan(&e.ID, &e.ProjectID, &e.ChainID, &e.Kind, &e.Name, &e.URL, &e.Enabled, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update endpoint: %w", err)
	}
	return &e, nil
}

// DeleteEndpoint removes an endpoint by ID.
func (p *Pool) DeleteEndpoint(ctx context.Context, id uuid.UUID) error {
	_, err := p.db.Exec(ctx, `DELETE FROM endpoints WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete endpoint: %w", err)
	}
	return nil
}

// GetEndpoint returns a single endpoint by ID.
func (p *Pool) GetEndpoint(ctx context.Context, id uuid.UUID) (*Endpoint, error) {
	var e Endpoint
	err := p.db.QueryRow(ctx, `
		SELECT id, project_id, chain_id, kind, name, url, enabled, created_at, updated_at
		FROM endpoints
		WHERE id = $1
	`, id).Scan(&e.ID, &e.ProjectID, &e.ChainID, &e.Kind, &e.Name, &e.URL, &e.Enabled, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get endpoint: %w", err)
	}
	return &e, nil
}
