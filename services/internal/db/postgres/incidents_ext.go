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
	Summary        string
	Evidence       string // JSONB stored as raw string
	AIExplanation  string
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
			SELECT id, project_id, rule_id, status, severity, title,
			       COALESCE(summary,''), COALESCE(evidence::text,'[]'), COALESCE(ai_explanation,''),
			       started_at, acknowledged_at, resolved_at, created_at
			FROM incidents
			WHERE project_id = $1 AND status = $2
			ORDER BY started_at DESC
		`, projectID, *status)
	} else {
		rows, err = p.db.Query(ctx, `
			SELECT id, project_id, rule_id, status, severity, title,
			       COALESCE(summary,''), COALESCE(evidence::text,'[]'), COALESCE(ai_explanation,''),
			       started_at, acknowledged_at, resolved_at, created_at
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
			&i.Title, &i.Summary, &i.Evidence, &i.AIExplanation,
			&i.StartedAt, &i.AcknowledgedAt, &i.ResolvedAt, &i.CreatedAt); err != nil {
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
		SELECT id, project_id, rule_id, status, severity, title,
		       COALESCE(summary,''), COALESCE(evidence::text,'[]'), COALESCE(ai_explanation,''),
		       started_at, acknowledged_at, resolved_at, created_at
		FROM incidents
		WHERE id = $1
	`, id).Scan(&i.ID, &i.ProjectID, &i.RuleID, &i.Status, &i.Severity,
		&i.Title, &i.Summary, &i.Evidence, &i.AIExplanation,
		&i.StartedAt, &i.AcknowledgedAt, &i.ResolvedAt, &i.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get incident: %w", err)
	}
	return &i, nil
}

// CreateIncident inserts a new incident and returns it.
func (p *Pool) CreateIncident(ctx context.Context, projectID uuid.UUID, ruleID *uuid.UUID, severity, title, summary, aiExplanation string, evidence []byte) (*Incident, error) {
	if evidence == nil {
		evidence = []byte("[]")
	}
	var i Incident
	err := p.db.QueryRow(ctx, `
		INSERT INTO incidents (project_id, rule_id, status, severity, title, summary, evidence, ai_explanation)
		VALUES ($1, $2, 'open', $3, $4, $5, $6::jsonb, $7)
		RETURNING id, project_id, rule_id, status, severity, title,
		          COALESCE(summary,''), COALESCE(evidence::text,'[]'), COALESCE(ai_explanation,''),
		          started_at, acknowledged_at, resolved_at, created_at
	`, projectID, ruleID, severity, title, summary, string(evidence), aiExplanation).
		Scan(&i.ID, &i.ProjectID, &i.RuleID, &i.Status, &i.Severity,
			&i.Title, &i.Summary, &i.Evidence, &i.AIExplanation,
			&i.StartedAt, &i.AcknowledgedAt, &i.ResolvedAt, &i.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create incident: %w", err)
	}
	return &i, nil
}

// UpdateIncidentStatus sets the status and optionally sets acknowledged_at or resolved_at.
func (p *Pool) UpdateIncidentStatus(ctx context.Context, id uuid.UUID, status string) (*Incident, error) {
	var i Incident
	err := p.db.QueryRow(ctx, `
		UPDATE incidents
		SET
			status          = $2,
			acknowledged_at = CASE WHEN $2 = 'acknowledged' THEN now() ELSE acknowledged_at END,
			resolved_at     = CASE WHEN $2 = 'resolved' THEN now() ELSE resolved_at END,
			updated_at      = now()
		WHERE id = $1
		RETURNING id, project_id, rule_id, status, severity, title,
		          COALESCE(summary,''), COALESCE(evidence::text,'[]'), COALESCE(ai_explanation,''),
		          started_at, acknowledged_at, resolved_at, created_at
	`, id, status).
		Scan(&i.ID, &i.ProjectID, &i.RuleID, &i.Status, &i.Severity,
			&i.Title, &i.Summary, &i.Evidence, &i.AIExplanation,
			&i.StartedAt, &i.AcknowledgedAt, &i.ResolvedAt, &i.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("update incident status: %w", err)
	}
	return &i, nil
}
