package slack

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// offsetPattern matches duration strings like 30m, 4h, 7d, 2w.
var offsetPattern = regexp.MustCompile(`^(\d+)([mhdw])$`)

// parseDateOrOffset resolves either an ISO date string or a duration offset
// of the form `\d+[mhdw]` relative to now. If endOfDay is true, a YYYY-MM-DD
// input is resolved to 23:59:59.999999999 UTC instead of 00:00:00 UTC.
func parseDateOrOffset(s string, now time.Time, endOfDay bool) (time.Time, error) {
	m := offsetPattern.FindStringSubmatch(s)
	if m != nil {
		n, _ := strconv.Atoi(m[1])
		var d time.Duration
		switch m[2] {
		case "m":
			d = time.Duration(n) * time.Minute
		case "h":
			d = time.Duration(n) * time.Hour
		case "d":
			d = time.Duration(n) * 24 * time.Hour
		case "w":
			d = time.Duration(n) * 7 * 24 * time.Hour
		}
		return now.Add(-d), nil
	}
	if endOfDay {
		return parseDateStringEndOfDay(s)
	}
	return parseDateString(s)
}

// ParseRelativeDateRange parses --since / --until style flag strings.
// Each string may be empty, an ISO date, RFC 3339, or a duration offset.
// Returns an error if the resolved From is after the resolved To.
func ParseRelativeDateRange(since, until string) (DateRange, error) {
	now := time.Now().UTC()
	var dr DateRange

	if since != "" {
		t, err := parseDateOrOffset(since, now, false)
		if err != nil {
			return dr, fmt.Errorf("parsing --since %q: %w", since, err)
		}
		dr.From = &t
	}

	if until != "" {
		t, err := parseDateOrOffset(until, now, true)
		if err != nil {
			return dr, fmt.Errorf("parsing --until %q: %w", until, err)
		}
		dr.To = &t
	}

	if dr.From != nil && dr.To != nil && dr.From.After(*dr.To) {
		return dr, fmt.Errorf("--since (%s) resolves to a time after --until (%s)", since, until)
	}

	return dr, nil
}

// DateRange holds optional start and end timestamps for time-bounded queries.
type DateRange struct {
	From *time.Time // nil means "no lower bound"
	To   *time.Time // nil means "no upper bound"
}

// ParseDateRange parses the --from and --to flag strings into a DateRange.
// --from: empty → nil; YYYY-MM-DD → 00:00:00 UTC; RFC 3339 → as-is.
// --to:   empty → nil; YYYY-MM-DD → 23:59:59.999999999 UTC; RFC 3339 → as-is.
// Returns an error if From > To.
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
		t, err := parseDateStringEndOfDay(to)
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
// YYYY-MM-DD is resolved to 00:00:00 UTC (start of day).
func parseDateString(s string) (time.Time, error) {
	if t, err := time.ParseInLocation("2006-01-02", s, time.UTC); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unrecognized format %q: use YYYY-MM-DD or RFC 3339 (e.g. 2025-01-15T09:30:00Z)", s)
}

// parseDateStringEndOfDay accepts YYYY-MM-DD or RFC 3339.
// YYYY-MM-DD is resolved to 23:59:59.999999999 UTC (end of day) so that a
// same-day --from/--to range covers the full day. RFC 3339 is used as-is.
func parseDateStringEndOfDay(s string) (time.Time, error) {
	if t, err := time.ParseInLocation("2006-01-02", s, time.UTC); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, time.UTC), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unrecognized format %q: use YYYY-MM-DD or RFC 3339 (e.g. 2025-01-15T09:30:00Z)", s)
}
