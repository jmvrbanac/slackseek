package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jmvrbanac/slackseek/internal/cache"
	"github.com/jmvrbanac/slackseek/internal/output"
	"github.com/jmvrbanac/slackseek/internal/slack"
	"github.com/jmvrbanac/slackseek/internal/tokens"
	"github.com/spf13/cobra"
)

// usersRunFunc is the injectable users pipeline for testing.
type usersRunFunc func(
	ctx context.Context,
	workspace tokens.Workspace,
) ([]slack.User, error)

// addUsersCmd attaches the users command to parent using the given
// injectable dependencies. This signature enables test injection.
func addUsersCmd(
	parent *cobra.Command,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn usersRunFunc,
) {
	parent.AddCommand(newUsersCmd(extractFn, runFn))
}

func newUsersCmd(
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn usersRunFunc,
) *cobra.Command {
	users := &cobra.Command{
		Use:   "users",
		Short: "Browse workspace users",
	}
	users.AddCommand(newUsersListCmd(extractFn, runFn))
	return users
}

func runUsersListE(
	cmd *cobra.Command,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn usersRunFunc,
	includeDeleted, includeBot bool,
) error {
	result, err := extractFn()
	if err != nil {
		return fmt.Errorf(
			"failed to extract Slack credentials: %w\n"+
				"Ensure the Slack desktop application is installed and you are logged in.\n"+
				"Run `slackseek auth show` to diagnose credential extraction.",
			err,
		)
	}
	ws, err := SelectWorkspace(result.Workspaces, flagWorkspace)
	if err != nil {
		return err
	}
	for _, w := range result.Warnings {
		fmt.Fprintln(os.Stderr, "Warning:", w)
	}
	users, err := runFn(cmd.Context(), ws)
	if err != nil {
		return fmt.Errorf(
			"users list failed: %w\n"+
				"Verify your token with `slackseek auth show`.",
			err,
		)
	}
	filtered := filterUsers(users, includeDeleted, includeBot)
	return output.PrintUsers(cmd.OutOrStdout(), output.Format(flagFormat), filtered)
}

func newUsersListCmd(
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn usersRunFunc,
) *cobra.Command {
	var (
		includeDeleted bool
		includeBot     bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workspace users",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runUsersListE(cmd, extractFn, runFn, includeDeleted, includeBot)
		},
	}
	cmd.Flags().BoolVar(&includeDeleted, "deleted", false, "include deactivated users")
	cmd.Flags().BoolVar(&includeBot, "bot", false, "include bot accounts")
	return cmd
}

// filterUsers filters the user list based on the deleted and bot flags.
func filterUsers(users []slack.User, includeDeleted, includeBot bool) []slack.User {
	filtered := make([]slack.User, 0, len(users))
	for _, u := range users {
		if u.IsDeleted && !includeDeleted {
			continue
		}
		if u.IsBot && !includeBot {
			continue
		}
		filtered = append(filtered, u)
	}
	return filtered
}

// defaultRunUsers is the production implementation of usersRunFunc.
func defaultRunUsers(
	ctx context.Context,
	workspace tokens.Workspace,
) ([]slack.User, error) {
	c := slack.NewClientWithCache(workspace.Token, workspace.Cookie, nil, buildCacheStore(workspace), cache.WorkspaceKey(workspace.URL))
	c.SetRateLimitCallback(func(d time.Duration) {
		if d > 30*time.Second {
			fmt.Fprintf(os.Stderr, "rate limited — waiting %ds\n", int(d.Seconds()))
		}
	})
	return c.ListUsers(ctx)
}

func init() {
	addUsersCmd(rootCmd, tokens.DefaultExtract, defaultRunUsers)
}
