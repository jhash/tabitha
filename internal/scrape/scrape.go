// Package scrape converts chord-chart pages from external sites
// (Ultimate Guitar, e-chords) into tabitha's own chord-over-lyric
// plaintext, ready to hand to transcription.Parse/MarshalDocument and
// store as a transcription_versions row — the same shape digest_song
// produces from a Google Doc, just fed from a different source.
package scrape

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/jhash/tabitha/internal/transcription"
)

// scrapeUserAgent identifies tabitha to the sites it scrapes, per common
// scraping etiquette, rather than posing as a browser.
const scrapeUserAgent = "tabitha-scraper/1.0 (+https://github.com/jhash/tabitha; personal chord-archive tool)"

// Song is one scraped chord chart, already converted into tabitha's own
// format.
type Song struct {
	Title  string
	Artist string
	// Key is the song's key/tonality if the source page states one
	// (empty otherwise). Maps to transcription_versions.key.
	Key string
	// RawText is the plaintext chord-over-lyric chart, in the exact
	// format internal/transcription.Parse expects — this is what gets
	// stored as transcription_versions.raw_text.
	RawText string
	// Blocks is RawText already parsed, for callers that want the
	// structured form without parsing it themselves.
	Blocks []transcription.Block
}

// Provider fetches and converts one external chord-chart site's page
// format into a Song.
type Provider interface {
	// Name identifies the provider for transcription_versions.source,
	// e.g. "ultimate_guitar_scrape".
	Name() string
	// Host is the domain this provider handles, with any "www." or
	// "tabs."-style subdomain stripped (matched against the request
	// URL's own hostname, stripped the same way — see hostFor).
	Host() string
	Fetch(ctx context.Context, rawURL string) (Song, error)
}

var providers = []Provider{
	&UltimateGuitar{},
	&EChords{},
}

// ErrUnsupportedSite means rawURL's host doesn't match any registered
// provider.
var ErrUnsupportedSite = errors.New("scrape: no provider registered for this URL's host")

// ForURL finds the provider that handles rawURL's host.
func ForURL(rawURL string) (Provider, error) {
	host, err := hostFor(rawURL)
	if err != nil {
		return nil, err
	}
	for _, p := range providers {
		if p.Host() == host {
			return p, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrUnsupportedSite, host)
}

// knownSubdomains are stripped from a URL's hostname before matching it
// against a Provider.Host() — both plain and tab-subdomain URLs
// (tabs.ultimate-guitar.com vs. www.ultimate-guitar.com) point at the
// same site.
var knownSubdomains = []string{"www.", "tabs."}

// hostFor normalizes rawURL's hostname for provider matching.
func hostFor(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("scrape: parsing url %q: %w", rawURL, err)
	}
	host := strings.ToLower(u.Hostname())
	for _, prefix := range knownSubdomains {
		host = strings.TrimPrefix(host, prefix)
	}
	return host, nil
}
