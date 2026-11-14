package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Project represents a project row.
type Project struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	Slug      string
	Name      string
	Env       string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateProject inserts a new project.
func (p *Pool) CreateProject(ctx context.Context, orgID uuid.UUID, slug, name, env string) (*Project, error) {
	var proj Project
	err := p.db.QueryRow(ctx, `
		INSERT INTO projects (org_id, slug, name, env)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (org_id, slug) DO UPDATE SET name = EXCLUDED.name
		RETURNING id, org_id, slug, name, env, created_at, updated_at
	`, orgID, slug, name, env).Scan(
		&proj.ID, &proj.OrgID, &proj.Slug, &proj.Name, &proj.Env, &proj.CreatedAt, &proj.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	return &proj, nil
}

// GetFirstProjectByOrgID returns the oldest project for an org, or nil if none exist.
func (p *Pool) GetFirstProjectByOrgID(ctx context.Context, orgID uuid.UUID) (*Project, error) {
	var proj Project
	err := p.db.QueryRow(ctx, `
		SELECT id, org_id, slug, name, env, created_at, updated_at
		FROM projects WHERE org_id = $1
		ORDER BY created_at ASC LIMIT 1
	`, orgID).Scan(
		&proj.ID, &proj.OrgID, &proj.Slug, &proj.Name, &proj.Env, &proj.CreatedAt, &proj.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get first project: %w", err)
	}
	return &proj, nil
}
