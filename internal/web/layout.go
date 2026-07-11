// Package web holds tabitha's SSR HTML layer: gomponents-rendered pages,
// shared layout chrome, and route handlers.
package web

import (
	g "maragu.dev/gomponents"
	c "maragu.dev/gomponents/components"
	. "maragu.dev/gomponents/html"
)

const siteName = "tabitha"

// Page renders a full HTML5 document with tabitha's shared chrome: a
// self-hosted, preloaded Lora font (no CDN, no FOUT), the Roux-derived
// reset plus our stylesheet, self-hosted htmx with hx-boost enabled
// site-wide, and a plain header (with an /admin link for authenticated
// superadmins). sidebar is optional (nil renders none — most public
// pages don't have one).
func Page(title, description string, sidebar g.Node, isSuperadmin bool, body ...g.Node) g.Node {
	return page(title, description, sidebar, false, isSuperadmin, body...)
}

// PageWide is Page, but without the readable-prose max-width cap on the
// main content column — for pages built around a wide table rather than
// running text (the home page's songs table), so it can use as much of
// the window as .layout allows instead of wrapping/squeezing at ~42rem.
// The header's own container widens to match, so "tabitha" stays aligned
// with the content's left edge instead of sitting narrower than a wide
// table.
func PageWide(title, description string, sidebar g.Node, isSuperadmin bool, body ...g.Node) g.Node {
	return page(title, description, sidebar, true, isSuperadmin, body...)
}

func page(title, description string, sidebar g.Node, wide, isSuperadmin bool, body ...g.Node) g.Node {
	mainClass := "container"
	headerClass := "site-header-inner"
	if wide {
		mainClass += " container-wide"
		headerClass += " container-wide"
	}

	return c.HTML5(c.HTML5Props{
		Title:       title + " · " + siteName,
		Description: description,
		Language:    "en",
		HTMLAttrs:   g.Group{g.Attr("hx-boost", "true")},
		Head: g.Group{
			// charset and viewport meta tags are already inserted by
			// components.HTML5 itself — don't duplicate them here.
			Link(
				Rel("preload"),
				Href("/static/fonts/Lora-Variable.woff2"),
				As("font"),
				Type("font/woff2"),
				CrossOrigin("anonymous"),
			),
			Link(Rel("stylesheet"), Href("/static/css/reset.css")),
			Link(Rel("stylesheet"), Href("/static/css/style.css")),
		},
		Body: g.Group{
			Header(Class("site-header"),
				Div(Class(headerClass),
					A(Class("site-title"), Href("/"), g.Text(siteName)),
					g.If(isSuperadmin, A(Class("site-admin-link"), Href("/admin"), g.Text("Admin"))),
				),
			),
			layoutRow(sidebar, wide, mainClass, body...),
			Script(Src("/static/js/htmx.min.js")),
		},
	})
}

func layoutRow(sidebar g.Node, wide bool, mainClass string, body ...g.Node) g.Node {
	classes := "layout"
	if sidebar == nil {
		classes += " no-sidebar"
	}
	if wide {
		classes += " layout-wide"
	}

	children := make([]g.Node, 0, 2)
	if sidebar != nil {
		children = append(children, Div(Class("sidebar"), sidebar))
	}
	children = append(children, Main(Class(mainClass), g.Group(body)))

	return Div(append([]g.Node{Class(classes)}, children...)...)
}
