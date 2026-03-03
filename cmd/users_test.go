package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/jmvrbanac/slackseek/internal/slack"
	"github.com/jmvrbanac/slackseek/internal/tokens"
)

// runUsersCmd builds a fresh root command with injectable users
// dependencies, executes the provided args, and returns stdout, stderr, error.
func runUsersCmd(
	t *testing.T,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn usersRunFunc,
	args ...string,
) (stdout, stderr string, err error) {
	t.Helper()
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	root := NewRootCmd()
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	addUsersCmd(root, extractFn, runFn)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

var defaultUsersExtractFn = func() (tokens.TokenExtractionResult, error) {
	return tokens.TokenExtractionResult{
		Workspaces: []tokens.Workspace{
			{Name: "Test", URL: "https://test.slack.com", Token: "xoxs-test"},
		},
	}, nil
}

// testUsers holds three fixture users: active, bot, and deleted.
var testUsers = []slack.User{
	{ID: "U1", DisplayName: "alice", RealName: "Alice", IsBot: false, IsDeleted: false},
	{ID: "U2", DisplayName: "bob-bot", RealName: "Bob Bot", IsBot: true, IsDeleted: false},
	{ID: "U3", DisplayName: "charlie", RealName: "Charlie", IsBot: false, IsDeleted: true},
}

func TestUsersCmd_NoFlagsFiltersDeletedAndBots(t *testing.T) {
	runFn := func(_ context.Context, _ tokens.Workspace) ([]slack.User, error) {
		return testUsers, nil
	}

	stdout, _, err := runUsersCmd(t, defaultUsersExtractFn, runFn, "users", "list", "--format", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var arr []map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(stdout), &arr); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\nstdout: %s", jsonErr, stdout)
	}
	// Only alice should be in the output (not a bot, not deleted).
	if len(arr) != 1 {
		t.Errorf("expected 1 user (active non-bot), got %d: %v", len(arr), arr)
	}
}

func TestUsersCmd_DeletedFlagIncludesDeletedUsers(t *testing.T) {
	runFn := func(_ context.Context, _ tokens.Workspace) ([]slack.User, error) {
		return testUsers, nil
	}

	stdout, _, err := runUsersCmd(t, defaultUsersExtractFn, runFn, "users", "list", "--deleted", "--format", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var arr []map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(stdout), &arr); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\nstdout: %s", jsonErr, stdout)
	}
	// Alice (active) and Charlie (deleted) should appear; Bob (bot) should not.
	if len(arr) != 2 {
		t.Errorf("expected 2 users (active + deleted, no bots), got %d: %v", len(arr), arr)
	}
}

func TestUsersCmd_BotFlagIncludesBotUsers(t *testing.T) {
	runFn := func(_ context.Context, _ tokens.Workspace) ([]slack.User, error) {
		return testUsers, nil
	}

	stdout, _, err := runUsersCmd(t, defaultUsersExtractFn, runFn, "users", "list", "--bot", "--format", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var arr []map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(stdout), &arr); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\nstdout: %s", jsonErr, stdout)
	}
	// Alice (active) and Bob Bot (bot) should appear; Charlie (deleted) should not.
	if len(arr) != 2 {
		t.Errorf("expected 2 users (active + bots, no deleted), got %d: %v", len(arr), arr)
	}
}

func TestUsersCmd_JSONOutputMatchesSchema(t *testing.T) {
	runFn := func(_ context.Context, _ tokens.Workspace) ([]slack.User, error) {
		return []slack.User{testUsers[0]}, nil
	}

	stdout, _, err := runUsersCmd(t, defaultUsersExtractFn, runFn, "users", "list", "--format", "json")
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
	for _, field := range []string{"id", "display_name", "real_name", "email", "is_bot", "is_deleted"} {
		if _, ok := arr[0][field]; !ok {
			t.Errorf("expected JSON field %q in output: %v", field, arr[0])
		}
	}
}
