package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Incident represents an active or resolved alert incident.
type Incident struct {
	ID             uuid.UUID
	ProjectID      uuid.UUID
	RuleID         *uuid.UUID
	Status         string
	Severity       string
	Title          string
	StartedAt      time.Time
	AcknowledgedAt *time.Time
	ResolvedAt     *time.Time
	CreatedAt      time.Time
}

// ListIncidents returns incidents for a project, optionally filtered by status.
func (p *Pool) ListIncidents(ctx context.Context, projectID uuid.UUID, status *string) ([]Incident, error) {
	var rows interface {
		Next() bool
		Scan(...any) error
		Close()
		Err() error
	}
	var err error
	if status != nil {
		rows, err = p.db.Query(ctx, `
			SELECT id, project_id, rule_id, status, severity, title, started_at, acknowledged_at, resolved_at, created_at
			FROM incidents
			WHERE project_id = $1 AND status = $2
			ORDER BY started_at DESC
		`, projectID, *status)
	} else {
		rows, err = p.db.Query(ctx, `
			SELECT id, project_id, rule_id, status, severity, title, started_at, acknowledged_at, resolved_at, created_at
			FROM incidents
			WHERE project_id = $1
			ORDER BY started_at DESC
		`, projectID)
	}
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	defer rows.Close()

	var out []Incident
	for rows.Next() {
		var i Incident
		if err := rows.Scan(&i.ID, &i.ProjectID, &i.RuleID, &i.Status, &i.Severity,
			&i.Title, &i.StartedAt, &i.AcknowledgedAt, &i.ResolvedAt, &i.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan incident: %w", err)
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// GetIncident returns a single incident by ID.
func (p *Pool) GetIncident(ctx context.Context, id uuid.UUID) (*Incident, error) {
	var i Incident
	err := p.db.QueryRow(ctx, `
		SELECT id, project_id, rule_id, status, severity, title, started_at, acknowledged_at, resolved_at, created_at
		FROM incidents
		WHERE id = $1
	`, id).Scan(&i.ID, &i.ProjectID, &i.RuleID, &i.Status, &i.Severity,
		&i.Title, &i.StartedAt, &i.AcknowledgedAt, &i.ResolvedAt, &i.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get incident: %w", err)
	}
	return &i, nil
}
