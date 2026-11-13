package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// ListPolicies returns all policies for a project.
func (p *Pool) ListPolicies(ctx context.Context, projectID uuid.UUID) ([]Policy, error) {
	rows, err := p.db.Query(ctx, `
		SELECT id, project_id, name, version, rules, enabled, created_at, updated_at
		FROM policies
		WHERE project_id = $1
		ORDER BY created_at DESC
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list policies: %w", err)
	}
	defer rows.Close()

	var out []Policy
	for rows.Next() {
		var pol Policy
		var rulesJSON []byte
		if err := rows.Scan(&pol.ID, &pol.ProjectID, &pol.Name, &pol.Version,
			&rulesJSON, &pol.Enabled, &pol.CreatedAt, &pol.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan policy: %w", err)
		}
		if err := json.Unmarshal(rulesJSON, &pol.Rules); err != nil {
			return nil, fmt.Errorf("parse policy rules: %w", err)
		}
		out = append(out, pol)
	}
	return out, rows.Err()
}

// CreatePolicy inserts a new policy and returns it.
func (p *Pool) CreatePolicy(ctx context.Context, projectID uuid.UUID, name string, rules PolicyRules, enabled bool) (*Policy, error) {
	rulesJSON, err := json.Marshal(rules)
	if err != nil {
		return nil, fmt.Errorf("marshal policy rules: %w", err)
	}
	var pol Policy
	var rulesOut []byte
	err = p.db.QueryRow(ctx, `
		INSERT INTO policies (project_id, name, rules, enabled)
		VALUES ($1, $2, $3, $4)
		RETURNING id, project_id, name, version, rules, enabled, created_at, updated_at
	`, projectID, name, rulesJSON, enabled).
		Scan(&pol.ID, &pol.ProjectID, &pol.Name, &pol.Version, &rulesOut, &pol.Enabled, &pol.CreatedAt, &pol.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create policy: %w", err)
	}
	if err := json.Unmarshal(rulesOut, &pol.Rules); err != nil {
		return nil, fmt.Errorf("parse policy rules: %w", err)
	}
	return &pol, nil
}

// UpdatePolicy applies partial updates to a policy.
func (p *Pool) UpdatePolicy(ctx context.Context, id uuid.UUID, name *string, rules *PolicyRules, enabled *bool) (*Policy, error) {
	var rulesJSON []byte
	var err error
	if rules != nil {
		rulesJSON, err = json.Marshal(rules)
		if err != nil {
			return nil, fmt.Errorf("marshal policy rules: %w", err)
		}
	}

	var pol Policy
	var rulesOut []byte
	err = p.db.QueryRow(ctx, `
		UPDATE policies
		SET
			name       = COALESCE($2, name),
			rules      = CASE WHEN $3::jsonb IS NOT NULL THEN $3::jsonb ELSE rules END,
			enabled    = COALESCE($4, enabled),
			version    = version + 1,
			updated_at = now()
		WHERE id = $1
		RETURNING id, project_id, name, version, rules, enabled, created_at, updated_at
	`, id, name, rulesJSON, enabled).
		Scan(&pol.ID, &pol.ProjectID, &pol.Name, &pol.Version, &rulesOut, &pol.Enabled, &pol.CreatedAt, &pol.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update policy: %w", err)
	}
	if err := json.Unmarshal(rulesOut, &pol.Rules); err != nil {
		return nil, fmt.Errorf("parse policy rules: %w", err)
	}
	return &pol, nil
}

// DeletePolicy removes a policy by ID.
func (p *Pool) DeletePolicy(ctx context.Context, id uuid.UUID) error {
	_, err := p.db.Exec(ctx, `DELETE FROM policies WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete policy: %w", err)
	}
	return nil
}
