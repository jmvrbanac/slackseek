package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/jmvrbanac/slackseek/internal/cache"
)

// slackIDPattern matches Slack user IDs (U…) and workspace-app IDs (W…).
var slackIDPattern = regexp.MustCompile(`^[UW][A-Z0-9]+$`)

// listUsersCached is the testable inner implementation of ListUsers.
// When store is non-nil and cacheKey is non-empty it attempts a cache load
// before calling listFn. On a miss it calls listFn and persists the result.
func listUsersCached(
	ctx context.Context,
	store *cache.Store,
	cacheKey string,
	listFn func(context.Context) ([]User, error),
) ([]User, error) {
	if store != nil && cacheKey != "" {
		data, hit, err := store.LoadStable(cacheKey, "users")
		if err != nil {
			return nil, fmt.Errorf("cache load users: %w", err)
		}
		if hit {
			var users []User
			if jsonErr := json.Unmarshal(data, &users); jsonErr == nil {
				return users, nil
			}
		}
	}
	users, err := listFn(ctx)
	if err != nil {
		return nil, err
	}
	if store != nil && cacheKey != "" {
		if data, jsonErr := json.Marshal(users); jsonErr == nil {
			_ = store.Save(cacheKey, "users", data)
		}
	}
	return users, nil
}

// ListUsers returns all workspace members by fetching from users.list.
// The slack-go library handles cursor pagination internally.
func (c *Client) ListUsers(ctx context.Context) ([]User, error) {
	return listUsersCached(ctx, c.store, c.cacheKey, func(ctx context.Context) ([]User, error) {
		if err := c.tier2.Wait(ctx); err != nil {
			return nil, err
		}
		apiUsers, err := c.api.GetUsersContext(ctx)
		if err != nil {
			return nil, fmt.Errorf("list users: %w", err)
		}
		result := make([]User, len(apiUsers))
		for i, u := range apiUsers {
			result[i] = User{
				ID:          u.ID,
				DisplayName: u.Profile.DisplayName,
				RealName:    u.RealName,
				Email:       u.Profile.Email,
				IsBot:       u.IsBot,
				IsDeleted:   u.Deleted,
			}
		}
		return result, nil
	})
}

// mergeUser loads the current users cache file, replaces or appends the entry
// for u.ID, and writes the updated slice back atomically.
func mergeUser(store *cache.Store, key string, u User) error {
	data, _, err := store.LoadStable(key, "users")
	if err != nil {
		return fmt.Errorf("merge user load: %w", err)
	}
	var users []User
	if data != nil {
		_ = json.Unmarshal(data, &users) // ignore corrupt cache; overwrite
	}
	replaced := false
	for i, existing := range users {
		if existing.ID == u.ID {
			users[i] = u
			replaced = true
			break
		}
	}
	if !replaced {
		users = append(users, u)
	}
	out, err := json.Marshal(users)
	if err != nil {
		return fmt.Errorf("merge user marshal: %w", err)
	}
	return store.Save(key, "users", out)
}

// FetchUser fetches a single user by ID from users.info (Tier 4), merges the
// result into the local cache file (non-fatal on write failure), and returns
// the User. Returns an error if the API call fails.
func (c *Client) FetchUser(ctx context.Context, id string) (User, error) {
	if err := c.tier4.Wait(ctx); err != nil {
		return User{}, err
	}
	apiUser, err := c.api.GetUserInfoContext(ctx, id)
	if err != nil {
		return User{}, fmt.Errorf("fetch user %s: %w", id, err)
	}
	u := User{
		ID:          apiUser.ID,
		DisplayName: apiUser.Profile.DisplayName,
		RealName:    apiUser.RealName,
		Email:       apiUser.Profile.Email,
		IsBot:       apiUser.IsBot,
		IsDeleted:   apiUser.Deleted,
	}
	if c.store != nil && c.cacheKey != "" {
		if mergeErr := mergeUser(c.store, c.cacheKey, u); mergeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not update user cache: %v\n", mergeErr)
		}
	}
	return u, nil
}

// ResolveUser maps a display name, real name, or Slack user/bot ID to a Slack
// user ID. Exact Slack IDs (matching ^[UW][A-Z0-9]+$) are returned as-is
// without making an API call.
func (c *Client) ResolveUser(ctx context.Context, nameOrID string) (string, error) {
	return resolveUser(ctx, nameOrID, c.ListUsers)
}

// resolveUser is the testable inner implementation shared by ResolveUser.
func resolveUser(ctx context.Context, nameOrID string, listFn func(context.Context) ([]User, error)) (string, error) {
	if slackIDPattern.MatchString(nameOrID) {
		return nameOrID, nil
	}

	users, err := listFn(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve user %q: %w", nameOrID, err)
	}

	lower := strings.ToLower(nameOrID)
	var matches []User
	for _, u := range users {
		if strings.Contains(strings.ToLower(u.DisplayName), lower) ||
			strings.Contains(strings.ToLower(u.RealName), lower) {
			matches = append(matches, u)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf(
			"user %q not found: use `slackseek users list` to see available display names and IDs",
			nameOrID,
		)
	case 1:
		return matches[0].ID, nil
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = fmt.Sprintf("%s (%s)", m.DisplayName, m.ID)
		}
		return "", fmt.Errorf(
			"user %q is ambiguous — matches: %s: use a more specific name or a Slack user ID",
			nameOrID, strings.Join(names, ", "),
		)
	}
}
