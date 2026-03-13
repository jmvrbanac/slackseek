package mcp

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jmvrbanac/slackseek/internal/tokens"
)

func makeWorkspace(name string) tokens.Workspace {
	return tokens.Workspace{Name: name, URL: "https://" + name + ".slack.com", Token: "xoxc-test"}
}

func successExtract(name string) func() (tokens.TokenExtractionResult, error) {
	return func() (tokens.TokenExtractionResult, error) {
		return tokens.TokenExtractionResult{Workspaces: []tokens.Workspace{makeWorkspace(name)}}, nil
	}
}

func errorExtract(msg string) func() (tokens.TokenExtractionResult, error) {
	return func() (tokens.TokenExtractionResult, error) {
		return tokens.TokenExtractionResult{}, errors.New(msg)
	}
}

// TestTokenCache_GetPopulatesOnFirstCall verifies get() calls extractFn on empty cache.
func TestTokenCache_GetPopulatesOnFirstCall(t *testing.T) {
	calls := 0
	tc := &tokenCache{
		extractFn: func() (tokens.TokenExtractionResult, error) {
			calls++
			return tokens.TokenExtractionResult{Workspaces: []tokens.Workspace{makeWorkspace("acme")}}, nil
		},
	}
	ws, err := tc.get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ws) != 1 || ws[0].Name != "acme" {
		t.Fatalf("unexpected workspaces: %+v", ws)
	}
	if calls != 1 {
		t.Fatalf("expected 1 extractFn call, got %d", calls)
	}
}

// TestTokenCache_GetHitsWithinTTL verifies get() does not re-extract when within TTL.
func TestTokenCache_GetHitsWithinTTL(t *testing.T) {
	calls := 0
	tc := &tokenCache{
		workspaces: []tokens.Workspace{makeWorkspace("acme")},
		fetchedAt:  time.Now(),
		extractFn: func() (tokens.TokenExtractionResult, error) {
			calls++
			return tokens.TokenExtractionResult{Workspaces: []tokens.Workspace{makeWorkspace("new")}}, nil
		},
	}
	ws, err := tc.get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws[0].Name != "acme" {
		t.Fatalf("expected cached workspace, got %q", ws[0].Name)
	}
	if calls != 0 {
		t.Fatalf("expected 0 extractFn calls within TTL, got %d", calls)
	}
}

// TestTokenCache_GetRefreshesAfterTTL verifies get() re-extracts when TTL expires.
func TestTokenCache_GetRefreshesAfterTTL(t *testing.T) {
	calls := 0
	tc := &tokenCache{
		workspaces: []tokens.Workspace{makeWorkspace("old")},
		fetchedAt:  time.Now().Add(-tokenTTL - time.Second),
		extractFn: func() (tokens.TokenExtractionResult, error) {
			calls++
			return tokens.TokenExtractionResult{Workspaces: []tokens.Workspace{makeWorkspace("fresh")}}, nil
		},
	}
	ws, err := tc.get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws[0].Name != "fresh" {
		t.Fatalf("expected refreshed workspace, got %q", ws[0].Name)
	}
	if calls != 1 {
		t.Fatalf("expected 1 extractFn call after TTL, got %d", calls)
	}
}

// TestTokenCache_GetPropagatesExtractError verifies get() returns error when extractFn fails.
func TestTokenCache_GetPropagatesExtractError(t *testing.T) {
	tc := &tokenCache{extractFn: errorExtract("no creds")}
	_, err := tc.get()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, err) || err.Error() == "" {
		t.Fatalf("expected non-empty error, got %v", err)
	}
}

// TestTokenCache_RefreshAlwaysReExtracts verifies refresh() calls extractFn even within TTL.
func TestTokenCache_RefreshAlwaysReExtracts(t *testing.T) {
	calls := 0
	tc := &tokenCache{
		workspaces: []tokens.Workspace{makeWorkspace("old")},
		fetchedAt:  time.Now(),
		extractFn: func() (tokens.TokenExtractionResult, error) {
			calls++
			return tokens.TokenExtractionResult{Workspaces: []tokens.Workspace{makeWorkspace("refreshed")}}, nil
		},
	}
	ws, err := tc.refresh()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws[0].Name != "refreshed" {
		t.Fatalf("expected refreshed workspace, got %q", ws[0].Name)
	}
	if calls != 1 {
		t.Fatalf("expected 1 extractFn call from refresh, got %d", calls)
	}
}

// TestTokenCache_RefreshPropagatesError verifies refresh() returns error when extractFn fails.
func TestTokenCache_RefreshPropagatesError(t *testing.T) {
	tc := &tokenCache{
		workspaces: []tokens.Workspace{makeWorkspace("old")},
		fetchedAt:  time.Now(),
		extractFn:  errorExtract("auth failed"),
	}
	_, err := tc.refresh()
	if err == nil {
		t.Fatal("expected error from refresh, got nil")
	}
}

// TestTokenCache_ConcurrentGet verifies no data race on concurrent get() calls.
func TestTokenCache_ConcurrentGet(t *testing.T) {
	tc := &tokenCache{extractFn: successExtract("acme")}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := tc.get(); err != nil {
				t.Errorf("concurrent get error: %v", err)
			}
		}()
	}
	wg.Wait()
}
