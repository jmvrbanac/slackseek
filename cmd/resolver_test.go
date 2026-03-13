package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmvrbanac/slackseek/internal/cache"
	"github.com/jmvrbanac/slackseek/internal/tokens"
)

// T027: --refresh-cache clears the workspace dir, triggering a cold start for
// entity caches (users, channels, groups). This verifies the mechanism used by
// buildCacheStore when flagRefreshCache is true.

func TestBuildCacheStore_RefreshCacheClearsWorkspaceDir(t *testing.T) {
	dir := t.TempDir()
	ws := tokens.Workspace{URL: "https://test.slack.com"}
	key := cache.WorkspaceKey(ws.URL)
	store := cache.NewStore(dir, time.Hour)

	// Seed entity cache files simulating a warm cache.
	for _, kind := range []string{"users", "channels", "user_groups"} {
		data, _ := json.Marshal([]map[string]string{{"id": "X001"}})
		if err := store.Save(key, kind, data); err != nil {
			t.Fatalf("Save %s: %v", kind, err)
		}
	}
	usersFile := filepath.Join(dir, key, "users.json")
	if _, err := os.Stat(usersFile); err != nil {
		t.Fatalf("expected users.json to exist before clear: %v", err)
	}

	// --refresh-cache calls store.Clear(key) before building the resolver.
	if err := store.Clear(key); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	// After clear, all entity files must be gone (cold start).
	for _, kind := range []string{"users", "channels", "user_groups"} {
		_, hit, _ := store.LoadStable(key, kind)
		if hit {
			t.Errorf("expected %s cache to be gone after Clear, but got a hit", kind)
		}
	}
}

func TestBuildCacheStore_RefreshCacheColdStartTriggersAPIOnListCached(t *testing.T) {
	dir := t.TempDir()
	ws := tokens.Workspace{URL: "https://test.slack.com"}
	key := cache.WorkspaceKey(ws.URL)
	store := cache.NewStore(dir, time.Hour)

	// Seed users cache (simulates previous warm run).
	data, _ := json.Marshal([]map[string]string{{"id": "U001"}})
	_ = store.Save(key, "users", data)

	// Simulate --refresh-cache: clear workspace dir.
	_ = store.Clear(key)

	// After clear, LoadStable must return a miss — listUsersCached would call the API.
	_, hit, _ := store.LoadStable(key, "users")
	if hit {
		t.Error("expected cold start (miss) after --refresh-cache clear, got cache hit")
	}
}
