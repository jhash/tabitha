// Package web holds tabitha's SSR HTML layer: gomponents-rendered pages,
// shared layout chrome, and route handlers.
package web

import (
	"strings"

	g "maragu.dev/gomponents"
	c "maragu.dev/gomponents/components"
	. "maragu.dev/gomponents/html"
)

const siteName = "tabitha"

// Page renders a full HTML5 document with tabitha's shared chrome: a
// self-hosted, preloaded Lora font (no CDN, no FOUT), the Roux-derived
// reset plus our stylesheet, self-hosted htmx with hx-boost enabled
// site-wide, and a plain header. sidebar is optional (nil renders none —
// most public pages don't have one).
func Page(title, description string, sidebar g.Node, body ...g.Node) g.Node {
	return page(title, description, sidebar, "container", body...)
}

// PageWide is Page, but without the readable-prose max-width cap on the
// main content column — for pages built around a wide table rather than
// running text (the home page's songs table), so it can use as much of
// the window as .layout allows instead of wrapping/squeezing at ~42rem.
func PageWide(title, description string, sidebar g.Node, body ...g.Node) g.Node {
	return page(title, description, sidebar, "container container-wide", body...)
}

func page(title, description string, sidebar g.Node, mainClass string, body ...g.Node) g.Node {
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
				Div(Class("site-header-inner"),
					A(Class("site-title"), Href("/"), g.Text(siteName)),
				),
			),
			layoutRow(sidebar, mainClass, body...),
			Script(Src("/static/js/htmx.min.js")),
		},
	})
}

func layoutRow(sidebar g.Node, mainClass string, body ...g.Node) g.Node {
	classes := "layout"
	if sidebar == nil {
		classes += " no-sidebar"
	}
	if strings.Contains(mainClass, "container-wide") {
		classes += " layout-wide"
	}

	children := make([]g.Node, 0, 2)
	if sidebar != nil {
		children = append(children, Div(Class("sidebar"), sidebar))
	}
	children = append(children, Main(Class(mainClass), g.Group(body)))

	return Div(append([]g.Node{Class(classes)}, children...)...)
}
