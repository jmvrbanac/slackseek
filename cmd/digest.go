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

// digestRunFunc is the injectable digest pipeline for testing.
type digestRunFunc func(
	ctx context.Context,
	workspace tokens.Workspace,
	userID string,
	dr slack.DateRange,
) ([]slack.Message, error)

// addDigestCmd attaches the digest command to parent.
func addDigestCmd(
	parent *cobra.Command,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn digestRunFunc,
) {
	parent.AddCommand(newDigestCmd(extractFn, runFn))
}

func newDigestCmd(
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn digestRunFunc,
) *cobra.Command {
	var userArg string
	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Show per-channel message digest for a user",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if userArg == "" {
				return fmt.Errorf("--user / -u is required")
			}
			return runDigestE(cmd, userArg, extractFn, runFn)
		},
	}
	cmd.Flags().StringVarP(&userArg, "user", "u", "", "user display name, real name, or Slack ID (required)")
	return cmd
}

func runDigestE(
	cmd *cobra.Command,
	userArg string,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn digestRunFunc,
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

	messages, err := runFn(cmd.Context(), ws, userArg, ParsedDateRange)
	if err != nil {
		return fmt.Errorf("digest for user %q failed: %w", userArg, err)
	}

	resolver := buildResolver(cmd.Context(), ws)
	groups := output.GroupByChannel(messages)
	return output.PrintDigest(cmd.OutOrStdout(), output.Format(flagFormat), groups, resolver)
}

// defaultRunDigest is the production implementation of digestRunFunc.
func defaultRunDigest(
	ctx context.Context,
	workspace tokens.Workspace,
	userArg string,
	dr slack.DateRange,
) ([]slack.Message, error) {
	c := slack.NewClientWithCache(workspace.Token, workspace.Cookie, nil, buildCacheStore(workspace), cache.WorkspaceKey(workspace.URL))
	userID, err := c.ResolveUser(ctx, userArg)
	if err != nil {
		return nil, fmt.Errorf("resolve user %q: %w", userArg, err)
	}
	return c.GetUserMessages(ctx, userID, "", dr, 0)
}

func init() {
	addDigestCmd(rootCmd, tokens.DefaultExtract, defaultRunDigest)
}
