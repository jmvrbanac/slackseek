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

// T037: digest command tests

func runDigestCmd(
	t *testing.T,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn digestRunFunc,
	args ...string,
) (stdout, stderr string, err error) {
	t.Helper()
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	root := NewRootCmd()
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	addDigestCmd(root, extractFn, runFn)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

var defaultDigestExtractFn = func() (tokens.TokenExtractionResult, error) {
	return tokens.TokenExtractionResult{
		Workspaces: []tokens.Workspace{
			{Name: "Test", URL: "https://test.slack.com", Token: "xoxs-test"},
		},
	}, nil
}

func TestDigestCmd_RequiresUserFlag(t *testing.T) {
	runFn := func(_ context.Context, _ tokens.Workspace, _ string, _ slack.DateRange) ([]slack.Message, error) {
		return nil, nil
	}
	_, _, err := runDigestCmd(t, defaultDigestExtractFn, runFn, "digest")
	if err == nil {
		t.Fatal("expected error when --user flag is absent")
	}
	if !strings.Contains(err.Error(), "user") && !strings.Contains(err.Error(), "-u") {
		t.Errorf("expected user-related error, got: %v", err)
	}
}

func TestDigestCmd_OutputGroupsByChannel(t *testing.T) {
	t1 := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	runFn := func(_ context.Context, _ tokens.Workspace, _ string, _ slack.DateRange) ([]slack.Message, error) {
		return []slack.Message{
			{Timestamp: "1.0", Time: t1, UserID: "alice", ChannelName: "general", Text: "Hello"},
			{Timestamp: "2.0", Time: t1, UserID: "alice", ChannelName: "random", Text: "World"},
		}, nil
	}

	stdout, _, err := runDigestCmd(t, defaultDigestExtractFn, runFn, "digest", "--user", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "general") {
		t.Errorf("expected 'general' in digest output, got:\n%s", stdout)
	}
}
