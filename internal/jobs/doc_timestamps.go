package jobs

import "time"

// parseDriveTime parses the RFC3339 timestamps the Drive API returns for
// File.CreatedTime/ModifiedTime. Pulled out as its own function so the
// format assumption is unit-tested without needing a real Drive API call.
func parseDriveTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}
