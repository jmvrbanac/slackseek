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

// messagesRunFunc is the injectable messages pipeline for testing. It receives
// the resolved workspace, the user arg, the optional channel arg, the parsed
// date range, and the limit, then returns matching messages.
type messagesRunFunc func(
	ctx context.Context,
	workspace tokens.Workspace,
	userArg, channel string,
	dr slack.DateRange,
	limit int,
) ([]slack.Message, error)

// addMessagesCmd attaches the messages command to parent using the given
// injectable dependencies. This signature enables test injection.
func addMessagesCmd(
	parent *cobra.Command,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn messagesRunFunc,
) {
	parent.AddCommand(newMessagesCmd(extractFn, runFn))
}

func runMessagesE(
	cmd *cobra.Command,
	args []string,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn messagesRunFunc,
	channel string,
	limit int,
) error {
	userArg := args[0]
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
	messages, err := runFn(cmd.Context(), ws, userArg, channel, ParsedDateRange, limit)
	if err != nil {
		return fmt.Errorf(
			"messages for user %q failed: %w\n"+
				"Check the user with `slackseek users list` or verify your token with `slackseek auth show`.",
			userArg, err,
		)
	}
	return output.PrintMessages(cmd.OutOrStdout(), output.Format(flagFormat), messages)
}

func newMessagesCmd(
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn messagesRunFunc,
) *cobra.Command {
	var (
		channel string
		limit   int
	)
	cmd := &cobra.Command{
		Use:   "messages <user>",
		Short: "Retrieve messages from a specific user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMessagesE(cmd, args, extractFn, runFn, channel, limit)
		},
	}
	cmd.Flags().StringVarP(&channel, "channel", "c", "", "limit results to this channel name or ID")
	cmd.Flags().IntVarP(&limit, "limit", "n", 1000, "maximum number of messages to return (0 = unlimited)")
	return cmd
}

// defaultRunMessages is the production implementation of messagesRunFunc.
func defaultRunMessages(
	ctx context.Context,
	workspace tokens.Workspace,
	userArg, channelArg string,
	dr slack.DateRange,
	limit int,
) ([]slack.Message, error) {
	c := slack.NewClient(workspace.Token, workspace.Cookie, nil)

	userID, err := c.ResolveUser(ctx, userArg)
	if err != nil {
		return nil, fmt.Errorf("resolve user %q: %w", userArg, err)
	}

	channelID := channelArg
	if channelArg != "" {
		channelID, err = c.ResolveChannel(ctx, channelArg)
		if err != nil {
			return nil, fmt.Errorf("resolve channel %q: %w", channelArg, err)
		}
	}

	return c.GetUserMessages(ctx, userID, channelID, dr, limit)
}

func init() {
	addMessagesCmd(rootCmd, tokens.DefaultExtract, defaultRunMessages)
}
