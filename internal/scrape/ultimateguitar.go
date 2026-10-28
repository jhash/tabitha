package scrape

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
)

// UltimateGuitar scrapes tabs.ultimate-guitar.com "Chords" pages.
//
// Since ~2018 the page no longer lays out chords as individual DOM
// elements at all (the 2017 PhantomJS-era jhash/tabitha-chord-scraper
// walked <span> nodes inside .js-tab-content, which no longer exists) —
// instead the whole rendered page is a React app hydrated from one JSON
// blob embedded as the data-content attribute of <div class="js-store">.
// That JSON already contains the tab as plain text with lightweight
// [ch]/[tab] bracket markup (confirmed against a live page fetch), which
// is far easier to convert than a DOM walk would be. See convertBracketMarkup.
type UltimateGuitar struct {
	// HTTPClient is used for the fetch if set, otherwise http.DefaultClient.
	// Exposed for tests to point at an httptest.Server.
	HTTPClient *http.Client
}

func (p *UltimateGuitar) Name() string { return "ultimate_guitar_scrape" }
func (p *UltimateGuitar) Host() string { return "ultimate-guitar.com" }

func (p *UltimateGuitar) client() *http.Client {
	if p.HTTPClient != nil {
		return p.HTTPClient
	}
	return http.DefaultClient
}

func (p *UltimateGuitar) Fetch(ctx context.Context, rawURL string) (Song, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Song{}, fmt.Errorf("ultimate_guitar: building request: %w", err)
	}
	req.Header.Set("User-Agent", scrapeUserAgent)

	resp, err := p.client().Do(req)
	if err != nil {
		return Song{}, fmt.Errorf("ultimate_guitar: fetching %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Song{}, fmt.Errorf("ultimate_guitar: reading response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return Song{}, fmt.Errorf("ultimate_guitar: %s returned HTTP %d", rawURL, resp.StatusCode)
	}
	return ParseUltimateGuitarHTML(string(body))
}

// ugStoreRe extracts the JSON blob out of <div class="js-store"
// data-content="...">. Non-greedy up to the first "></div>" that follows
// the opening quote — the attribute value HTML-escapes its own quotes
// (&quot;), so it never contains a literal '"' that could end the match
// early.
var ugStoreRe = regexp.MustCompile(`class="js-store" data-content="(.*?)"></div>`)

// ugStore is the small slice of Ultimate Guitar's page-data JSON this
// package actually needs — the real blob has hundreds of unrelated
// fields (recommended tabs, ads, comments...), all ignored here.
type ugStore struct {
	Store struct {
		Page struct {
			Data struct {
				Tab struct {
					SongName     string `json:"song_name"`
					ArtistName   string `json:"artist_name"`
					Type         string `json:"type"`
					TonalityName string `json:"tonality_name"`
				} `json:"tab"`
				TabView struct {
					WikiTab struct {
						Content string `json:"content"`
					} `json:"wiki_tab"`
				} `json:"tab_view"`
			} `json:"data"`
		} `json:"page"`
	} `json:"store"`
}

// ParseUltimateGuitarHTML converts one already-fetched Ultimate Guitar
// page's HTML into a Song. Exported (and kept free of any network
// concern) so it's directly unit-testable against a saved fixture.
func ParseUltimateGuitarHTML(pageHTML string) (Song, error) {
	m := ugStoreRe.FindStringSubmatch(pageHTML)
	if m == nil {
		return Song{}, fmt.Errorf("ultimate_guitar: js-store data not found in page (layout may have changed)")
	}

	var store ugStore
	if err := json.Unmarshal([]byte(html.UnescapeString(m[1])), &store); err != nil {
		return Song{}, fmt.Errorf("ultimate_guitar: parsing js-store json: %w", err)
	}

	tab := store.Store.Page.Data.Tab
	if tab.Type != "" && tab.Type != "Chords" {
		return Song{}, fmt.Errorf("ultimate_guitar: page type %q isn't a chord chart (only \"Chords\" pages are supported, not Tabs/Pro/Bass)", tab.Type)
	}

	content := store.Store.Page.Data.TabView.WikiTab.Content
	if content == "" {
		return Song{}, fmt.Errorf("ultimate_guitar: page has no tab content")
	}

	return fromPlainText(tab.SongName, tab.ArtistName, tab.TonalityName, convertBracketMarkup(content)), nil
}
