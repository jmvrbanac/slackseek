package output

import (
	"strings"
	"testing"
	"time"

	"github.com/jmvrbanac/slackseek/internal/slack"
)

// T036: digest unit tests

func makeDigestMessages() []slack.Message {
	t1 := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	return []slack.Message{
		{Timestamp: "1000.000000", Time: t1, UserID: "alice", ChannelName: "general", Text: "Hello"},
		{Timestamp: "1001.000000", Time: t1, UserID: "alice", ChannelName: "general", Text: "World"},
		{Timestamp: "1002.000000", Time: t1, UserID: "alice", ChannelName: "general", Text: "Three"},
		{Timestamp: "1003.000000", Time: t1, UserID: "alice", ChannelName: "random", Text: "Random one"},
	}
}

func TestGroupByChannel_SortedDescendingByCount(t *testing.T) {
	msgs := makeDigestMessages()
	groups := GroupByChannel(msgs)

	if len(groups) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(groups))
	}
	if groups[0].ChannelName != "general" {
		t.Errorf("expected 'general' first (3 messages), got %q", groups[0].ChannelName)
	}
	if len(groups[0].Messages) != 3 {
		t.Errorf("expected 3 messages in general, got %d", len(groups[0].Messages))
	}
}

func TestGroupByChannel_EmptyMessages(t *testing.T) {
	groups := GroupByChannel(nil)
	if len(groups) != 0 {
		t.Errorf("expected 0 groups for nil messages, got %d", len(groups))
	}
}

func TestPrintDigest_TextContainsChannelHeaders(t *testing.T) {
	msgs := makeDigestMessages()
	groups := GroupByChannel(msgs)

	var sb strings.Builder
	if err := PrintDigest(&sb, FormatText, groups, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := sb.String()

	if !strings.Contains(out, "## #general") {
		t.Errorf("expected '## #general' in output, got:\n%s", out)
	}
}

func TestPrintDigest_JSONContainsChannelAndCount(t *testing.T) {
	msgs := makeDigestMessages()
	groups := GroupByChannel(msgs)

	var sb strings.Builder
	if err := PrintDigest(&sb, FormatJSON, groups, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := sb.String()

	for _, key := range []string{`"channel"`, `"count"`, `"messages"`} {
		if !strings.Contains(out, key) {
			t.Errorf("expected JSON key %q in output, got:\n%s", key, out)
		}
	}
}

func TestGroupByChannel_PreviewTruncatesAt80(t *testing.T) {
	longText := strings.Repeat("x", 100)
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	msgs := []slack.Message{
		{Timestamp: "1.000000", Time: t1, UserID: "u1", ChannelName: "ch", Text: longText},
	}
	groups := GroupByChannel(msgs)
	var sb strings.Builder
	if err := PrintDigest(&sb, FormatText, groups, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := sb.String()
	// The truncated preview should not contain the full 100-char string
	if strings.Contains(out, longText) {
		t.Error("expected long text to be truncated in digest preview")
	}
}
