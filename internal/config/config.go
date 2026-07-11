// Package config loads tabitha's runtime configuration from the environment.
package config

import (
	"errors"
	"os"
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
}

func Load() (Config, error) {
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
	}, nil
}
