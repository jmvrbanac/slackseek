package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jmvrbanac/slackseek/internal/output"
	"github.com/jmvrbanac/slackseek/internal/slack"
	"github.com/jmvrbanac/slackseek/internal/tokens"
	"github.com/spf13/cobra"
)

// historyRunFunc is the injectable history pipeline for testing. It receives
// the resolved workspace, the channel name/ID, a resolved workspace name,
// the parsed date range, the limit, and the threads flag.
type historyRunFunc func(
	ctx context.Context,
	workspace tokens.Workspace,
	channel, workspaceName string,
	dr slack.DateRange,
	limit int,
	threads bool,
) ([]slack.Message, error)

// addHistoryCmd attaches the history command to parent using the given
// injectable dependencies. This signature enables test injection.
func addHistoryCmd(
	parent *cobra.Command,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn historyRunFunc,
) {
	parent.AddCommand(newHistoryCmd(extractFn, runFn))
}

func runHistoryE(
	cmd *cobra.Command,
	args []string,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn historyRunFunc,
	threads bool,
	limit int,
) error {
	channel := args[0]
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
	messages, err := runFn(cmd.Context(), ws, channel, ws.Name, ParsedDateRange, limit, threads)
	if err != nil {
		return fmt.Errorf(
			"history for channel %q failed: %w\n"+
				"Check the channel name with `slackseek channels list` or verify your token with `slackseek auth show`.",
			channel, err,
		)
	}
	return output.PrintMessages(cmd.OutOrStdout(), output.Format(flagFormat), messages)
}

func newHistoryCmd(
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn historyRunFunc,
) *cobra.Command {
	var (
		threads bool
		limit   int
	)
	cmd := &cobra.Command{
		Use:   "history <channel>",
		Short: "Retrieve message history for a channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHistoryE(cmd, args, extractFn, runFn, threads, limit)
		},
	}
	cmd.Flags().BoolVarP(&threads, "threads", "T", true, "include inline thread replies")
	cmd.Flags().IntVarP(&limit, "limit", "n", 1000, "maximum number of messages to return (0 = unlimited)")
	return cmd
}

// defaultRunHistory is the production implementation of historyRunFunc.
func defaultRunHistory(
	ctx context.Context,
	workspace tokens.Workspace,
	channel, _ string,
	dr slack.DateRange,
	limit int,
	threads bool,
) ([]slack.Message, error) {
	c := slack.NewClient(workspace.Token, workspace.Cookie, nil)
	c.SetRateLimitCallback(func(d time.Duration) {
		if d > 30*time.Second {
			fmt.Fprintf(os.Stderr, "rate limited — waiting %ds\n", int(d.Seconds()))
		}
	})

	channelID, err := c.ResolveChannel(ctx, channel)
	if err != nil {
		return nil, err
	}

	return c.FetchHistory(ctx, channelID, dr, limit, threads)
}

func init() {
	addHistoryCmd(rootCmd, tokens.DefaultExtract, defaultRunHistory)
}
