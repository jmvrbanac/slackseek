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

// T033: postmortem command tests

func runPostmortemCmd(
	t *testing.T,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn postmortemRunFunc,
	args ...string,
) (stdout, stderr string, err error) {
	t.Helper()
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	root := NewRootCmd()
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	addPostmortemCmd(root, extractFn, runFn)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

var defaultPostmortemExtractFn = func() (tokens.TokenExtractionResult, error) {
	return tokens.TokenExtractionResult{
		Workspaces: []tokens.Workspace{
			{Name: "Test", URL: "https://test.slack.com", Token: "xoxs-test"},
		},
	}, nil
}

func makePostmortemMsgs() []slack.Message {
	t1 := time.Date(2026, 2, 25, 15, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 25, 15, 5, 0, 0, time.UTC)
	return []slack.Message{
		{Timestamp: "1.0", Time: t1, UserID: "alice", ChannelName: "ic-5697", Text: "Deploy started"},
		{Timestamp: "2.0", Time: t2, UserID: "bob", ChannelName: "ic-5697", Text: "Error found"},
	}
}

func TestPostmortemCmd_DefaultFormatIsMarkdown(t *testing.T) {
	runFn := func(_ context.Context, _ tokens.Workspace, _ string, _ slack.DateRange) ([]slack.Message, error) {
		return makePostmortemMsgs(), nil
	}

	stdout, _, err := runPostmortemCmd(t, defaultPostmortemExtractFn, runFn, "postmortem", "ic-5697")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "# Incident:") {
		t.Errorf("expected markdown header in default output, got:\n%s", stdout)
	}
}

func TestPostmortemCmd_JSONContainsRequiredKeys(t *testing.T) {
	runFn := func(_ context.Context, _ tokens.Workspace, _ string, _ slack.DateRange) ([]slack.Message, error) {
		return makePostmortemMsgs(), nil
	}

	stdout, _, err := runPostmortemCmd(t, defaultPostmortemExtractFn, runFn, "postmortem", "--format", "json", "ic-5697")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, key := range []string{`"channel"`, `"period"`, `"participants"`, `"timeline"`} {
		if !strings.Contains(stdout, key) {
			t.Errorf("expected JSON key %q in output, got:\n%s", key, stdout)
		}
	}
}

func TestPostmortemCmd_MissingChannelExitsWithError(t *testing.T) {
	runFn := func(_ context.Context, _ tokens.Workspace, _ string, _ slack.DateRange) ([]slack.Message, error) {
		return nil, nil
	}
	_, _, err := runPostmortemCmd(t, defaultPostmortemExtractFn, runFn, "postmortem")
	if err == nil {
		t.Fatal("expected error when channel argument is missing")
	}
}
