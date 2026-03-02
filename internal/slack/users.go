package slack

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// slackIDPattern matches Slack user IDs (U…) and workspace-app IDs (W…).
var slackIDPattern = regexp.MustCompile(`^[UW][A-Z0-9]+$`)

// ListUsers returns all workspace members by fetching from users.list.
// The slack-go library handles cursor pagination internally.
func (c *Client) ListUsers(ctx context.Context) ([]User, error) {
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
