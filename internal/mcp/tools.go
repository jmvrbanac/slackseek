package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jmvrbanac/slackseek/internal/cache"
	"github.com/jmvrbanac/slackseek/internal/slack"
	"github.com/jmvrbanac/slackseek/internal/tokens"
)

// slackClient is a narrow interface over *slack.Client covering only the
// methods used by MCP tool handlers. *slack.Client satisfies this interface
// without modification.
type slackClient interface {
	SearchMessages(ctx context.Context, query string, limit int) ([]slack.SearchResult, error)
	FetchHistory(ctx context.Context, channelID string, dr slack.DateRange, limit int, threads bool) ([]slack.Message, error)
	GetUserMessages(ctx context.Context, userID, channelID string, dr slack.DateRange, limit int) ([]slack.Message, error)
	FetchThread(ctx context.Context, channelID, threadTS string) ([]slack.Message, error)
	ListChannels(ctx context.Context, types []string, includeArchived bool) ([]slack.Channel, error)
	ListUsers(ctx context.Context) ([]slack.User, error)
	ResolveChannel(ctx context.Context, nameOrID string) (string, error)
	ResolveUser(ctx context.Context, nameOrID string) (string, error)
	FetchUser(ctx context.Context, id string) (slack.User, error)
	FetchChannel(ctx context.Context, id string) (slack.Channel, error)
	ListUserGroups(ctx context.Context) ([]slack.UserGroup, error)
	ForceRefreshUserGroups(ctx context.Context) ([]slack.UserGroup, error)
}

// mcpCacheTTL is the cache TTL used when building a Slack client for MCP tool
// calls. 24 hours matches the default CLI cache TTL.
const mcpCacheTTL = 24 * time.Hour

// parseDateRange resolves since/until strings into a slack.DateRange.
// It delegates to ParseRelativeDateRange when either argument is non-empty
// (supporting ISO date, RFC 3339, and relative durations like "7d"), and to
// ParseDateRange when both are absolute or empty.
func parseDateRange(since, until string) (slack.DateRange, error) {
	if since != "" || until != "" {
		dr, err := slack.ParseRelativeDateRange(since, until)
		if err != nil {
			return dr, fmt.Errorf("invalid date range: %w", err)
		}
		return dr, nil
	}
	return slack.ParseDateRange("", "")
}

// selectWorkspace picks a workspace by name or URL from the slice.
// When selector is empty the first workspace is returned. Unlike the CLI
// version this function does not write to stderr (stdout/stderr are the
// MCP transport on stdio).
func selectWorkspace(workspaces []tokens.Workspace, selector string) (tokens.Workspace, error) {
	if len(workspaces) == 0 {
		return tokens.Workspace{}, fmt.Errorf(
			"no Slack workspaces found — ensure the Slack desktop app is running and you are logged in",
		)
	}
	if selector == "" {
		return workspaces[0], nil
	}
	lower := strings.ToLower(selector)
	for _, ws := range workspaces {
		if strings.ToLower(ws.Name) == lower || ws.URL == selector {
			return ws, nil
		}
	}
	names := make([]string, len(workspaces))
	for i, w := range workspaces {
		names[i] = w.Name
	}
	return tokens.Workspace{}, fmt.Errorf(
		"workspace %q not found — available workspaces: %s",
		selector, strings.Join(names, ", "),
	)
}

// buildMCPClient constructs a *slack.Client backed by a file cache for the
// given workspace. The cache TTL is fixed at mcpCacheTTL (24 h).
func buildMCPClient(ws tokens.Workspace) slackClient {
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		return slack.NewClient(ws.Token, ws.Cookie, nil)
	}
	store := cache.NewStore(filepath.Join(userCacheDir, "slackseek"), mcpCacheTTL)
	return slack.NewClientWithCache(ws.Token, ws.Cookie, nil, store, cache.WorkspaceKey(ws.URL))
}

// buildMCPResolver constructs a *slack.Resolver for the given workspace.
// Unlike cmd.buildResolver it does not read global flags or write to stderr.
// Returns nil on any error; callers fall back to raw IDs in that case.
func buildMCPResolver(ctx context.Context, ws tokens.Workspace, c slackClient) *slack.Resolver {
	users, err := c.ListUsers(ctx)
	if err != nil {
		return nil
	}
	channels, err := c.ListChannels(ctx, nil, false)
	if err != nil {
		return nil
	}
	groups, _ := c.ListUserGroups(ctx)

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
	return slack.NewResolverWithFetch(users, channels, groups, fetchUser, fetchChannel, fetchGroups)
}
