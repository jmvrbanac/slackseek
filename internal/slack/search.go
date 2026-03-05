package slack

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	slackgo "github.com/slack-go/slack"
)

// channelIDPattern matches Slack channel/group/DM IDs (uppercase letter + alphanumerics).
var channelIDPattern = regexp.MustCompile(`^[CDGW][A-Z0-9]{5,}$`)

// BuildSearchQuery composes a Slack full-text search query from a base query
// and optional modifiers. Empty optional fields are omitted.
//
// Example: BuildSearchQuery("test", "general", "U123", dr)
// →  "test in:#general from:U123 after:2025-01-01 before:2025-02-01"
func BuildSearchQuery(query, channel, userID string, dr DateRange) string {
	parts := []string{query}
	if channel != "" {
		// Channel IDs (C…, D…, G…) don't need the '#' prefix in Slack search.
		if channelIDPattern.MatchString(channel) {
			parts = append(parts, "in:"+channel)
		} else {
			parts = append(parts, "in:#"+channel)
		}
	}
	if userID != "" {
		parts = append(parts, "from:"+userID)
	}
	if dr.From != nil {
		parts = append(parts, "after:"+dr.From.Format("2006-01-02"))
	}
	if dr.To != nil {
		parts = append(parts, "before:"+dr.To.Format("2006-01-02"))
	}
	return strings.Join(parts, " ")
}

// SearchMessages executes a full-text search and returns up to limit results.
// A limit of 0 means unlimited (all pages are fetched).
func (c *Client) SearchMessages(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	params := slackgo.NewSearchParameters()
	params.Count = 100 // request maximum page size
	var results []SearchResult
	for page := 1; ; page++ {
		params.Page = page
		if err := c.tier2.Wait(ctx); err != nil {
			return nil, err
		}
		sm, err := c.searchPage(ctx, query, params, page)
		if err != nil {
			return nil, err
		}
		for _, m := range sm.Matches {
			results = append(results, convertSearchMatch(m))
			if limit > 0 && len(results) >= limit {
				return results, nil
			}
		}
		if page >= sm.Paging.Pages || len(sm.Matches) == 0 {
			break
		}
	}
	return results, nil
}

func (c *Client) searchPage(ctx context.Context, query string, params slackgo.SearchParameters, page int) (*slackgo.SearchMessages, error) {
	var sm *slackgo.SearchMessages
	err := c.callWithRetry(ctx, func() error {
		var callErr error
		sm, callErr = c.api.SearchMessagesContext(ctx, query, params)
		return callErr
	})
	if err != nil {
		return nil, fmt.Errorf("search messages (page %d): %w", page, err)
	}
	return sm, nil
}

func convertSearchMatch(m slackgo.SearchMessage) SearchResult {
	return SearchResult{
		Message: Message{
			Timestamp:   m.Timestamp,
			Time:        parseSlackTS(m.Timestamp),
			UserID:      m.User,
			Text:        m.Text,
			ChannelID:   m.Channel.ID,
			ChannelName: m.Channel.Name,
		},
		Permalink: m.Permalink,
	}
}

// parseSlackTS converts a Slack timestamp string (e.g. "1700000000.123456")
// to a UTC time.Time. Returns zero time on parse failure.
func parseSlackTS(ts string) time.Time {
	dot := strings.IndexByte(ts, '.')
	if dot < 0 {
		dot = len(ts)
	}
	sec, err := strconv.ParseInt(ts[:dot], 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(sec, 0).UTC()
}
