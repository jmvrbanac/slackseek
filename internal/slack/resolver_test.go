package slack

import "testing"

// T001: Unit tests for NewResolver, UserDisplayName, and ChannelName.

func TestNewResolver_NilSlicesDoNotPanic(t *testing.T) {
	r := NewResolver(nil, nil)
	if r == nil {
		t.Fatal("expected non-nil resolver from nil slices")
	}
}

func TestNewResolver_EmptySlicesFallBackToRawID(t *testing.T) {
	r := NewResolver([]User{}, []Channel{})
	if got := r.UserDisplayName("U123"); got != "U123" {
		t.Errorf("UserDisplayName: expected raw ID 'U123', got %q", got)
	}
	if got := r.ChannelName("C123"); got != "C123" {
		t.Errorf("ChannelName: expected raw ID 'C123', got %q", got)
	}
}

func TestNewResolver_UserDisplayNamePreferred(t *testing.T) {
	users := []User{
		{ID: "U001", DisplayName: "alice", RealName: "Alice Smith"},
	}
	r := NewResolver(users, nil)
	if got := r.UserDisplayName("U001"); got != "alice" {
		t.Errorf("expected display name 'alice', got %q", got)
	}
}

func TestNewResolver_UserRealNameFallback(t *testing.T) {
	users := []User{
		{ID: "U002", DisplayName: "", RealName: "Bob Jones"},
	}
	r := NewResolver(users, nil)
	if got := r.UserDisplayName("U002"); got != "Bob Jones" {
		t.Errorf("expected real name 'Bob Jones', got %q", got)
	}
}

func TestNewResolver_BothNamesEmptyFallsBackToID(t *testing.T) {
	users := []User{
		{ID: "U003", DisplayName: "", RealName: ""},
	}
	r := NewResolver(users, nil)
	if got := r.UserDisplayName("U003"); got != "U003" {
		t.Errorf("expected raw ID 'U003', got %q", got)
	}
}

func TestNewResolver_UnknownUserIDFallsBackToID(t *testing.T) {
	r := NewResolver(nil, nil)
	if got := r.UserDisplayName("U999"); got != "U999" {
		t.Errorf("expected raw ID 'U999', got %q", got)
	}
}

func TestNewResolver_ChannelLookup(t *testing.T) {
	channels := []Channel{
		{ID: "C001", Name: "general"},
	}
	r := NewResolver(nil, channels)
	if got := r.ChannelName("C001"); got != "general" {
		t.Errorf("expected 'general', got %q", got)
	}
}

func TestNewResolver_UnknownChannelIDFallsBackToID(t *testing.T) {
	r := NewResolver(nil, nil)
	if got := r.ChannelName("C999"); got != "C999" {
		t.Errorf("expected raw ID 'C999', got %q", got)
	}
}

// T016: Resolver built from empty slices returns raw IDs (not empty strings).

func TestNewResolver_EmptySlicesReturnNonEmptyFallback(t *testing.T) {
	r := NewResolver([]User{}, []Channel{})
	if got := r.UserDisplayName("U123"); got != "U123" {
		t.Errorf("expected raw ID 'U123' from empty-slice resolver, got %q", got)
	}
	if got := r.ChannelName("C123"); got != "C123" {
		t.Errorf("expected raw ID 'C123' from empty-slice resolver, got %q", got)
	}
}
