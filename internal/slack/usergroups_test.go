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

// ugRoundTripper is a function-based RoundTripper for mocking HTTP in user-group tests.
type ugRoundTripper func(*http.Request) (*http.Response, error)

func (f ugRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func mockUGResponse(body string) *http.Client {
	return &http.Client{Transport: ugRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}
}

// T026(a): successful response returns slice of UserGroup with correct fields.
func TestListUserGroups_SuccessReturnsCorrectFields(t *testing.T) {
	body := `{"ok":true,"usergroups":[{"id":"S001","handle":"eng","name":"Engineering"}]}`
	c := NewClient("token", "", mockUGResponse(body))
	groups, err := c.ListUserGroups(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	g := groups[0]
	if g.ID != "S001" {
		t.Errorf("expected ID 'S001', got %q", g.ID)
	}
	if g.Handle != "eng" {
		t.Errorf("expected Handle 'eng', got %q", g.Handle)
	}
	if g.Name != "Engineering" {
		t.Errorf("expected Name 'Engineering', got %q", g.Name)
	}
}

// T026(b): response with "ok": false returns an error.
func TestListUserGroups_OkFalseReturnsError(t *testing.T) {
	body := `{"ok":false,"error":"not_authed"}`
	c := NewClient("token", "", mockUGResponse(body))
	_, err := c.ListUserGroups(context.Background())
	if err == nil {
		t.Fatal("expected error for ok=false, got nil")
	}
}

// T026(c): empty usergroups array returns an empty slice without error.
func TestListUserGroups_EmptyArrayReturnsEmptySlice(t *testing.T) {
	body := `{"ok":true,"usergroups":[]}`
	c := NewClient("token", "", mockUGResponse(body))
	groups, err := c.ListUserGroups(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
}

// --- listUserGroupsCached direct unit tests ---

func TestListUserGroupsCached_NilStore_APIAlwaysCalled(t *testing.T) {
	called := 0
	listFn := func(_ context.Context) ([]UserGroup, error) {
		called++
		return []UserGroup{{ID: "S001", Handle: "eng"}}, nil
	}
	groups, err := listUserGroupsCached(context.Background(), nil, "", listFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 1 {
		t.Errorf("expected 1 API call, got %d", called)
	}
	if len(groups) != 1 || groups[0].ID != "S001" {
		t.Errorf("unexpected groups: %v", groups)
	}
}

func TestListUserGroupsCached_FreshCache_APINotCalled(t *testing.T) {
	store := cache.NewStore(t.TempDir(), time.Hour)
	key := "testkey"
	payload, _ := json.Marshal([]UserGroup{{ID: "S001", Handle: "eng"}})
	if err := store.Save(key, "user_groups", payload); err != nil {
		t.Fatalf("Save: %v", err)
	}
	called := 0
	listFn := func(_ context.Context) ([]UserGroup, error) {
		called++
		return nil, nil
	}
	groups, err := listUserGroupsCached(context.Background(), store, key, listFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 0 {
		t.Errorf("expected no API call on cache hit, got %d", called)
	}
	if len(groups) != 1 || groups[0].ID != "S001" {
		t.Errorf("expected cached data, got %v", groups)
	}
}

// T012: stale cache (past TTL) must be returned as a hit — LoadStable ignores TTL.
func TestListUserGroupsCached_StaleCache_ReturnedAsCacheHit(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, time.Hour)
	key := "testkey"
	oldPayload, _ := json.Marshal([]UserGroup{{ID: "SOLD", Handle: "old-team"}})
	if err := store.Save(key, "user_groups", oldPayload); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path := filepath.Join(dir, key, "user_groups.json")
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
	called := 0
	listFn := func(_ context.Context) ([]UserGroup, error) {
		called++
		return []UserGroup{{ID: "SNEW", Handle: "new-team"}}, nil
	}
	groups, err := listUserGroupsCached(context.Background(), store, key, listFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 0 {
		t.Errorf("expected 0 API calls for stale cache (TTL ignored), got %d", called)
	}
	if len(groups) != 1 || groups[0].ID != "SOLD" {
		t.Errorf("expected stale cached data to be returned, got %v", groups)
	}
}
