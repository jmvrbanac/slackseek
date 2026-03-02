package slack

import (
	"fmt"
	"time"
)

// DateRange holds optional start and end timestamps for time-bounded queries.
type DateRange struct {
	From *time.Time // nil means "no lower bound"
	To   *time.Time // nil means "no upper bound"
}

// ParseDateRange parses the --from and --to flag strings into a DateRange.
// Each string may be empty (produces nil), YYYY-MM-DD (parsed as 00:00:00 UTC),
// or RFC 3339. Returns an error if From > To.
func ParseDateRange(from, to string) (DateRange, error) {
	var dr DateRange

	if from != "" {
		t, err := parseDateString(from)
		if err != nil {
			return dr, fmt.Errorf("parsing --from %q: %w", from, err)
		}
		dr.From = &t
	}

	if to != "" {
		t, err := parseDateString(to)
		if err != nil {
			return dr, fmt.Errorf("parsing --to %q: %w", to, err)
		}
		dr.To = &t
	}

	if dr.From != nil && dr.To != nil && dr.From.After(*dr.To) {
		return dr, fmt.Errorf("--from (%s) must be before --to (%s): use a start date earlier than the end date", from, to)
	}

	return dr, nil
}

// parseDateString accepts YYYY-MM-DD or RFC 3339 and returns a UTC time.Time.
func parseDateString(s string) (time.Time, error) {
	if t, err := time.ParseInLocation("2006-01-02", s, time.UTC); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unrecognized format %q: use YYYY-MM-DD or RFC 3339 (e.g. 2025-01-15T09:30:00Z)", s)
}
