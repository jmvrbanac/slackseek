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

func TestNewResolver_UserRealNamePreferred(t *testing.T) {
	users := []User{
		{ID: "U001", DisplayName: "alice", RealName: "Alice Smith"},
	}
	r := NewResolver(users, nil)
	if got := r.UserDisplayName("U001"); got != "Alice Smith" {
		t.Errorf("expected real name 'Alice Smith', got %q", got)
	}
}

func TestNewResolver_UserDisplayNameFallback(t *testing.T) {
	users := []User{
		{ID: "U002", DisplayName: "bob", RealName: ""},
	}
	r := NewResolver(users, nil)
	if got := r.UserDisplayName("U002"); got != "bob" {
		t.Errorf("expected display name 'bob', got %q", got)
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

func TestResolveMentions_ReplacesKnownUserID(t *testing.T) {
	users := []User{{ID: "U001", RealName: "Alice Smith"}}
	r := NewResolver(users, nil)
	got := r.ResolveMentions("Hello <@U001>, how are you?")
	want := "Hello @Alice Smith, how are you?"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveMentions_FallsBackToIDForUnknownUser(t *testing.T) {
	r := NewResolver(nil, nil)
	got := r.ResolveMentions("Hello <@U999>!")
	want := "Hello @U999!"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveMentions_NoMentionsUnchanged(t *testing.T) {
	r := NewResolver(nil, nil)
	got := r.ResolveMentions("just plain text")
	if got != "just plain text" {
		t.Errorf("got %q, want %q", got, "just plain text")
	}
}

func TestResolveMentions_MultipleMentions(t *testing.T) {
	users := []User{
		{ID: "U001", RealName: "Alice"},
		{ID: "U002", RealName: "Bob"},
	}
	r := NewResolver(users, nil)
	got := r.ResolveMentions("<@U001> and <@U002>")
	want := "@Alice and @Bob"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
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
