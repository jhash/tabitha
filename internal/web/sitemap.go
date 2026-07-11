package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/jhash/tabitha/internal/db"
)

// SitemapHandler serves /sitemap.xml: the home page plus every slugged
// song. appURL is the site's canonical origin (e.g.
// "https://tabitha.jakehash.com"), with no trailing slash.
func SitemapHandler(q *db.Queries, appURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := q.ListSongSlugsForSitemap(r.Context())
		if err != nil {
			http.Error(w, "failed to build sitemap", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		_, _ = w.Write([]byte(buildSitemapXML(appURL, rows)))
	}
}

func buildSitemapXML(appURL string, rows []db.ListSongSlugsForSitemapRow) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	b.WriteString(fmt.Sprintf("  <url><loc>%s/</loc></url>\n", appURL))
	for _, row := range rows {
		b.WriteString(fmt.Sprintf("  <url><loc>%s/songs/%s</loc>", appURL, row.Slug))
		if row.UpdatedAt.Valid {
			b.WriteString(fmt.Sprintf("<lastmod>%s</lastmod>", row.UpdatedAt.Time.Format("2006-01-02")))
		}
		b.WriteString("</url>\n")
	}
	b.WriteString("</urlset>\n")
	return b.String()
}

// RobotsTxtHandler serves /robots.txt: fully permissive, pointing at the
// sitemap so crawlers discover every song page without needing to follow
// links from the home page's default (hidden-undigested) view.
func RobotsTxtHandler(appURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(buildRobotsTxt(appURL)))
	}
}

func buildRobotsTxt(appURL string) string {
	return fmt.Sprintf("User-agent: *\nAllow: /\n\nSitemap: %s/sitemap.xml\n", appURL)
}
