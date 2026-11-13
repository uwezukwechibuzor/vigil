package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds the CLI configuration.
type Config struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
}

// loadConfig reads ~/.config/vigil/config.json, returning empty Config if not found.
func loadConfig() Config {
	path := filepath.Join(configDir(), "config.json")
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return Config{}
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}
	}
	return cfg
}

// saveConfig writes the config to ~/.config/vigil/config.json.
func saveConfig(cfg Config) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.json"), data, 0o600)
}
