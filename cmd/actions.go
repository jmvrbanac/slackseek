package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/jmvrbanac/slackseek/internal/cache"
	"github.com/jmvrbanac/slackseek/internal/output"
	"github.com/jmvrbanac/slackseek/internal/slack"
	"github.com/jmvrbanac/slackseek/internal/tokens"
	"github.com/spf13/cobra"
)

// actionsRunFunc is the injectable actions pipeline for testing.
type actionsRunFunc func(
	ctx context.Context,
	workspace tokens.Workspace,
	channel string,
	dr slack.DateRange,
) ([]slack.Message, error)

// addActionsCmd attaches the actions command to parent.
func addActionsCmd(
	parent *cobra.Command,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn actionsRunFunc,
) {
	parent.AddCommand(newActionsCmd(extractFn, runFn))
}

func newActionsCmd(
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn actionsRunFunc,
) *cobra.Command {
	return &cobra.Command{
		Use:   "actions <channel>",
		Short: "Extract commitment/action-item messages from a channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runActionsE(cmd, args[0], extractFn, runFn)
		},
	}
}

func runActionsE(
	cmd *cobra.Command,
	channel string,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn actionsRunFunc,
) error {
	result, err := extractFn()
	if err != nil {
		return fmt.Errorf("failed to extract Slack credentials: %w", err)
	}
	ws, err := SelectWorkspace(result.Workspaces, flagWorkspace)
	if err != nil {
		return err
	}
	for _, w := range result.Warnings {
		fmt.Fprintln(os.Stderr, "Warning:", w)
	}

	messages, err := runFn(cmd.Context(), ws, channel, ParsedDateRange)
	if err != nil {
		return fmt.Errorf("actions for channel %q failed: %w", channel, err)
	}

	resolver := buildResolver(cmd.Context(), ws)
	items := output.ExtractActions(messages, resolver)
	return output.PrintActions(cmd.OutOrStdout(), output.Format(flagFormat), items)
}

// defaultRunActions is the production implementation of actionsRunFunc.
func defaultRunActions(
	ctx context.Context,
	workspace tokens.Workspace,
	channel string,
	dr slack.DateRange,
) ([]slack.Message, error) {
	c := slack.NewClientWithCache(workspace.Token, workspace.Cookie, nil, buildCacheStore(workspace), cache.WorkspaceKey(workspace.URL))
	channelID, err := c.ResolveChannel(ctx, channel)
	if err != nil {
		return nil, err
	}
	return c.FetchHistory(ctx, channelID, dr, 0, false)
}

func init() {
	addActionsCmd(rootCmd, tokens.DefaultExtract, defaultRunActions)
}
