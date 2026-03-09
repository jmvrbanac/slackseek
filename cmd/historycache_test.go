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

	type tc struct {
		name         string
		dr           slack.DateRange
		fetchedCount int
		limit        int
		want         string
	}
	tests := []tc{
		{"nil From", slack.DateRange{From: nil, To: &pastEnd}, 0, 0, ""},
		{"nil To", slack.DateRange{From: &past, To: nil}, 0, 0, ""},
		{"multi-day range", slack.DateRange{From: &past, To: &nextDay}, 0, 0, ""},
		{"today", slack.DateRange{From: &todayStart, To: &now}, 0, 0, ""},
		{"future", slack.DateRange{From: &future, To: &future}, 0, 0, ""},
		{"past limit=0", slack.DateRange{From: &past, To: &pastEnd}, 0, 0, "2026-01-01"},
		{"past count < limit", slack.DateRange{From: &past, To: &pastEnd}, 5, 10, "2026-01-01"},
		{"truncated count == limit", slack.DateRange{From: &past, To: &pastEnd}, 10, 10, ""},
	}
	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			got := cacheableDayKey(c.dr, c.fetchedCount, c.limit)
			if got != c.want {
				t.Errorf("cacheableDayKey() = %q, want %q", got, c.want)
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

// ===== Phase 2: isMultiDay and enumeratePastDays =====

// T003: Table-driven tests for isMultiDay covering all branches including open-ended ranges.
func TestIsMultiDay(t *testing.T) {
	past1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	past2 := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	sameEnd := time.Date(2026, 1, 1, 23, 59, 59, 0, time.UTC)
	now := time.Now().UTC()

	tests := []struct {
		name string
		dr   slack.DateRange
		want bool
	}{
		{"both nil", slack.DateRange{}, false},
		{"from nil", slack.DateRange{To: &past2}, false},
		{"to nil past from (--since 1w)", slack.DateRange{From: &past1}, true},
		{"to nil today from (--since 4h)", slack.DateRange{From: &now}, false},
		{"same-day range", slack.DateRange{From: &past1, To: &sameEnd}, false},
		{"multi-day range", slack.DateRange{From: &past1, To: &past2}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isMultiDay(tc.dr); got != tc.want {
				t.Errorf("isMultiDay() = %v, want %v", got, tc.want)
			}
		})
	}
}

// T005: Table-driven tests for enumeratePastDays covering all edge cases.
func TestEnumeratePastDays(t *testing.T) {
	now := time.Now().UTC()
	today := now.Truncate(24 * time.Hour)
	yesterday := today.Add(-24 * time.Hour)
	threeDaysAgo := today.Add(-3 * 24 * time.Hour)
	twoWeeksAgo := today.Add(-14 * 24 * time.Hour)

	jan1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	jan4 := time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)

	type tc struct {
		name     string
		from, to time.Time
		wantNil  bool
		wantLen  int
		checkFn  func(t *testing.T, got []string)
	}
	tests := []tc{
		{name: "nil from (zero)", from: time.Time{}, to: jan4, wantNil: true},
		{name: "nil to (zero)", from: jan1, to: time.Time{}, wantNil: true},
		{name: "single-day range", from: jan1, to: jan1.Add(12 * time.Hour), wantNil: true},
		{
			name: "range entirely in past", from: jan1, to: jan4, wantLen: 3,
			checkFn: func(t *testing.T, got []string) {
				if got[0] != "2026-01-01" || got[2] != "2026-01-03" {
					t.Errorf("expected 2026-01-01..2026-01-03, got %v", got)
				}
			},
		},
		{
			name: "range ending today excludes today", from: threeDaysAgo, to: today, wantLen: 3,
			checkFn: func(t *testing.T, got []string) {
				if got[len(got)-1] != yesterday.Format("2006-01-02") {
					t.Errorf("last day should be yesterday, got %q", got[len(got)-1])
				}
			},
		},
		{
			name: "multi-week range", from: twoWeeksAgo, to: today, wantLen: 14,
			checkFn: func(t *testing.T, got []string) {
				if got[0] != twoWeeksAgo.Format("2006-01-02") {
					t.Errorf("first day should be %s, got %q", twoWeeksAgo.Format("2006-01-02"), got[0])
				}
			},
		},
	}
	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			got := enumeratePastDays(c.from, c.to)
			if c.wantNil {
				if got != nil {
					t.Errorf("want nil, got %v", got)
				}
				return
			}
			if len(got) != c.wantLen {
				t.Fatalf("got %d days, want %d: %v", len(got), c.wantLen, got)
			}
			if c.checkFn != nil {
				c.checkFn(t, got)
			}
		})
	}
}

