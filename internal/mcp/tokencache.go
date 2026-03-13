package mcp

import (
	"fmt"
	"sync"
	"time"

	"github.com/jmvrbanac/slackseek/internal/tokens"
)

// tokenTTL is the proactive refresh interval. Credentials older than this are
// re-extracted even if no 401 has been observed, guarding against silent token
// degradation in long-running server sessions.
const tokenTTL = 5 * time.Minute

// tokenCache is a thread-safe in-memory cache for Slack workspace credentials.
// It prevents re-extraction on every tool call while ensuring stale credentials
// are refreshed proactively (TTL) or reactively (on 401, call refresh()).
type tokenCache struct {
	mu         sync.Mutex
	workspaces []tokens.Workspace
	fetchedAt  time.Time
	extractFn  func() (tokens.TokenExtractionResult, error)
}

// get returns cached workspaces if within TTL, otherwise calls refresh().
// The lock is held only for the cache read/write, not during extraction.
func (tc *tokenCache) get() ([]tokens.Workspace, error) {
	tc.mu.Lock()
	if !tc.fetchedAt.IsZero() && time.Since(tc.fetchedAt) < tokenTTL {
		ws := tc.workspaces
		tc.mu.Unlock()
		return ws, nil
	}
	tc.mu.Unlock()
	return tc.refresh()
}

// refresh unconditionally re-extracts credentials and updates the cache.
// It is called by get() on TTL miss and by tool handlers on HTTP 401.
func (tc *tokenCache) refresh() ([]tokens.Workspace, error) {
	result, err := tc.extractFn()
	if err != nil {
		return nil, fmt.Errorf("extracting Slack credentials: %w", err)
	}
	tc.mu.Lock()
	tc.workspaces = result.Workspaces
	tc.fetchedAt = time.Now()
	tc.mu.Unlock()
	return result.Workspaces, nil
}
