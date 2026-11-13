package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Policy is a named rule set governing flight checks for a project.
type Policy struct {
	ID        uuid.UUID
	ProjectID uuid.UUID
	Name      string
	Version   int
	Rules     PolicyRules
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PolicyRules holds the structured flight-check constraints for a policy.
type PolicyRules struct {
	// AllowedChains is a list of allowed chain IDs (uint64). Empty means all allowed.
	AllowedChains []uint64 `json:"allowed_chains,omitempty"`
	// DeniedContracts is a list of contract addresses (checksummed or lowercase) that are blocked.
	DeniedContracts []string `json:"denied_contracts,omitempty"`
	// MaxValueWei is the maximum transaction value in wei as a decimal string.
	MaxValueWei string `json:"max_value_wei,omitempty"`
}

// ErrPolicyNotFound is returned when no policy matches the given ID.
var ErrPolicyNotFound = errors.New("policy not found")

// GetPolicy fetches a policy by its UUID.
func (p *Pool) GetPolicy(ctx context.Context, id uuid.UUID) (*Policy, error) {
	var pol Policy
	var rulesJSON []byte

	err := p.db.QueryRow(ctx, `
		SELECT id, project_id, name, version, rules, enabled, created_at, updated_at
		FROM policies
		WHERE id = $1
	`, id).Scan(&pol.ID, &pol.ProjectID, &pol.Name, &pol.Version, &rulesJSON, &pol.Enabled, &pol.CreatedAt, &pol.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPolicyNotFound
		}
		return nil, fmt.Errorf("get policy: %w", err)
	}

	if err := json.Unmarshal(rulesJSON, &pol.Rules); err != nil {
		return nil, fmt.Errorf("parse policy rules: %w", err)
	}
	return &pol, nil
}

// IsAddressDenied returns true when the given address is on the project's denylist.
func (p *Pool) IsAddressDenied(ctx context.Context, projectID uuid.UUID, address string) (bool, error) {
	var exists bool
	err := p.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM denylist
			WHERE project_id = $1 AND lower(address) = lower($2)
		)
	`, projectID, address).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("is address denied: %w", err)
	}
	return exists, nil
}
