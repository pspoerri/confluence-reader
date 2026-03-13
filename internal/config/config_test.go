package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_MissingFile(t *testing.T) {
	// Temporarily override HOME to a temp dir with no config.
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	dir := filepath.Join(tmp, ".config", "confluence-reader")
	os.MkdirAll(dir, 0o755)

	data := `{"base_url":"https://example.atlassian.net","email":"user@example.com","api_token":"secret-token"}`
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(data), 0o644)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseURL != "https://example.atlassian.net" {
		t.Errorf("unexpected base_url: %s", cfg.BaseURL)
	}
	if cfg.Email != "user@example.com" {
		t.Errorf("unexpected email: %s", cfg.Email)
	}
	if cfg.APIToken != "secret-token" {
		t.Errorf("unexpected api_token: %s", cfg.APIToken)
	}
}

func TestSave_CreatesAndWritesConfig(t *testing.T) {
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	cfg := &Config{
		BaseURL:  "https://test.atlassian.net",
		Email:    "test@example.com",
		APIToken: "test-token-123",
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("unexpected error saving config: %v", err)
	}

	// Verify the file was created with correct permissions.
	p := filepath.Join(tmp, ".config", "confluence-reader", "config.json")
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("config file not found after save: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("expected file permissions 0600, got %04o", perm)
	}

	// Verify the content round-trips through Load.
	loaded, err := Load()
	if err != nil {
		t.Fatalf("unexpected error loading saved config: %v", err)
	}
	if loaded.BaseURL != cfg.BaseURL {
		t.Errorf("base_url: got %q, want %q", loaded.BaseURL, cfg.BaseURL)
	}
	if loaded.Email != cfg.Email {
		t.Errorf("email: got %q, want %q", loaded.Email, cfg.Email)
	}
	if loaded.APIToken != cfg.APIToken {
		t.Errorf("api_token: got %q, want %q", loaded.APIToken, cfg.APIToken)
	}
}

func TestLoad_MissingFields(t *testing.T) {
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	dir := filepath.Join(tmp, ".config", "confluence-reader")
	os.MkdirAll(dir, 0o755)

	// Only base_url, missing email and api_token.
	data := `{"base_url":"https://example.atlassian.net"}`
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(data), 0o644)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing fields")
	}
}
