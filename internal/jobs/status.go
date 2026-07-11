package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// JobSummary is a flattened, display-ready view of a river_job row for
// /admin/tools' status table — so Jake can see what's being digested and
// why something failed without going to psql.
type JobSummary struct {
	ID        int64
	Kind      string
	State     string
	Attempt   int
	Detail    string // e.g. "song_id=132" for digest_song, empty for toc_sync
	LastError string // empty if the job hasn't errored (yet)
}

func jobRowToSummary(row *rivertype.JobRow) JobSummary {
	summary := JobSummary{
		ID:      row.ID,
		Kind:    row.Kind,
		State:   string(row.State),
		Attempt: row.Attempt,
	}
	if row.Kind == "digest_song" {
		var args DigestSongArgs
		if err := json.Unmarshal(row.EncodedArgs, &args); err == nil {
			summary.Detail = fmt.Sprintf("song_id=%d", args.SongID)
		}
	}
	if len(row.Errors) > 0 {
		summary.LastError = row.Errors[len(row.Errors)-1].Error
	}
	return summary
}

// RecentJobs returns the most recent digest_song and toc_sync jobs
// (newest first) for /admin/tools' compact status view.
func RecentJobs(ctx context.Context, client *river.Client[pgx.Tx], limit int) ([]JobSummary, error) {
	page, err := RecentJobsPage(ctx, client, limit, "")
	if err != nil {
		return nil, err
	}
	return page.Jobs, nil
}

// JobPage is one page of RecentJobsPage's cursor-paginated job list.
// NextCursor is an opaque token for the next page's "after" cursor, empty
// when this is the last page.
type JobPage struct {
	Jobs       []JobSummary
	NextCursor string
}

// RecentJobsPage returns one cursor-paginated page of recent digest_song
// and toc_sync jobs (newest first), for /admin/jobs' full paginated view.
// afterCursor is an opaque token previously returned as some page's
// NextCursor, or "" for the first page.
func RecentJobsPage(ctx context.Context, client *river.Client[pgx.Tx], limit int, afterCursor string) (JobPage, error) {
	params := river.NewJobListParams().
		Kinds("digest_song", "toc_sync").
		OrderBy(river.JobListOrderByID, river.SortOrderDesc).
		First(limit)

	if afterCursor != "" {
		cursor := &river.JobListCursor{}
		if err := cursor.UnmarshalText([]byte(afterCursor)); err != nil {
			return JobPage{}, fmt.Errorf("jobs: invalid page cursor: %w", err)
		}
		params = params.After(cursor)
	}

	result, err := client.JobList(ctx, params)
	if err != nil {
		return JobPage{}, fmt.Errorf("jobs: listing recent jobs: %w", err)
	}

	summaries := make([]JobSummary, len(result.Jobs))
	for i, row := range result.Jobs {
		summaries[i] = jobRowToSummary(row)
	}

	var next string
	if len(result.Jobs) == limit && result.LastCursor != nil {
		if text, err := result.LastCursor.MarshalText(); err == nil {
			next = string(text)
		}
	}

	return JobPage{Jobs: summaries, NextCursor: next}, nil
}
