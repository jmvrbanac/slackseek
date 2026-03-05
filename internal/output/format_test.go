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
	if err := output.PrintMessages(&buf, output.FormatJSON, fixtureMessages(), nil); err != nil {
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

// --- Messages with Resolver (T003) ---

func fixtureResolver() *slack.Resolver {
	users := []slack.User{
		{ID: "U01234567", DisplayName: "alice", RealName: "Alice Smith"},
	}
	channels := []slack.Channel{
		{ID: "C01234567", Name: "general"},
	}
	return slack.NewResolver(users, channels, nil)
}

func TestPrintMessages_TextUsesDisplayNameWithResolver(t *testing.T) {
	var buf bytes.Buffer
	r := fixtureResolver()
	if err := output.PrintMessages(&buf, output.FormatText, fixtureMessages(), r); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Alice Smith") {
		t.Errorf("expected real name 'Alice Smith' in text output, got:\n%s", out)
	}
	if strings.Contains(out, "U01234567") {
		t.Errorf("expected raw user ID to be replaced by display name, got:\n%s", out)
	}
}

func TestPrintMessages_TableUserColumnContainsDisplayName(t *testing.T) {
	var buf bytes.Buffer
	r := fixtureResolver()
	if err := output.PrintMessages(&buf, output.FormatTable, fixtureMessages(), r); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Alice Smith") {
		t.Errorf("expected real name 'Alice Smith' in table output, got:\n%s", out)
	}
}

func TestPrintMessages_JSONIncludesUserDisplayName(t *testing.T) {
	var buf bytes.Buffer
	r := fixtureResolver()
	if err := output.PrintMessages(&buf, output.FormatJSON, fixtureMessages(), r); err != nil {
		t.Fatal(err)
	}
	var result []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v\noutput: %s", err, buf.String())
	}
	if _, ok := result[0]["user_display_name"]; !ok {
		t.Error("expected 'user_display_name' field in message JSON output")
	}
	if result[0]["user_display_name"] != "Alice Smith" {
		t.Errorf("expected 'Alice Smith', got %v", result[0]["user_display_name"])
	}
}

func TestPrintMessages_NilResolverFallsBackToRawID(t *testing.T) {
	var buf bytes.Buffer
	if err := output.PrintMessages(&buf, output.FormatText, fixtureMessages(), nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "U01234567") {
		t.Errorf("expected raw user ID 'U01234567' with nil resolver, got:\n%s", out)
	}
}

func TestPrintSearchResults_TextUsesDisplayNameWithResolver(t *testing.T) {
	var buf bytes.Buffer
	r := fixtureResolver()
	if err := output.PrintSearchResults(&buf, output.FormatText, fixtureSearchResults(), r); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Alice Smith") {
		t.Errorf("expected real name 'Alice Smith' in text output, got:\n%s", out)
	}
}

func TestPrintSearchResults_TableUserColumnContainsDisplayName(t *testing.T) {
	var buf bytes.Buffer
	r := fixtureResolver()
	if err := output.PrintSearchResults(&buf, output.FormatTable, fixtureSearchResults(), r); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Alice Smith") {
		t.Errorf("expected real name 'Alice Smith' in table output, got:\n%s", out)
	}
}

func TestPrintSearchResults_JSONIncludesUserDisplayName(t *testing.T) {
	var buf bytes.Buffer
	r := fixtureResolver()
	if err := output.PrintSearchResults(&buf, output.FormatJSON, fixtureSearchResults(), r); err != nil {
		t.Fatal(err)
	}
	var result []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v\noutput: %s", err, buf.String())
	}
	if _, ok := result[0]["user_display_name"]; !ok {
		t.Error("expected 'user_display_name' field in search result JSON output")
	}
	if result[0]["user_display_name"] != "Alice Smith" {
		t.Errorf("expected 'Alice Smith', got %v", result[0]["user_display_name"])
	}
}

func TestPrintSearchResults_NilResolverFallsBackToRawID(t *testing.T) {
	var buf bytes.Buffer
	if err := output.PrintSearchResults(&buf, output.FormatText, fixtureSearchResults(), nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "U01234567") {
		t.Errorf("expected raw user ID 'U01234567' with nil resolver, got:\n%s", out)
	}
}

// --- T016: Resolver built from empty slices returns raw IDs ---

