// Package cloudflare calls Cloudflare's cache-purge API, so a song page
// or the home page gets invalidated automatically when its content
// changes, instead of relying on a manual dashboard purge.
package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const defaultBaseURL = "https://api.cloudflare.com/client/v4"

// Client purges Cloudflare's cache for a configured zone. Configured
// reports whether both APIToken and ZoneID are set — callers should skip
// purging entirely (not error) when it isn't, same pattern as
// auth.GoogleConfigured, so a dev environment without Cloudflare
// credentials doesn't break.
type Client struct {
	APIToken   string
	ZoneID     string
	HTTPClient *http.Client

	// baseURL overrides defaultBaseURL — empty in production, pointed at
	// an httptest.Server in tests.
	baseURL string
}

// Configured reports whether both credentials needed to call the API are
// present.
func (c *Client) Configured() bool {
	return c.APIToken != "" && c.ZoneID != ""
}

type purgeRequest struct {
	Files []string `json:"files"`
}

type purgeResponse struct {
	Success bool `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

// PurgeURLs invalidates Cloudflare's cached copies of the given URLs
// (exact matches, e.g. "https://tabitha.jakehash.com/songs/africa"). A nil
// or empty list is a no-op — no API call, no error.
func (c *Client) PurgeURLs(ctx context.Context, urls []string) error {
	if len(urls) == 0 {
		return nil
	}

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	baseURL := c.baseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	body, err := json.Marshal(purgeRequest{Files: urls})
	if err != nil {
		return fmt.Errorf("cloudflare: encoding purge request: %w", err)
	}

	url := fmt.Sprintf("%s/zones/%s/purge_cache", baseURL, c.ZoneID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("cloudflare: building purge request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare: purge request failed: %w", err)
	}
	defer resp.Body.Close()

	var parsed purgeResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return fmt.Errorf("cloudflare: decoding purge response: %w", err)
	}
	if !parsed.Success {
		return fmt.Errorf("cloudflare: purge failed (status %d): %+v", resp.StatusCode, parsed.Errors)
	}
	return nil
}
