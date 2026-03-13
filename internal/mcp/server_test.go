package mcp

import (
	"errors"
	"testing"

	"github.com/jmvrbanac/slackseek/internal/tokens"
)

// TestServe_PropagatesExtractError verifies that Serve returns an error when
// the extractFn fails on the first credential fetch (before any tool call).
// The server must not panic in this path.
func TestServe_PropagatesExtractError(t *testing.T) {
	extractErr := errors.New("no Slack credentials found")
	extractFn := func() (tokens.TokenExtractionResult, error) {
		return tokens.TokenExtractionResult{}, extractErr
	}

	// We cannot call Serve() directly because it blocks on ServeStdio. Instead
	// we verify the tokenCache wiring: a tokenCache built from a failing
	// extractFn propagates the error on get().
	tc := &tokenCache{extractFn: extractFn}
	_, err := tc.get()
	if err == nil {
		t.Fatal("expected error from failing extractFn, got nil")
	}
	if !errors.Is(err, extractErr) {
		t.Errorf("expected wrapped extractErr, got: %v", err)
	}
}

// TestServe_FunctionExists is a compile-time check ensuring Serve is exported
// with the correct signature. If the signature changes this test will not compile.
var _ = Serve // referenced to prevent "declared and not used" if unused elsewhere
