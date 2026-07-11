package jobs

import (
	"testing"
	"time"
)

func TestParseDriveTimeParsesRFC3339(t *testing.T) {
	got, err := parseDriveTime("2020-01-02T15:04:05.000Z")
	if err != nil {
		t.Fatalf("parseDriveTime() error = %v", err)
	}
	want := time.Date(2020, 1, 2, 15, 4, 5, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("parseDriveTime() = %v, want %v", got, want)
	}
}

func TestParseDriveTimeRejectsGarbage(t *testing.T) {
	if _, err := parseDriveTime("not a timestamp"); err == nil {
		t.Error("parseDriveTime() error = nil, want an error for unparseable input")
	}
}
