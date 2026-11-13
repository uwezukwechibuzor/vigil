package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	vigil "github.com/uwezukwechibuzor/vigil/sdks/go"
)

func runsCmd() *cobra.Command {
	var agentID, projectID string
	cmd := &cobra.Command{
		Use:   "runs",
		Short: "List agent runs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client := newClient()
			runs, err := client.ListRuns(cmd.Context(), vigil.RunListParams{
				AgentID:   agentID,
				ProjectID: projectID,
			})
			if err != nil {
				return fmt.Errorf("list runs: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(runs)
			}

			if len(runs) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No runs found.")
				return nil
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-36s  %-36s  %5s  %s\n", "RUN ID", "AGENT ID", "SPANS", "CREATED AT")
			for _, r := range runs {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-36s  %-36s  %5d  %s\n",
					r.RunID, r.AgentID, r.SpanCount, r.CreatedAt.Format("2006-01-02T15:04:05Z"))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&agentID, "agent-id", "", "Filter by agent ID")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Filter by project ID")
	return cmd
}
