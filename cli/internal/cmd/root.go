// Package cmd contains all CLI subcommands.
package cmd

import (
	"os"

	"github.com/spf13/cobra"
	vigil "github.com/uwezukwechibuzor/vigil/sdks/go"
)

var jsonOutput bool

// Root returns the root cobra command.
func Root() *cobra.Command {
	root := &cobra.Command{
		Use:   "vigil",
		Short: "Vigil — reliability layer for onchain AI agents",
		Long: `vigil is the CLI for the Vigil reliability API.

Use it to run flight checks, monitor chain health, inspect agent runs, and manage incidents.`,
	}
	root.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	root.AddCommand(
		loginCmd(),
		checkCmd(),
		healthCmd(),
		runsCmd(),
		incidentsCmd(),
	)
	return root
}

// newClient creates a Vigil API client from env or config.
func newClient() *vigil.Client {
	cfg := loadConfig()
	opts := []vigil.Option{}
	if cfg.APIKey != "" {
		opts = append(opts, vigil.WithAPIKey(cfg.APIKey))
	}
	if cfg.BaseURL != "" {
		opts = append(opts, vigil.WithBaseURL(cfg.BaseURL))
	}
	return vigil.New(opts...)
}

// configDir returns the directory for the CLI config file.
func configDir() string {
	dir := os.Getenv("VIGIL_CONFIG_DIR")
	if dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".vigil"
	}
	return home + "/.config/vigil"
}
