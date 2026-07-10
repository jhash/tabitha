// Package jobs wires up River, tabitha's Postgres-backed job queue, and
// defines its jobs (toc_sync, digest_song).
package jobs

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/jhash/tabitha/internal/db"
)

// MigrateUp applies River's own schema migrations (its job/queue tables),
// separate from tabitha's application schema in the top-level migrations
// package.
func MigrateUp(ctx context.Context, pool *pgxpool.Pool) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("jobs: creating river migrator: %w", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("jobs: running river migrations: %w", err)
	}
	return nil
}

// NewClient builds a River client with all of tabitha's workers registered.
func NewClient(pool *pgxpool.Pool, queries *db.Queries) (*river.Client[pgx.Tx], error) {
	workers := river.NewWorkers()
	river.AddWorker(workers, &TocSyncWorker{Queries: queries})
	river.AddWorker(workers, &DigestSongWorker{Queries: queries})

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 5},
		},
		Workers: workers,
	})
	if err != nil {
		return nil, fmt.Errorf("jobs: creating river client: %w", err)
	}
	return client, nil
}

// EnqueueTocSync inserts a toc_sync job. Used by both the CLI
// (`tabitha jobs enqueue toc-sync`) and, later, /admin/tools.
func EnqueueTocSync(ctx context.Context, client *river.Client[pgx.Tx]) error {
	_, err := client.Insert(ctx, TocSyncArgs{}, nil)
	return err
}
