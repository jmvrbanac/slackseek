package slack

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestGetUserMessages_ComposesFromQuery(t *testing.T) {
	var capturedQuery string
	searchFn := func(_ context.Context, query string, _ int) ([]SearchResult, error) {
		capturedQuery = query
		return nil, nil
	}

	_, err := getUserMessages(context.Background(), "U123", "", DateRange{}, 0, searchFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(capturedQuery, "from:U123") {
		t.Errorf("expected query to contain 'from:U123', got %q", capturedQuery)
	}
}

func TestGetUserMessages_AddsChannelModifier(t *testing.T) {
	var capturedQuery string
	searchFn := func(_ context.Context, query string, _ int) ([]SearchResult, error) {
		capturedQuery = query
		return nil, nil
	}

	_, err := getUserMessages(context.Background(), "U123", "general", DateRange{}, 0, searchFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(capturedQuery, "in:#general") {
		t.Errorf("expected query to contain 'in:#general', got %q", capturedQuery)
	}
}

func TestGetUserMessages_AddsDateRange(t *testing.T) {
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	dr := DateRange{From: &from, To: &to}

	var capturedQuery string
	searchFn := func(_ context.Context, query string, _ int) ([]SearchResult, error) {
		capturedQuery = query
		return nil, nil
	}

	_, err := getUserMessages(context.Background(), "U123", "", dr, 0, searchFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(capturedQuery, "after:2025-01-01") {
		t.Errorf("expected query to contain 'after:2025-01-01', got %q", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "before:2025-02-01") {
		t.Errorf("expected query to contain 'before:2025-02-01', got %q", capturedQuery)
	}
}

func TestGetUserMessages_MapsToMessages(t *testing.T) {
	results := []SearchResult{
		{
			Message: Message{
				UserID:      "U123",
				Text:        "hello",
				ChannelID:   "C1",
				ChannelName: "general",
			},
			Permalink: "https://slack.com/p",
		},
		{
			Message: Message{
				UserID:      "U123",
				Text:        "world",
				ChannelID:   "C2",
				ChannelName: "random",
			},
		},
	}
	searchFn := func(_ context.Context, _ string, _ int) ([]SearchResult, error) {
		return results, nil
	}

	msgs, err := getUserMessages(context.Background(), "U123", "", DateRange{}, 0, searchFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].ChannelName != "general" {
		t.Errorf("expected ChannelName 'general', got %q", msgs[0].ChannelName)
	}
	if msgs[1].ChannelName != "random" {
		t.Errorf("expected ChannelName 'random', got %q", msgs[1].ChannelName)
	}
	if msgs[0].ChannelID != "C1" {
		t.Errorf("expected ChannelID 'C1', got %q", msgs[0].ChannelID)
	}
}

func TestGetUserMessages_RespectsLimit(t *testing.T) {
	all := []SearchResult{
		{Message: Message{Text: "a"}},
		{Message: Message{Text: "b"}},
		{Message: Message{Text: "c"}},
	}
	searchFn := func(_ context.Context, _ string, limit int) ([]SearchResult, error) {
		if limit > 0 && len(all) > limit {
			return all[:limit], nil
		}
		return all, nil
	}

	msgs, err := getUserMessages(context.Background(), "U123", "", DateRange{}, 2, searchFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) > 2 {
		t.Errorf("expected at most 2 messages (limit=2), got %d", len(msgs))
	}
}
