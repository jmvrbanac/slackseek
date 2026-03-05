package slack

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	slackgo "github.com/slack-go/slack"

	"github.com/jmvrbanac/slackseek/internal/cache"
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

// --- ListChannels cache tests (T005) ---

func TestListChannelsCached_NilStore_APIAlwaysCalled(t *testing.T) {
	called := 0
	listFn := func(_ context.Context) ([]Channel, error) {
		called++
		return []Channel{{ID: "C001", Name: "general"}}, nil
	}
	channels, err := listChannelsCached(context.Background(), nil, "", listFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 1 {
		t.Errorf("expected 1 API call, got %d", called)
	}
	if len(channels) != 1 || channels[0].ID != "C001" {
		t.Errorf("unexpected channels: %v", channels)
	}
}

func TestListChannelsCached_FreshCache_APINotCalled(t *testing.T) {
	store := cache.NewStore(t.TempDir(), time.Hour)
	key := "testkey"
	payload, err := json.Marshal([]Channel{{ID: "C001", Name: "general"}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := store.Save(key, "channels", payload); err != nil {
		t.Fatalf("Save: %v", err)
	}
	called := 0
	listFn := func(_ context.Context) ([]Channel, error) {
		called++
		return nil, nil
	}
	channels, err := listChannelsCached(context.Background(), store, key, listFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 0 {
		t.Errorf("expected no API call on cache hit, got %d", called)
	}
	if len(channels) != 1 || channels[0].ID != "C001" {
		t.Errorf("expected cached data, got %v", channels)
	}
}

func TestListChannelsCached_CacheMiss_APICalledAndSaved(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, time.Hour)
	key := "testkey"
	called := 0
	listFn := func(_ context.Context) ([]Channel, error) {
		called++
		return []Channel{{ID: "C002", Name: "random"}}, nil
	}
	channels, err := listChannelsCached(context.Background(), store, key, listFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 1 {
		t.Errorf("expected 1 API call, got %d", called)
	}
	if len(channels) != 1 || channels[0].ID != "C002" {
		t.Errorf("unexpected channels: %v", channels)
	}
	_, hit, _ := store.Load(key, "channels")
	if !hit {
		t.Error("expected cache to be written after API call")
	}
}

func TestListChannelsCached_StaleCache_APICalledAndOverwritten(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, time.Hour)
	key := "testkey"
	oldPayload, _ := json.Marshal([]Channel{{ID: "COLD", Name: "old"}})
	if err := store.Save(key, "channels", oldPayload); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path := filepath.Join(dir, key, "channels.json")
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
	called := 0
	listFn := func(_ context.Context) ([]Channel, error) {
		called++
		return []Channel{{ID: "CNEW", Name: "new"}}, nil
	}
	channels, err := listChannelsCached(context.Background(), store, key, listFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 1 {
		t.Errorf("expected 1 API call for stale cache, got %d", called)
	}
	if len(channels) != 1 || channels[0].ID != "CNEW" {
		t.Errorf("expected fresh data, got %v", channels)
	}
}

// T042: listChannelsPages callback tests

func TestListChannelsPages_CallbackInvokedPerPage(t *testing.T) {
	page1 := []slackgo.Channel{{GroupConversation: slackgo.GroupConversation{Conversation: slackgo.Conversation{ID: "C001"}}}, {GroupConversation: slackgo.GroupConversation{Conversation: slackgo.Conversation{ID: "C002"}}}}
	page2 := []slackgo.Channel{{GroupConversation: slackgo.GroupConversation{Conversation: slackgo.Conversation{ID: "C003"}}}}
	pageCalls := 0
	pageFn := func(_ context.Context, _ []string, _ bool, cursor string) ([]slackgo.Channel, string, error) {
		pageCalls++
		if cursor == "" {
			return page1, "cursor1", nil
		}
		return page2, "", nil
	}
	var progress []int
	_, err := listChannelsPages(context.Background(), []string{"public_channel"}, false, func(n int) { progress = append(progress, n) }, pageFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pageCalls != 2 {
		t.Errorf("expected 2 page fetches, got %d", pageCalls)
	}
	if len(progress) != 2 {
		t.Fatalf("expected 2 progress calls, got %v", progress)
	}
	if progress[0] != 2 {
		t.Errorf("expected progress[0]=2 (after page1), got %d", progress[0])
	}
	if progress[1] != 3 {
		t.Errorf("expected progress[1]=3 (after page2), got %d", progress[1])
	}
}

func TestListChannelsPages_NilCallbackDoesNotPanic(t *testing.T) {
	pageFn := func(_ context.Context, _ []string, _ bool, _ string) ([]slackgo.Channel, string, error) {
		return []slackgo.Channel{{GroupConversation: slackgo.GroupConversation{Conversation: slackgo.Conversation{ID: "C001"}}}}, "", nil
	}
	channels, err := listChannelsPages(context.Background(), []string{"public_channel"}, false, nil, pageFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(channels) != 1 {
		t.Errorf("expected 1 channel, got %d", len(channels))
	}
}

func TestListChannelsPages_CallbackNotInvokedOnCacheHit(t *testing.T) {
	store := cache.NewStore(t.TempDir(), time.Hour)
	key := "testkey"
	payload, _ := json.Marshal([]Channel{{ID: "C001", Name: "general"}})
	_ = store.Save(key, "channels", payload)

	callbackInvoked := false
	listFn := func(_ context.Context) ([]Channel, error) {
		// This should not be called on a cache hit; callback lives inside here
		callbackInvoked = true
		return nil, nil
	}
	_, err := listChannelsCached(context.Background(), store, key, listFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callbackInvoked {
		t.Error("listFn (and thus progress callback) must not be invoked on cache hit")
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
