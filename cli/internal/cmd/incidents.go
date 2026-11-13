package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	vigil "github.com/uwezukwechibuzor/vigil/sdks/go"
)

func incidentsCmd() *cobra.Command {
	var projectID, status string
	cmd := &cobra.Command{
		Use:   "incidents",
		Short: "List incidents",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client := newClient()
			incidents, err := client.ListIncidents(cmd.Context(), vigil.IncidentListParams{
				ProjectID: projectID,
				Status:    status,
			})
			if err != nil {
				return fmt.Errorf("list incidents: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(incidents)
			}

			if len(incidents) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No incidents found.")
				return nil
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-36s  %-12s  %-8s  %s\n", "ID", "STATUS", "SEVERITY", "TITLE")
			for _, inc := range incidents {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-36s  %-12s  %-8s  %s\n",
					inc.ID, inc.Status, inc.Severity, inc.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project-id", "", "Filter by project ID")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status (open|acknowledged|resolved)")
	return cmd
}
