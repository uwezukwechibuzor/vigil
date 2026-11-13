package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var chainNameToID = map[string]int64{
	"ethereum": 1,
	"base":     8453,
	"arbitrum": 42161,
	"optimism": 10,
	"polygon":  137,
}

func healthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health <chain>",
		Short: "Show chain health",
		Long: `Show the health of an EVM chain.

<chain> can be a chain ID (e.g. 8453) or a name (ethereum, base, arbitrum, optimism, polygon).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chainArg := strings.ToLower(strings.TrimSpace(args[0]))

			var chainID int64
			if id, ok := chainNameToID[chainArg]; ok {
				chainID = id
			} else {
				n, err := strconv.ParseInt(chainArg, 10, 64)
				if err != nil {
					return fmt.Errorf("unknown chain %q: provide a chain ID or one of: ethereum, base, arbitrum", chainArg)
				}
				chainID = n
			}

			client := newClient()
			health, err := client.ChainHealth(cmd.Context(), chainID)
			if err != nil {
				return fmt.Errorf("chain health: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(health)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "status:      %s\n", health.Status)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "head_lag:    %d blocks\n", health.HeadLagBlocks)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "reorgs_24h:  %d\n", health.Reorgs24h)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "fee_level:   %s\n", health.FeeLevel)
			if len(health.WorstEndpoints) > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "worst_eps:   %s\n", strings.Join(health.WorstEndpoints, ", "))
			}
			return nil
		},
	}
}
