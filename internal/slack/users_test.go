package slack

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmvrbanac/slackseek/internal/cache"
)

func TestResolveUser_SlackIDPassthrough(t *testing.T) {
	id, err := resolveUser(context.Background(), "U01234567", func(_ context.Context) ([]User, error) {
		t.Error("listFn should not be called for a Slack user ID")
		return nil, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "U01234567" {
		t.Errorf("expected U01234567, got %q", id)
	}
}

func TestResolveUser_WorkspaceBotIDPassthrough(t *testing.T) {
	id, err := resolveUser(context.Background(), "W01234567", func(_ context.Context) ([]User, error) {
		t.Error("listFn should not be called for a workspace-app ID")
		return nil, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "W01234567" {
		t.Errorf("expected W01234567, got %q", id)
	}
}

func TestResolveUser_DisplayNameMatch(t *testing.T) {
	users := []User{
		{ID: "U001", DisplayName: "jane.doe", RealName: "Jane Doe"},
		{ID: "U002", DisplayName: "john.smith", RealName: "John Smith"},
	}
	id, err := resolveUser(context.Background(), "jane.doe", func(_ context.Context) ([]User, error) {
		return users, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "U001" {
		t.Errorf("expected U001, got %q", id)
	}
}

func TestResolveUser_RealNameSubstringMatch(t *testing.T) {
	users := []User{
		{ID: "U001", DisplayName: "jdoe", RealName: "Jane Doe"},
	}
	id, err := resolveUser(context.Background(), "Jane", func(_ context.Context) ([]User, error) {
		return users, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "U001" {
		t.Errorf("expected U001, got %q", id)
	}
}

func TestResolveUser_AmbiguousMatch(t *testing.T) {
	users := []User{
		{ID: "U001", DisplayName: "john.doe", RealName: "John Doe"},
		{ID: "U002", DisplayName: "john.smith", RealName: "John Smith"},
	}
	_, err := resolveUser(context.Background(), "john", func(_ context.Context) ([]User, error) {
		return users, nil
	})
	if err == nil {
		t.Fatal("expected error for ambiguous match, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected 'ambiguous' in error message, got: %v", err)
	}
}

// --- ListUsers cache tests (T006) ---

func TestListUsersCached_NilStore_APIAlwaysCalled(t *testing.T) {
	called := 0
	listFn := func(_ context.Context) ([]User, error) {
		called++
		return []User{{ID: "U001", DisplayName: "alice"}}, nil
	}
	users, err := listUsersCached(context.Background(), nil, "", listFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 1 {
		t.Errorf("expected 1 API call, got %d", called)
	}
	if len(users) != 1 || users[0].ID != "U001" {
		t.Errorf("unexpected users: %v", users)
	}
}

func TestListUsersCached_FreshCache_APINotCalled(t *testing.T) {
	store := cache.NewStore(t.TempDir(), time.Hour)
	key := "testkey"
	payload, err := json.Marshal([]User{{ID: "U001", DisplayName: "alice"}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := store.Save(key, "users", payload); err != nil {
		t.Fatalf("Save: %v", err)
	}
	called := 0
	listFn := func(_ context.Context) ([]User, error) {
		called++
		return nil, nil
	}
	users, err := listUsersCached(context.Background(), store, key, listFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 0 {
		t.Errorf("expected no API call on cache hit, got %d", called)
	}
	if len(users) != 1 || users[0].ID != "U001" {
		t.Errorf("expected cached data, got %v", users)
	}
}

func TestListUsersCached_CacheMiss_APICalledAndSaved(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, time.Hour)
	key := "testkey"
	called := 0
	listFn := func(_ context.Context) ([]User, error) {
		called++
		return []User{{ID: "U002", DisplayName: "bob"}}, nil
	}
	users, err := listUsersCached(context.Background(), store, key, listFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 1 {
		t.Errorf("expected 1 API call, got %d", called)
	}
	if len(users) != 1 || users[0].ID != "U002" {
		t.Errorf("unexpected users: %v", users)
	}
	_, hit, _ := store.Load(key, "users")
	if !hit {
		t.Error("expected cache to be written after API call")
	}
}

func TestListUsersCached_StaleCache_APICalledAndOverwritten(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, time.Hour)
	key := "testkey"
	oldPayload, _ := json.Marshal([]User{{ID: "UOLD", DisplayName: "old"}})
	if err := store.Save(key, "users", oldPayload); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path := filepath.Join(dir, key, "users.json")
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
	called := 0
	listFn := func(_ context.Context) ([]User, error) {
		called++
		return []User{{ID: "UNEW", DisplayName: "new"}}, nil
	}
	users, err := listUsersCached(context.Background(), store, key, listFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 1 {
		t.Errorf("expected 1 API call for stale cache, got %d", called)
	}
	if len(users) != 1 || users[0].ID != "UNEW" {
		t.Errorf("expected fresh data, got %v", users)
	}
}

func TestResolveUser_NoMatch(t *testing.T) {
	users := []User{
		{ID: "U001", DisplayName: "jane.doe", RealName: "Jane Doe"},
	}
	_, err := resolveUser(context.Background(), "notexist", func(_ context.Context) ([]User, error) {
		return users, nil
	})
	if err == nil {
		t.Fatal("expected error for no match, got nil")
	}
}
