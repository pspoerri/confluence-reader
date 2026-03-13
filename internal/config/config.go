package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	configDir  = "confluence-reader"
	configFile = "config.json"
)

// Config holds the Confluence connection settings.
type Config struct {
	BaseURL  string `json:"base_url"`
	Email    string `json:"email"`
	APIToken string `json:"api_token"`
}

// configPath returns ~/.config/confluence-reader/config.json.
func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".config", configDir, configFile), nil
}

// Load reads the config from ~/.config/confluence-reader/config.json.
func Load() (*Config, error) {
	p, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"config not found at %s\n\nRun 'confluence-reader configure' to set up your credentials.", p)
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", p, err)
	}

	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("config: base_url is required")
	}
	if cfg.Email == "" {
		return nil, fmt.Errorf("config: email is required")
	}
	if cfg.APIToken == "" {
		return nil, fmt.Errorf("config: api_token is required")
	}

	return &cfg, nil
}

// Save writes the config to ~/.config/confluence-reader/config.json.
// It creates the directory if needed.
func Save(cfg *Config) error {
	p, err := configPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(p, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// Path returns the config file path for display purposes.
func Path() (string, error) {
	return configPath()
}

// StateDir returns the cache/state directory ~/.config/confluence-reader/.
func StateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".config", configDir), nil
}
