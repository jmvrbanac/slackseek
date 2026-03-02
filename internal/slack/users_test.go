package slack

import (
	"context"
	"strings"
	"testing"
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
