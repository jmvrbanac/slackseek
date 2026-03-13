package slack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
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

// T010: stale cache (past TTL) must be returned as a hit — LoadStable ignores TTL.
func TestListUsersCached_StaleCache_ReturnedAsCacheHit(t *testing.T) {
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
	if called != 0 {
		t.Errorf("expected 0 API calls for stale cache (TTL ignored), got %d", called)
	}
	if len(users) != 1 || users[0].ID != "UOLD" {
		t.Errorf("expected stale cached data to be returned, got %v", users)
	}
}

// --- FetchUser tests (T016) ---

// mockUserInfoResponse returns an *http.Client whose transport returns body on every request.
// Reuses the ugRoundTripper type defined in usergroups_test.go (same package).
func mockUserInfoResponse(body string) *http.Client {
	return &http.Client{Transport: ugRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}
}

func TestFetchUser_SingleAPICallAndCorrectFields(t *testing.T) {
	body := `{"ok":true,"user":{"id":"U001","real_name":"Alice Smith","deleted":false,"is_bot":false,"profile":{"display_name":"alice","email":"alice@example.com"}}}`
	c := NewClient("token", "", mockUserInfoResponse(body))
	u, err := c.FetchUser(context.Background(), "U001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID != "U001" {
		t.Errorf("expected ID 'U001', got %q", u.ID)
	}
	if u.RealName != "Alice Smith" {
		t.Errorf("expected RealName 'Alice Smith', got %q", u.RealName)
	}
	if u.DisplayName != "alice" {
		t.Errorf("expected DisplayName 'alice', got %q", u.DisplayName)
	}
	if u.Email != "alice@example.com" {
		t.Errorf("expected Email 'alice@example.com', got %q", u.Email)
	}
}

func TestFetchUser_CacheMerge(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, time.Hour)
	key := "testkey"
	// Pre-seed cache with an existing user.
	existing, _ := json.Marshal([]User{{ID: "U000", RealName: "Existing User"}})
	_ = store.Save(key, "users", existing)

	body := `{"ok":true,"user":{"id":"U001","real_name":"Alice Smith","deleted":false,"is_bot":false,"profile":{"display_name":"alice","email":""}}}`
	c := NewClientWithCache("token", "", mockUserInfoResponse(body), store, key)
	_, err := c.FetchUser(context.Background(), "U001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The cache file must now contain both the existing user and the new one.
	data, hit, _ := store.LoadStable(key, "users")
	if !hit {
		t.Fatal("expected cache file to exist after FetchUser")
	}
	var users []User
	if err := json.Unmarshal(data, &users); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	found := false
	for _, u := range users {
		if u.ID == "U001" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected U001 to be present in cache after FetchUser, got %+v", users)
	}
	if len(users) < 2 {
		t.Errorf("expected at least 2 users (existing + new), got %d", len(users))
	}
}

func TestFetchUser_MergeReplacesExistingEntry(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, time.Hour)
	key := "testkey"
	old, _ := json.Marshal([]User{{ID: "U001", RealName: "Old Name"}})
	_ = store.Save(key, "users", old)

	body := `{"ok":true,"user":{"id":"U001","real_name":"New Name","deleted":false,"is_bot":false,"profile":{"display_name":"u1","email":""}}}`
	c := NewClientWithCache("token", "", mockUserInfoResponse(body), store, key)
	_, err := c.FetchUser(context.Background(), "U001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _, _ := store.LoadStable(key, "users")
	var users []User
	_ = json.Unmarshal(data, &users)
	if len(users) != 1 {
		t.Errorf("expected exactly 1 user after replace, got %d", len(users))
	}
	if users[0].RealName != "New Name" {
		t.Errorf("expected updated RealName 'New Name', got %q", users[0].RealName)
	}
}

func TestFetchUser_APIError_ReturnsError(t *testing.T) {
	body := `{"ok":false,"error":"user_not_found"}`
	c := NewClient("token", "", mockUserInfoResponse(body))
	_, err := c.FetchUser(context.Background(), "UBAD")
	if err == nil {
		t.Fatal("expected error for ok=false response, got nil")
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
