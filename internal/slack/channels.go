package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	slackgo "github.com/slack-go/slack"

	"github.com/jmvrbanac/slackseek/internal/cache"
)

// histPageFetcher fetches one page of channel history. Injectable for testing.
type histPageFetcher func(ctx context.Context, channelID, oldest, latest, cursor string) ([]slackgo.Message, bool, string, error)

// replyPageFetcher fetches one page of thread replies. Injectable for testing.
type replyPageFetcher func(ctx context.Context, channelID, threadTS, cursor string) ([]slackgo.Message, bool, string, error)

// channelsPageFetcher fetches one page of conversations. Injectable for testing.
type channelsPageFetcher func(ctx context.Context, types []string, excludeArchived bool, cursor string) ([]slackgo.Channel, string, error)

// listChannelsCached is the testable inner implementation of ListChannels.
// When store is non-nil and cacheKey is non-empty it attempts a cache load
// before calling listFn. On a miss it calls listFn and persists the result.
func listChannelsCached(
	ctx context.Context,
	store *cache.Store,
	cacheKey string,
	listFn func(context.Context) ([]Channel, error),
) ([]Channel, error) {
	if store != nil && cacheKey != "" {
		data, hit, err := store.LoadStable(cacheKey, "channels")
		if err != nil {
			return nil, fmt.Errorf("cache load channels: %w", err)
		}
		if hit {
			var channels []Channel
			if jsonErr := json.Unmarshal(data, &channels); jsonErr == nil {
				return channels, nil
			}
		}
	}
	channels, err := listFn(ctx)
	if err != nil {
		return nil, err
	}
	if store != nil && cacheKey != "" {
		if data, jsonErr := json.Marshal(channels); jsonErr == nil {
			_ = store.Save(cacheKey, "channels", data)
		}
	}
	return channels, nil
}

// ListChannels returns all channels by paginating conversations.list.
// Pass nil types to include all channel types. Pass includeArchived=true to
// include archived channels.
func (c *Client) ListChannels(ctx context.Context, types []string, includeArchived bool) ([]Channel, error) {
	if len(types) == 0 {
		types = []string{"public_channel", "private_channel", "mpim", "im"}
	}
	return listChannelsCached(ctx, c.store, c.cacheKey, func(ctx context.Context) ([]Channel, error) {
		return listChannelsPages(ctx, types, !includeArchived, c.pageFetchedFn, c.channelsPageFetch)
	})
}

// channelsPageFetch fetches one page of conversations, respecting the tier2 rate limiter.
func (c *Client) channelsPageFetch(ctx context.Context, types []string, excludeArchived bool, cursor string) ([]slackgo.Channel, string, error) {
	if err := c.tier2.Wait(ctx); err != nil {
		return nil, "", err
	}
	var channels []slackgo.Channel
	var next string
	err := c.callWithRetry(ctx, func() error {
		var callErr error
		channels, next, callErr = c.api.GetConversationsContext(ctx, &slackgo.GetConversationsParameters{
			Types:           types,
			ExcludeArchived: excludeArchived,
			Limit:           1000,
			Cursor:          cursor,
		})
		return callErr
	})
	if err != nil {
		return nil, "", err
	}
	return channels, next, nil
}

// listChannelsPages paginates through all channels using pageFn. progressFn is
// called after each page with the cumulative count of channels fetched so far.
// progressFn may be nil.
func listChannelsPages(
	ctx context.Context,
	types []string,
	excludeArchived bool,
	progressFn func(int),
	pageFn channelsPageFetcher,
) ([]Channel, error) {
	var result []Channel
	cursor := ""
	for {
		channels, next, err := pageFn(ctx, types, excludeArchived, cursor)
		if err != nil {
			return nil, fmt.Errorf("list channels: %w", err)
		}
		for _, ch := range channels {
			result = append(result, slackChannelToChannel(ch))
		}
		if progressFn != nil {
			progressFn(len(result))
		}
		if next == "" {
			break
		}
		cursor = next
	}
	return result, nil
}

func slackChannelToChannel(ch slackgo.Channel) Channel {
	chType := "public_channel"
	switch {
	case ch.IsIM:
		chType = "im"
	case ch.IsMpIM:
		chType = "mpim"
	case ch.IsPrivate:
		chType = "private_channel"
	}
	return Channel{
		ID:          ch.ID,
		Name:        ch.Name,
		Type:        chType,
		MemberCount: ch.NumMembers,
		Topic:       ch.Topic.Value,
		IsArchived:  ch.IsArchived,
	}
}

