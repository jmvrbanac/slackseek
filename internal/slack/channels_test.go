package slack

import (
	"context"
	"strings"
	"testing"
	"time"

	slackgo "github.com/slack-go/slack"
)

// --- resolveChannel tests ---

func TestResolveChannel_SlackIDPassthrough(t *testing.T) {
	id, err := resolveChannel(context.Background(), "C01234567", func(_ context.Context) ([]Channel, error) {
		t.Error("listFn should not be called for a Slack channel ID")
		return nil, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "C01234567" {
		t.Errorf("expected C01234567, got %q", id)
	}
}

func TestResolveChannel_NameMatchReturnsID(t *testing.T) {
	channels := []Channel{
		{ID: "C001", Name: "general"},
		{ID: "C002", Name: "random"},
	}
	id, err := resolveChannel(context.Background(), "general", func(_ context.Context) ([]Channel, error) {
		return channels, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "C001" {
		t.Errorf("expected C001, got %q", id)
	}
}

func TestResolveChannel_NameMatchCaseInsensitive(t *testing.T) {
	channels := []Channel{
		{ID: "C001", Name: "general"},
	}
	// Use mixed-case to avoid matching the Slack channel ID pattern (all-uppercase).
	id, err := resolveChannel(context.Background(), "General", func(_ context.Context) ([]Channel, error) {
		return channels, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "C001" {
		t.Errorf("expected C001, got %q", id)
	}
}

func TestResolveChannel_AmbiguousMatchReturnsError(t *testing.T) {
	channels := []Channel{
		{ID: "C001", Name: "general"},
		{ID: "C002", Name: "general"},
	}
	_, err := resolveChannel(context.Background(), "general", func(_ context.Context) ([]Channel, error) {
		return channels, nil
	})
	if err == nil {
		t.Fatal("expected error for ambiguous match, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected 'ambiguous' in error message, got: %v", err)
	}
}

func TestResolveChannel_NoMatchReturnsError(t *testing.T) {
	channels := []Channel{
		{ID: "C001", Name: "general"},
	}
	_, err := resolveChannel(context.Background(), "notexist", func(_ context.Context) ([]Channel, error) {
		return channels, nil
	})
	if err == nil {
		t.Fatal("expected error for no match, got nil")
	}
	if !strings.Contains(err.Error(), "notexist") {
		t.Errorf("expected channel name in error message, got: %v", err)
	}
}

// --- fetchHistory tests ---

func makeSlackMsg(ts, user, text string, replyCount int) slackgo.Message {
	var m slackgo.Message
	m.Timestamp = ts
	m.User = user
	m.Text = text
	m.ReplyCount = replyCount
	return m
}

func noopReplFn(_ context.Context, _, _, _ string) ([]slackgo.Message, bool, string, error) {
	return nil, false, "", nil
}

func TestFetchHistory_DateRangeToUnixStrings(t *testing.T) {
	from := time.Unix(1700000000, 0).UTC()
	to := time.Unix(1700100000, 0).UTC()
	dr := DateRange{From: &from, To: &to}

	var capturedOldest, capturedLatest string
	histFn := func(_ context.Context, _, oldest, latest, _ string) ([]slackgo.Message, bool, string, error) {
		capturedOldest = oldest
		capturedLatest = latest
		return nil, false, "", nil
	}

	_, err := fetchHistory(context.Background(), "C123", dr, 0, false, histFn, noopReplFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedOldest != "1700000000" {
		t.Errorf("expected oldest=1700000000, got %q", capturedOldest)
	}
	if capturedLatest != "1700100000" {
		t.Errorf("expected latest=1700100000, got %q", capturedLatest)
	}
}

func TestFetchHistory_NilDateRangePassesEmptyStrings(t *testing.T) {
	dr := DateRange{}
	var capturedOldest, capturedLatest string
	histFn := func(_ context.Context, _, oldest, latest, _ string) ([]slackgo.Message, bool, string, error) {
		capturedOldest = oldest
		capturedLatest = latest
		return nil, false, "", nil
	}

	_, err := fetchHistory(context.Background(), "C123", dr, 0, false, histFn, noopReplFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedOldest != "" {
		t.Errorf("expected empty oldest string, got %q", capturedOldest)
	}
	if capturedLatest != "" {
		t.Errorf("expected empty latest string, got %q", capturedLatest)
	}
}

func TestFetchHistory_ThreadRepliesInterleavedAfterParent(t *testing.T) {
	parent := makeSlackMsg("100.000000", "U1", "parent", 1)
	reply := makeSlackMsg("101.000000", "U2", "reply", 0)

	histFn := func(_ context.Context, _, _, _, _ string) ([]slackgo.Message, bool, string, error) {
		return []slackgo.Message{parent}, false, "", nil
	}
	replFn := func(_ context.Context, _, _, _ string) ([]slackgo.Message, bool, string, error) {
		// conversations.replies includes parent as first element
		return []slackgo.Message{parent, reply}, false, "", nil
	}

	msgs, err := fetchHistory(context.Background(), "C123", DateRange{}, 0, true, histFn, replFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (root + reply), got %d", len(msgs))
	}
	if msgs[0].ThreadDepth != 0 {
		t.Errorf("expected root ThreadDepth=0, got %d", msgs[0].ThreadDepth)
	}
	if msgs[1].ThreadDepth != 1 {
		t.Errorf("expected reply ThreadDepth=1, got %d", msgs[1].ThreadDepth)
	}
	// reply appears after parent (sorted ascending)
	if msgs[0].Timestamp >= msgs[1].Timestamp {
		t.Errorf("expected parent before reply; got %s then %s", msgs[0].Timestamp, msgs[1].Timestamp)
	}
}

func TestFetchHistory_ThreadsFalseSkipsReplies(t *testing.T) {
	parent := makeSlackMsg("100.000000", "U1", "parent", 2)

	repliesCalled := false
	histFn := func(_ context.Context, _, _, _, _ string) ([]slackgo.Message, bool, string, error) {
		return []slackgo.Message{parent}, false, "", nil
	}
	replFn := func(_ context.Context, _, _, _ string) ([]slackgo.Message, bool, string, error) {
		repliesCalled = true
		return nil, false, "", nil
	}

	msgs, err := fetchHistory(context.Background(), "C123", DateRange{}, 0, false, histFn, replFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repliesCalled {
		t.Error("replies fetcher should not be called when threads=false")
	}
	if len(msgs) != 1 {
		t.Errorf("expected 1 message, got %d", len(msgs))
	}
}

func TestFetchHistory_LimitRespectedAcrossRootAndReplies(t *testing.T) {
	m1 := makeSlackMsg("100.000000", "U1", "first", 2)
	reply1 := makeSlackMsg("101.000000", "U2", "r1", 0)
	reply2 := makeSlackMsg("102.000000", "U2", "r2", 0)
	m2 := makeSlackMsg("200.000000", "U1", "second", 0)

	histFn := func(_ context.Context, _, _, _, _ string) ([]slackgo.Message, bool, string, error) {
		// API returns newest-first
		return []slackgo.Message{m2, m1}, false, "", nil
	}
	replFn := func(_ context.Context, _, threadTS, _ string) ([]slackgo.Message, bool, string, error) {
		if threadTS == "100.000000" {
			return []slackgo.Message{m1, reply1, reply2}, false, "", nil
		}
		return nil, false, "", nil
	}

	msgs, err := fetchHistory(context.Background(), "C123", DateRange{}, 2, true, histFn, replFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) > 2 {
		t.Errorf("expected at most 2 messages (limit=2), got %d", len(msgs))
	}
}

func TestFetchHistory_MessagesSortedAscendingByTimestamp(t *testing.T) {
	// History API returns newest-first
	m1 := makeSlackMsg("200.000000", "U1", "second", 0)
	m2 := makeSlackMsg("100.000000", "U1", "first", 0)

	histFn := func(_ context.Context, _, _, _, _ string) ([]slackgo.Message, bool, string, error) {
		return []slackgo.Message{m1, m2}, false, "", nil
	}

	msgs, err := fetchHistory(context.Background(), "C123", DateRange{}, 0, false, histFn, noopReplFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Timestamp != "100.000000" {
		t.Errorf("expected oldest message first (100.000000), got %q", msgs[0].Timestamp)
	}
	if msgs[1].Timestamp != "200.000000" {
		t.Errorf("expected newest message second (200.000000), got %q", msgs[1].Timestamp)
	}
}
