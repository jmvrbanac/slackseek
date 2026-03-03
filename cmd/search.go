package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/jmvrbanac/slackseek/internal/output"
	"github.com/jmvrbanac/slackseek/internal/slack"
	"github.com/jmvrbanac/slackseek/internal/tokens"
	"github.com/spf13/cobra"
)

// searchRunFunc is the injectable search pipeline for testing. It receives the
// resolved workspace, the raw query, optional channel and user arguments, the
// parsed date range, and the limit, then returns matching search results.
type searchRunFunc func(
	ctx context.Context,
	workspace tokens.Workspace,
	query, channel, userArg string,
	dr slack.DateRange,
	limit int,
) ([]slack.SearchResult, error)

// addSearchCmd attaches the search command to parent using the given injectable
// dependencies. This signature enables test injection.
func addSearchCmd(
	parent *cobra.Command,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn searchRunFunc,
) {
	parent.AddCommand(newSearchCmd(extractFn, runFn))
}

func runSearchE(
	cmd *cobra.Command,
	args []string,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn searchRunFunc,
	channel, userArg string,
	limit int,
) error {
	query := args[0]
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
	results, err := runFn(cmd.Context(), ws, query, channel, userArg, ParsedDateRange, limit)
	if err != nil {
		return fmt.Errorf(
			"search %q failed: %w\n"+
				"Check your Slack token with `slackseek auth show` or try a simpler query.",
			query, err,
		)
	}
	return output.PrintSearchResults(cmd.OutOrStdout(), output.Format(flagFormat), results)
}

func newSearchCmd(
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn searchRunFunc,
) *cobra.Command {
	var (
		channel string
		userArg string
		limit   int
	)
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search messages across the workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearchE(cmd, args, extractFn, runFn, channel, userArg, limit)
		},
	}
	cmd.Flags().StringVarP(&channel, "channel", "c", "", "limit results to this channel name or ID")
	cmd.Flags().StringVarP(&userArg, "user", "u", "", "limit results to this user (display name, real name, or Slack ID)")
	cmd.Flags().IntVarP(&limit, "limit", "n", 100, "maximum number of results to return (0 = unlimited)")
	return cmd
}

// defaultRunSearch is the production implementation of searchRunFunc.
func defaultRunSearch(
	ctx context.Context,
	workspace tokens.Workspace,
	query, channel, userArg string,
	dr slack.DateRange,
	limit int,
) ([]slack.SearchResult, error) {
	c := slack.NewClient(workspace.Token, workspace.Cookie, nil)

	var userID string
	if userArg != "" {
		var err error
		userID, err = c.ResolveUser(ctx, userArg)
		if err != nil {
			return nil, fmt.Errorf("resolve user %q: %w", userArg, err)
		}
	}

	q := slack.BuildSearchQuery(query, channel, userID, dr)
	return c.SearchMessages(ctx, q, limit)
}

func init() {
	addSearchCmd(rootCmd, tokens.DefaultExtract, defaultRunSearch)
}
