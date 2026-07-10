package web

import (
	"context"
	"fmt"
	"strings"

	"github.com/jhash/tabitha/internal/db"
)

// SongQueryParams describes the home page's combined search+sort+filter
// request. Sort/Order are validated against allowlists before ever
// reaching SQL — never string-interpolated from raw user input.
type SongQueryParams struct {
	Search  string
	Sort    string // one of sortColumns, or "" for the default
	Order   string // "asc" or "desc"
	Status  string
	AddedBy string // matches a user's name or email, case-insensitively
}

// sortColumnExprs maps a validated sort key to its literal ORDER BY
// expression. Never built from user input directly — isValidSort gates
// which keys are even looked up here.
var sortColumnExprs = map[string]string{
	"title":    "lower(title)",
	"artist":   "lower(artist)",
	"status":   "lower(status)",
	"updated":  "updated_at",
	"added":    "created_at",
	"added_by": "lower(coalesce(added_by_name, added_by_email, ''))",
}

func isValidOrder(order string) bool {
	return order == "asc" || order == "desc"
}

// buildSongsQuery constructs the home page's search+sort+filter SQL.
// Fuzzy search (pg_trgm similarity) scores title highest, then artist,
// then genres — and becomes the default sort whenever a search term is
// present, overriding any explicit column sort. Status/added-by filters
// compose with either sort mode. Pure and unit-testable: no DB access, no
// string-interpolated user input (only $-placeholders and an allowlisted
// literal ORDER BY expression).
func buildSongsQuery(p SongQueryParams) (string, []any) {
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	search := strings.TrimSpace(p.Search)
	status := strings.TrimSpace(p.Status)
	addedBy := strings.TrimSpace(p.AddedBy)

	statusParam := arg(status)
	addedByParam := arg(addedBy)

	var orderBy string
	if search != "" && p.Sort == "" {
		searchParam := arg(search)
		orderBy = fmt.Sprintf(
			"(greatest(similarity(title, %[1]s), 0) * 3 + greatest(similarity(artist, %[1]s), 0) * 2 + greatest(similarity(genres, %[1]s), 0)) DESC, lower(title) ASC",
			searchParam,
		)
	} else {
		col, ok := sortColumnExprs[p.Sort]
		if !ok {
			col = sortColumnExprs["title"]
		}
		order := "ASC"
		if isValidOrder(p.Order) {
			order = strings.ToUpper(p.Order)
		}
		orderBy = col + " " + order
	}

	query := fmt.Sprintf(`
		SELECT * FROM (
			SELECT
				s.id, s.title, s.artist, s.status,
				u.name AS added_by_name, u.email AS added_by_email,
				s.created_at, s.updated_at,
				coalesce(string_agg(DISTINCT g.name, ', '), '') AS genres
			FROM songs s
			LEFT JOIN users u ON u.id = s.added_by_user_id
			LEFT JOIN song_genres sg ON sg.song_id = s.id
			LEFT JOIN genres g ON g.id = sg.genre_id
			WHERE (%[1]s = '' OR s.status = %[1]s)
			  AND (%[2]s = '' OR lower(u.name) = lower(%[2]s) OR lower(u.email) = lower(%[2]s))
			GROUP BY s.id, u.name, u.email
		) song_data
		ORDER BY %[3]s
	`, statusParam, addedByParam, orderBy)

	return query, args
}

// ListSongsQuery runs the home page's combined search+sort+filter query.
func ListSongsQuery(ctx context.Context, dbtx db.DBTX, p SongQueryParams) ([]SongRow, error) {
	query, args := buildSongsQuery(p)
	rows, err := dbtx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying songs: %w", err)
	}
	defer rows.Close()

	var out []SongRow
	for rows.Next() {
		var s SongRow
		var addedByName, addedByEmail, genres *string
		if err := rows.Scan(&s.ID, &s.Title, &s.Artist, &s.Status, &addedByName, &addedByEmail, &s.CreatedAt, &s.UpdatedAt, &genres); err != nil {
			return nil, fmt.Errorf("scanning song row: %w", err)
		}
		s.AddedByName = deref(addedByName)
		s.AddedByEmail = deref(addedByEmail)
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating song rows: %w", err)
	}
	return out, nil
}
