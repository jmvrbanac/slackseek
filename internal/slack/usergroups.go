package slack

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmvrbanac/slackseek/internal/cache"
	slackgo "github.com/slack-go/slack"
)

// listUserGroupsCached is the testable inner implementation of ListUserGroups.
// When store is non-nil and cacheKey is non-empty it attempts a cache load
// before calling listFn. On a miss it calls listFn and persists the result.
func listUserGroupsCached(
	ctx context.Context,
	store *cache.Store,
	cacheKey string,
	listFn func(context.Context) ([]UserGroup, error),
) ([]UserGroup, error) {
	if store != nil && cacheKey != "" {
		data, hit, err := store.Load(cacheKey, "user_groups")
		if err != nil {
			return nil, fmt.Errorf("cache load user_groups: %w", err)
		}
		if hit {
			var groups []UserGroup
			if jsonErr := json.Unmarshal(data, &groups); jsonErr == nil {
				return groups, nil
			}
		}
	}
	groups, err := listFn(ctx)
	if err != nil {
		return nil, err
	}
	if store != nil && cacheKey != "" {
		if data, jsonErr := json.Marshal(groups); jsonErr == nil {
			_ = store.Save(cacheKey, "user_groups", data)
		}
	}
	return groups, nil
}

// ListUserGroups returns all user groups (subteams) from the usergroups.list API.
// Results are cached using the workspace-scoped cache store.
func (c *Client) ListUserGroups(ctx context.Context) ([]UserGroup, error) {
	return listUserGroupsCached(ctx, c.store, c.cacheKey, func(ctx context.Context) ([]UserGroup, error) {
		apiGroups, err := c.api.GetUserGroupsContext(ctx, slackgo.GetUserGroupsOptionIncludeDisabled(false))
		if err != nil {
			return nil, fmt.Errorf("list user groups: %w", err)
		}
		result := make([]UserGroup, len(apiGroups))
		for i, g := range apiGroups {
			result[i] = UserGroup{
				ID:     g.ID,
				Handle: g.Handle,
				Name:   g.Name,
			}
		}
		return result, nil
	})
}
