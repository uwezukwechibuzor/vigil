package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Membership represents a user's role in an org.
type Membership struct {
	OrgID     uuid.UUID
	UserID    uuid.UUID
	Role      string
	CreatedAt time.Time
}

// CreateMembership inserts a new org membership.
func (p *Pool) CreateMembership(ctx context.Context, orgID, userID uuid.UUID, role string) (*Membership, error) {
	var m Membership
	err := p.db.QueryRow(ctx, `
		INSERT INTO memberships (org_id, user_id, role)
		VALUES ($1, $2, $3)
		RETURNING org_id, user_id, role, created_at
	`, orgID, userID, role).Scan(&m.OrgID, &m.UserID, &m.Role, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create membership: %w", err)
	}
	return &m, nil
}

// GetMembership fetches a user's membership in an org.
func (p *Pool) GetMembership(ctx context.Context, userID uuid.UUID) (*Membership, error) {
	var m Membership
	err := p.db.QueryRow(ctx, `
		SELECT org_id, user_id, role, created_at
		FROM memberships WHERE user_id = $1
		ORDER BY created_at ASC LIMIT 1
	`, userID).Scan(&m.OrgID, &m.UserID, &m.Role, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get membership: %w", err)
	}
	return &m, nil
}
