// Package jobs wires up River, tabitha's Postgres-backed job queue, and
// defines its jobs (toc_sync, digest_song).
package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"golang.org/x/time/rate"

	"github.com/jhash/tabitha/internal/config"
	"github.com/jhash/tabitha/internal/db"
)

// digestSongRateLimit keeps combined Sheets+Docs API calls comfortably
// under Google's default 60-reads/minute-per-user quota (confirmed hit
// running a 50-song batch through /admin/tools), shared across every
// digest_song job this process runs rather than per-job.
const digestSongRateLimit = 45

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
// encryptionKey may be nil wherever DigestSongWorker is never exercised
// (e.g. `jobs enqueue toc-sync`, which only ever touches TocSyncWorker).
func NewClient(pool *pgxpool.Pool, queries *db.Queries, cfg config.Config, encryptionKey []byte) (*river.Client[pgx.Tx], error) {
	rateLimiter := rate.NewLimiter(rate.Every(time.Minute/digestSongRateLimit), 1)

	workers := river.NewWorkers()
	river.AddWorker(workers, &TocSyncWorker{Queries: queries})
	river.AddWorker(workers, &DigestSongWorker{Queries: queries, Config: cfg, EncryptionKey: encryptionKey, RateLimiter: rateLimiter})

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
// (`tabitha jobs enqueue toc-sync`) and /admin/tools.
func EnqueueTocSync(ctx context.Context, client *river.Client[pgx.Tx]) error {
	_, err := client.Insert(ctx, TocSyncArgs{}, nil)
	return err
}

// EnqueueDigestSong inserts a digest_song job for one song. Used by
// /admin/tools' "digest by title" trigger.
func EnqueueDigestSong(ctx context.Context, client *river.Client[pgx.Tx], songID int64) error {
	_, err := client.Insert(ctx, DigestSongArgs{SongID: songID}, nil)
	return err
}

// EnqueueDigestSongsForUndigested inserts a digest_song job for each of the
// oldest (by id) undigested songs, up to limit. Used by /admin/tools'
// batch trigger to derisk the full 1,925-song catalog on a small,
// checkable slice first rather than running everything at once.
func EnqueueDigestSongsForUndigested(ctx context.Context, client *river.Client[pgx.Tx], q *db.Queries, limit int32) (int, error) {
	ids, err := q.ListSongIDsWithoutCurrentVersion(ctx, limit)
	if err != nil {
		return 0, fmt.Errorf("jobs: listing undigested songs: %w", err)
	}
	for _, id := range ids {
		if _, err := client.Insert(ctx, DigestSongArgs{SongID: id}, nil); err != nil {
			return 0, fmt.Errorf("jobs: enqueuing digest_song for song %d: %w", id, err)
		}
	}
	return len(ids), nil
}