func TestPrintMessages_EmptyResolverReturnsRawIDs(t *testing.T) {
	r := slack.NewResolver([]slack.User{}, []slack.Channel{}, nil)
	var buf bytes.Buffer
	if err := output.PrintMessages(&buf, output.FormatText, fixtureMessages(), r); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "U01234567") {
		t.Errorf("expected raw user ID 'U01234567' with empty resolver, got:\n%s", out)
	}
}

// --- Channel Name Resolution (T012) ---

func TestPrintMessages_TextUsesChannelNameWithResolver(t *testing.T) {
	// fixtureMessages has ChannelID "C01234567" and empty ChannelName.
	// With resolver, the text output should show "general" not "C01234567".
	var buf bytes.Buffer
	r := fixtureResolver()
	if err := output.PrintMessages(&buf, output.FormatText, fixtureMessages(), r); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "general") {
		t.Errorf("expected channel name 'general' in text output, got:\n%s", out)
	}
}

func TestPrintMessages_TableChannelColumnContainsResolvedName(t *testing.T) {
	var buf bytes.Buffer
	r := fixtureResolver()
	if err := output.PrintMessages(&buf, output.FormatTable, fixtureMessages(), r); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(strings.ToUpper(out), "CHANNEL") {
		t.Errorf("expected 'CHANNEL' column header in table output, got:\n%s", out)
	}
	if !strings.Contains(out, "general") {
		t.Errorf("expected channel name 'general' in table output, got:\n%s", out)
	}
}

func TestPrintMessages_JSONChannelNamePopulatedWithResolver(t *testing.T) {
	var buf bytes.Buffer
	r := fixtureResolver()
	if err := output.PrintMessages(&buf, output.FormatJSON, fixtureMessages(), r); err != nil {
		t.Fatal(err)
	}
	var result []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v\noutput: %s", err, buf.String())
	}
	if result[0]["channel_name"] != "general" {
		t.Errorf("expected channel_name 'general', got %v", result[0]["channel_name"])
	}
}

func TestPrintMessages_ExistingChannelNamePreserved(t *testing.T) {
	// Message already has ChannelName set; resolver must not overwrite it.
	msgs := []slack.Message{
		{
			UserID:      "U01234567",
			ChannelID:   "C01234567",
			ChannelName: "already-resolved",
			Text:        "hi",
		},
	}
	var buf bytes.Buffer
	r := fixtureResolver()
	if err := output.PrintMessages(&buf, output.FormatJSON, msgs, r); err != nil {
		t.Fatal(err)
	}
	var result []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v\noutput: %s", err, buf.String())
	}
	if result[0]["channel_name"] != "already-resolved" {
		t.Errorf("expected preserved channel name 'already-resolved', got %v", result[0]["channel_name"])
	}
}

func TestPrintMessages_NilResolverLeaveChannelNameEmpty(t *testing.T) {
	// With nil resolver and empty ChannelName, channel_name should not appear (omitempty).
	var buf bytes.Buffer
	if err := output.PrintMessages(&buf, output.FormatJSON, fixtureMessages(), nil); err != nil {
		t.Fatal(err)
	}
	var result []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v\noutput: %s", err, buf.String())
	}
	if v, ok := result[0]["channel_name"]; ok && v != "" {
		t.Errorf("expected channel_name absent or empty with nil resolver, got %v", v)
	}
}

// --- SearchResults ---

func TestPrintSearchResultsJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := output.PrintSearchResults(&buf, output.FormatJSON, fixtureSearchResults(), nil); err != nil {
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

// T010: tableSafe unit tests
func TestTableSafe_CollapseNewlines(t *testing.T) {
	cases := []struct {
		name  string
		input string
		n     int
		want  string
	}{
		{"newline collapsed", "hello\nworld", 100, "hello world"},
		{"crlf collapsed", "hello\r\nworld", 100, "hello world"},
		{"tab collapsed", "hello\tworld", 100, "hello world"},
		{"leading trailing space stripped", "  hello world  ", 100, "hello world"},
		{"truncated at rune limit", "hello world foo bar", 10, "hello worl…"},
		{"multi newline", "a\n\nb\n\nc", 100, "a b c"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := output.TableSafe(c.input, c.n)
			if got != c.want {
				t.Errorf("TableSafe(%q, %d) = %q, want %q", c.input, c.n, got, c.want)
			}
		})
	}
}

