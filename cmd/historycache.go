package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/jmvrbanac/slackseek/internal/cache"
	"github.com/jmvrbanac/slackseek/internal/slack"
)

// historyFetchFunc is the API fetch signature, injectable for tests.
type historyFetchFunc func(ctx context.Context, channelID string, dr slack.DateRange, limit int, threads bool) ([]slack.Message, error)

// cacheableDayKey returns the YYYY-MM-DD cache key when all four cacheability
// conditions hold, and "" otherwise.
//
// State table (per data-model.md):
//
//	(dr.From=nil OR dr.To=nil)       → "" (non-cacheable)
//	(From.date != To.date in UTC)    → "" (multi-day range)
//	(To >= now UTC)                  → "" (today or future)
//	(limit > 0 AND count >= limit)   → "" (likely truncated)
//	else                             → From.UTC().Format("2006-01-02")
func cacheableDayKey(dr slack.DateRange, fetchedCount, limit int) string {
	if dr.From == nil || dr.To == nil {
		return ""
	}
	fromDate := dr.From.UTC().Format("2006-01-02")
	toDate := dr.To.UTC().Format("2006-01-02")
	if fromDate != toDate {
		return ""
	}
	today := time.Now().UTC().Format("2006-01-02")
	if toDate >= today {
		return ""
	}
	if limit > 0 && fetchedCount >= limit {
		return ""
	}
	return fromDate
}

// cacheKind builds the cache kind string for a history entry.
// Returns "" when dateKey is empty.
func cacheKind(channelID, dateKey string) string {
	if dateKey == "" {
		return ""
	}
	return "history/" + channelID + "/" + dateKey
}

// isMultiDay returns true when both From and To are non-nil and span different UTC dates.
func isMultiDay(dr slack.DateRange) bool {
	if dr.From == nil || dr.To == nil {
		return false
	}
	return dr.From.UTC().Format("2006-01-02") != dr.To.UTC().Format("2006-01-02")
}

// enumeratePastDays returns ascending YYYY-MM-DD strings for each complete UTC day
// strictly before today within [from, to). Returns nil for zero inputs or single-day ranges.
func enumeratePastDays(from, to time.Time) []string {
	if from.IsZero() || to.IsZero() {
		return nil
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	fromDay := from.UTC().Truncate(24 * time.Hour)
	toDay := to.UTC().Truncate(24 * time.Hour)
	if !fromDay.Before(toDay) {
		return nil
	}
	var days []string
	for d := fromDay; d.Before(toDay) && d.Before(today); d = d.Add(24 * time.Hour) {
		days = append(days, d.Format("2006-01-02"))
	}
	return days
}

// partitionByDay buckets messages by UTC calendar day using a two-pass algorithm.
// Pass 1 builds a root-day index; pass 2 assigns replies to their root's day via ThreadTS.
func partitionByDay(msgs []slack.Message) map[string][]slack.Message {
	rootDay := make(map[string]string)
	for _, m := range msgs {
		if m.ThreadDepth == 0 {
			rootDay[m.Timestamp] = m.Time.UTC().Format("2006-01-02")
		}
	}
	buckets := make(map[string][]slack.Message)
	for _, m := range msgs {
		var day string
		if m.ThreadDepth == 0 {
			day = m.Time.UTC().Format("2006-01-02")
		} else {
			day = rootDay[m.ThreadTS]
		}
		if day != "" {
			buckets[day] = append(buckets[day], m)
		}
	}
	return buckets
}

// loadCachedMessages tries to load messages for a single day from the cache.
// Returns nil, false when noCache=true, store is nil, on miss, or on unmarshal error.
func loadCachedMessages(noCache bool, store *cache.Store, wsKey, channelID, day string) ([]slack.Message, bool) {
	if noCache || store == nil {
		return nil, false
	}
	data, hit, err := store.LoadStable(wsKey, cacheKind(channelID, day))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cache read failed: %v\n", err)
		return nil, false
	}
	if !hit {
		return nil, false
	}
	var msgs []slack.Message
	if json.Unmarshal(data, &msgs) != nil {
		return nil, false
	}
	return msgs, true
}

// appendFinalGap closes any open past-day gap and appends today's live gap.
// When the last miss was yesterday the open gap is extended as open-ended (To=nil).
func appendFinalGap(gaps []slack.DateRange, gapStart *time.Time, lastMissDay, yesterdayStr string, today time.Time) []slack.DateRange {
	if gapStart != nil && lastMissDay == yesterdayStr {
		return append(gaps, slack.DateRange{From: gapStart, To: nil})
	}
	if gapStart != nil {
		end, _ := time.Parse("2006-01-02", lastMissDay)
		end = end.Add(24 * time.Hour)
		gaps = append(gaps, slack.DateRange{From: gapStart, To: &end})
	}
	todayCopy := today
	return append(gaps, slack.DateRange{From: &todayCopy, To: nil})
}

// saveFetchedDays writes each complete past-day bucket from fetched to the cache store.
func saveFetchedDays(store *cache.Store, wsKey, channelID, today string, fetched map[string][]slack.Message) {
	if store == nil {
		return
	}
	for day, msgs := range fetched {
		if day >= today {
			continue
		}
		if data, err := json.Marshal(msgs); err == nil {
			_ = store.SaveStable(wsKey, cacheKind(channelID, day), data)
		}
	}
}

