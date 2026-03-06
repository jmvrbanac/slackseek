package cmd

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jmvrbanac/slackseek/internal/cache"
	"github.com/jmvrbanac/slackseek/internal/slack"
)

// T008: Table-driven test for cacheableDayKey covering all state-table branches.
func TestCacheableDayKey(t *testing.T) {
	past := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	pastEnd := time.Date(2026, 1, 1, 23, 59, 59, 0, time.UTC)
	now := time.Now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	future := now.Add(48 * time.Hour)
	nextDay := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		dr           slack.DateRange
		fetchedCount int
		limit        int
		want         string
	}{
		{
			name: "nil From returns empty",
			dr:   slack.DateRange{From: nil, To: &pastEnd},
			want: "",
		},
		{
			name: "nil To returns empty",
			dr:   slack.DateRange{From: &past, To: nil},
			want: "",
		},
		{
			name: "multi-day range returns empty",
			dr:   slack.DateRange{From: &past, To: &nextDay},
			want: "",
		},
		{
			name: "today returns empty",
			dr:   slack.DateRange{From: &todayStart, To: &now},
			want: "",
		},
		{
			name: "future returns empty",
			dr:   slack.DateRange{From: &future, To: &future},
			want: "",
		},
		{
			name:  "past full day limit=0 returns date",
			dr:    slack.DateRange{From: &past, To: &pastEnd},
			limit: 0,
			want:  "2026-01-01",
		},
		{
			name:         "past full day count < limit returns date",
			dr:           slack.DateRange{From: &past, To: &pastEnd},
			fetchedCount: 5,
			limit:        10,
			want:         "2026-01-01",
		},
		{
			name:         "truncated count == limit returns empty",
			dr:           slack.DateRange{From: &past, To: &pastEnd},
			fetchedCount: 10,
			limit:        10,
			want:         "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := cacheableDayKey(tc.dr, tc.fetchedCount, tc.limit)
			if got != tc.want {
				t.Errorf("cacheableDayKey() = %q, want %q", got, tc.want)
			}
		})
	}
}

