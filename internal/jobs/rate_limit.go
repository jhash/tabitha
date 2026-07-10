package jobs

import (
	"errors"
	"time"

	"github.com/riverqueue/river"
	"google.golang.org/api/googleapi"
)

// isRateLimitError reports whether err is (or wraps) a Google API 429,
// which digest_song treats specially: snooze and retry later without
// burning an attempt, rather than a normal failure (see docs/jeff-domain-notes.md
// context: the TOC sheet has ~1,925 songs, easily enough to blow through
// Sheets' default 60-reads/minute-per-user quota on a big batch).
func isRateLimitError(err error) bool {
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		return gerr.Code == 429
	}
	return false
}

// rateLimitSnoozeDuration is comfortably longer than Google's 1-minute
// quota window, so a snoozed job's retry lands after the quota has reset.
const rateLimitSnoozeDuration = 90 * time.Second

// snoozeOnRateLimit turns a rate-limit error into a river.JobSnooze so the
// job retries later without burning one of its limited attempts — a batch
// of many songs can otherwise exhaust MaxAttempts purely on quota noise
// before the quota window even resets. Other errors pass through
// unchanged.
func snoozeOnRateLimit(err error) error {
	if err != nil && isRateLimitError(err) {
		return river.JobSnooze(rateLimitSnoozeDuration)
	}
	return err
}
