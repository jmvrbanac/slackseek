package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jmvrbanac/slackseek/internal/slack"
	"github.com/jmvrbanac/slackseek/internal/tokens"
)

// runSearchCmd builds a fresh root command with the given search dependencies
// injected, executes the provided args, and returns stdout, stderr, and error.
func runSearchCmd(
	t *testing.T,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn searchRunFunc,
	args ...string,
) (stdout, stderr string, err error) {
	t.Helper()
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	root := NewRootCmd()
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	addSearchCmd(root, extractFn, runFn)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

var defaultSearchExtractFn = func() (tokens.TokenExtractionResult, error) {
	return tokens.TokenExtractionResult{
		Workspaces: []tokens.Workspace{
			{Name: "Test", URL: "https://test.slack.com", Token: "xoxs-test"},
		},
	}, nil
}

var noopSearchFn searchRunFunc = func(
	_ context.Context,
	_ tokens.Workspace,
	_, _, _ string,
	_ slack.DateRange,
	_ int,
) ([]slack.SearchResult, error) {
	return nil, nil
}

func TestSearchCmd_MissingQueryExitsWithError(t *testing.T) {
	_, _, err := runSearchCmd(t, defaultSearchExtractFn, noopSearchFn, "search")
	if err == nil {
		t.Fatal("expected error when <query> argument is missing, got nil")
	}
}

func TestSearchCmd_LimitFlagPassedToRunFn(t *testing.T) {
	var capturedLimit int
	runFn := func(_ context.Context, _ tokens.Workspace, _, _, _ string, _ slack.DateRange, limit int) ([]slack.SearchResult, error) {
		capturedLimit = limit
		return nil, nil
	}
	_, _, err := runSearchCmd(t, defaultSearchExtractFn, runFn, "search", "test", "--limit", "2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedLimit != 2 {
		t.Errorf("expected limit=2, got %d", capturedLimit)
	}
}

func TestSearchCmd_JSONOutputMatchesSchema(t *testing.T) {
	results := []slack.SearchResult{
		{
			Message: slack.Message{
				UserID:      "U1",
				Text:        "hello",
				ChannelName: "general",
				ChannelID:   "C1",
			},
			Permalink: "https://slack.com/p",
		},
	}
	runFn := func(_ context.Context, _ tokens.Workspace, _, _, _ string, _ slack.DateRange, _ int) ([]slack.SearchResult, error) {
		return results, nil
	}

	stdout, _, err := runSearchCmd(t, defaultSearchExtractFn, runFn, "search", "test", "--format", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var arr []map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(stdout), &arr); jsonErr != nil {
		t.Fatalf("invalid JSON output: %v\nstdout: %s", jsonErr, stdout)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 JSON element, got %d", len(arr))
	}
	for _, field := range []string{"user_id", "text", "permalink", "channel_name"} {
		if _, ok := arr[0][field]; !ok {
			t.Errorf("expected JSON field %q in output: %v", field, arr[0])
		}
	}
}

func TestSearchCmd_UserAndChannelFlagsPassedToRunFn(t *testing.T) {
	var capturedChannel, capturedUser string
	runFn := func(_ context.Context, _ tokens.Workspace, _, channel, userArg string, _ slack.DateRange, _ int) ([]slack.SearchResult, error) {
		capturedChannel = channel
		capturedUser = userArg
		return nil, nil
	}

	_, _, err := runSearchCmd(t, defaultSearchExtractFn, runFn, "search", "test", "--channel", "general", "--user", "U123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedChannel != "general" {
		t.Errorf("expected channel='general', got %q", capturedChannel)
	}
	if capturedUser != "U123" {
		t.Errorf("expected user='U123', got %q", capturedUser)
	}
}

// TestSearchCmd_NilResolverShowsRawID verifies that when buildResolver returns
// nil (as it does in the test environment where no real Slack server is
// available), the raw user ID is preserved as-is in the output.
func TestSearchCmd_NilResolverShowsRawID(t *testing.T) {
	const rawUserID = "U999RAW"
	results := []slack.SearchResult{
		{
			Message: slack.Message{
				UserID:      rawUserID,
				Text:        "raw id test",
				ChannelName: "general",
				ChannelID:   "C1",
			},
			Permalink: "https://slack.com/p/raw",
		},
	}
	runFn := func(_ context.Context, _ tokens.Workspace, _, _, _ string, _ slack.DateRange, _ int) ([]slack.SearchResult, error) {
		return results, nil
	}

	// buildResolver will return nil in the test environment (no real Slack
	// server reachable), so output must fall back to the raw user ID.
	stdout, _, err := runSearchCmd(t, defaultSearchExtractFn, runFn, "search", "test", "--format", "text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, rawUserID) {
		t.Errorf("expected raw user ID %q in output, got: %s", rawUserID, stdout)
	}
}
