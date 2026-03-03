package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jmvrbanac/slackseek/internal/slack"
	"github.com/jmvrbanac/slackseek/internal/tokens"
)

// runHistoryCmd builds a fresh root command with the given history dependencies
// injected, executes the provided args, and returns stdout, stderr, and error.
func runHistoryCmd(
	t *testing.T,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn historyRunFunc,
	args ...string,
) (stdout, stderr string, err error) {
	t.Helper()
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	root := NewRootCmd()
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	addHistoryCmd(root, extractFn, runFn)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

var defaultHistoryExtractFn = func() (tokens.TokenExtractionResult, error) {
	return tokens.TokenExtractionResult{
		Workspaces: []tokens.Workspace{
			{Name: "Test", URL: "https://test.slack.com", Token: "xoxs-test"},
		},
	}, nil
}

var noopHistoryRunFn historyRunFunc = func(
	_ context.Context,
	_ tokens.Workspace,
	_, _ string,
	_ slack.DateRange,
	_ int,
	_ bool,
) ([]slack.Message, error) {
	return nil, nil
}

func TestHistoryCmd_MissingChannelExitsWithError(t *testing.T) {
	_, _, err := runHistoryCmd(t, defaultHistoryExtractFn, noopHistoryRunFn, "history")
	if err == nil {
		t.Fatal("expected error when <channel> argument is missing, got nil")
	}
}

func TestHistoryCmd_LimitFlagPassedToRunFn(t *testing.T) {
	var capturedLimit int
	runFn := func(_ context.Context, _ tokens.Workspace, _, _ string, _ slack.DateRange, limit int, _ bool) ([]slack.Message, error) {
		capturedLimit = limit
		return nil, nil
	}
	_, _, err := runHistoryCmd(t, defaultHistoryExtractFn, runFn, "history", "general", "--limit", "10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedLimit != 10 {
		t.Errorf("expected limit=10, got %d", capturedLimit)
	}
}

func TestHistoryCmd_ThreadsFalsePassedToRunFn(t *testing.T) {
	capturedThreads := true
	runFn := func(_ context.Context, _ tokens.Workspace, _, _ string, _ slack.DateRange, _ int, threads bool) ([]slack.Message, error) {
		capturedThreads = threads
		return nil, nil
	}
	_, _, err := runHistoryCmd(t, defaultHistoryExtractFn, runFn, "history", "general", "--threads=false")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedThreads {
		t.Error("expected threads=false when --threads=false is passed")
	}
}

func TestHistoryCmd_InvalidChannelExitsWithActionableError(t *testing.T) {
	channelName := "nonexistent-channel"
	runFn := func(_ context.Context, _ tokens.Workspace, channel, _ string, _ slack.DateRange, _ int, _ bool) ([]slack.Message, error) {
		return nil, errors.New("channel " + channel + " not found")
	}
	_, _, err := runHistoryCmd(t, defaultHistoryExtractFn, runFn, "history", channelName)
	if err == nil {
		t.Fatal("expected error for invalid channel, got nil")
	}
	if !strings.Contains(err.Error(), channelName) {
		t.Errorf("expected channel name %q in error, got: %v", channelName, err)
	}
}

func TestHistoryCmd_TableOutputContainsExpectedColumns(t *testing.T) {
	msgs := []slack.Message{
		{
			Timestamp: "1700000000.000000",
			UserID:    "U123",
			Text:      "hello world",
		},
	}
	runFn := func(_ context.Context, _ tokens.Workspace, _, _ string, _ slack.DateRange, _ int, _ bool) ([]slack.Message, error) {
		return msgs, nil
	}

	stdout, _, err := runHistoryCmd(t, defaultHistoryExtractFn, runFn, "history", "general", "--format", "table")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// tablewriter renders headers in uppercase.
	for _, col := range []string{"TIMESTAMP", "USER", "TEXT", "DEPTH", "REACTIONS"} {
		if !strings.Contains(stdout, col) {
			t.Errorf("expected column %q in table output, not found in:\n%s", col, stdout)
		}
	}
}

func TestHistoryCmd_ChannelArgPassedToRunFn(t *testing.T) {
	var capturedChannel string
	runFn := func(_ context.Context, _ tokens.Workspace, channel, _ string, _ slack.DateRange, _ int, _ bool) ([]slack.Message, error) {
		capturedChannel = channel
		return nil, nil
	}
	_, _, err := runHistoryCmd(t, defaultHistoryExtractFn, runFn, "history", "C01234567")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedChannel != "C01234567" {
		t.Errorf("expected channel=C01234567, got %q", capturedChannel)
	}
}