// mergeChannel loads the current channels cache file, replaces or appends the
// entry for ch.ID, and writes the updated slice back atomically.
func mergeChannel(store *cache.Store, key string, ch Channel) error {
	data, _, err := store.LoadStable(key, "channels")
	if err != nil {
		return fmt.Errorf("merge channel load: %w", err)
	}
	var channels []Channel
	if data != nil {
		_ = json.Unmarshal(data, &channels)
	}
	replaced := false
	for i, existing := range channels {
		if existing.ID == ch.ID {
			channels[i] = ch
			replaced = true
			break
		}
	}
	if !replaced {
		channels = append(channels, ch)
	}
	out, err := json.Marshal(channels)
	if err != nil {
		return fmt.Errorf("merge channel marshal: %w", err)
	}
	return store.Save(key, "channels", out)
}

// FetchChannel fetches a single channel by ID from conversations.info (Tier 3),
// merges the result into the local cache file (non-fatal on write failure), and
// returns the Channel. Returns an error if the API call fails.
func (c *Client) FetchChannel(ctx context.Context, id string) (Channel, error) {
	if err := c.tier3.Wait(ctx); err != nil {
		return Channel{}, err
	}
	var apiCh *slackgo.Channel
	err := c.callWithRetry(ctx, func() error {
		var callErr error
		apiCh, callErr = c.api.GetConversationInfoContext(ctx, &slackgo.GetConversationInfoInput{
			ChannelID: id,
		})
		return callErr
	})
	if err != nil {
		return Channel{}, fmt.Errorf("fetch channel %s: %w", id, err)
	}
	ch := slackChannelToChannel(*apiCh)
	if c.store != nil && c.cacheKey != "" {
		if mergeErr := mergeChannel(c.store, c.cacheKey, ch); mergeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not update channel cache: %v\n", mergeErr)
		}
	}
	return ch, nil
}

// ResolveChannel maps a channel name or Slack channel ID to a Slack channel ID.
// Exact Slack IDs (matching channelIDPattern) are returned as-is without an API call.
func (c *Client) ResolveChannel(ctx context.Context, nameOrID string) (string, error) {
	return resolveChannel(ctx, nameOrID, func(ctx context.Context) ([]Channel, error) {
		// IMs and MPIMs have no human-readable name; skip them and archived
		// channels to minimise the number of API pages fetched.
		return c.ListChannels(ctx, []string{"public_channel", "private_channel"}, false)
	})
}

// resolveChannel is the testable inner implementation shared by ResolveChannel.
func resolveChannel(ctx context.Context, nameOrID string, listFn func(context.Context) ([]Channel, error)) (string, error) {
	// Strip leading # or @ prefix (common when copying from the Slack UI).
	name := strings.TrimLeft(nameOrID, "#@")

	if channelIDPattern.MatchString(name) {
		return name, nil
	}
	channels, err := listFn(ctx)
	if err != nil {
		return "", fmt.Errorf("list channels while resolving %q: %w", nameOrID, err)
	}
	lower := strings.ToLower(name)
	var matches []Channel
	for _, ch := range channels {
		if strings.ToLower(ch.Name) == lower {
			matches = append(matches, ch)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf(
			"channel %q not found: use `slackseek channels list` to see available channels and IDs",
			nameOrID,
		)
	case 1:
		return matches[0].ID, nil
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = fmt.Sprintf("%s (%s)", m.Name, m.ID)
		}
		return "", fmt.Errorf(
			"channel %q is ambiguous — matches: %s: use a channel ID instead",
			nameOrID, strings.Join(names, ", "),
		)
	}
}

// FetchThread returns the root message and all replies for the given thread.
// The first element is always the root message; subsequent elements are replies
// in chronological order.
func (c *Client) FetchThread(ctx context.Context, channelID, threadTS string) ([]Message, error) {
	return fetchThread(ctx, channelID, threadTS, c.repliesPageFetch)
}

// fetchThread is the testable inner implementation of FetchThread.
func fetchThread(ctx context.Context, channelID, threadTS string, replFn replyPageFetcher) ([]Message, error) {
	var all []Message
	cursor := ""
	for {
		msgs, hasMore, next, err := replFn(ctx, channelID, threadTS, cursor)
		if err != nil {
			return nil, fmt.Errorf("fetch thread %s/%s: %w", channelID, threadTS, err)
		}
		for _, m := range msgs {
			depth := 1
			if m.Timestamp == threadTS {
				depth = 0
			}
			all = append(all, convertSlackMsg(m, channelID, depth))
		}
		if !hasMore || next == "" {
			break
		}
		cursor = next
	}
	return all, nil
}

// FetchHistory retrieves channel messages with optional inline thread replies,
// sorted ascending by timestamp. A limit of 0 means unlimited.
func (c *Client) FetchHistory(ctx context.Context, channelID string, dr DateRange, limit int, threads bool) ([]Message, error) {
	return fetchHistory(ctx, channelID, dr, limit, threads, c.historyPageFetch, c.repliesPageFetch)
}

