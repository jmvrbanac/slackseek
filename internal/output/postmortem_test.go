package output

import (
	"strings"
	"testing"
	"time"

	"github.com/jmvrbanac/slackseek/internal/slack"
)

// T032: postmortem unit tests

func makeTestMessages() []slack.Message {
	t1 := time.Date(2026, 2, 25, 15, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 25, 15, 5, 0, 0, time.UTC)
	t3 := time.Date(2026, 2, 25, 15, 10, 0, 0, time.UTC)
	return []slack.Message{
		{Timestamp: "1740495600.000000", Time: t1, UserID: "carol", ChannelName: "ic-5697", Text: "Deploy started"},
		{Timestamp: "1740495900.000000", Time: t2, UserID: "alice", ChannelName: "ic-5697", Text: "Error reported", ThreadDepth: 0},
		{Timestamp: "1740496000.000000", Time: t3, UserID: "bob", ChannelName: "ic-5697", Text: "Reply to error", ThreadTS: "1740495900.000000", ThreadDepth: 1},
	}
}

func TestBuildIncidentDoc_ParticipantsSortedAndDeduped(t *testing.T) {
	msgs := makeTestMessages()
	doc := BuildIncidentDoc(msgs, nil)

	if len(doc.Participants) == 0 {
		t.Fatal("expected participants, got none")
	}

	// Check sorted
	for i := 1; i < len(doc.Participants); i++ {
		if doc.Participants[i] < doc.Participants[i-1] {
			t.Errorf("participants not sorted at index %d: %v", i, doc.Participants)
		}
	}

	// Check no duplicates
	seen := make(map[string]bool)
	for _, p := range doc.Participants {
		if seen[p] {
			t.Errorf("duplicate participant %q", p)
		}
		seen[p] = true
	}
}

func TestBuildIncidentDoc_TimelineChronologicalOrder(t *testing.T) {
	msgs := makeTestMessages()
	doc := BuildIncidentDoc(msgs, nil)

	for i := 1; i < len(doc.Timeline); i++ {
		if doc.Timeline[i].Time.Before(doc.Timeline[i-1].Time) {
			t.Errorf("timeline not in chronological order at index %d", i)
		}
	}
}

func TestBuildIncidentDoc_ThreadRepliesCount(t *testing.T) {
	t1 := time.Date(2026, 2, 25, 15, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 25, 15, 5, 0, 0, time.UTC)
	t3 := time.Date(2026, 2, 25, 15, 6, 0, 0, time.UTC)
	msgs := []slack.Message{
		{Timestamp: "100.000000", Time: t1, UserID: "alice", Text: "root message"},
		{Timestamp: "101.000000", Time: t2, UserID: "bob", Text: "reply 1", ThreadTS: "100.000000", ThreadDepth: 1},
		{Timestamp: "102.000000", Time: t3, UserID: "carol", Text: "reply 2", ThreadTS: "100.000000", ThreadDepth: 1},
	}
	doc := BuildIncidentDoc(msgs, nil)

	if len(doc.Timeline) == 0 {
		t.Fatal("expected timeline rows")
	}
	if doc.Timeline[0].Replies != 2 {
		t.Errorf("expected 2 replies for root, got %d", doc.Timeline[0].Replies)
	}
}

func TestBuildIncidentDoc_EmptyMessages(t *testing.T) {
	doc := BuildIncidentDoc(nil, nil)
	if doc.Channel != "" {
		t.Error("expected empty doc for nil messages")
	}
}

func TestPrintPostmortem_MarkdownContainsRequiredSections(t *testing.T) {
	msgs := makeTestMessages()
	doc := BuildIncidentDoc(msgs, nil)

	var sb strings.Builder
	if err := PrintPostmortem(&sb, FormatMarkdown, doc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := sb.String()

	for _, expected := range []string{"# Incident:", "**Period:**", "**Participants:**", "## Timeline"} {
		if !strings.Contains(out, expected) {
			t.Errorf("expected %q in postmortem output, got:\n%s", expected, out)
		}
	}
}

func TestPrintPostmortem_JSONContainsRequiredKeys(t *testing.T) {
	msgs := makeTestMessages()
	doc := BuildIncidentDoc(msgs, nil)

	var sb strings.Builder
	if err := PrintPostmortem(&sb, FormatJSON, doc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := sb.String()

	for _, key := range []string{`"channel"`, `"period"`, `"participants"`, `"timeline"`} {
		if !strings.Contains(out, key) {
			t.Errorf("expected JSON key %q in output, got:\n%s", key, out)
		}
	}
}
