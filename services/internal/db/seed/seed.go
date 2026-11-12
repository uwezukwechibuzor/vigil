// Package seed inserts demo data into a freshly migrated Vigil database.
package seed

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uwezukwechibuzor/vigil/services/internal/db/postgres"
)

// DefaultChains are the seed chains: Ethereum mainnet, Base, Arbitrum One.
var DefaultChains = []struct {
	ChainID     int64
	Name        string
	NativeToken string
	BlockTimeMs int
	RPCENV      string // env var that holds the public RPC URL
}{
	{1, "ethereum", "ETH", 12000, "SEED_ETH_RPC"},
	{8453, "base", "ETH", 2000, "SEED_BASE_RPC"},
	{42161, "arbitrum-one", "ETH", 250, "SEED_ARB_RPC"},
}

// Run inserts a demo org, project, API key, and the three canonical chains.
// The API key plaintext is printed to stdout once and never persisted.
func Run(ctx context.Context, pool *postgres.Pool, logger *slog.Logger) error {
	// Org
	org, err := pool.CreateOrg(ctx, "demo", "Demo Org", "free")
	if err != nil {
		return fmt.Errorf("seed org: %w", err)
	}
	logger.Info("seed: org created", "id", org.ID, "slug", org.Slug)

	// API key
	plaintext, key, err := pool.GenerateAPIKey(ctx, org.ID, nil, "demo-key", []string{"read", "write"}, nil)
	if err != nil {
		return fmt.Errorf("seed api key: %w", err)
	}
	logger.Info("seed: api key created", "prefix", key.KeyPrefix)
	fmt.Printf("\n  Demo API key (shown once): %s\n\n", plaintext)

	// Chains
	for _, c := range DefaultChains {
		if _, err := pool.CreateChain(ctx, c.ChainID, c.Name, c.NativeToken, c.BlockTimeMs); err != nil {
			logger.Warn("seed: chain already exists or error", "chain", c.Name, "error", err)
		} else {
			logger.Info("seed: chain created", "name", c.Name, "chain_id", c.ChainID)
		}
	}

	return nil
}
