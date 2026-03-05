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

// metricsRunFunc is the injectable metrics pipeline for testing.
type metricsRunFunc func(
	ctx context.Context,
	workspace tokens.Workspace,
	channel string,
	dr slack.DateRange,
) ([]slack.Message, error)

// addMetricsCmd attaches the metrics command to parent.
func addMetricsCmd(
	parent *cobra.Command,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn metricsRunFunc,
) {
	parent.AddCommand(newMetricsCmd(extractFn, runFn))
}

func newMetricsCmd(
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn metricsRunFunc,
) *cobra.Command {
	return &cobra.Command{
		Use:   "metrics <channel>",
		Short: "Show aggregated metrics for a channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMetricsE(cmd, args[0], extractFn, runFn)
		},
	}
}

func runMetricsE(
	cmd *cobra.Command,
	channel string,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn metricsRunFunc,
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
		return fmt.Errorf("metrics for channel %q failed: %w", channel, err)
	}

	resolver := buildResolver(cmd.Context(), ws)
	m := output.ComputeMetrics(messages, resolver)
	return output.PrintMetrics(cmd.OutOrStdout(), output.Format(flagFormat), m)
}

// defaultRunMetrics is the production implementation of metricsRunFunc.
func defaultRunMetrics(
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
	return c.FetchHistory(ctx, channelID, dr, 0, true)
}

func init() {
	addMetricsCmd(rootCmd, tokens.DefaultExtract, defaultRunMetrics)
}
