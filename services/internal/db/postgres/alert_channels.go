package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AlertChannel is a notification destination for alert rules.
type AlertChannel struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	Kind      string
	Name      string
	Config    map[string]any
	CreatedAt time.Time
}

// ListAlertChannels returns all alert channels for an org.
func (p *Pool) ListAlertChannels(ctx context.Context, orgID uuid.UUID) ([]AlertChannel, error) {
	rows, err := p.db.Query(ctx, `
		SELECT id, org_id, kind, name, config, created_at
		FROM alert_channels
		WHERE org_id = $1
		ORDER BY created_at DESC
	`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list alert channels: %w", err)
	}
	defer rows.Close()

	var out []AlertChannel
	for rows.Next() {
		var ch AlertChannel
		var cfgJSON []byte
		if err := rows.Scan(&ch.ID, &ch.OrgID, &ch.Kind, &ch.Name, &cfgJSON, &ch.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan alert channel: %w", err)
		}
		if err := json.Unmarshal(cfgJSON, &ch.Config); err != nil {
			return nil, fmt.Errorf("parse alert channel config: %w", err)
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}

// CreateAlertChannel inserts a new alert channel and returns it.
func (p *Pool) CreateAlertChannel(ctx context.Context, orgID uuid.UUID, kind, name string, config map[string]any) (*AlertChannel, error) {
	cfgJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	var ch AlertChannel
	var cfgOut []byte
	err = p.db.QueryRow(ctx, `
		INSERT INTO alert_channels (org_id, kind, name, config)
		VALUES ($1, $2, $3, $4)
		RETURNING id, org_id, kind, name, config, created_at
	`, orgID, kind, name, cfgJSON).
		Scan(&ch.ID, &ch.OrgID, &ch.Kind, &ch.Name, &cfgOut, &ch.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create alert channel: %w", err)
	}
	if err := json.Unmarshal(cfgOut, &ch.Config); err != nil {
		return nil, fmt.Errorf("parse alert channel config: %w", err)
	}
	return &ch, nil
}

// GetAlertChannel returns a single alert channel by ID.
func (p *Pool) GetAlertChannel(ctx context.Context, id uuid.UUID) (*AlertChannel, error) {
	var ch AlertChannel
	var cfgJSON []byte
	err := p.db.QueryRow(ctx, `
		SELECT id, org_id, kind, name, config, created_at
		FROM alert_channels
		WHERE id = $1
	`, id).Scan(&ch.ID, &ch.OrgID, &ch.Kind, &ch.Name, &cfgJSON, &ch.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get alert channel: %w", err)
	}
	if err := json.Unmarshal(cfgJSON, &ch.Config); err != nil {
		return nil, fmt.Errorf("parse alert channel config: %w", err)
	}
	return &ch, nil
}

// UpdateAlertChannel applies partial updates to an alert channel.
func (p *Pool) UpdateAlertChannel(ctx context.Context, id uuid.UUID, name *string, config map[string]any) (*AlertChannel, error) {
	var cfgJSON []byte
	var err error
	if config != nil {
		cfgJSON, err = json.Marshal(config)
		if err != nil {
			return nil, fmt.Errorf("marshal config: %w", err)
		}
	}

	var ch AlertChannel
	var cfgOut []byte
	err = p.db.QueryRow(ctx, `
		UPDATE alert_channels
		SET
			name   = COALESCE($2, name),
			config = CASE WHEN $3::jsonb IS NOT NULL THEN $3::jsonb ELSE config END
		WHERE id = $1
		RETURNING id, org_id, kind, name, config, created_at
	`, id, name, cfgJSON).
		Scan(&ch.ID, &ch.OrgID, &ch.Kind, &ch.Name, &cfgOut, &ch.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("update alert channel: %w", err)
	}
	if err := json.Unmarshal(cfgOut, &ch.Config); err != nil {
		return nil, fmt.Errorf("parse alert channel config: %w", err)
	}
	return &ch, nil
}

// DeleteAlertChannel removes an alert channel by ID.
func (p *Pool) DeleteAlertChannel(ctx context.Context, id uuid.UUID) error {
	_, err := p.db.Exec(ctx, `DELETE FROM alert_channels WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete alert channel: %w", err)
	}
	return nil
}
