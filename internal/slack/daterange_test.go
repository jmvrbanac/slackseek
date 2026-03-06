package slack_test

import (
	"testing"
	"time"

	"github.com/jmvrbanac/slackseek/internal/slack"
)

func TestParseDateRange_YYYYMMDDFrom(t *testing.T) {
	dr, err := slack.ParseDateRange("2025-02-01", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dr.From == nil {
		t.Fatal("expected From to be non-nil")
	}
	expected := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	if !dr.From.Equal(expected) {
		t.Errorf("expected From=%v, got %v", expected, *dr.From)
	}
	if dr.To != nil {
		t.Error("expected To to be nil when only --from provided")
	}
}

func TestParseDateRange_YYYYMMDDTo(t *testing.T) {
	dr, err := slack.ParseDateRange("", "2025-03-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dr.To == nil {
		t.Fatal("expected To to be non-nil")
	}
	// YYYY-MM-DD --to resolves to end of day (FR-014)
	expected := time.Date(2025, 3, 1, 23, 59, 59, 999999999, time.UTC)
	if !dr.To.Equal(expected) {
		t.Errorf("expected To=%v, got %v", expected, *dr.To)
	}
	if dr.From != nil {
		t.Error("expected From to be nil when only --to provided")
	}
}

func TestParseDateRange_RFC3339(t *testing.T) {
	dr, err := slack.ParseDateRange("2025-02-01T10:30:00Z", "2025-03-01T10:30:00Z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dr.From == nil || dr.To == nil {
		t.Fatal("expected both From and To to be non-nil")
	}
	wantFrom := time.Date(2025, 2, 1, 10, 30, 0, 0, time.UTC)
	if !dr.From.Equal(wantFrom) {
		t.Errorf("expected From=%v, got %v", wantFrom, *dr.From)
	}
}

func TestParseDateRange_FromAfterTo(t *testing.T) {
	_, err := slack.ParseDateRange("2025-02-01", "2025-01-01")
	if err == nil {
		t.Error("expected error when From > To, got nil")
	}
}

func TestParseDateRange_NilFields(t *testing.T) {
	dr, err := slack.ParseDateRange("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dr.From != nil || dr.To != nil {
		t.Error("expected both From and To to be nil when flags omitted")
	}
}

func TestParseDateRange_InvalidFormat(t *testing.T) {
	_, err := slack.ParseDateRange("not-a-date", "")
	if err == nil {
		t.Error("expected error for invalid date format, got nil")
	}
}

// --- T008: ParseRelativeDateRange tests ---

func TestParseRelativeDateRange_SinceHours(t *testing.T) {
	dr, err := slack.ParseRelativeDateRange("4h", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dr.From == nil {
		t.Fatal("expected non-nil From for 4h offset")
	}
	// From should be approximately 4 hours ago
	diff := time.Since(*dr.From)
	if diff < 3*time.Hour || diff > 5*time.Hour {
		t.Errorf("expected From ~4h ago, got diff=%v", diff)
	}
}

func TestParseRelativeDateRange_SinceWeeks(t *testing.T) {
	dr, err := slack.ParseRelativeDateRange("2w", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dr.From == nil {
		t.Fatal("expected non-nil From for 2w offset")
	}
	diff := time.Since(*dr.From)
	if diff < 13*24*time.Hour || diff > 15*24*time.Hour {
		t.Errorf("expected From ~2w ago, got diff=%v", diff)
	}
}

func TestParseRelativeDateRange_ISODatePassThrough(t *testing.T) {
	dr, err := slack.ParseRelativeDateRange("2025-01-15", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dr.From == nil {
		t.Fatal("expected non-nil From")
	}
	if dr.From.Year() != 2025 || dr.From.Month() != 1 || dr.From.Day() != 15 {
		t.Errorf("expected 2025-01-15, got %v", *dr.From)
	}
}

func TestParseRelativeDateRange_UnrecognisedReturnsError(t *testing.T) {
	_, err := slack.ParseRelativeDateRange("invalid!", "")
	if err == nil {
		t.Fatal("expected error for unrecognised --since input")
	}
}

func TestParseRelativeDateRange_BothEmpty(t *testing.T) {
	dr, err := slack.ParseRelativeDateRange("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dr.From != nil || dr.To != nil {
		t.Error("expected nil From and To for empty inputs")
	}
}

// --- T055: end-of-day behaviour for YYYY-MM-DD --to / --until ---

func TestParseDateRange_SameDayReturnsEndOfDay(t *testing.T) {
	dr, err := slack.ParseDateRange("2026-03-05", "2026-03-05")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantFrom := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	wantTo := time.Date(2026, 3, 5, 23, 59, 59, 999999999, time.UTC)
	if !dr.From.Equal(wantFrom) {
		t.Errorf("From: expected %v, got %v", wantFrom, *dr.From)
	}
	if !dr.To.Equal(wantTo) {
		t.Errorf("To: expected %v, got %v", wantTo, *dr.To)
	}
}

func TestParseDateRange_ToEndOfDayDateOnly(t *testing.T) {
	dr, err := slack.ParseDateRange("", "2026-01-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 1, 15, 23, 59, 59, 999999999, time.UTC)
	if !dr.To.Equal(want) {
		t.Errorf("expected To=%v, got %v", want, *dr.To)
	}
}

func TestParseDateRange_ToRFC3339Unchanged(t *testing.T) {
	dr, err := slack.ParseDateRange("", "2026-01-15T09:00:00Z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)
	if !dr.To.Equal(want) {
		t.Errorf("RFC3339 --to should be unchanged: expected %v, got %v", want, *dr.To)
	}
}

func TestParseRelativeDateRange_UntilISODateEndOfDay(t *testing.T) {
	dr, err := slack.ParseRelativeDateRange("", "2026-03-05")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 3, 5, 23, 59, 59, 999999999, time.UTC)
	if !dr.To.Equal(want) {
		t.Errorf("expected To=%v, got %v", want, *dr.To)
	}
}

func TestParseRelativeDateRange_UntilOffsetUnchanged(t *testing.T) {
	dr, err := slack.ParseRelativeDateRange("", "1d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// offset: should be approximately 1 day ago (not end-of-day of today)
	diff := time.Since(*dr.To)
	if diff < 20*time.Hour || diff > 28*time.Hour {
		t.Errorf("expected To ~1d ago, got diff=%v", diff)
	}
}
