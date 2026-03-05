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

// channelsRunFunc is the injectable channels pipeline for testing.
type channelsRunFunc func(
	ctx context.Context,
	workspace tokens.Workspace,
	types []string,
	includeArchived bool,
) ([]slack.Channel, error)

// validChannelTypes maps CLI flag values to Slack API type strings.
var validChannelTypes = map[string]string{
	"public":  "public_channel",
	"private": "private_channel",
	"mpim":    "mpim",
	"im":      "im",
}

// addChannelsCmd attaches the channels command to parent using the given
// injectable dependencies. This signature enables test injection.
func addChannelsCmd(
	parent *cobra.Command,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn channelsRunFunc,
) {
	parent.AddCommand(newChannelsCmd(extractFn, runFn))
}

func newChannelsCmd(
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn channelsRunFunc,
) *cobra.Command {
	channels := &cobra.Command{
		Use:   "channels",
		Short: "Browse workspace channels",
	}
	channels.AddCommand(newChannelsListCmd(extractFn, runFn))
	return channels
}

// resolveChannelTypes converts the --type CLI flag to the Slack API types slice.
func resolveChannelTypes(channelType string) ([]string, error) {
	if channelType == "" {
		return nil, nil
	}
	apiType, ok := validChannelTypes[channelType]
	if !ok {
		return nil, fmt.Errorf("invalid --type %q: must be one of public, private, mpim, im", channelType)
	}
	return []string{apiType}, nil
}

func runChannelsListE(
	cmd *cobra.Command,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn channelsRunFunc,
	channelType string,
	archived bool,
) error {
	types, err := resolveChannelTypes(channelType)
	if err != nil {
		return err
	}
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
	channels, err := runFn(cmd.Context(), ws, types, archived)
	if err != nil {
		return fmt.Errorf(
			"channels list failed: %w\n"+
				"Verify your token with `slackseek auth show`.",
			err,
		)
	}
	return output.PrintChannels(cmd.OutOrStdout(), output.Format(flagFormat), channels)
}

func newChannelsListCmd(
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn channelsRunFunc,
) *cobra.Command {
	var (
		channelType string
		archived    bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workspace channels",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runChannelsListE(cmd, extractFn, runFn, channelType, archived)
		},
	}
	cmd.Flags().StringVar(&channelType, "type", "", "channel type filter: public, private, mpim, im")
	cmd.Flags().BoolVar(&archived, "archived", false, "include archived channels")
	return cmd
}

// defaultRunChannels is the production implementation of channelsRunFunc.
func defaultRunChannels(
	ctx context.Context,
	workspace tokens.Workspace,
	types []string,
	includeArchived bool,
) ([]slack.Channel, error) {
	c := slack.NewClientWithCache(workspace.Token, workspace.Cookie, nil, buildCacheStore(workspace), cache.WorkspaceKey(workspace.URL))
	c.SetRateLimitCallback(func(d time.Duration) {
		if d > 30*time.Second {
			fmt.Fprintf(os.Stderr, "rate limited — waiting %ds\n", int(d.Seconds()))
		}
	})
	var lastCount int
	c.SetPageFetchedCallback(func(n int) {
		lastCount = n
		fmt.Fprintf(os.Stderr, "\rfetching channels: %d fetched...", n)
	})
	channels, err := c.ListChannels(ctx, types, includeArchived)
	if lastCount > 0 {
		if err != nil {
			fmt.Fprintln(os.Stderr)
		} else {
			fmt.Fprintf(os.Stderr, "\rfetching channels: %d fetched — done\n", lastCount)
		}
	}
	return channels, err
}

func init() {
	addChannelsCmd(rootCmd, tokens.DefaultExtract, defaultRunChannels)
}
