package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// User represents a user row.
type User struct {
	ID           uuid.UUID
	Email        string
	DisplayName  string
	AvatarURL    string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// CreateUser inserts a new user with a bcrypt password hash.
func (p *Pool) CreateUser(ctx context.Context, email, passwordHash, displayName string) (*User, error) {
	var u User
	err := p.db.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ($1, $2, $3)
		RETURNING id, email, password_hash, display_name, avatar_url, created_at, updated_at
	`, email, passwordHash, displayName).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}

// GetUserByEmail fetches a user by email.
func (p *Pool) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := p.db.QueryRow(ctx, `
		SELECT id, email, password_hash, display_name, avatar_url, created_at, updated_at
		FROM users WHERE email = $1
	`, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}
