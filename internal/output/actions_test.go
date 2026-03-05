package output

import (
	"strings"
	"testing"
	"time"

	"github.com/jmvrbanac/slackseek/internal/slack"
)

// T044: actions unit tests

func makeMsg(text string) slack.Message {
	return slack.Message{
		Timestamp: "1.000000",
		Time:      time.Date(2026, 2, 25, 16, 0, 0, 0, time.UTC),
		UserID:    "alice",
		Text:      text,
	}
}

func TestExtractActions_IllPattern(t *testing.T) {
	items := ExtractActions([]slack.Message{makeMsg("I'll send the report")}, nil)
	if len(items) == 0 {
		t.Error("expected match for \"I'll\"")
	}
}

func TestExtractActions_IWillPattern(t *testing.T) {
	items := ExtractActions([]slack.Message{makeMsg("I will investigate this")}, nil)
	if len(items) == 0 {
		t.Error("expected match for \"I will\"")
	}
}

func TestExtractActions_WillDoPattern(t *testing.T) {
	items := ExtractActions([]slack.Message{makeMsg("will do, thanks")}, nil)
	if len(items) == 0 {
		t.Error("expected match for \"will do\"")
	}
}

func TestExtractActions_OnItPattern(t *testing.T) {
	items := ExtractActions([]slack.Message{makeMsg("I'm on it")}, nil)
	if len(items) == 0 {
		t.Error("expected match for \"on it\"")
	}
}

func TestExtractActions_ActionItemPattern(t *testing.T) {
	items := ExtractActions([]slack.Message{makeMsg("Action item: fix the build")}, nil)
	if len(items) == 0 {
		t.Error("expected match for \"action item\"")
	}
}

func TestExtractActions_TODOPattern(t *testing.T) {
	items := ExtractActions([]slack.Message{makeMsg("TODO: update docs")}, nil)
	if len(items) == 0 {
		t.Error("expected match for TODO")
	}
}

func TestExtractActions_FollowUpPattern(t *testing.T) {
	items := ExtractActions([]slack.Message{makeMsg("will follow up tomorrow")}, nil)
	if len(items) == 0 {
		t.Error("expected match for \"follow up\"")
	}
}

func TestExtractActions_CaseInsensitive(t *testing.T) {
	items := ExtractActions([]slack.Message{makeMsg("todo: check logs")}, nil)
	if len(items) == 0 {
		t.Error("expected case-insensitive match for 'todo'")
	}
}

func TestExtractActions_NonMatchingReturnsEmpty(t *testing.T) {
	items := ExtractActions([]slack.Message{makeMsg("hello world, how are you?")}, nil)
	if len(items) != 0 {
		t.Errorf("expected no match for non-commitment message, got %d", len(items))
	}
}

func TestPrintActions_TextChecklistFormat(t *testing.T) {
	items := []ActionItem{
		{Who: "alice", Text: "I'll send the report", Timestamp: time.Date(2026, 2, 25, 16, 10, 0, 0, time.UTC)},
	}
	var sb strings.Builder
	if err := PrintActions(&sb, FormatText, items); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := sb.String()
	if !strings.Contains(out, "[ ]") {
		t.Errorf("expected checklist marker '[ ]' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "alice") {
		t.Errorf("expected 'alice' in output, got:\n%s", out)
	}
}

func TestPrintActions_EmptyPrintsSummary(t *testing.T) {
	var sb strings.Builder
	if err := PrintActions(&sb, FormatText, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := sb.String()
	if out == "" {
		t.Error("expected summary line for empty actions")
	}
}

func TestPrintActions_JSONOutputSchema(t *testing.T) {
	items := []ActionItem{
		{Who: "alice", Text: "I'll do it", Timestamp: time.Now()},
	}
	var sb strings.Builder
	if err := PrintActions(&sb, FormatJSON, items); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := sb.String()
	for _, key := range []string{`"user"`, `"text"`, `"timestamp"`} {
		if !strings.Contains(out, key) {
			t.Errorf("expected JSON key %q in actions output, got:\n%s", key, out)
		}
	}
}
