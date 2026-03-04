package slack

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
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
