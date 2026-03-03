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

// runChannelsCmd builds a fresh root command with injectable channels
// dependencies, executes the provided args, and returns stdout, stderr, error.
func runChannelsCmd(
	t *testing.T,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn channelsRunFunc,
	args ...string,
) (stdout, stderr string, err error) {
	t.Helper()
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	root := NewRootCmd()
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	addChannelsCmd(root, extractFn, runFn)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

var defaultChannelsExtractFn = func() (tokens.TokenExtractionResult, error) {
	return tokens.TokenExtractionResult{
		Workspaces: []tokens.Workspace{
			{Name: "Test", URL: "https://test.slack.com", Token: "xoxs-test"},
		},
	}, nil
}

var noopChannelsRunFn channelsRunFunc = func(
	_ context.Context,
	_ tokens.Workspace,
	_ []string,
	_ bool,
) ([]slack.Channel, error) {
	return nil, nil
}

func TestChannelsCmd_TypePublicPassedAsPublicChannel(t *testing.T) {
	var capturedTypes []string
	runFn := func(_ context.Context, _ tokens.Workspace, types []string, _ bool) ([]slack.Channel, error) {
		capturedTypes = types
		return nil, nil
	}

	_, _, err := runChannelsCmd(t, defaultChannelsExtractFn, runFn, "channels", "list", "--type", "public")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(capturedTypes) != 1 || capturedTypes[0] != "public_channel" {
		t.Errorf("expected types=[\"public_channel\"], got %v", capturedTypes)
	}
}

func TestChannelsCmd_ArchivedFlagPassedToRunFn(t *testing.T) {
	var capturedArchived bool
	runFn := func(_ context.Context, _ tokens.Workspace, _ []string, includeArchived bool) ([]slack.Channel, error) {
		capturedArchived = includeArchived
		return nil, nil
	}

	_, _, err := runChannelsCmd(t, defaultChannelsExtractFn, runFn, "channels", "list", "--archived")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !capturedArchived {
		t.Error("expected includeArchived=true when --archived is passed")
	}
}

func TestChannelsCmd_TableOutputContainsExpectedColumns(t *testing.T) {
	channels := []slack.Channel{
		{ID: "C1", Name: "general", Type: "public_channel", MemberCount: 10, Topic: "Hello"},
	}
	runFn := func(_ context.Context, _ tokens.Workspace, _ []string, _ bool) ([]slack.Channel, error) {
		return channels, nil
	}

	stdout, _, err := runChannelsCmd(t, defaultChannelsExtractFn, runFn, "channels", "list", "--format", "table")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	upper := strings.ToUpper(stdout)
	for _, col := range []string{"ID", "NAME", "TYPE", "MEMBERS", "TOPIC"} {
		if !strings.Contains(upper, col) {
			t.Errorf("expected column %q in table output, not found in:\n%s", col, stdout)
		}
	}
}

func TestChannelsCmd_JSONOutputMatchesSchema(t *testing.T) {
	channels := []slack.Channel{
		{ID: "C01234567", Name: "general", Type: "public_channel", MemberCount: 42, Topic: "Welcome", IsArchived: false},
	}
	runFn := func(_ context.Context, _ tokens.Workspace, _ []string, _ bool) ([]slack.Channel, error) {
		return channels, nil
	}

	stdout, _, err := runChannelsCmd(t, defaultChannelsExtractFn, runFn, "channels", "list", "--format", "json")
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
	for _, field := range []string{"id", "name", "type", "member_count", "topic", "is_archived"} {
		if _, ok := arr[0][field]; !ok {
			t.Errorf("expected JSON field %q in output: %v", field, arr[0])
		}
	}
}

func TestChannelsCmd_InvalidTypeExitsWithCode1(t *testing.T) {
	_, _, err := runChannelsCmd(t, defaultChannelsExtractFn, noopChannelsRunFn, "channels", "list", "--type", "invalid")
	if err == nil {
		t.Fatal("expected error for invalid --type, got nil")
	}
}
