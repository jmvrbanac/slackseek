package slack

import (
	"context"
	"fmt"
)

// getUserMessages is the testable inner implementation shared by GetUserMessages.
// searchFn is injectable for unit tests.
func getUserMessages(
	ctx context.Context,
	userID, channelID string,
	dr DateRange,
	limit int,
	searchFn func(ctx context.Context, query string, limit int) ([]SearchResult, error),
) ([]Message, error) {
	q := BuildSearchQuery("", channelID, userID, dr)
	results, err := searchFn(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("get user messages for %q: %w", userID, err)
	}
	msgs := make([]Message, len(results))
	for i, sr := range results {
		msgs[i] = sr.Message
	}
	return msgs, nil
}

// GetUserMessages returns messages authored by userID, optionally filtered to
// a single channel and date range. A limit of 0 means unlimited.
func (c *Client) GetUserMessages(ctx context.Context, userID, channelID string, dr DateRange, limit int) ([]Message, error) {
	return getUserMessages(ctx, userID, channelID, dr, limit, func(ctx context.Context, query string, limit int) ([]SearchResult, error) {
		return c.SearchMessages(ctx, query, limit)
	})
}
