package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Chain is a supported EVM chain.
type Chain struct {
	ID          uuid.UUID
	ChainID     int64
	Name        string
	NativeToken string
	BlockTimeMs int
	Enabled     bool
	CreatedAt   time.Time
}

// CreateChain inserts a new chain record.
func (p *Pool) CreateChain(ctx context.Context, chainID int64, name, nativeToken string, blockTimeMs int) (*Chain, error) {
	var c Chain
	err := p.db.QueryRow(ctx, `
		INSERT INTO chains (chain_id, name, native_token, block_time_ms)
		VALUES ($1, $2, $3, $4)
		RETURNING id, chain_id, name, native_token, block_time_ms, enabled, created_at
	`, chainID, name, nativeToken, blockTimeMs).
		Scan(&c.ID, &c.ChainID, &c.Name, &c.NativeToken, &c.BlockTimeMs, &c.Enabled, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create chain: %w", err)
	}
	return &c, nil
}