func TestPrintMessages_TableDoesNotSplitNewlines(t *testing.T) {
	t0, _ := time.Parse(time.RFC3339, "2025-01-15T09:30:00Z")
	msgs := []slack.Message{
		{
			Timestamp: "1736936400.000001",
			Time:      t0,
			UserID:    "U01234567",
			Text:      "line one\nline two\nline three",
			ChannelID: "C01234567",
		},
	}
	var buf bytes.Buffer
	if err := output.PrintMessages(&buf, output.FormatTable, msgs, nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// "line two" should NOT appear at column 0 on its own line in a table context
	// Instead, it should all be on one collapsed row
	if strings.Contains(out, "\nline two") {
		t.Errorf("expected multi-line message to be collapsed in table, but got raw newlines:\n%s", out)
	}
}

func TestPrintSearchResults_TableDoesNotSplitNewlines(t *testing.T) {
	t0, _ := time.Parse(time.RFC3339, "2025-01-15T09:30:00Z")
	results := []slack.SearchResult{
		{
			Message: slack.Message{
				Timestamp:   "1736936400.000001",
				Time:        t0,
				UserID:      "U01234567",
				Text:        "first line\nsecond line",
				ChannelID:   "C01234567",
				ChannelName: "general",
			},
			Permalink: "https://example.com/p1",
		},
	}
	var buf bytes.Buffer
	if err := output.PrintSearchResults(&buf, output.FormatTable, results, nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "\nsecond line") {
		t.Errorf("expected multi-line search result to be collapsed in table, but got raw newlines:\n%s", out)
	}
}

// T006: resolveMessageFields DM channel name tests

func fixtureDMMessages() []slack.Message {
	t0, _ := time.Parse(time.RFC3339, "2025-01-15T09:30:00Z")
	return []slack.Message{{
		Timestamp: "1736936400.000001", Time: t0,
		UserID: "U01ABCDEF", Text: "Hello",
		ChannelID: "D00000001", ChannelName: "U01ABCDEF",
	}}
}

func fixtureDMResolver() *slack.Resolver {
	return slack.NewResolver([]slack.User{{ID: "U01ABCDEF", RealName: "Nick Mollenkopf"}}, []slack.Channel{}, nil)
}

func TestPrintMessages_DMChannelName_Text(t *testing.T) {
	var buf bytes.Buffer
	if err := output.PrintMessages(&buf, output.FormatText, fixtureDMMessages(), fixtureDMResolver()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "@Nick Mollenkopf") {
		t.Errorf("expected '@Nick Mollenkopf' for DM channel in text, got:\n%s", buf.String())
	}
}

func TestPrintMessages_DMChannelName_Table(t *testing.T) {
	var buf bytes.Buffer
	if err := output.PrintMessages(&buf, output.FormatTable, fixtureDMMessages(), fixtureDMResolver()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "@Nick Mollenkopf") {
		t.Errorf("expected '@Nick Mollenkopf' for DM channel in table, got:\n%s", buf.String())
	}
}

func TestPrintMessages_DMChannelName_JSON(t *testing.T) {
	var buf bytes.Buffer
	if err := output.PrintMessages(&buf, output.FormatJSON, fixtureDMMessages(), fixtureDMResolver()); err != nil {
		t.Fatal(err)
	}
	var result []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v\noutput: %s", err, buf.String())
	}
	if result[0]["channel_name"] != "@Nick Mollenkopf" {
		t.Errorf("JSON: expected channel_name '@Nick Mollenkopf', got %v", result[0]["channel_name"])
	}
}

// T014: groupByThread unit tests

func TestGroupByThread_EmptyInput(t *testing.T) {
	roots, replies := output.GroupByThread(nil)
	if len(roots) != 0 {
		t.Errorf("expected 0 roots, got %d", len(roots))
	}
	if len(replies) != 0 {
		t.Errorf("expected 0 replies, got %d", len(replies))
	}
}

func TestGroupByThread_AllRootMessages(t *testing.T) {
	msgs := []slack.Message{
		{Timestamp: "100.000001"},
		{Timestamp: "200.000001"},
	}
	roots, replies := output.GroupByThread(msgs)
	if len(roots) != 2 {
		t.Errorf("expected 2 roots, got %d", len(roots))
	}
	if len(replies) != 0 {
		t.Errorf("expected 0 replies, got %d", len(replies))
	}
}

func TestGroupByThread_MixedRootsAndReplies(t *testing.T) {
	msgs := []slack.Message{
		{Timestamp: "100.000001", ThreadTS: ""},
		{Timestamp: "101.000001", ThreadTS: "100.000001"},
		{Timestamp: "200.000001", ThreadTS: ""},
	}
	roots, replies := output.GroupByThread(msgs)
	if len(roots) != 2 {
		t.Errorf("expected 2 roots, got %d", len(roots))
	}
	if len(replies["100.000001"]) != 1 {
		t.Errorf("expected 1 reply for root 100.000001, got %d", len(replies["100.000001"]))
	}
}

