package output_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jmvrbanac/slackseek/internal/output"
	"github.com/jmvrbanac/slackseek/internal/slack"
	"github.com/jmvrbanac/slackseek/internal/tokens"
)

func fixtureWorkspaces() []tokens.Workspace {
	return []tokens.Workspace{
		{Name: "Acme Corp", URL: "https://acme.slack.com", Token: "xoxs-abcdefghijklmnop", Cookie: "cookie1234"},
		{Name: "Beta Inc", URL: "https://beta.slack.com", Token: "xoxs-defghijklmnopqrs", Cookie: "cookie5678"},
	}
}

func fixtureChannels() []slack.Channel {
	return []slack.Channel{
		{ID: "C01234567", Name: "general", Type: "public_channel", MemberCount: 10, Topic: "General chat"},
	}
}

func fixtureMessages() []slack.Message {
	t, _ := time.Parse(time.RFC3339, "2025-01-15T09:30:00Z")
	return []slack.Message{
		{
			Timestamp: "1736936400.000001",
			Time:      t,
			UserID:    "U01234567",
			Text:      "Hello world",
			ChannelID: "C01234567",
		},
	}
}

func fixtureUsers() []slack.User {
	return []slack.User{
		{ID: "U01234567", DisplayName: "jane.doe", RealName: "Jane Doe", Email: "jane@acme.com"},
	}
}

func fixtureSearchResults() []slack.SearchResult {
	t, _ := time.Parse(time.RFC3339, "2025-01-15T09:30:00Z")
	return []slack.SearchResult{
		{
			Message: slack.Message{
				Timestamp:   "1736936400.000001",
				Time:        t,
				UserID:      "U01234567",
				Text:        "Found it",
				ChannelID:   "C01234567",
				ChannelName: "general",
			},
			Permalink: "https://acme.slack.com/archives/C01234567/p1736936400000001",
		},
	}
}

// --- Workspaces ---

func TestPrintWorkspacesJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := output.PrintWorkspaces(&buf, output.FormatJSON, fixtureWorkspaces()); err != nil {
		t.Fatal(err)
	}
	var result []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON array, got error: %v\noutput: %s", err, buf.String())
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(result))
	}
	if result[0]["name"] != "Acme Corp" {
		t.Errorf("expected name 'Acme Corp', got %v", result[0]["name"])
	}
	if _, ok := result[0]["token"]; !ok {
		t.Error("expected 'token' field in JSON output")
	}
	if _, ok := result[0]["cookie"]; !ok {
		t.Error("expected 'cookie' field in JSON output")
	}
}

func TestPrintWorkspacesTable(t *testing.T) {
	var buf bytes.Buffer
	if err := output.PrintWorkspaces(&buf, output.FormatTable, fixtureWorkspaces()); err != nil {
		t.Fatal(err)
	}
	out := strings.ToUpper(buf.String())
	for _, col := range []string{"NAME", "URL", "TOKEN", "COOKIE"} {
		if !strings.Contains(out, col) {
			t.Errorf("expected column header %q in table output:\n%s", col, buf.String())
		}
	}
}

func TestPrintWorkspacesText(t *testing.T) {
	var buf bytes.Buffer
	if err := output.PrintWorkspaces(&buf, output.FormatText, fixtureWorkspaces()); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines for 2 workspaces, got %d:\n%s", len(lines), buf.String())
	}
}

// --- Channels ---

func TestPrintChannelsJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := output.PrintChannels(&buf, output.FormatJSON, fixtureChannels()); err != nil {
		t.Fatal(err)
	}
	var result []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON array: %v\noutput: %s", err, buf.String())
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(result))
	}
	if result[0]["id"] != "C01234567" {
		t.Errorf("expected id 'C01234567', got %v", result[0]["id"])
	}
}

func TestPrintChannelsTable(t *testing.T) {
	var buf bytes.Buffer
	if err := output.PrintChannels(&buf, output.FormatTable, fixtureChannels()); err != nil {
		t.Fatal(err)
	}
	out := strings.ToUpper(buf.String())
	for _, col := range []string{"ID", "NAME", "TYPE", "MEMBERS", "TOPIC"} {
		if !strings.Contains(out, col) {
			t.Errorf("expected column header %q in table output:\n%s", col, buf.String())
		}
	}
}

// --- Messages ---

func TestPrintMessagesJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := output.PrintMessages(&buf, output.FormatJSON, fixtureMessages()); err != nil {
		t.Fatal(err)
	}
	var result []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON array: %v\noutput: %s", err, buf.String())
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
	for _, field := range []string{"timestamp", "slack_ts", "user_id", "text", "channel_id"} {
		if _, ok := result[0][field]; !ok {
			t.Errorf("expected field %q in message JSON output", field)
		}
	}
}

// --- Users ---

func TestPrintUsersJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := output.PrintUsers(&buf, output.FormatJSON, fixtureUsers()); err != nil {
		t.Fatal(err)
	}
	var result []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON array: %v\noutput: %s", err, buf.String())
	}
	if result[0]["id"] != "U01234567" {
		t.Errorf("expected id 'U01234567', got %v", result[0]["id"])
	}
	for _, field := range []string{"id", "display_name", "real_name", "email", "is_bot", "is_deleted"} {
		if _, ok := result[0][field]; !ok {
			t.Errorf("expected field %q in user JSON output", field)
		}
	}
}

func TestPrintUsersTable(t *testing.T) {
	var buf bytes.Buffer
	if err := output.PrintUsers(&buf, output.FormatTable, fixtureUsers()); err != nil {
		t.Fatal(err)
	}
	out := strings.ToUpper(buf.String())
	for _, col := range []string{"ID", "DISPLAY", "REAL"} {
		if !strings.Contains(out, col) {
			t.Errorf("expected column header %q in table output:\n%s", col, buf.String())
		}
	}
}

// --- SearchResults ---

func TestPrintSearchResultsJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := output.PrintSearchResults(&buf, output.FormatJSON, fixtureSearchResults()); err != nil {
		t.Fatal(err)
	}
	var result []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON array: %v\noutput: %s", err, buf.String())
	}
	if _, ok := result[0]["permalink"]; !ok {
		t.Error("expected 'permalink' field in search result JSON")
	}
	if _, ok := result[0]["channel_name"]; !ok {
		t.Error("expected 'channel_name' field in search result JSON")
	}
}
