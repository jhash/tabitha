package db

// DB exposes the underlying DBTX for hand-rolled queries that sqlc can't
// express — dynamic ORDER BY/WHERE clauses, e.g. the home page's combined
// search+sort+filter query (see internal/web/song_query.go). Not generated,
// so sqlc regeneration never touches it.
func (q *Queries) DB() DBTX {
	return q.db
}