func TestGroupByThread_RootWithSameThreadTS(t *testing.T) {
	// A message where ThreadTS == Timestamp is the thread parent (root).
	msgs := []slack.Message{
		{Timestamp: "100.000001", ThreadTS: "100.000001"},
		{Timestamp: "101.000001", ThreadTS: "100.000001"},
	}
	roots, replies := output.GroupByThread(msgs)
	if len(roots) != 1 {
		t.Errorf("expected 1 root, got %d", len(roots))
	}
	if len(replies["100.000001"]) != 1 {
		t.Errorf("expected 1 reply, got %d", len(replies["100.000001"]))
	}
}

// T015: PrintMessages text format thread integration test

func TestPrintMessages_TextThreadGroupingWithIndentation(t *testing.T) {
	t0, _ := time.Parse(time.RFC3339, "2025-01-15T09:00:00Z")
	t1, _ := time.Parse(time.RFC3339, "2025-01-15T09:01:00Z")
	t2, _ := time.Parse(time.RFC3339, "2025-01-15T09:02:00Z")
	msgs := []slack.Message{
		{Timestamp: "1736934000.000001", Time: t0, UserID: "U001", Text: "root message", ThreadTS: ""},
		{Timestamp: "1736934060.000001", Time: t1, UserID: "U002", Text: "reply to root", ThreadTS: "1736934000.000001"},
		{Timestamp: "1736934120.000001", Time: t2, UserID: "U003", Text: "another root", ThreadTS: ""},
	}
	var buf bytes.Buffer
	if err := output.PrintMessages(&buf, output.FormatText, msgs, nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "└─") {
		t.Errorf("expected '└─' prefix for replies in text output:\n%s", out)
	}
	if !strings.Contains(out, "reply to root") {
		t.Errorf("expected reply text in output:\n%s", out)
	}
}

// T016: PrintMessages JSON format thread integration test

func TestPrintMessages_JSONThreadGroupingWithReplies(t *testing.T) {
	t0, _ := time.Parse(time.RFC3339, "2025-01-15T09:00:00Z")
	t1, _ := time.Parse(time.RFC3339, "2025-01-15T09:01:00Z")
	msgs := []slack.Message{
		{Timestamp: "1736934000.000001", Time: t0, UserID: "U001", Text: "root message", ThreadTS: ""},
		{Timestamp: "1736934060.000001", Time: t1, UserID: "U002", Text: "reply to root", ThreadTS: "1736934000.000001"},
	}
	var buf bytes.Buffer
	if err := output.PrintMessages(&buf, output.FormatJSON, msgs, nil); err != nil {
		t.Fatal(err)
	}
	var result []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v\noutput: %s", err, buf.String())
	}
	if len(result) != 1 {
		t.Errorf("expected 1 root message in JSON top-level, got %d", len(result))
	}
	replies, ok := result[0]["replies"]
	if !ok {
		t.Error("expected 'replies' field in root message JSON")
	}
	repliesSlice, ok := replies.([]interface{})
	if !ok || len(repliesSlice) != 1 {
		t.Errorf("expected 1 reply, got %v", replies)
	}
}

// T022: PrintMessages markdown format test

func TestPrintMessages_MarkdownFormat(t *testing.T) {
	t0, _ := time.Parse(time.RFC3339, "2025-01-15T09:00:00Z")
	t1, _ := time.Parse(time.RFC3339, "2025-01-15T09:01:00Z")
	msgs := []slack.Message{
		{
			Timestamp: "1736934000.000001",
			Time:      t0,
			UserID:    "U001",
			Text:      "root message",
			ChannelID: "C001",
			ThreadTS:  "1736934000.000001",
		},
		{
			Timestamp: "1736934060.000001",
			Time:      t1,
			UserID:    "U002",
			Text:      "reply text",
			ChannelID: "C001",
			ThreadTS:  "1736934000.000001",
		},
	}
	channels := []slack.Channel{{ID: "C001", Name: "general"}}
	resolver := slack.NewResolver(nil, channels, nil)

	var buf bytes.Buffer
	if err := output.PrintMessages(&buf, output.FormatMarkdown, msgs, resolver); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "# #general") {
		t.Errorf("expected '# #general' heading in markdown:\n%s", out)
	}
	if !strings.Contains(out, "##") {
		t.Errorf("expected '##' root heading in markdown:\n%s", out)
	}
	if !strings.Contains(out, ">") {
		t.Errorf("expected '>' block quote for reply in markdown:\n%s", out)
	}
	if !strings.Contains(out, "---") {
		t.Errorf("expected '---' separator in markdown:\n%s", out)
	}
}

