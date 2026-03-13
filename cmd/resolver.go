package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/jmvrbanac/slackseek/internal/cache"
	"github.com/jmvrbanac/slackseek/internal/slack"
	"github.com/jmvrbanac/slackseek/internal/tokens"
)

// buildResolver constructs a *slack.Resolver for the given workspace by fetching
// user and channel lists (typically from cache). Returns nil on any error or when
// --no-cache is set, causing callers to fall back to raw IDs in output.
func buildResolver(ctx context.Context, ws tokens.Workspace) *slack.Resolver {
	if flagNoCache {
		return nil
	}
	c := slack.NewClientWithCache(
		ws.Token, ws.Cookie, nil,
		buildCacheStore(ws),
		cache.WorkspaceKey(ws.URL),
	)
	users, err := c.ListUsers(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not resolve IDs: %v\n", err)
		return nil
	}
	channels, err := c.ListChannels(ctx, nil, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not resolve IDs: %v\n", err)
		return nil
	}
	groups, err := c.ListUserGroups(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not resolve user groups: %v\n", err)
		groups = nil
	}
	fetchUser, fetchChannel, fetchGroups := buildFetchCallbacks(ctx, c)
	return slack.NewResolverWithFetch(users, channels, groups, fetchUser, fetchChannel, fetchGroups)
}

// buildFetchCallbacks returns the three on-miss callbacks used by NewResolverWithFetch.
func buildFetchCallbacks(ctx context.Context, c *slack.Client) (
	func(string) (string, error),
	func(string) (string, error),
	func() ([]slack.UserGroup, error),
) {
	fetchUser := func(id string) (string, error) {
		u, err := c.FetchUser(ctx, id)
		if err != nil {
			return "", err
		}
		if u.RealName != "" {
			return u.RealName, nil
		}
		return u.DisplayName, nil
	}
	fetchChannel := func(id string) (string, error) {
		ch, err := c.FetchChannel(ctx, id)
		if err != nil {
			return "", err
		}
		return ch.Name, nil
	}
	fetchGroups := func() ([]slack.UserGroup, error) {
		return c.ForceRefreshUserGroups(ctx)
	}
	return fetchUser, fetchChannel, fetchGroups
}
