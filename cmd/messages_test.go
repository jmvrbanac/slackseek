package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jmvrbanac/slackseek/internal/slack"
	"github.com/jmvrbanac/slackseek/internal/tokens"
)

// runMessagesCmd builds a fresh root command with injectable messages
// dependencies, executes the provided args, and returns stdout, stderr, error.
func runMessagesCmd(
	t *testing.T,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn messagesRunFunc,
	args ...string,
) (stdout, stderr string, err error) {
	t.Helper()
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	root := NewRootCmd()
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	addMessagesCmd(root, extractFn, runFn)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

var defaultMessagesExtractFn = func() (tokens.TokenExtractionResult, error) {
	return tokens.TokenExtractionResult{
		Workspaces: []tokens.Workspace{
			{Name: "Test", URL: "https://test.slack.com", Token: "xoxs-test"},
		},
	}, nil
}

var noopMessagesFn messagesRunFunc = func(
	_ context.Context,
	_ tokens.Workspace,
	_, _ string,
	_ slack.DateRange,
	_ int,
) ([]slack.Message, error) {
	return nil, nil
}

func TestMessagesCmd_MissingUserArgExitsWithError(t *testing.T) {
	_, _, err := runMessagesCmd(t, defaultMessagesExtractFn, noopMessagesFn, "messages")
	if err == nil {
		t.Fatal("expected error when <user> argument is missing, got nil")
	}
}

func TestMessagesCmd_ChannelFlagPassedToRunFn(t *testing.T) {
	var capturedChannel string
	runFn := func(_ context.Context, _ tokens.Workspace, _, channel string, _ slack.DateRange, _ int) ([]slack.Message, error) {
		capturedChannel = channel
		return nil, nil
	}

	_, _, err := runMessagesCmd(t, defaultMessagesExtractFn, runFn, "messages", "U123", "--channel", "general")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedChannel != "general" {
		t.Errorf("expected channel='general', got %q", capturedChannel)
	}
}

func TestMessagesCmd_UnknownUserExitsWithCode1(t *testing.T) {
	extractFn := func() (tokens.TokenExtractionResult, error) {
		return tokens.TokenExtractionResult{
			Workspaces: []tokens.Workspace{
				{Name: "Test", URL: "https://test.slack.com", Token: "xoxs-test"},
			},
		}, nil
	}
	runFn := func(_ context.Context, _ tokens.Workspace, _, _ string, _ slack.DateRange, _ int) ([]slack.Message, error) {
		return nil, errors.New("user \"nobody\" not found: use `slackseek users list` to see available display names and IDs")
	}

	_, _, err := runMessagesCmd(t, extractFn, runFn, "messages", "nobody")
	if err == nil {
		t.Fatal("expected error for unknown user, got nil")
	}
}

func TestMessagesCmd_NilResolverShowsRawID(t *testing.T) {
	const rawUserID = "U999RAW"
	msgs := []slack.Message{
		{
			UserID:      rawUserID,
			Text:        "raw id test",
			ChannelName: "general",
			ChannelID:   "C1",
		},
	}
	runFn := func(_ context.Context, _ tokens.Workspace, _, _ string, _ slack.DateRange, _ int) ([]slack.Message, error) {
		return msgs, nil
	}

	// buildResolver will fail to contact the (non-existent) Slack API and return
	// nil, so output should contain the raw user ID unchanged.
	stdout, _, err := runMessagesCmd(t, defaultMessagesExtractFn, runFn, "messages", rawUserID, "--format", "text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, rawUserID) {
		t.Errorf("expected raw user ID %q in output, got: %s", rawUserID, stdout)
	}
}

func TestMessagesCmd_JSONOutputMatchesSchema(t *testing.T) {
	msgs := []slack.Message{
		{
			UserID:      "U1",
			Text:        "hello",
			ChannelName: "general",
			ChannelID:   "C1",
		},
	}
	runFn := func(_ context.Context, _ tokens.Workspace, _, _ string, _ slack.DateRange, _ int) ([]slack.Message, error) {
		return msgs, nil
	}

	stdout, _, err := runMessagesCmd(t, defaultMessagesExtractFn, runFn, "messages", "U1", "--format", "json")
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
	for _, field := range []string{"user_id", "text", "channel_id", "channel_name"} {
		if _, ok := arr[0][field]; !ok {
			t.Errorf("expected JSON field %q in output: %v", field, arr[0])
		}
	}
}
