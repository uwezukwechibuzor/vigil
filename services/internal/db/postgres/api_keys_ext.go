package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// ListAPIKeys returns all non-revoked API keys for an org.
func (p *Pool) ListAPIKeys(ctx context.Context, orgID uuid.UUID) ([]APIKey, error) {
	rows, err := p.db.Query(ctx, `
		SELECT id, org_id, project_id, name, key_prefix, key_hash, scopes, expires_at, last_used_at, revoked_at, created_at
		FROM api_keys
		WHERE org_id = $1
		ORDER BY created_at DESC
	`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var out []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.OrgID, &k.ProjectID, &k.Name, &k.KeyPrefix, &k.KeyHash,
			&k.Scopes, &k.ExpiresAt, &k.LastUsedAt, &k.RevokedAt, &k.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// RevokeAPIKey sets revoked_at to now for the given key.
func (p *Pool) RevokeAPIKey(ctx context.Context, id uuid.UUID) error {
	_, err := p.db.Exec(ctx, `
		UPDATE api_keys SET revoked_at = now() WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	return nil
}
