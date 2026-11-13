package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	vigil "github.com/uwezukwechibuzor/vigil/sdks/go"
)

func loginCmd() *cobra.Command {
	var baseURL string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with the Vigil API",
		Long:  "Prompt for an API key, validate it, and save it to ~/.config/vigil/config.json.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, _ = fmt.Fprint(cmd.OutOrStdout(), "API key: ")
			reader := bufio.NewReader(os.Stdin)
			key, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("read api key: %w", err)
			}
			key = strings.TrimSpace(key)
			if key == "" {
				return fmt.Errorf("api key cannot be empty")
			}

			// Validate by calling GET /v1/chains
			client := vigil.New(vigil.WithAPIKey(key), vigil.WithBaseURL(baseURL))
			_, valErr := client.ListChains(cmd.Context())
			if valErr != nil {
				return fmt.Errorf("invalid API key: %w", valErr)
			}

			cfg := Config{APIKey: key, BaseURL: baseURL}
			if err := saveConfig(cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Logged in. Config saved.")
			return nil
		},
	}
	cmd.Flags().StringVar(&baseURL, "base-url", "https://api.vigil.dev", "Vigil API base URL")
	return cmd
}
