package web

import (
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/jhash/tabitha/internal/db"
)

func TestBuildSitemapXMLIncludesHomeAndSongURLs(t *testing.T) {
	rows := []db.ListSongSlugsForSitemapRow{
		{Slug: "africa", UpdatedAt: pgtype.Timestamptz{Time: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), Valid: true}},
	}
	xml := buildSitemapXML("https://tabitha.jakehash.com", rows)

	if !strings.Contains(xml, "<loc>https://tabitha.jakehash.com/</loc>") {
		t.Errorf("expected the home page URL, got: %s", xml)
	}
	if !strings.Contains(xml, "<loc>https://tabitha.jakehash.com/songs/africa</loc>") {
		t.Errorf("expected the song URL, got: %s", xml)
	}
	if !strings.Contains(xml, "<lastmod>2026-01-02</lastmod>") {
		t.Errorf("expected the song's lastmod date, got: %s", xml)
	}
}

func TestBuildSitemapXMLIsValidXMLDeclaration(t *testing.T) {
	xml := buildSitemapXML("https://tabitha.jakehash.com", nil)
	if !strings.HasPrefix(xml, `<?xml version="1.0" encoding="UTF-8"?>`) {
		t.Errorf("expected an XML declaration, got: %s", xml)
	}
	if !strings.Contains(xml, `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`) {
		t.Errorf("expected the sitemap urlset namespace, got: %s", xml)
	}
}

func TestRobotsTxtPointsAtSitemap(t *testing.T) {
	got := buildRobotsTxt("https://tabitha.jakehash.com")
	if !strings.Contains(got, "Sitemap: https://tabitha.jakehash.com/sitemap.xml") {
		t.Errorf("expected a Sitemap directive, got: %s", got)
	}
	if !strings.Contains(got, "User-agent: *") || !strings.Contains(got, "Allow: /") {
		t.Errorf("expected a permissive robots policy, got: %s", got)
	}
}