// ===== Phase 3: partitionByDay, buildGapRanges, fetchHistoryMultiDayCached =====

// T007: Root messages land in their own UTC day bucket.
func TestPartitionByDay_RootMessages(t *testing.T) {
	day1 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)

	msgs := []slack.Message{
		{Timestamp: "a", Time: day1, ThreadDepth: 0},
		{Timestamp: "b", Time: day2, ThreadDepth: 0},
	}
	got := partitionByDay(msgs)
	if len(got["2026-01-01"]) != 1 || got["2026-01-01"][0].Timestamp != "a" {
		t.Errorf("2026-01-01 bucket: got %v", got["2026-01-01"])
	}
	if len(got["2026-01-02"]) != 1 || got["2026-01-02"][0].Timestamp != "b" {
		t.Errorf("2026-01-02 bucket: got %v", got["2026-01-02"])
	}
}

// T008: Reply with ThreadDepth=1 at 00:10 day+1 is bucketed under root's day (23:55 day 0).
func TestPartitionByDay_RepliesInheritRootDay(t *testing.T) {
	rootTime := time.Date(2026, 1, 1, 23, 55, 0, 0, time.UTC)
	replyTime := time.Date(2026, 1, 2, 0, 10, 0, 0, time.UTC)

	root := slack.Message{Timestamp: "root-ts", Time: rootTime, ThreadDepth: 0}
	reply := slack.Message{Timestamp: "reply-ts", Time: replyTime, ThreadDepth: 1, ThreadTS: "root-ts"}

	got := partitionByDay([]slack.Message{root, reply})

	if len(got["2026-01-01"]) != 2 {
		t.Errorf("expected 2 msgs in 2026-01-01 (root+reply), got %d: %v", len(got["2026-01-01"]), got["2026-01-01"])
	}
	for _, m := range got["2026-01-02"] {
		if m.Timestamp == "reply-ts" {
			t.Error("reply should not be in 2026-01-02 bucket")
		}
	}
}

// T009: All past days in store → gaps contains only today's range.
func TestBuildGapRanges_AllCached(t *testing.T) {
	store := cache.NewStore(t.TempDir(), time.Hour)
	wsKey, channelID := "testws", "C01"

	days := []string{"2026-01-01", "2026-01-02", "2026-01-03"}
	for _, day := range days {
		data, _ := json.Marshal([]slack.Message{{Timestamp: day + ".0"}})
		_ = store.SaveStable(wsKey, cacheKind(channelID, day), data)
	}

	cached, gaps, err := buildGapRanges(days, store, wsKey, channelID, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, day := range days {
		if _, ok := cached[day]; !ok {
			t.Errorf("expected cached[%q]", day)
		}
	}
	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap (today only), got %d: %v", len(gaps), gaps)
	}
	todayStr := time.Now().UTC().Truncate(24 * time.Hour).Format("2006-01-02")
	if gaps[0].From == nil || gaps[0].From.UTC().Format("2006-01-02") != todayStr {
		t.Errorf("expected today gap, got From=%v", gaps[0].From)
	}
	if gaps[0].To != nil {
		t.Errorf("expected today gap To=nil, got %v", gaps[0].To)
	}
}

