package jobs

import (
	"testing"
	"time"

	"github.com/riverqueue/river/rivertype"
)

func TestJobRowToSummaryExtractsSongIDForDigestSong(t *testing.T) {
	row := &rivertype.JobRow{
		ID:          7,
		Kind:        "digest_song",
		State:       rivertype.JobStateCompleted,
		Attempt:     1,
		EncodedArgs: []byte(`{"song_id": 132}`),
	}

	got := jobRowToSummary(row)

	if got.ID != 7 || got.Kind != "digest_song" || got.State != "completed" || got.Attempt != 1 {
		t.Errorf("got %+v, unexpected core fields", got)
	}
	if got.Detail != "song_id=132" {
		t.Errorf("Detail = %q, want %q", got.Detail, "song_id=132")
	}
	if got.LastError != "" {
		t.Errorf("LastError = %q, want empty (no errors)", got.LastError)
	}
}

func TestJobRowToSummaryHasEmptyDetailForTocSync(t *testing.T) {
	row := &rivertype.JobRow{
		ID:          8,
		Kind:        "toc_sync",
		State:       rivertype.JobStateCompleted,
		EncodedArgs: []byte(`{}`),
	}

	got := jobRowToSummary(row)
	if got.Detail != "" {
		t.Errorf("Detail = %q, want empty for toc_sync", got.Detail)
	}
}

func TestJobRowToSummaryUsesMostRecentError(t *testing.T) {
	now := time.Now()
	row := &rivertype.JobRow{
		ID:      9,
		Kind:    "digest_song",
		State:   rivertype.JobStateRetryable,
		Attempt: 2,
		Errors: []rivertype.AttemptError{
			{At: now.Add(-time.Minute), Attempt: 1, Error: "first failure"},
			{At: now, Attempt: 2, Error: "second failure"},
		},
		EncodedArgs: []byte(`{"song_id": 5}`),
	}

	got := jobRowToSummary(row)
	if got.LastError != "second failure" {
		t.Errorf("LastError = %q, want %q", got.LastError, "second failure")
	}
}
