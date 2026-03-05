package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jmvrbanac/slackseek/internal/slack"
	"github.com/jmvrbanac/slackseek/internal/tokens"
)

func runThreadCmd(
	t *testing.T,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn threadRunFunc,
	args ...string,
) (stdout, stderr string, err error) {
	t.Helper()
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	root := NewRootCmd()
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	addThreadCmd(root, extractFn, runFn)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

var defaultThreadExtractFn = func() (tokens.TokenExtractionResult, error) {
	return tokens.TokenExtractionResult{
		Workspaces: []tokens.Workspace{
			{Name: "Test", URL: "https://test.slack.com", Token: "xoxs-test"},
		},
	}, nil
}

var defaultThreadMsgs = []slack.Message{
	{
		Timestamp: "1700000000.000000",
		Time:      time.Date(2023, 11, 14, 22, 13, 20, 0, time.UTC),
		UserID:    "alice",
		Text:      "The deploy is failing.",
		ThreadDepth: 0,
	},
	{
		Timestamp: "1700000060.000000",
		Time:      time.Date(2023, 11, 14, 22, 14, 20, 0, time.UTC),
		UserID:    "bob",
		Text:      "Investigating now.",
		ThreadTS:  "1700000000.000000",
		ThreadDepth: 1,
	},
}

// T017: thread command tests

func TestThreadCmd_TextOutputIncludesParticipants(t *testing.T) {
	runFn := func(_ context.Context, _ tokens.Workspace, _, _ string) ([]slack.Message, error) {
		return defaultThreadMsgs, nil
	}

	stdout, _, err := runThreadCmd(t, defaultThreadExtractFn, runFn,
		"thread", "https://test.slack.com/archives/C01234567/p1700000000000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "Participants") {
		t.Errorf("expected 'Participants' in text output, got:\n%s", stdout)
	}
}

func TestThreadCmd_JSONOutputContainsParticipantsField(t *testing.T) {
	runFn := func(_ context.Context, _ tokens.Workspace, _, _ string) ([]slack.Message, error) {
		return defaultThreadMsgs, nil
	}

	stdout, _, err := runThreadCmd(t, defaultThreadExtractFn, runFn,
		"thread", "--format", "json",
		"https://test.slack.com/archives/C01234567/p1700000000000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(stdout), &out); jsonErr != nil {
		t.Fatalf("invalid JSON output: %v\nstdout: %s", jsonErr, stdout)
	}
	if _, ok := out["participants"]; !ok {
		t.Errorf("expected 'participants' field in JSON output: %v", out)
	}
	if _, ok := out["messages"]; !ok {
		t.Errorf("expected 'messages' field in JSON output: %v", out)
	}
}

func TestThreadCmd_UnrecognisedURLReturnsError(t *testing.T) {
	runFn := func(_ context.Context, _ tokens.Workspace, _, _ string) ([]slack.Message, error) {
		return nil, nil
	}

	_, _, err := runThreadCmd(t, defaultThreadExtractFn, runFn,
		"thread", "https://example.com/not-a-slack-url")
	if err == nil {
		t.Fatal("expected error for unrecognised URL")
	}
}

func TestThreadCmd_MissingURLExitsWithError(t *testing.T) {
	runFn := func(_ context.Context, _ tokens.Workspace, _, _ string) ([]slack.Message, error) {
		return nil, nil
	}

	_, _, err := runThreadCmd(t, defaultThreadExtractFn, runFn, "thread")
	if err == nil {
		t.Fatal("expected error when URL argument is missing")
	}
}