// buildGapRanges checks the cache for each past day, loads hits into cached, and merges
// contiguous misses into gap DateRanges. When the last miss is yesterday (adjacent to today)
// the past gap is merged with today into one open-ended range; otherwise today is appended
// as a separate [todayMidnight, nil) gap.
func buildGapRanges(
	pastDays []string,
	store *cache.Store,
	wsKey, channelID string,
	noCache bool,
) (cached map[string][]slack.Message, gaps []slack.DateRange, err error) {
	cached = make(map[string][]slack.Message)
	today := time.Now().UTC().Truncate(24 * time.Hour)
	yesterdayStr := today.Add(-24 * time.Hour).Format("2006-01-02")

	var gapStart *time.Time
	var lastMissDay string

	for _, day := range pastDays {
		dayT, _ := time.Parse("2006-01-02", day)
		msgs, loaded := loadCachedMessages(noCache, store, wsKey, channelID, day)
		if loaded {
			cached[day] = msgs
			if gapStart != nil {
				end := dayT
				gaps = append(gaps, slack.DateRange{From: gapStart, To: &end})
				gapStart = nil
			}
		} else {
			if gapStart == nil {
				dt := dayT
				gapStart = &dt
			}
			lastMissDay = day
		}
	}

	gaps = appendFinalGap(gaps, gapStart, lastMissDay, yesterdayStr, today)
	return cached, gaps, nil
}

// fetchHistoryMultiDayCached is the testable inner implementation for multi-day ranges.
// It enumerates past days, loads cached ones, fetches each gap with limit=0, partitions
// results by day, saves complete past-day buckets, merges all messages, and applies limit.
func fetchHistoryMultiDayCached(
	ctx context.Context,
	fetchFn historyFetchFunc,
	store *cache.Store,
	wsKey, channelID string,
	dr slack.DateRange,
	limit int,
	threads bool,
	noCache bool,
) ([]slack.Message, error) {
	pastDays := enumeratePastDays(*dr.From, *dr.To)
	cached, gaps, err := buildGapRanges(pastDays, store, wsKey, channelID, noCache)
	if err != nil {
		return nil, err
	}

	today := time.Now().UTC().Format("2006-01-02")
	fetched := make(map[string][]slack.Message)
	for _, gap := range gaps {
		msgs, fetchErr := fetchFn(ctx, channelID, gap, 0, threads)
		if fetchErr != nil {
			return nil, fetchErr
		}
		for day, bucket := range partitionByDay(msgs) {
			fetched[day] = append(fetched[day], bucket...)
		}
	}

	saveFetchedDays(store, wsKey, channelID, today, fetched)

	var all []slack.Message
	for _, msgs := range cached {
		all = append(all, msgs...)
	}
	for _, msgs := range fetched {
		all = append(all, msgs...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Timestamp < all[j].Timestamp })
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// FetchHistoryCached checks the day cache before calling FetchHistory and
// writes to cache on a miss (or when noCache=true).
func FetchHistoryCached(
	ctx context.Context,
	c *slack.Client,
	store *cache.Store,
	wsKey, channelID string,
	dr slack.DateRange,
	limit int,
	threads bool,
	noCache bool,
) ([]slack.Message, error) {
	if isMultiDay(dr) {
		return fetchHistoryMultiDayCached(ctx, c.FetchHistory, store, wsKey, channelID, dr, limit, threads, noCache)
	}
	return fetchHistoryCachedInner(ctx, c.FetchHistory, store, wsKey, channelID, dr, limit, threads, noCache)
}

// fetchHistoryCachedInner is the testable inner implementation. It accepts an
// injectable fetchFn so tests can substitute a fake without an HTTP server.
func fetchHistoryCachedInner(
	ctx context.Context,
	fetchFn historyFetchFunc,
	store *cache.Store,
	wsKey, channelID string,
	dr slack.DateRange,
	limit int,
	threads bool,
	noCache bool,
) ([]slack.Message, error) {
	// Pre-fetch check: use limit=0 to test only date conditions (no truncation
	// check yet — we don't know the count before fetching).
	preKey := cacheKind(channelID, cacheableDayKey(dr, 0, 0))

	if !noCache && preKey != "" && store != nil {
		if data, hit, err := store.LoadStable(wsKey, preKey); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cache read failed: %v\n", err)
		} else if hit {
			var msgs []slack.Message
			if jsonErr := json.Unmarshal(data, &msgs); jsonErr == nil {
				return msgs, nil
			}
		}
	}

	msgs, err := fetchFn(ctx, channelID, dr, limit, threads)
	if err != nil {
		return nil, err
	}

	// Post-fetch check: full four conditions with actual count.
	postKey := cacheKind(channelID, cacheableDayKey(dr, len(msgs), limit))
	if postKey != "" && store != nil {
		data, marshalErr := json.Marshal(msgs)
		if marshalErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not cache messages: %v\n", marshalErr)
		} else {
			_ = store.SaveStable(wsKey, postKey, data)
		}
	}

	return msgs, nil
}