// T010: No entries → one contiguous range covering all past days + today.
func TestBuildGapRanges_AllUncached(t *testing.T) {
	store := cache.NewStore(t.TempDir(), time.Hour)
	wsKey, channelID := "testws", "C01"

	today := time.Now().UTC().Truncate(24 * time.Hour)
	yesterday := today.Add(-24 * time.Hour)
	threeDaysAgo := today.Add(-3 * 24 * time.Hour)

	days := []string{
		threeDaysAgo.Format("2006-01-02"),
		threeDaysAgo.Add(24 * time.Hour).Format("2006-01-02"),
		yesterday.Format("2006-01-02"),
	}

	cached, gaps, err := buildGapRanges(days, store, wsKey, channelID, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cached) != 0 {
		t.Errorf("expected empty cached map, got %v", cached)
	}
	// All uncached with yesterday last → merged with today → one open-ended gap
	if len(gaps) != 1 {
		t.Fatalf("expected 1 contiguous gap, got %d: %v", len(gaps), gaps)
	}
	if gaps[0].From == nil || gaps[0].From.UTC().Format("2006-01-02") != threeDaysAgo.Format("2006-01-02") {
		t.Errorf("expected gap From=%s, got %v", threeDaysAgo.Format("2006-01-02"), gaps[0].From)
	}
	if gaps[0].To != nil {
		t.Errorf("expected open-ended gap (To=nil), got To=%v", gaps[0].To)
	}
}

