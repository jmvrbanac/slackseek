package slack

import (
	"testing"
)

// T012: ParsePermalink unit tests

func TestParsePermalink_RootPermalink(t *testing.T) {
	url := "https://acme.slack.com/archives/C01234567/p1700000000123456"
	p, err := ParsePermalink(url)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.WorkspaceURL != "https://acme.slack.com" {
		t.Errorf("expected workspace URL 'https://acme.slack.com', got %q", p.WorkspaceURL)
	}
	if p.ChannelID != "C01234567" {
		t.Errorf("expected channel ID 'C01234567', got %q", p.ChannelID)
	}
	if p.ThreadTS != "1700000000.123456" {
		t.Errorf("expected thread TS '1700000000.123456', got %q", p.ThreadTS)
	}
}

func TestParsePermalink_ReplyWithThreadTS(t *testing.T) {
	url := "https://acme.slack.com/archives/C01234567/p1700000001000000?thread_ts=1700000000.123456&cid=C01234567"
	p, err := ParsePermalink(url)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// thread_ts in query string takes precedence for root thread TS
	if p.ThreadTS != "1700000000.123456" {
		t.Errorf("expected thread TS from ?thread_ts param, got %q", p.ThreadTS)
	}
}

func TestParsePermalink_WrongScheme(t *testing.T) {
	_, err := ParsePermalink("http://acme.slack.com/archives/C01234567/p1700000000123456")
	if err == nil {
		t.Fatal("expected error for http:// scheme")
	}
}

func TestParsePermalink_MissingPPrefix(t *testing.T) {
	_, err := ParsePermalink("https://acme.slack.com/archives/C01234567/1700000000123456")
	if err == nil {
		t.Fatal("expected error for missing p prefix in ts segment")
	}
}

func TestParsePermalink_MalformedPath(t *testing.T) {
	_, err := ParsePermalink("https://acme.slack.com/archives/")
	if err == nil {
		t.Fatal("expected error for malformed path")
	}
}

func TestParsePermalink_NonSlackURL(t *testing.T) {
	_, err := ParsePermalink("https://example.com/something")
	if err == nil {
		t.Fatal("expected error for non-archives URL")
	}
}
