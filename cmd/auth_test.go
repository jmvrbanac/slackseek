package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jmvrbanac/slackseek/internal/tokens"
)

// runAuthCmd builds a fresh root command with the given extractFn injected
// into the auth subcommand, then executes the provided args and returns
// stdout, stderr, and the error (if any).
func runAuthCmd(t *testing.T, fn func() (tokens.TokenExtractionResult, error), args ...string) (stdout, stderr string, err error) {
	t.Helper()
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	root := NewRootCmd()
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	addAuthCmd(root, fn)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

// twoWorkspaces is a fixture with a single workspace containing a full token
// and cookie.
func oneWorkspace() tokens.TokenExtractionResult {
	return tokens.TokenExtractionResult{
		Workspaces: []tokens.Workspace{
			{
				Name:   "Acme Corp",
				URL:    "https://acme.slack.com",
				Token:  "xoxs-111-222-333-444444",
				Cookie: "abcdef12",
			},
		},
	}
}

func TestAuthShow_TableOutput(t *testing.T) {
	stdout, _, err := runAuthCmd(t,
		func() (tokens.TokenExtractionResult, error) { return oneWorkspace(), nil },
		"auth", "show", "--format", "table",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// tablewriter uppercases header cells by default.
	for _, col := range []string{"NAME", "URL", "TOKEN", "COOKIE"} {
		if !strings.Contains(stdout, col) {
			t.Errorf("expected column %q in table output, stdout:\n%s", col, stdout)
		}
	}
	if !strings.Contains(stdout, "Acme Corp") {
		t.Errorf("expected workspace name in output, stdout:\n%s", stdout)
	}
}

func TestAuthShow_JSONOutput(t *testing.T) {
	stdout, _, err := runAuthCmd(t,
		func() (tokens.TokenExtractionResult, error) { return oneWorkspace(), nil },
		"auth", "show", "--format", "json",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result []map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(stdout), &result); jsonErr != nil {
		t.Fatalf("invalid JSON output: %v\nstdout: %s", jsonErr, stdout)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 JSON element, got %d", len(result))
	}
	for _, field := range []string{"name", "url", "token", "cookie"} {
		if _, ok := result[0][field]; !ok {
			t.Errorf("expected JSON field %q, got: %v", field, result[0])
		}
	}
}

func TestAuthShow_TokenAndCookieTruncated(t *testing.T) {
	stdout, _, err := runAuthCmd(t,
		func() (tokens.TokenExtractionResult, error) { return oneWorkspace(), nil },
		"auth", "show", "--format", "text",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Full token is "xoxs-111-222-333-444444"; truncated to 12 chars = "xoxs-111-222"
	if strings.Contains(stdout, "333-444444") {
		t.Error("full token should not appear in text output")
	}
}

func TestAuthExport_OutputFormat(t *testing.T) {
	stdout, _, err := runAuthCmd(t,
		func() (tokens.TokenExtractionResult, error) { return oneWorkspace(), nil },
		"auth", "export",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Expect: export SLACK_TOKEN_ACME_CORP=xoxs-111-222-333-444444
	if !strings.Contains(stdout, "export SLACK_TOKEN_") {
		t.Errorf("expected 'export SLACK_TOKEN_*' prefix, stdout:\n%s", stdout)
	}
	// Full token must appear in export output (not truncated).
	if !strings.Contains(stdout, "xoxs-111-222-333-444444") {
		t.Errorf("expected full token in export output, stdout:\n%s", stdout)
	}
}

func TestAuthShow_ExtractionFailure(t *testing.T) {
	_, stderr, err := runAuthCmd(t,
		func() (tokens.TokenExtractionResult, error) {
			return tokens.TokenExtractionResult{}, errors.New("no Slack installation found")
		},
		"auth", "show",
	)
	if err == nil {
		t.Fatal("expected non-zero exit for extraction failure")
	}
	if !strings.Contains(stderr, "no Slack installation found") {
		t.Errorf("expected actionable error in stderr, got: %q", stderr)
	}
}
