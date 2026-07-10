package web

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	g "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	"github.com/jhash/tabitha/internal/db"
)

// SongRow is the view-model for one row of the home page table.
type SongRow struct {
	ID           int64
	Title        string
	Artist       string
	Status       string
	AddedByName  string
	AddedByEmail string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

var sortColumns = []string{"title", "artist", "updated", "added", "status", "added_by"}

func isValidSort(sort string) bool {
	for _, c := range sortColumns {
		if c == sort {
			return true
		}
	}
	return false
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// HomeHandler renders the public table of contents: every song, searchable
// (fuzzy, ranked title > artist > genre), filterable by status/added-by,
// and sortable by any column with a click-to-toggle direction.
func HomeHandler(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := parseSongQueryParams(r.URL.Query())

		songs, err := ListSongsQuery(r.Context(), q.DB(), params)
		if err != nil {
			http.Error(w, "failed to load songs", http.StatusInternalServerError)
			return
		}
		statuses, err := q.ListDistinctStatuses(r.Context())
		if err != nil {
			http.Error(w, "failed to load statuses", http.StatusInternalServerError)
			return
		}
		addedByUsers, err := q.ListDistinctAddedByUsers(r.Context())
		if err != nil {
			http.Error(w, "failed to load added-by list", http.StatusInternalServerError)
			return
		}

		page := Page("Songs", "Jeff's music transcription catalog", nil, homeContent(songs, params, statuses, addedByUsers))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = page.Render(w)
	}
}

func parseSongQueryParams(q url.Values) SongQueryParams {
	sort := q.Get("sort")
	if sort != "" && !isValidSort(sort) {
		sort = ""
	}
	order := q.Get("order")
	if !isValidOrder(order) {
		order = ""
	}
	return SongQueryParams{
		Search:  q.Get("search"),
		Sort:    sort,
		Order:   order,
		Status:  q.Get("status"),
		AddedBy: q.Get("added_by"),
	}
}

// effectiveSort/effectiveOrder resolve what's actually driving the current
// result order, including the implicit defaults (relevance when
// searching, title otherwise; asc when unspecified) — used so sort
// headers know which one (if any) is "active" and which arrow to show.
func effectiveSort(p SongQueryParams) string {
	if p.Sort != "" {
		return p.Sort
	}
	if p.Search != "" {
		return "relevance"
	}
	return "title"
}

func effectiveOrder(p SongQueryParams) string {
	if isValidOrder(p.Order) {
		return p.Order
	}
	return "asc"
}

// withSort returns the query string for a header link: current
// search/status/added-by preserved, sort set to column, order toggled if
// column is already the active sort.
func withSort(p SongQueryParams, column string) string {
	order := "asc"
	if effectiveSort(p) == column && effectiveOrder(p) == "asc" {
		order = "desc"
	}
	v := url.Values{}
	if p.Search != "" {
		v.Set("search", p.Search)
	}
	if p.Status != "" {
		v.Set("status", p.Status)
	}
	if p.AddedBy != "" {
		v.Set("added_by", p.AddedBy)
	}
	v.Set("sort", column)
	v.Set("order", order)
	return "/?" + v.Encode()
}

func homeContent(songs []SongRow, params SongQueryParams, statuses []string, addedByUsers []db.ListDistinctAddedByUsersRow) g.Node {
	return Div(
		H1(g.Text("Songs")),
		searchAndFilterForm(params, statuses, addedByUsers),
		homeTable(songs, params),
	)
}

func searchAndFilterForm(params SongQueryParams, statuses []string, addedByUsers []db.ListDistinctAddedByUsersRow) g.Node {
	return FormEl(Method("get"), Action("/"),
		g.Attr("hx-get", "/"), g.Attr("hx-target", "body"), g.Attr("hx-push-url", "true"),
		g.Attr("hx-trigger", "submit, change, keyup changed delay:300ms from:input[type=search]"),
		Input(Type("search"), Name("search"), Value(params.Search), Placeholder("Search title, artist, genre…")),
		Select(Name("status"),
			Option(Value(""), g.Text("Any status")),
			g.Map(statuses, func(s string) g.Node {
				return Option(Value(s), g.If(s == params.Status, Selected()), g.Text(s))
			}),
		),
		Select(Name("added_by"),
			Option(Value(""), g.Text("Anyone")),
			g.Map(addedByUsers, func(u db.ListDistinctAddedByUsersRow) g.Node {
				label := u.Name
				if label == "" {
					label = u.Email
				}
				return Option(Value(label), g.If(label == params.AddedBy, Selected()), g.Text(label))
			}),
		),
		Input(Type("hidden"), Name("sort"), Value(params.Sort)),
		Input(Type("hidden"), Name("order"), Value(params.Order)),
		Button(Type("submit"), g.Text("Search")),
	)
}

func homeTable(songs []SongRow, params SongQueryParams) g.Node {
	return Table(
		THead(
			Tr(
				sortHeader("Title", "title", params),
				sortHeader("Artist", "artist", params),
				sortHeader("Status", "status", params),
				sortHeader("Last Updated", "updated", params),
				sortHeader("Added", "added", params),
				sortHeader("Added By", "added_by", params),
			),
		),
		TBody(
			g.Map(songs, func(s SongRow) g.Node {
				return Tr(
					Td(A(Href(fmt.Sprintf("/songs/%d", s.ID)), g.Text(s.Title))),
					Td(g.Text(s.Artist)),
					Td(g.Text(s.Status)),
					Td(g.Text(formatDate(s.UpdatedAt))),
					Td(g.Text(formatDate(s.CreatedAt))),
					Td(g.Text(addedByLabel(s))),
				)
			}),
		),
	)
}

func addedByLabel(s SongRow) string {
	if s.AddedByName != "" {
		return s.AddedByName
	}
	return s.AddedByEmail
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func sortHeader(label, column string, params SongQueryParams) g.Node {
	href := withSort(params, column)
	active := effectiveSort(params) == column
	var arrow string
	if active {
		if effectiveOrder(params) == "desc" {
			arrow = " ▾"
		} else {
			arrow = " ▴"
		}
	}
	return Th(A(Href(href), g.Attr("hx-get", href), g.Attr("hx-target", "body"), g.Attr("hx-push-url", "true"),
		g.Text(label),
		g.Text(arrow),
	))
}
