package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jmvrbanac/slackseek/internal/slack"
	"github.com/jmvrbanac/slackseek/internal/tokens"
)

// T045: actions command tests

func runActionsCmd(
	t *testing.T,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn actionsRunFunc,
	args ...string,
) (stdout, stderr string, err error) {
	t.Helper()
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	root := NewRootCmd()
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	addActionsCmd(root, extractFn, runFn)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

var defaultActionsExtractFn = func() (tokens.TokenExtractionResult, error) {
	return tokens.TokenExtractionResult{
		Workspaces: []tokens.Workspace{
			{Name: "Test", URL: "https://test.slack.com", Token: "xoxs-test"},
		},
	}, nil
}

func TestActionsCmd_ChecklistFormat(t *testing.T) {
	t1 := time.Date(2026, 2, 25, 16, 0, 0, 0, time.UTC)
	runFn := func(_ context.Context, _ tokens.Workspace, _ string, _ slack.DateRange) ([]slack.Message, error) {
		return []slack.Message{
			{Timestamp: "1.0", Time: t1, UserID: "alice", Text: "I'll send the postmortem draft by EOD"},
			{Timestamp: "2.0", Time: t1, UserID: "bob", Text: "hello world, no commitment here"},
		}, nil
	}

	stdout, _, err := runActionsCmd(t, defaultActionsExtractFn, runFn, "actions", "general")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "[ ]") {
		t.Errorf("expected checklist marker '[ ]' in output, got:\n%s", stdout)
	}
}

func TestActionsCmd_EmptyResultPrintsSummary(t *testing.T) {
	runFn := func(_ context.Context, _ tokens.Workspace, _ string, _ slack.DateRange) ([]slack.Message, error) {
		return []slack.Message{
			{Timestamp: "1.0", Time: time.Now(), UserID: "alice", Text: "hello world"},
		}, nil
	}

	stdout, _, err := runActionsCmd(t, defaultActionsExtractFn, runFn, "actions", "general")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout == "" {
		t.Error("expected summary output for empty actions, got empty string")
	}
}

func TestActionsCmd_JSONOutputSchema(t *testing.T) {
	t1 := time.Date(2026, 2, 25, 16, 0, 0, 0, time.UTC)
	runFn := func(_ context.Context, _ tokens.Workspace, _ string, _ slack.DateRange) ([]slack.Message, error) {
		return []slack.Message{
			{Timestamp: "1.0", Time: t1, UserID: "alice", Text: "I'll do this task"},
		}, nil
	}

	stdout, _, err := runActionsCmd(t, defaultActionsExtractFn, runFn, "actions", "--format", "json", "general")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, key := range []string{`"user"`, `"text"`, `"timestamp"`} {
		if !strings.Contains(stdout, key) {
			t.Errorf("expected JSON key %q in actions output, got:\n%s", key, stdout)
		}
	}
}

func TestActionsCmd_MissingChannelExitsWithError(t *testing.T) {
	runFn := func(_ context.Context, _ tokens.Workspace, _ string, _ slack.DateRange) ([]slack.Message, error) {
		return nil, nil
	}
	_, _, err := runActionsCmd(t, defaultActionsExtractFn, runFn, "actions")
	if err == nil {
		t.Fatal("expected error when channel argument is missing")
	}
}
