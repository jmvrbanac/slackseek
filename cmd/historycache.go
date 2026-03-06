package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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