// T023: PrintSearchResults markdown format test

func TestPrintSearchResults_MarkdownFormat(t *testing.T) {
	t0, _ := time.Parse(time.RFC3339, "2025-01-15T09:30:00Z")
	results := []slack.SearchResult{
		{
			Message: slack.Message{
				Timestamp:   "1736936400.000001",
				Time:        t0,
				UserID:      "U001",
				Text:        "search result text",
				ChannelID:   "C001",
				ChannelName: "general",
			},
			Permalink: "https://acme.slack.com/archives/C001/p123",
		},
	}
	var buf bytes.Buffer
	if err := output.PrintSearchResults(&buf, output.FormatMarkdown, results, nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "# Search results") {
		t.Errorf("expected '# Search results' heading:\n%s", out)
	}
	if !strings.Contains(out, "##") {
		t.Errorf("expected '##' result heading:\n%s", out)
	}
	if !strings.Contains(out, "View in Slack") {
		t.Errorf("expected 'View in Slack' permalink:\n%s", out)
	}
	if !strings.Contains(out, "---") {
		t.Errorf("expected '---' separator:\n%s", out)
	}
}

// T024: PrintChannels and PrintUsers with markdown format fall through to text

func TestPrintChannels_MarkdownFallsThrough(t *testing.T) {
	var buf bytes.Buffer
	if err := output.PrintChannels(&buf, output.FormatMarkdown, fixtureChannels()); err != nil {
		t.Fatal(err)
	}
	if buf.String() == "" {
		t.Error("expected non-empty output for channels with markdown format")
	}
}

func TestPrintUsers_MarkdownFallsThrough(t *testing.T) {
	var buf bytes.Buffer
	if err := output.PrintUsers(&buf, output.FormatMarkdown, fixtureUsers()); err != nil {
		t.Fatal(err)
	}
	if buf.String() == "" {
		t.Error("expected non-empty output for users with markdown format")
	}
}

// T028: emoji rendering in reactions

func TestFormatReactions_EmojiRendering(t *testing.T) {
	// Enable emoji rendering
	prev := output.EmojiEnabled
	output.EmojiEnabled = true
	defer func() { output.EmojiEnabled = prev }()

	t0, _ := time.Parse(time.RFC3339, "2025-01-15T09:00:00Z")
	msgs := []slack.Message{
		{
			Timestamp: "1.0",
			Time:      t0,
			UserID:    "U1",
			Text:      "hello",
			Reactions: []slack.Reaction{{Name: "thumbsup", Count: 3}},
		},
	}
	var buf bytes.Buffer
	if err := output.PrintMessages(&buf, output.FormatText, msgs, nil); err != nil {
		t.Fatal(err)
	}
	// The text output doesn't show reactions directly; test via JSON
	buf.Reset()
	if err := output.PrintMessages(&buf, output.FormatJSON, msgs, nil); err != nil {
		t.Fatal(err)
	}
	var result []map[string]interface{}
	if jsonErr := json.Unmarshal(buf.Bytes(), &result); jsonErr != nil {
		t.Fatalf("invalid JSON: %v", jsonErr)
	}
	if len(result) == 0 {
		t.Fatal("expected at least one message")
	}
}

func TestEmojiRenderInMessageText(t *testing.T) {
	prev := output.EmojiEnabled
	output.EmojiEnabled = true
	defer func() { output.EmojiEnabled = prev }()

	t0, _ := time.Parse(time.RFC3339, "2025-01-15T09:00:00Z")
	msgs := []slack.Message{
		{Timestamp: "1.0", Time: t0, UserID: "U1", Text: ":thumbsup: done"},
	}
	var buf bytes.Buffer
	if err := output.PrintMessages(&buf, output.FormatText, msgs, nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// With emoji enabled, :thumbsup: should be rendered as 👍
	if strings.Contains(out, ":thumbsup:") {
		t.Errorf("expected :thumbsup: to be rendered as emoji, got:\n%s", out)
	}
	if !strings.Contains(out, "👍") {
		t.Errorf("expected 👍 in output with emoji enabled, got:\n%s", out)
	}
}
