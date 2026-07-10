// Package migrations embeds the SQL migration files so they ship inside the
// compiled binary — no separate migrations directory needs to exist on disk
// wherever tabitha is deployed.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
