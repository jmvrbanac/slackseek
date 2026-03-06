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

// postmortemRunFunc is the injectable postmortem pipeline for testing.
type postmortemRunFunc func(
	ctx context.Context,
	workspace tokens.Workspace,
	channel string,
	dr slack.DateRange,
	noCache bool,
) ([]slack.Message, error)

// addPostmortemCmd attaches the postmortem command to parent.
func addPostmortemCmd(
	parent *cobra.Command,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn postmortemRunFunc,
) {
	parent.AddCommand(newPostmortemCmd(extractFn, runFn))
}

func newPostmortemCmd(
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn postmortemRunFunc,
) *cobra.Command {
	var noCache bool
	cmd := &cobra.Command{
		Use:   "postmortem <channel>",
		Short: "Generate a structured incident postmortem from channel history",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPostmortemE(cmd, args[0], extractFn, runFn, noCache)
		},
	}
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "bypass cache and force a fresh API fetch")
	return cmd
}

func runPostmortemE(
	cmd *cobra.Command,
	channel string,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn postmortemRunFunc,
	noCache bool,
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

	messages, err := runFn(cmd.Context(), ws, channel, ParsedDateRange, noCache)
	if err != nil {
		return fmt.Errorf("postmortem for channel %q failed: %w", channel, err)
	}

	resolver := buildResolver(cmd.Context(), ws)
	doc := output.BuildIncidentDoc(messages, resolver)

	// Default format for postmortem is markdown
	format := output.Format(flagFormat)
	if format == output.FormatText {
		format = output.FormatMarkdown
	}

	return output.PrintPostmortem(cmd.OutOrStdout(), format, doc)
}

// defaultRunPostmortem is the production implementation of postmortemRunFunc.
func defaultRunPostmortem(
	ctx context.Context,
	workspace tokens.Workspace,
	channel string,
	dr slack.DateRange,
	noCache bool,
) ([]slack.Message, error) {
	store := buildCacheStore(workspace)
	wsKey := cache.WorkspaceKey(workspace.URL)
	c := slack.NewClientWithCache(workspace.Token, workspace.Cookie, nil, store, wsKey)
	channelID, err := c.ResolveChannel(ctx, channel)
	if err != nil {
		return nil, err
	}
	return FetchHistoryCached(ctx, c, store, wsKey, channelID, dr, 0, true, noCache)
}

func init() {
	addPostmortemCmd(rootCmd, tokens.DefaultExtract, defaultRunPostmortem)
}
