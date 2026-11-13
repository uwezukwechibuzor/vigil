package postgres

import (
	"context"
	"fmt"
)

// ListChains returns all chains ordered by chain_id.
func (p *Pool) ListChains(ctx context.Context) ([]Chain, error) {
	rows, err := p.db.Query(ctx, `
		SELECT id, chain_id, name, native_token, block_time_ms, enabled, created_at
		FROM chains
		ORDER BY chain_id
	`)
	if err != nil {
		return nil, fmt.Errorf("list chains: %w", err)
	}
	defer rows.Close()

	var out []Chain
	for rows.Next() {
		var c Chain
		if err := rows.Scan(&c.ID, &c.ChainID, &c.Name, &c.NativeToken, &c.BlockTimeMs, &c.Enabled, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan chain: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetChainByEVMID returns the chain with the given EVM chain_id integer.
func (p *Pool) GetChainByEVMID(ctx context.Context, chainID int64) (*Chain, error) {
	var c Chain
	err := p.db.QueryRow(ctx, `
		SELECT id, chain_id, name, native_token, block_time_ms, enabled, created_at
		FROM chains
		WHERE chain_id = $1
	`, chainID).Scan(&c.ID, &c.ChainID, &c.Name, &c.NativeToken, &c.BlockTimeMs, &c.Enabled, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get chain by evm id: %w", err)
	}
	return &c, nil
}
