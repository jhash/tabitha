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
// (newest first) for /admin/tools' status view.
func RecentJobs(ctx context.Context, client *river.Client[pgx.Tx], limit int) ([]JobSummary, error) {
	result, err := client.JobList(ctx, river.NewJobListParams().
		Kinds("digest_song", "toc_sync").
		OrderBy(river.JobListOrderByID, river.SortOrderDesc).
		First(limit))
	if err != nil {
		return nil, fmt.Errorf("jobs: listing recent jobs: %w", err)
	}
	summaries := make([]JobSummary, len(result.Jobs))
	for i, row := range result.Jobs {
		summaries[i] = jobRowToSummary(row)
	}
	return summaries, nil
}
