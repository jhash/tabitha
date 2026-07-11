package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsDevLoginEnabledToFalse(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("DEV_LOGIN_ENABLED", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.DevLoginEnabled {
		t.Error("DevLoginEnabled = true, want false by default")
	}
}

func TestLoadEnablesDevLoginWhenSetToTrue(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("DEV_LOGIN_ENABLED", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if !cfg.DevLoginEnabled {
		t.Error("DevLoginEnabled = false, want true when DEV_LOGIN_ENABLED=true")
	}
}

func TestLoadReadsDotEnvFileFromCurrentDirectory(t *testing.T) {
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("APP_URL")
	t.Cleanup(func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("APP_URL")
	})

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("DATABASE_URL=postgres://from-dotenv\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.DatabaseURL != "postgres://from-dotenv" {
		t.Errorf("DatabaseURL = %q, want postgres://from-dotenv", cfg.DatabaseURL)
	}
}

func TestLoadDefaultsAppURLWhenUnset(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("APP_URL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.AppURL != "https://tabitha.jakehash.com" {
		t.Errorf("AppURL = %q, want default", cfg.AppURL)
	}
}

func TestLoadUsesProvidedAppURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("APP_URL", "http://localhost:8080")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.AppURL != "http://localhost:8080" {
		t.Errorf("AppURL = %q, want http://localhost:8080", cfg.AppURL)
	}
}

func TestLoadDefaultsPortWhenUnset(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want 8080", cfg.Port)
	}
}

func TestLoadReadsCloudflareCredentials(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("CLOUDFLARE_API_TOKEN", "test-token")
	t.Setenv("CLOUDFLARE_ZONE_ID", "test-zone")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.CloudflareAPIToken != "test-token" || cfg.CloudflareZoneID != "test-zone" {
		t.Errorf("CloudflareAPIToken/ZoneID = %q/%q, want test-token/test-zone", cfg.CloudflareAPIToken, cfg.CloudflareZoneID)
	}
}

func TestLoadErrorsWhenDatabaseURLMissing(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error when DATABASE_URL is unset, got nil")
	}
}
