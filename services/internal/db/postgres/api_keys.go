package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// APIKey represents a stored API key (hash only — plaintext never stored).
type APIKey struct {
	ID         uuid.UUID
	OrgID      uuid.UUID
	ProjectID  *uuid.UUID
	Name       string
	KeyPrefix  string
	KeyHash    string
	Scopes     []string
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

// GenerateAPIKey creates a new random API key, stores its hash, and returns the plaintext once.
func (p *Pool) GenerateAPIKey(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, name string, scopes []string, expiresAt *time.Time) (plaintext string, key *APIKey, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate key bytes: %w", err)
	}
	plaintext = "vgl_" + hex.EncodeToString(raw)
	prefix := plaintext[:12]
	hash := sha256.Sum256([]byte(plaintext))
	hashHex := hex.EncodeToString(hash[:])

	var k APIKey
	err = p.db.QueryRow(ctx, `
		INSERT INTO api_keys (org_id, project_id, name, key_prefix, key_hash, scopes, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, org_id, project_id, name, key_prefix, key_hash, scopes, expires_at, last_used_at, revoked_at, created_at
	`, orgID, projectID, name, prefix, hashHex, scopes, expiresAt).
		Scan(&k.ID, &k.OrgID, &k.ProjectID, &k.Name, &k.KeyPrefix, &k.KeyHash, &k.Scopes, &k.ExpiresAt, &k.LastUsedAt, &k.RevokedAt, &k.CreatedAt)
	if err != nil {
		return "", nil, fmt.Errorf("insert api key: %w", err)
	}
	return plaintext, &k, nil
}

// LookupAPIKey looks up an API key by its SHA-256 hash of the plaintext.
func (p *Pool) LookupAPIKey(ctx context.Context, plaintext string) (*APIKey, error) {
	hash := sha256.Sum256([]byte(plaintext))
	hashHex := hex.EncodeToString(hash[:])

	var k APIKey
	err := p.db.QueryRow(ctx, `
		SELECT id, org_id, project_id, name, key_prefix, key_hash, scopes, expires_at, last_used_at, revoked_at, created_at
		FROM api_keys
		WHERE key_hash = $1 AND revoked_at IS NULL
	`, hashHex).Scan(&k.ID, &k.OrgID, &k.ProjectID, &k.Name, &k.KeyPrefix, &k.KeyHash, &k.Scopes, &k.ExpiresAt, &k.LastUsedAt, &k.RevokedAt, &k.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("lookup api key: %w", err)
	}
	return &k, nil
}
