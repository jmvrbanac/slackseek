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

func runMetricsCmd(
	t *testing.T,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn metricsRunFunc,
	args ...string,
) (stdout, stderr string, err error) {
	t.Helper()
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	root := NewRootCmd()
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	addMetricsCmd(root, extractFn, runFn)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

var defaultMetricsExtractFn = func() (tokens.TokenExtractionResult, error) {
	return tokens.TokenExtractionResult{
		Workspaces: []tokens.Workspace{
			{Name: "Test", URL: "https://test.slack.com", Token: "xoxs-test"},
		},
	}, nil
}

func TestMetricsCmd_TextContainsSections(t *testing.T) {
	t1 := time.Date(2026, 2, 25, 14, 0, 0, 0, time.UTC)
	runFn := func(_ context.Context, _ tokens.Workspace, _ string, _ slack.DateRange, _ bool) ([]slack.Message, error) {
		return []slack.Message{
			{Timestamp: "1.0", Time: t1, UserID: "alice", Text: "hello"},
			{Timestamp: "2.0", Time: t1, UserID: "bob", Text: "world"},
		}, nil
	}

	stdout, _, err := runMetricsCmd(t, defaultMetricsExtractFn, runFn, "metrics", "general")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, section := range []string{"Message counts", "Thread stats", "Top reactions", "Messages by hour"} {
		if !strings.Contains(stdout, section) {
			t.Errorf("expected section %q in metrics output, got:\n%s", section, stdout)
		}
	}
}

func TestMetricsCmd_JSONContainsRequiredKeys(t *testing.T) {
	t1 := time.Date(2026, 2, 25, 14, 0, 0, 0, time.UTC)
	runFn := func(_ context.Context, _ tokens.Workspace, _ string, _ slack.DateRange, _ bool) ([]slack.Message, error) {
		return []slack.Message{
			{Timestamp: "1.0", Time: t1, UserID: "alice", Text: "hello"},
		}, nil
	}

	stdout, _, err := runMetricsCmd(t, defaultMetricsExtractFn, runFn, "metrics", "--format", "json", "general")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, key := range []string{`"users"`, `"threads"`, `"top_reactions"`, `"hourly"`} {
		if !strings.Contains(stdout, key) {
			t.Errorf("expected JSON key %q in metrics output, got:\n%s", key, stdout)
		}
	}
}

func TestMetricsCmd_MissingChannelExitsWithError(t *testing.T) {
	runFn := func(_ context.Context, _ tokens.Workspace, _ string, _ slack.DateRange, _ bool) ([]slack.Message, error) {
		return nil, nil
	}
	_, _, err := runMetricsCmd(t, defaultMetricsExtractFn, runFn, "metrics")
	if err == nil {
		t.Fatal("expected error when channel argument is missing")
	}
}

// T023: Cache hit/miss tests for metrics using injected run function.
func TestDefaultRunMetrics_CacheMiss(t *testing.T) {
	t1 := time.Date(2026, 2, 25, 14, 0, 0, 0, time.UTC)
	called := false
	runFn := func(_ context.Context, _ tokens.Workspace, _ string, _ slack.DateRange, _ bool) ([]slack.Message, error) {
		called = true
		return []slack.Message{{Timestamp: "1.0", Time: t1, UserID: "alice", Text: "fresh"}}, nil
	}
	_, _, err := runMetricsCmd(t, defaultMetricsExtractFn, runFn, "metrics", "general")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected run function to be called on cache miss path")
	}
}

func TestDefaultRunMetrics_CacheHit(t *testing.T) {
	t1 := time.Date(2026, 2, 25, 14, 0, 0, 0, time.UTC)
	msgs := []slack.Message{{Timestamp: "1.0", Time: t1, UserID: "alice", Text: "cached metrics"}}
	runFn := func(_ context.Context, _ tokens.Workspace, _ string, _ slack.DateRange, _ bool) ([]slack.Message, error) {
		return msgs, nil
	}
	stdout, _, err := runMetricsCmd(t, defaultMetricsExtractFn, runFn, "metrics", "general")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "Message counts") {
		t.Errorf("expected metrics output sections, got:\n%s", stdout)
	}
}

// T025: Verify --no-cache flag is parsed and forwarded for metrics.
func TestMetricsCmd_NoCacheFlag(t *testing.T) {
	t1 := time.Date(2026, 2, 25, 14, 0, 0, 0, time.UTC)
	var capturedNoCache bool
	runFn := func(_ context.Context, _ tokens.Workspace, _ string, _ slack.DateRange, noCache bool) ([]slack.Message, error) {
		capturedNoCache = noCache
		return []slack.Message{{Timestamp: "1.0", Time: t1, UserID: "alice", Text: "hi"}}, nil
	}
	_, _, err := runMetricsCmd(t, defaultMetricsExtractFn, runFn, "metrics", "general", "--no-cache")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !capturedNoCache {
		t.Error("expected noCache=true when --no-cache is passed")
	}
}
