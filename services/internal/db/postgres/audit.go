package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AuditEntry is one immutable audit log record.
type AuditEntry struct {
	OrgID      uuid.UUID
	UserID     *uuid.UUID
	APIKeyID   *uuid.UUID
	Action     string
	Resource   string
	ResourceID string
	IPAddress  string
	UserAgent  string
	Metadata   map[string]any
	CreatedAt  time.Time
}

// InsertAuditLog appends an audit entry.
func (p *Pool) InsertAuditLog(ctx context.Context, entry AuditEntry) error {
	meta := entry.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal audit metadata: %w", err)
	}
	_, err = p.db.Exec(ctx, `
		INSERT INTO audit_log (org_id, user_id, api_key_id, action, resource, resource_id, ip_address, user_agent, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, entry.OrgID, entry.UserID, entry.APIKeyID, entry.Action, entry.Resource, entry.ResourceID,
		entry.IPAddress, entry.UserAgent, metaJSON)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}
