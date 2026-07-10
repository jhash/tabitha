package config

import "testing"

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

func TestLoadErrorsWhenDatabaseURLMissing(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error when DATABASE_URL is unset, got nil")
	}
}