// T009: No cache entry — API called, entry written.
func TestFetchHistoryCached_Miss(t *testing.T) {
	store := cache.NewStore(t.TempDir(), time.Hour)
	wsKey := "testws"
	channelID := "C01"

	past := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	pastEnd := time.Date(2026, 1, 1, 23, 59, 59, 0, time.UTC)
	dr := slack.DateRange{From: &past, To: &pastEnd}

	want := []slack.Message{{Timestamp: "1234.0", UserID: "U1", Text: "hello"}}
	apiCalls := 0
	fetchFn := func(_ context.Context, _ string, _ slack.DateRange, _ int, _ bool) ([]slack.Message, error) {
		apiCalls++
		return want, nil
	}

	got, err := fetchHistoryCachedInner(context.Background(), fetchFn, store, wsKey, channelID, dr, 0, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if apiCalls != 1 {
		t.Errorf("expected 1 API call on miss, got %d", apiCalls)
	}
	if len(got) != 1 || got[0].Text != want[0].Text {
		t.Errorf("got messages %v, want %v", got, want)
	}

	kind := "history/" + channelID + "/2026-01-01"
	_, hit, err := store.LoadStable(wsKey, kind)
	if err != nil {
		t.Fatalf("LoadStable: %v", err)
	}
	if !hit {
		t.Error("expected cache entry to be written on miss")
	}
}

// T010: Cache entry present — API NOT called, cached messages returned.
func TestFetchHistoryCached_Hit(t *testing.T) {
	store := cache.NewStore(t.TempDir(), time.Hour)
	wsKey := "testws"
	channelID := "C01"

	past := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	pastEnd := time.Date(2026, 1, 1, 23, 59, 59, 0, time.UTC)
	dr := slack.DateRange{From: &past, To: &pastEnd}

	cached := []slack.Message{{Timestamp: "1234.0", UserID: "U1", Text: "cached"}}
	data, err := json.Marshal(cached)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := store.SaveStable(wsKey, "history/"+channelID+"/2026-01-01", data); err != nil {
		t.Fatalf("SaveStable: %v", err)
	}

	apiCalls := 0
	fetchFn := func(_ context.Context, _ string, _ slack.DateRange, _ int, _ bool) ([]slack.Message, error) {
		apiCalls++
		return nil, nil
	}

	got, err := fetchHistoryCachedInner(context.Background(), fetchFn, store, wsKey, channelID, dr, 0, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if apiCalls != 0 {
		t.Errorf("expected 0 API calls on hit, got %d", apiCalls)
	}
	if len(got) != 1 || got[0].Text != "cached" {
		t.Errorf("expected cached message, got %v", got)
	}
}

// T011: Truncated result (count == limit) — no cache entry written.
func TestFetchHistoryCached_Truncated(t *testing.T) {
	store := cache.NewStore(t.TempDir(), time.Hour)
	wsKey := "testws"
	channelID := "C01"

	past := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	pastEnd := time.Date(2026, 1, 1, 23, 59, 59, 0, time.UTC)
	dr := slack.DateRange{From: &past, To: &pastEnd}

	limit := 2
	msgs := []slack.Message{
		{Timestamp: "1.0", UserID: "U1", Text: "a"},
		{Timestamp: "2.0", UserID: "U1", Text: "b"},
	}
	fetchFn := func(_ context.Context, _ string, _ slack.DateRange, _ int, _ bool) ([]slack.Message, error) {
		return msgs, nil
	}

	_, err := fetchHistoryCachedInner(context.Background(), fetchFn, store, wsKey, channelID, dr, limit, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kind := "history/" + channelID + "/2026-01-01"
	_, hit, err := store.LoadStable(wsKey, kind)
	if err != nil {
		t.Fatalf("LoadStable: %v", err)
	}
	if hit {
		t.Error("expected no cache entry for truncated result (count == limit)")
	}
}

// T017: When noCache=true, API is always called even when cache entry exists.
func TestFetchHistoryCached_NoCache_SkipsLoad(t *testing.T) {
	store := cache.NewStore(t.TempDir(), time.Hour)
	wsKey := "testws"
	channelID := "C01"

	past := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	pastEnd := time.Date(2026, 1, 1, 23, 59, 59, 0, time.UTC)
	dr := slack.DateRange{From: &past, To: &pastEnd}

	// Pre-populate cache with stale data.
	stale := []slack.Message{{Timestamp: "1.0", UserID: "U1", Text: "stale"}}
	staleData, _ := json.Marshal(stale)
	if err := store.SaveStable(wsKey, "history/"+channelID+"/2026-01-01", staleData); err != nil {
		t.Fatalf("SaveStable: %v", err)
	}

	fresh := []slack.Message{{Timestamp: "2.0", UserID: "U1", Text: "fresh"}}
	apiCalls := 0
	fetchFn := func(_ context.Context, _ string, _ slack.DateRange, _ int, _ bool) ([]slack.Message, error) {
		apiCalls++
		return fresh, nil
	}

	got, err := fetchHistoryCachedInner(context.Background(), fetchFn, store, wsKey, channelID, dr, 0, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if apiCalls != 1 {
		t.Errorf("expected 1 API call when noCache=true, got %d", apiCalls)
	}
	if len(got) != 1 || got[0].Text != "fresh" {
		t.Errorf("expected fresh message, got %v", got)
	}
}

// T018: After noCache=true fetch, LoadStable returns the freshly written entry.
func TestFetchHistoryCached_NoCache_StillWrites(t *testing.T) {
	store := cache.NewStore(t.TempDir(), time.Hour)
	wsKey := "testws"
	channelID := "C01"

	past := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	pastEnd := time.Date(2026, 1, 1, 23, 59, 59, 0, time.UTC)
	dr := slack.DateRange{From: &past, To: &pastEnd}

	fresh := []slack.Message{{Timestamp: "2.0", UserID: "U1", Text: "refreshed"}}
	fetchFn := func(_ context.Context, _ string, _ slack.DateRange, _ int, _ bool) ([]slack.Message, error) {
		return fresh, nil
	}

	_, err := fetchHistoryCachedInner(context.Background(), fetchFn, store, wsKey, channelID, dr, 0, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kind := "history/" + channelID + "/2026-01-01"
	data, hit, err := store.LoadStable(wsKey, kind)
	if err != nil {
		t.Fatalf("LoadStable: %v", err)
	}
	if !hit {
		t.Fatal("expected cache entry to be written after noCache=true fetch")
	}
	var msgs []slack.Message
	if jsonErr := json.Unmarshal(data, &msgs); jsonErr != nil {
		t.Fatalf("json.Unmarshal: %v", jsonErr)
	}
	if len(msgs) != 1 || msgs[0].Text != "refreshed" {
		t.Errorf("expected refreshed message in cache, got %v", msgs)
	}
}

// T012: Date range covers today — no cache entry written.
func TestFetchHistoryCached_Today(t *testing.T) {
	store := cache.NewStore(t.TempDir(), time.Hour)
	wsKey := "testws"
	channelID := "C01"

	now := time.Now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	dr := slack.DateRange{From: &todayStart, To: &now}

	fetchFn := func(_ context.Context, _ string, _ slack.DateRange, _ int, _ bool) ([]slack.Message, error) {
		return []slack.Message{{Timestamp: "1.0", UserID: "U1", Text: "today msg"}}, nil
	}

	_, err := fetchHistoryCachedInner(context.Background(), fetchFn, store, wsKey, channelID, dr, 0, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dateKey := now.Format("2006-01-02")
	kind := "history/" + channelID + "/" + dateKey
	_, hit, err := store.LoadStable(wsKey, kind)
	if err != nil {
		t.Fatalf("LoadStable: %v", err)
	}
	if hit {
		t.Error("expected no cache entry for today's messages")
	}
}
