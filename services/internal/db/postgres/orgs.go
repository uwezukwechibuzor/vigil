package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Org represents an organisation row.
type Org struct {
	ID        uuid.UUID
	Slug      string
	Name      string
	Plan      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateOrg inserts a new org and returns it.
func (p *Pool) CreateOrg(ctx context.Context, slug, name, plan string) (*Org, error) {
	var o Org
	err := p.db.QueryRow(ctx, `
		INSERT INTO orgs (slug, name, plan)
		VALUES ($1, $2, $3)
		RETURNING id, slug, name, plan, created_at, updated_at
	`, slug, name, plan).Scan(&o.ID, &o.Slug, &o.Name, &o.Plan, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create org: %w", err)
	}
	return &o, nil
}

// GetOrgBySlug fetches an org by its slug.
func (p *Pool) GetOrgBySlug(ctx context.Context, slug string) (*Org, error) {
	var o Org
	err := p.db.QueryRow(ctx, `
		SELECT id, slug, name, plan, created_at, updated_at
		FROM orgs WHERE slug = $1
	`, slug).Scan(&o.ID, &o.Slug, &o.Name, &o.Plan, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get org by slug: %w", err)
	}
	return &o, nil
}
