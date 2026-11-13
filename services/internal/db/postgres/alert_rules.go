package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AlertRule is a monitoring rule that triggers incidents.
type AlertRule struct {
	ID          uuid.UUID
	ProjectID   uuid.UUID
	Name        string
	Description string
	Severity    string
	Condition   map[string]any
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ListAlertRules returns all alert rules for a project.
func (p *Pool) ListAlertRules(ctx context.Context, projectID uuid.UUID) ([]AlertRule, error) {
	rows, err := p.db.Query(ctx, `
		SELECT id, project_id, name, COALESCE(description,''), severity, condition, enabled, created_at, updated_at
		FROM alert_rules
		WHERE project_id = $1
		ORDER BY created_at DESC
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list alert rules: %w", err)
	}
	defer rows.Close()

	var out []AlertRule
	for rows.Next() {
		var r AlertRule
		var condJSON []byte
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Name, &r.Description,
			&r.Severity, &condJSON, &r.Enabled, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan alert rule: %w", err)
		}
		if err := json.Unmarshal(condJSON, &r.Condition); err != nil {
			return nil, fmt.Errorf("parse alert rule condition: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CreateAlertRule inserts a new alert rule and returns it.
func (p *Pool) CreateAlertRule(ctx context.Context, projectID uuid.UUID, name, description, severity string, condition map[string]any, enabled bool) (*AlertRule, error) {
	condJSON, err := json.Marshal(condition)
	if err != nil {
		return nil, fmt.Errorf("marshal condition: %w", err)
	}
	var r AlertRule
	var condOut []byte
	err = p.db.QueryRow(ctx, `
		INSERT INTO alert_rules (project_id, name, description, severity, condition, enabled)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, project_id, name, COALESCE(description,''), severity, condition, enabled, created_at, updated_at
	`, projectID, name, description, severity, condJSON, enabled).
		Scan(&r.ID, &r.ProjectID, &r.Name, &r.Description, &r.Severity, &condOut, &r.Enabled, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create alert rule: %w", err)
	}
	if err := json.Unmarshal(condOut, &r.Condition); err != nil {
		return nil, fmt.Errorf("parse alert rule condition: %w", err)
	}
	return &r, nil
}

// GetAlertRule returns a single alert rule by ID.
func (p *Pool) GetAlertRule(ctx context.Context, id uuid.UUID) (*AlertRule, error) {
	var r AlertRule
	var condJSON []byte
	err := p.db.QueryRow(ctx, `
		SELECT id, project_id, name, COALESCE(description,''), severity, condition, enabled, created_at, updated_at
		FROM alert_rules
		WHERE id = $1
	`, id).Scan(&r.ID, &r.ProjectID, &r.Name, &r.Description, &r.Severity, &condJSON, &r.Enabled, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get alert rule: %w", err)
	}
	if err := json.Unmarshal(condJSON, &r.Condition); err != nil {
		return nil, fmt.Errorf("parse alert rule condition: %w", err)
	}
	return &r, nil
}

// UpdateAlertRule applies partial updates to an alert rule.
func (p *Pool) UpdateAlertRule(ctx context.Context, id uuid.UUID, name, description, severity *string, condition map[string]any, enabled *bool) (*AlertRule, error) {
	var condJSON []byte
	var err error
	if condition != nil {
		condJSON, err = json.Marshal(condition)
		if err != nil {
			return nil, fmt.Errorf("marshal condition: %w", err)
		}
	}

	var r AlertRule
	var condOut []byte
	err = p.db.QueryRow(ctx, `
		UPDATE alert_rules
		SET
			name        = COALESCE($2, name),
			description = COALESCE($3, description),
			severity    = COALESCE($4, severity),
			condition   = CASE WHEN $5::jsonb IS NOT NULL THEN $5::jsonb ELSE condition END,
			enabled     = COALESCE($6, enabled),
			updated_at  = now()
		WHERE id = $1
		RETURNING id, project_id, name, COALESCE(description,''), severity, condition, enabled, created_at, updated_at
	`, id, name, description, severity, condJSON, enabled).
		Scan(&r.ID, &r.ProjectID, &r.Name, &r.Description, &r.Severity, &condOut, &r.Enabled, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update alert rule: %w", err)
	}
	if err := json.Unmarshal(condOut, &r.Condition); err != nil {
		return nil, fmt.Errorf("parse alert rule condition: %w", err)
	}
	return &r, nil
}

// DeleteAlertRule removes an alert rule by ID.
func (p *Pool) DeleteAlertRule(ctx context.Context, id uuid.UUID) error {
	_, err := p.db.Exec(ctx, `DELETE FROM alert_rules WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete alert rule: %w", err)
	}
	return nil
}