// T011: Days 1–3 cached, days 4–5 not → two gap ranges (4–5 + today).
func TestBuildGapRanges_PartialCache(t *testing.T) {
	store := cache.NewStore(t.TempDir(), time.Hour)
	wsKey, channelID := "testws", "C01"

	days := []string{"2026-01-01", "2026-01-02", "2026-01-03", "2026-01-04", "2026-01-05"}
	for _, day := range days[:3] {
		data, _ := json.Marshal([]slack.Message{{Timestamp: day + ".0"}})
		_ = store.SaveStable(wsKey, cacheKind(channelID, day), data)
	}

	cached, gaps, err := buildGapRanges(days, store, wsKey, channelID, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, day := range days[:3] {
		if _, ok := cached[day]; !ok {
			t.Errorf("expected cached[%q]", day)
		}
	}
	for _, day := range days[3:] {
		if _, ok := cached[day]; ok {
			t.Errorf("unexpected cached[%q]", day)
		}
	}
	if len(gaps) != 2 {
		t.Fatalf("expected 2 gaps, got %d: %v", len(gaps), gaps)
	}
	if gaps[0].From == nil || gaps[0].From.UTC().Format("2006-01-02") != "2026-01-04" {
		t.Errorf("first gap From: want 2026-01-04, got %v", gaps[0].From)
	}
	if gaps[0].To == nil || gaps[0].To.UTC().Format("2006-01-02") != "2026-01-06" {
		t.Errorf("first gap To: want 2026-01-06, got %v", gaps[0].To)
	}
	todayStr := time.Now().UTC().Truncate(24 * time.Hour).Format("2006-01-02")
	if gaps[1].From == nil || gaps[1].From.UTC().Format("2006-01-02") != todayStr {
		t.Errorf("second gap: want today (%s), got %v", todayStr, gaps[1].From)
	}
	if gaps[1].To != nil {
		t.Errorf("second gap To: want nil, got %v", gaps[1].To)
	}
}

// T012: Cold cache; assert SaveStable called per past day; returned slice = all messages.
func TestFetchHistoryMultiDayCached_FirstRun(t *testing.T) {
	store := cache.NewStore(t.TempDir(), time.Hour)
	wsKey, channelID := "testws", "C01"

	now := time.Now().UTC()
	today := now.Truncate(24 * time.Hour)
	yesterday := today.Add(-24 * time.Hour)
	twoDaysAgo := today.Add(-2 * 24 * time.Hour)

	allMsgs := []slack.Message{
		{Timestamp: "1.0", Time: twoDaysAgo.Add(time.Hour), ThreadDepth: 0},
		{Timestamp: "2.0", Time: yesterday.Add(time.Hour), ThreadDepth: 0},
		{Timestamp: "3.0", Time: today.Add(time.Hour), ThreadDepth: 0},
	}
	fetchFn := func(_ context.Context, _ string, _ slack.DateRange, _ int, _ bool) ([]slack.Message, error) {
		return allMsgs, nil
	}

	from, to := twoDaysAgo, now
	dr := slack.DateRange{From: &from, To: &to}

	got, err := fetchHistoryMultiDayCached(context.Background(), fetchFn, store, wsKey, channelID, dr, 0, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 messages, got %d", len(got))
	}
	for _, day := range []string{twoDaysAgo.Format("2006-01-02"), yesterday.Format("2006-01-02")} {
		_, hit, e := store.LoadStable(wsKey, cacheKind(channelID, day))
		if e != nil {
			t.Fatalf("LoadStable: %v", e)
		}
		if !hit {
			t.Errorf("expected cache entry for %s", day)
		}
	}
	_, hit, _ := store.LoadStable(wsKey, cacheKind(channelID, today.Format("2006-01-02")))
	if hit {
		t.Error("today should not be cached")
	}
}

// T013: All past days cached; fetchFn returns only today's messages; fetchFn called once.
func TestFetchHistoryMultiDayCached_WarmCache(t *testing.T) {
	store := cache.NewStore(t.TempDir(), time.Hour)
	wsKey, channelID := "testws", "C01"

	now := time.Now().UTC()
	today := now.Truncate(24 * time.Hour)
	yesterday := today.Add(-24 * time.Hour)
	twoDaysAgo := today.Add(-2 * 24 * time.Hour)

	for day, msgs := range map[string][]slack.Message{
		twoDaysAgo.Format("2006-01-02"): {{Timestamp: "1.0", Time: twoDaysAgo.Add(time.Hour)}},
		yesterday.Format("2006-01-02"):  {{Timestamp: "2.0", Time: yesterday.Add(time.Hour)}},
	} {
		data, _ := json.Marshal(msgs)
		_ = store.SaveStable(wsKey, cacheKind(channelID, day), data)
	}

	fetchCalls := 0
	fetchFn := func(_ context.Context, _ string, _ slack.DateRange, _ int, _ bool) ([]slack.Message, error) {
		fetchCalls++
		return []slack.Message{{Timestamp: "3.0", Time: today.Add(time.Hour)}}, nil
	}

	from, to := twoDaysAgo, now
	dr := slack.DateRange{From: &from, To: &to}

	got, err := fetchHistoryMultiDayCached(context.Background(), fetchFn, store, wsKey, channelID, dr, 0, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fetchCalls != 1 {
		t.Errorf("expected 1 fetchFn call (today only), got %d", fetchCalls)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 messages (2 cached + today), got %d", len(got))
	}
}

// T014: 14-day cache pre-populated; 7-day range → fetchFn called once (today only).
func TestFetchHistoryMultiDayCached_OverlappingWindow(t *testing.T) {
	store := cache.NewStore(t.TempDir(), time.Hour)
	wsKey, channelID := "testws", "C01"

	now := time.Now().UTC()
	today := now.Truncate(24 * time.Hour)

	for i := 0; i < 14; i++ {
		day := today.Add(time.Duration(-(14 - i)) * 24 * time.Hour)
		dayStr := day.Format("2006-01-02")
		data, _ := json.Marshal([]slack.Message{{Timestamp: dayStr + ".0", Time: day.Add(time.Hour)}})
		_ = store.SaveStable(wsKey, cacheKind(channelID, dayStr), data)
	}

	fetchCalls := 0
	fetchFn := func(_ context.Context, _ string, _ slack.DateRange, _ int, _ bool) ([]slack.Message, error) {
		fetchCalls++
		return []slack.Message{{Timestamp: "today.0", Time: today.Add(time.Hour)}}, nil
	}

	from := today.Add(-7 * 24 * time.Hour)
	dr := slack.DateRange{From: &from, To: &now}

	got, err := fetchHistoryMultiDayCached(context.Background(), fetchFn, store, wsKey, channelID, dr, 0, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fetchCalls != 1 {
		t.Errorf("expected 1 fetchFn call (today only), got %d", fetchCalls)
	}
	if len(got) != 8 {
		t.Errorf("expected 8 messages (7 cached + today), got %d", len(got))
	}
}

// T015: Root at 23:55 day1, reply at 00:10 day2 → both in day1 cache; reply not in day2.
func TestFetchHistoryMultiDayCached_CrossDayThread(t *testing.T) {
	store := cache.NewStore(t.TempDir(), time.Hour)
	wsKey, channelID := "testws", "C01"

	now := time.Now().UTC()
	today := now.Truncate(24 * time.Hour)
	day1 := today.Add(-2 * 24 * time.Hour)
	day2 := today.Add(-24 * time.Hour) // yesterday

	root := slack.Message{
		Timestamp:   "root-ts",
		Time:        day1.Add(23*time.Hour + 55*time.Minute),
		ThreadDepth: 0,
	}
	reply := slack.Message{
		Timestamp:   "reply-ts",
		Time:        day2.Add(10 * time.Minute),
		ThreadDepth: 1,
		ThreadTS:    "root-ts",
	}

	fetchFn := func(_ context.Context, _ string, _ slack.DateRange, _ int, _ bool) ([]slack.Message, error) {
		return []slack.Message{root, reply}, nil
	}

	from := day1
	dr := slack.DateRange{From: &from, To: &now}

	_, err := fetchHistoryMultiDayCached(context.Background(), fetchFn, store, wsKey, channelID, dr, 0, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	day1Str := day1.Format("2006-01-02")
	data1, hit1, _ := store.LoadStable(wsKey, cacheKind(channelID, day1Str))
	if !hit1 {
		t.Fatal("expected cache entry for day1")
	}
	var day1Msgs []slack.Message
	_ = json.Unmarshal(data1, &day1Msgs)
	found := map[string]bool{}
	for _, m := range day1Msgs {
		found[m.Timestamp] = true
	}
	if !found["root-ts"] {
		t.Error("day1 cache should contain root")
	}
	if !found["reply-ts"] {
		t.Error("day1 cache should contain reply (cross-day thread)")
	}

	day2Str := day2.Format("2006-01-02")
	if data2, hit2, _ := store.LoadStable(wsKey, cacheKind(channelID, day2Str)); hit2 {
		var day2Msgs []slack.Message
		_ = json.Unmarshal(data2, &day2Msgs)
		for _, m := range day2Msgs {
			if m.Timestamp == "reply-ts" {
				t.Error("reply should not be in day2 cache")
			}
		}
	}
}

// ===== Phase 4: --no-cache =====

// T021: noCache=true bypasses LoadStable; fetchFn called for full past range.
func TestFetchHistoryMultiDayCached_NoCacheBypassesLoad(t *testing.T) {
	store := cache.NewStore(t.TempDir(), time.Hour)
	wsKey, channelID := "testws", "C01"

	// Pre-populate far-past days (not adjacent to today → 2 gaps with noCache)
	days := []string{"2026-01-01", "2026-01-02", "2026-01-03"}
	for _, day := range days {
		data, _ := json.Marshal([]slack.Message{{Timestamp: day + ".0"}})
		_ = store.SaveStable(wsKey, cacheKind(channelID, day), data)
	}

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)
	dr := slack.DateRange{From: &from, To: &to}

	fetchCalls := 0
	fetchFn := func(_ context.Context, _ string, _ slack.DateRange, _ int, _ bool) ([]slack.Message, error) {
		fetchCalls++
		return []slack.Message{}, nil
	}

	_, err := fetchHistoryMultiDayCached(context.Background(), fetchFn, store, wsKey, channelID, dr, 0, false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// noCache=true → all days are misses → past gap + today gap → ≥ 2 calls
	if fetchCalls < 2 {
		t.Errorf("expected fetchFn called ≥ 2 times (past + today), got %d", fetchCalls)
	}
}

// T022: noCache=true; SaveStable still called for each complete past day.
func TestFetchHistoryMultiDayCached_NoCacheStillWrites(t *testing.T) {
	store := cache.NewStore(t.TempDir(), time.Hour)
	wsKey, channelID := "testws", "C01"

	now := time.Now().UTC()
	today := now.Truncate(24 * time.Hour)
	yesterday := today.Add(-24 * time.Hour)
	twoDaysAgo := today.Add(-2 * 24 * time.Hour)

	fetchFn := func(_ context.Context, _ string, _ slack.DateRange, _ int, _ bool) ([]slack.Message, error) {
		return []slack.Message{
			{Timestamp: "1.0", Time: twoDaysAgo.Add(time.Hour), ThreadDepth: 0},
			{Timestamp: "2.0", Time: yesterday.Add(time.Hour), ThreadDepth: 0},
			{Timestamp: "3.0", Time: today.Add(time.Hour), ThreadDepth: 0},
		}, nil
	}

	from, to := twoDaysAgo, now
	dr := slack.DateRange{From: &from, To: &to}

	_, err := fetchHistoryMultiDayCached(context.Background(), fetchFn, store, wsKey, channelID, dr, 0, false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, day := range []string{twoDaysAgo.Format("2006-01-02"), yesterday.Format("2006-01-02")} {
		_, hit, e := store.LoadStable(wsKey, cacheKind(channelID, day))
		if e != nil {
			t.Fatalf("LoadStable: %v", e)
		}
		if !hit {
			t.Errorf("expected SaveStable called for %s with noCache=true", day)
		}
	}
	_, hit, _ := store.LoadStable(wsKey, cacheKind(channelID, today.Format("2006-01-02")))
	if hit {
		t.Error("today should not be cached even with noCache=true")
	}
}

// ===== Phase 5: --limit =====

// T025: fetchFn called with limit=0; SaveStable gets full buckets; return len == user limit.
func TestFetchHistoryMultiDayCached_LimitAppliedAtMerge(t *testing.T) {
	store := cache.NewStore(t.TempDir(), time.Hour)
	wsKey, channelID := "testws", "C01"

	now := time.Now().UTC()
	today := now.Truncate(24 * time.Hour)
	yesterday := today.Add(-24 * time.Hour)
	twoDaysAgo := today.Add(-2 * 24 * time.Hour)

	// 10 messages spread across 3 days (3 + 3 + 4)
	allMsgs := []slack.Message{
		{Timestamp: "1.0", Time: twoDaysAgo.Add(1 * time.Hour), ThreadDepth: 0},
		{Timestamp: "2.0", Time: twoDaysAgo.Add(2 * time.Hour), ThreadDepth: 0},
		{Timestamp: "3.0", Time: twoDaysAgo.Add(3 * time.Hour), ThreadDepth: 0},
		{Timestamp: "4.0", Time: yesterday.Add(1 * time.Hour), ThreadDepth: 0},
		{Timestamp: "5.0", Time: yesterday.Add(2 * time.Hour), ThreadDepth: 0},
		{Timestamp: "6.0", Time: yesterday.Add(3 * time.Hour), ThreadDepth: 0},
		{Timestamp: "7.0", Time: today.Add(1 * time.Hour), ThreadDepth: 0},
		{Timestamp: "8.0", Time: today.Add(2 * time.Hour), ThreadDepth: 0},
		{Timestamp: "9.0", Time: today.Add(3 * time.Hour), ThreadDepth: 0},
		{Timestamp: "10.0", Time: today.Add(4 * time.Hour), ThreadDepth: 0},
	}

	var capturedLimit int
	fetchFn := func(_ context.Context, _ string, _ slack.DateRange, limit int, _ bool) ([]slack.Message, error) {
		capturedLimit = limit
		return allMsgs, nil
	}

	from, to := twoDaysAgo, now
	dr := slack.DateRange{From: &from, To: &to}

	got, err := fetchHistoryMultiDayCached(context.Background(), fetchFn, store, wsKey, channelID, dr, 3, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedLimit != 0 {
		t.Errorf("fetchFn should be called with limit=0, got %d", capturedLimit)
	}
	data1, hit1, _ := store.LoadStable(wsKey, cacheKind(channelID, twoDaysAgo.Format("2006-01-02")))
	if !hit1 {
		t.Fatal("expected cache for twoDaysAgo")
	}
	var cached1 []slack.Message
	_ = json.Unmarshal(data1, &cached1)
	if len(cached1) != 3 {
		t.Errorf("twoDaysAgo cache should have 3 full messages, got %d", len(cached1))
	}
	if len(got) != 3 {
		t.Errorf("expected 3 messages (user limit), got %d", len(got))
	}
}

// T026: limit=0 (unlimited); all messages returned; past days cached in full.
func TestFetchHistoryMultiDayCached_LimitZeroUnlimited(t *testing.T) {
	store := cache.NewStore(t.TempDir(), time.Hour)
	wsKey, channelID := "testws", "C01"

	now := time.Now().UTC()
	today := now.Truncate(24 * time.Hour)
	yesterday := today.Add(-24 * time.Hour)
	twoDaysAgo := today.Add(-2 * 24 * time.Hour)

	allMsgs := []slack.Message{
		{Timestamp: "1.0", Time: twoDaysAgo.Add(time.Hour), ThreadDepth: 0},
		{Timestamp: "2.0", Time: yesterday.Add(time.Hour), ThreadDepth: 0},
		{Timestamp: "3.0", Time: today.Add(time.Hour), ThreadDepth: 0},
	}
	fetchFn := func(_ context.Context, _ string, _ slack.DateRange, _ int, _ bool) ([]slack.Message, error) {
		return allMsgs, nil
	}

	from, to := twoDaysAgo, now
	dr := slack.DateRange{From: &from, To: &to}

	got, err := fetchHistoryMultiDayCached(context.Background(), fetchFn, store, wsKey, channelID, dr, 0, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 messages (unlimited), got %d", len(got))
	}
	for _, day := range []string{twoDaysAgo.Format("2006-01-02"), yesterday.Format("2006-01-02")} {
		_, hit, e := store.LoadStable(wsKey, cacheKind(channelID, day))
		if e != nil {
			t.Fatalf("LoadStable: %v", e)
		}
		if !hit {
			t.Errorf("expected full cache for %s", day)
		}
	}
}

// Regression: --since 1w without --until (To=nil) must cache past days.
func TestFetchHistoryMultiDayCached_OpenEndedSince(t *testing.T) {
	store := cache.NewStore(t.TempDir(), time.Hour)
	wsKey, channelID := "testws", "C01"

	now := time.Now().UTC()
	today := now.Truncate(24 * time.Hour)
	yesterday := today.Add(-24 * time.Hour)
	twoDaysAgo := today.Add(-2 * 24 * time.Hour)

	allMsgs := []slack.Message{
		{Timestamp: "1.0", Time: twoDaysAgo.Add(time.Hour), ThreadDepth: 0},
		{Timestamp: "2.0", Time: yesterday.Add(time.Hour), ThreadDepth: 0},
		{Timestamp: "3.0", Time: today.Add(time.Hour), ThreadDepth: 0},
	}
	fetchFn := func(_ context.Context, _ string, _ slack.DateRange, _ int, _ bool) ([]slack.Message, error) {
		return allMsgs, nil
	}

	// Simulate --since 2d with no --until (To=nil)
	from := twoDaysAgo
	dr := slack.DateRange{From: &from, To: nil}

	if !isMultiDay(dr) {
		t.Fatal("isMultiDay should return true for open-ended range with past From")
	}

	got, err := fetchHistoryMultiDayCached(context.Background(), fetchFn, store, wsKey, channelID, dr, 0, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 messages, got %d", len(got))
	}
	for _, day := range []string{twoDaysAgo.Format("2006-01-02"), yesterday.Format("2006-01-02")} {
		_, hit, e := store.LoadStable(wsKey, cacheKind(channelID, day))
		if e != nil {
			t.Fatalf("LoadStable: %v", e)
		}
		if !hit {
			t.Errorf("expected cache entry for past day %s with open-ended range", day)
		}
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