func (c *Client) historyPageFetch(ctx context.Context, channelID, oldest, latest, cursor string) ([]slackgo.Message, bool, string, error) {
	if err := c.tier3.Wait(ctx); err != nil {
		return nil, false, "", err
	}
	var resp *slackgo.GetConversationHistoryResponse
	err := c.callWithRetry(ctx, func() error {
		var callErr error
		resp, callErr = c.api.GetConversationHistoryContext(ctx, &slackgo.GetConversationHistoryParameters{
			ChannelID: channelID,
			Oldest:    oldest,
			Latest:    latest,
			Cursor:    cursor,
			Limit:     200,
			Inclusive: true,
		})
		return callErr
	})
	if err != nil {
		return nil, false, "", err
	}
	return resp.Messages, resp.HasMore, resp.ResponseMetaData.NextCursor, nil
}

func (c *Client) repliesPageFetch(ctx context.Context, channelID, threadTS, cursor string) ([]slackgo.Message, bool, string, error) {
	if err := c.tier3.Wait(ctx); err != nil {
		return nil, false, "", err
	}
	var msgs []slackgo.Message
	var hasMore bool
	var nextCursor string
	err := c.callWithRetry(ctx, func() error {
		var callErr error
		msgs, hasMore, nextCursor, callErr = c.api.GetConversationRepliesContext(ctx, &slackgo.GetConversationRepliesParameters{
			ChannelID: channelID,
			Timestamp: threadTS,
			Cursor:    cursor,
			Limit:     200,
			Inclusive: true,
		})
		return callErr
	})
	return msgs, hasMore, nextCursor, err
}

// fetchHistory is the testable inner implementation of FetchHistory.
func fetchHistory(
	ctx context.Context,
	channelID string,
	dr DateRange,
	limit int,
	threads bool,
	histFn histPageFetcher,
	replFn replyPageFetcher,
) ([]Message, error) {
	oldest := timeToUnixStr(dr.From)
	latest := timeToUnixStr(dr.To)
	rootMsgs, err := collectHistoryPages(ctx, channelID, oldest, latest, histFn)
	if err != nil {
		return nil, err
	}
	return buildMsgList(ctx, channelID, rootMsgs, limit, threads, replFn)
}

func collectHistoryPages(ctx context.Context, channelID, oldest, latest string, histFn histPageFetcher) ([]slackgo.Message, error) {
	var result []slackgo.Message
	cursor := ""
	for {
		msgs, hasMore, next, err := histFn(ctx, channelID, oldest, latest, cursor)
		if err != nil {
			return nil, fmt.Errorf("fetch history page for %s: %w", channelID, err)
		}
		result = append(result, msgs...)
		if !hasMore || next == "" {
			break
		}
		cursor = next
	}
	return result, nil
}

func buildMsgList(
	ctx context.Context,
	channelID string,
	rootMsgs []slackgo.Message,
	limit int,
	threads bool,
	replFn replyPageFetcher,
) ([]Message, error) {
	var result []Message
	for _, rm := range rootMsgs {
		result = append(result, convertSlackMsg(rm, channelID, 0))
		if threads && rm.ReplyCount > 0 {
			replies, err := collectReplies(ctx, channelID, rm.Timestamp, replFn)
			if err != nil {
				return nil, err
			}
			result = append(result, replies...)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp < result[j].Timestamp
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func collectReplies(ctx context.Context, channelID, threadTS string, replFn replyPageFetcher) ([]Message, error) {
	var result []Message
	cursor := ""
	firstPage := true
	for {
		msgs, hasMore, next, err := replFn(ctx, channelID, threadTS, cursor)
		if err != nil {
			return nil, fmt.Errorf("fetch replies for thread %s: %w", threadTS, err)
		}
		start := 0
		if firstPage {
			start = 1 // skip parent message included as first element in replies response
			firstPage = false
		}
		for _, m := range msgs[start:] {
			result = append(result, convertSlackMsg(m, channelID, 1))
		}
		if !hasMore || next == "" {
			break
		}
		cursor = next
	}
	return result, nil
}

func convertSlackMsg(m slackgo.Message, channelID string, depth int) Message {
	reactions := make([]Reaction, len(m.Reactions))
	for i, r := range m.Reactions {
		reactions[i] = Reaction{Name: r.Name, Count: r.Count}
	}
	threadTS := m.ThreadTimestamp
	if depth == 0 {
		threadTS = "" // root messages don't report their own ts as thread_ts
	}
	return Message{
		Timestamp:   m.Timestamp,
		Time:        parseSlackTS(m.Timestamp),
		UserID:      m.User,
		Text:        m.Text,
		ChannelID:   channelID,
		ThreadTS:    threadTS,
		ThreadDepth: depth,
		Reactions:   reactions,
	}
}

// timeToUnixStr converts a *time.Time to a Unix timestamp string.
// Returns empty string for nil (no bound).
func timeToUnixStr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return strconv.FormatInt(t.Unix(), 10)
}
