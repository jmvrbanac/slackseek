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
	expected := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
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
