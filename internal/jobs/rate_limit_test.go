package jobs

import (
	"errors"
	"fmt"
	"testing"

	"github.com/riverqueue/river"
	"google.golang.org/api/googleapi"
)

func TestIsRateLimitErrorTrueFor429(t *testing.T) {
	err := &googleapi.Error{Code: 429, Message: "Quota exceeded"}
	if !isRateLimitError(err) {
		t.Errorf("isRateLimitError(429) = false, want true")
	}
}

func TestIsRateLimitErrorFalseForOtherStatus(t *testing.T) {
	err := &googleapi.Error{Code: 403, Message: "disabled"}
	if isRateLimitError(err) {
		t.Errorf("isRateLimitError(403) = true, want false")
	}
}

func TestIsRateLimitErrorTrueWhenWrapped(t *testing.T) {
	err := fmt.Errorf("fetching doc: %w", &googleapi.Error{Code: 429})
	if !isRateLimitError(err) {
		t.Errorf("isRateLimitError(wrapped 429) = false, want true")
	}
}

func TestIsRateLimitErrorFalseForNonGoogleError(t *testing.T) {
	if isRateLimitError(errors.New("some other error")) {
		t.Errorf("isRateLimitError(plain error) = true, want false")
	}
}

func TestSnoozeOnRateLimitReturnsJobSnoozeFor429(t *testing.T) {
	err := fmt.Errorf("fetching: %w", &googleapi.Error{Code: 429})
	got := snoozeOnRateLimit(err)

	var snoozeErr *river.JobSnoozeError
	if !errors.As(got, &snoozeErr) {
		t.Fatalf("snoozeOnRateLimit(429) = %v, want a *river.JobSnoozeError", got)
	}
}

func TestSnoozeOnRateLimitPassesThroughOtherErrors(t *testing.T) {
	err := errors.New("some other failure")
	if got := snoozeOnRateLimit(err); got != err {
		t.Errorf("snoozeOnRateLimit(other) = %v, want the original error unchanged", got)
	}
}

func TestSnoozeOnRateLimitPassesThroughNil(t *testing.T) {
	if got := snoozeOnRateLimit(nil); got != nil {
		t.Errorf("snoozeOnRateLimit(nil) = %v, want nil", got)
	}
}
