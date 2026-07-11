// Package config loads tabitha's runtime configuration from the environment.
package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL        string
	AppURL             string
	Port               string
	TokenEncryptionKey string
	GoogleKey          string
	GoogleSecret       string
	NtfyURL            string
	SessionSecret      string
	CloudflareAPIToken string
	CloudflareZoneID   string

	// DevLoginEnabled mounts /dev-login, which mints a superadmin session
	// with no auth check — only ever for local/CI e2e tests
	// (DEV_LOGIN_ENABLED=true), never in production.
	DevLoginEnabled bool
}

func Load() (Config, error) {
	// Best-effort: fine if .env doesn't exist (prod sets real env vars).
	// godotenv never overrides vars already set in the environment.
	_ = godotenv.Load()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return Config{}, errors.New("config: DATABASE_URL is required")
	}

	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		appURL = "https://tabitha.jakehash.com"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return Config{
		DatabaseURL:        databaseURL,
		AppURL:             appURL,
		Port:               port,
		TokenEncryptionKey: os.Getenv("TOKEN_ENCRYPTION_KEY"),
		GoogleKey:          os.Getenv("GOOGLE_KEY"),
		GoogleSecret:       os.Getenv("GOOGLE_SECRET"),
		NtfyURL:            os.Getenv("NTFY_URL"),
		SessionSecret:      os.Getenv("SESSION_SECRET"),
		CloudflareAPIToken: os.Getenv("CLOUDFLARE_API_TOKEN"),
		CloudflareZoneID:   os.Getenv("CLOUDFLARE_ZONE_ID"),
		DevLoginEnabled:    os.Getenv("DEV_LOGIN_ENABLED") == "true",
	}, nil
}
