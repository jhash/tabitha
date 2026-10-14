package scrape

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	xhtml "golang.org/x/net/html"
)

// EChords scrapes e-chords.com chord-chart pages.
//
// e-chords sits behind Cloudflare's bot-challenge ("Just a moment...",
// an interstitial Turnstile page) — confirmed live: every plain HTTP
// request to e-chords.com/chords/... gets HTTP 403 with that challenge
// page, even with full browser-shaped headers (User-Agent,
// Accept-Language, Referer). A plain net/http.Get can't get past this;
// it needs a real browser's JS execution. Fetch therefore fails fast
// with ErrBlockedByCloudflare rather than pretending to succeed — see
// its docs for the workaround.
//
// The actual chord layout this converts (a #core element with each
// chord symbol wrapped in a <u> tag, plain text everything else) is
// carried over from tabitha's 2017 PhantomJS scraper
// (jhash/tabitha-chord-scraper's scripts/echords.js), which already
// reverse-engineered it — this reimplements that same DOM contract
// against Go's html tree instead of PhantomJS/jQuery.
type EChords struct {
	HTTPClient *http.Client
}

func (p *EChords) Name() string { return "echords_scrape" }
func (p *EChords) Host() string { return "e-chords.com" }

func (p *EChords) client() *http.Client {
	if p.HTTPClient != nil {
		return p.HTTPClient
	}
	return http.DefaultClient
}

// ErrBlockedByCloudflare means the response was e-chords.com's
// bot-challenge interstitial rather than the real chord chart. Fetch a
// copy of the fully-rendered page HTML through an actual browser instead
// (e.g. the Chrome DevTools "Copy outerHTML" on <html>, or a browser
// automation tool that renders JS) and pass it directly to
// ParseEChordsHTML — no network fetch needed at that point.
var ErrBlockedByCloudflare = errors.New("scrape: e-chords.com returned its Cloudflare bot-challenge page instead of chord content; fetch the page through a real browser and use ParseEChordsHTML directly")

func (p *EChords) Fetch(ctx context.Context, rawURL string) (Song, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Song{}, fmt.Errorf("echords: building request: %w", err)
	}
	req.Header.Set("User-Agent", scrapeUserAgent)

	resp, err := p.client().Do(req)
	if err != nil {
		return Song{}, fmt.Errorf("echords: fetching %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Song{}, fmt.Errorf("echords: reading response body: %w", err)
	}
	if resp.StatusCode == http.StatusForbidden || strings.Contains(string(body), "Just a moment") {
		return Song{}, ErrBlockedByCloudflare
	}
	if resp.StatusCode != http.StatusOK {
		return Song{}, fmt.Errorf("echords: %s returned HTTP %d", rawURL, resp.StatusCode)
	}
	return ParseEChordsHTML(string(body))
}

// ParseEChordsHTML converts one already-fetched e-chords.com page's HTML
// into a Song. Exported (and kept free of any network concern) so a page
// captured through a real browser (see ErrBlockedByCloudflare) can be
// converted without a fetch, and so it's directly unit-testable against
// a saved fixture.
func ParseEChordsHTML(pageHTML string) (Song, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(pageHTML))
	if err != nil {
		return Song{}, fmt.Errorf("echords: parsing html: %w", err)
	}

	core := doc.Find("#core")
	if core.Length() == 0 {
		return Song{}, fmt.Errorf("echords: no #core element found (layout may have changed)")
	}

	text := walkChordSheet(core.Get(0))
	if strings.TrimSpace(text) == "" {
		return Song{}, fmt.Errorf("echords: #core element had no content")
	}

	title := strings.TrimSpace(doc.Find("h1").First().Text())
	artist := strings.TrimSpace(doc.Find("h2 a, .art_title a").First().Text())

	return fromPlainText(title, artist, "", convertBracketMarkup(text)), nil
}

// walkChordSheet flattens #core's DOM into plain text, marking each <u>
// (e-chords' chord symbol wrapper) with the same [ch]/[/ch] bracket
// markup Ultimate Guitar uses, so both providers share one downstream
// converter (convertBracketMarkup).
func walkChordSheet(n *xhtml.Node) string {
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		switch n.Type {
		case xhtml.TextNode:
			b.WriteString(n.Data)
			return
		case xhtml.ElementNode:
			switch n.Data {
			case "u":
				b.WriteString("[ch]")
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
				b.WriteString("[/ch]")
				return
			case "br":
				b.WriteString("\n")
				return
			case "script", "style":
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}
