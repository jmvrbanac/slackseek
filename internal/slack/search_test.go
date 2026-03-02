package slack_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jmvrbanac/slackseek/internal/slack"
)

// roundTripFunc is a function-based http.RoundTripper used to mock HTTP responses.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestBuildSearchQuery_AllFields(t *testing.T) {
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	dr := slack.DateRange{From: &from, To: &to}

	q := slack.BuildSearchQuery("test", "general", "U123", dr)

	for _, want := range []string{"test", "in:#general", "from:U123", "after:2025-01-01", "before:2025-02-01"} {
		if !strings.Contains(q, want) {
			t.Errorf("expected %q in query %q", want, q)
		}
	}
}

func TestBuildSearchQuery_OptionalFieldsOmitted(t *testing.T) {
	q := slack.BuildSearchQuery("hello", "", "", slack.DateRange{})
	if q != "hello" {
		t.Errorf("expected bare query %q, got %q", "hello", q)
	}
}

func TestBuildSearchQuery_NoDateRange(t *testing.T) {
	q := slack.BuildSearchQuery("test", "general", "U123", slack.DateRange{})
	if strings.Contains(q, "after:") || strings.Contains(q, "before:") {
		t.Errorf("no date modifiers expected when DateRange is empty, got %q", q)
	}
}

// mockSearchResponse returns an HTTP 200 with a valid Slack search.messages response body.
func mockSearchResponse(body string) http.RoundTripper {
	return roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})
}

const singleMatchBody = `{
  "ok": true,
  "query": "test",
  "messages": {
    "total": 1,
    "paging": {"count": 100, "total": 1, "page": 1, "pages": 1},
    "matches": [
      {
        "type": "message",
        "channel": {"id": "C123", "name": "general"},
        "user": "U123",
        "ts": "1700000000.123456",
        "text": "hello",
        "permalink": "https://slack.com/p"
      }
    ]
  }
}`

func TestSearchMessages_MapsAPIFields(t *testing.T) {
	c := slack.NewClient("xoxs-test", "", &http.Client{Transport: mockSearchResponse(singleMatchBody)})

	results, err := c.SearchMessages(context.Background(), "test", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.Permalink != "https://slack.com/p" {
		t.Errorf("expected permalink 'https://slack.com/p', got %q", r.Permalink)
	}
	if r.ChannelName != "general" {
		t.Errorf("expected ChannelName 'general', got %q", r.ChannelName)
	}
	if r.ChannelID != "C123" {
		t.Errorf("expected ChannelID 'C123', got %q", r.ChannelID)
	}
	if r.UserID != "U123" {
		t.Errorf("expected UserID 'U123', got %q", r.UserID)
	}
	if r.Text != "hello" {
		t.Errorf("expected Text 'hello', got %q", r.Text)
	}
}

func TestSearchMessages_RespectsLimit(t *testing.T) {
	c := slack.NewClient("xoxs-test", "", &http.Client{Transport: mockSearchResponse(singleMatchBody)})

	results, err := c.SearchMessages(context.Background(), "test", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) > 1 {
		t.Errorf("expected at most 1 result (limit=1), got %d", len(results))
	}
}
