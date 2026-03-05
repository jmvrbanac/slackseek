package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jmvrbanac/slackseek/internal/cache"
	"github.com/jmvrbanac/slackseek/internal/output"
	"github.com/jmvrbanac/slackseek/internal/slack"
	"github.com/jmvrbanac/slackseek/internal/tokens"
	"github.com/spf13/cobra"
)

// threadRunFunc is the injectable thread pipeline for testing.
type threadRunFunc func(
	ctx context.Context,
	workspace tokens.Workspace,
	channelID, threadTS string,
) ([]slack.Message, error)

// addThreadCmd attaches the thread command to parent.
func addThreadCmd(
	parent *cobra.Command,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn threadRunFunc,
) {
	parent.AddCommand(newThreadCmd(extractFn, runFn))
}

func newThreadCmd(
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn threadRunFunc,
) *cobra.Command {
	return &cobra.Command{
		Use:   "thread <permalink-url>",
		Short: "Fetch and display a Slack thread by permalink URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runThreadE(cmd, args[0], extractFn, runFn)
		},
	}
}

func runThreadE(
	cmd *cobra.Command,
	rawURL string,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn threadRunFunc,
) error {
	permalink, err := slack.ParsePermalink(rawURL)
	if err != nil {
		return fmt.Errorf("invalid Slack permalink %q: %w\nExpected format: https://<workspace>.slack.com/archives/<channelID>/p<ts>", rawURL, err)
	}

	result, err := extractFn()
	if err != nil {
		return fmt.Errorf("failed to extract Slack credentials: %w", err)
	}

	ws, err := selectWorkspaceByURL(result.Workspaces, permalink.WorkspaceURL, flagWorkspace)
	if err != nil {
		return err
	}
	for _, w := range result.Warnings {
		fmt.Fprintln(os.Stderr, "Warning:", w)
	}

	messages, err := runFn(cmd.Context(), ws, permalink.ChannelID, permalink.ThreadTS)
	if err != nil {
		return fmt.Errorf("fetch thread %s/%s: %w", permalink.ChannelID, permalink.ThreadTS, err)
	}

	resolver := buildResolver(cmd.Context(), ws)
	participants := collectParticipants(messages, resolver)

	if output.Format(flagFormat) == output.FormatJSON {
		return printThreadJSON(cmd, messages, resolver, permalink, participants)
	}

	if err := output.PrintMessages(cmd.OutOrStdout(), output.Format(flagFormat), messages, resolver); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nParticipants: %s\n", strings.Join(participants, ", "))
	return nil
}

func collectParticipants(messages []slack.Message, resolver *slack.Resolver) []string {
	seen := make(map[string]bool)
	var participants []string
	for _, m := range messages {
		name := m.UserID
		if resolver != nil {
			name = resolver.UserDisplayName(m.UserID)
		}
		if !seen[name] {
			seen[name] = true
			participants = append(participants, name)
		}
	}
	sort.Strings(participants)
	return participants
}

type threadOutputJSON struct {
	ThreadTS     string        `json:"thread_ts"`
	ChannelID    string        `json:"channel_id"`
	Participants []string      `json:"participants"`
	Messages     []interface{} `json:"messages"`
}

func printThreadJSON(cmd *cobra.Command, messages []slack.Message, resolver *slack.Resolver, permalink slack.ThreadPermalink, participants []string) error {
	msgs := make([]interface{}, 0, len(messages))
	for _, m := range messages {
		user := m.UserID
		if resolver != nil {
			user = resolver.UserDisplayName(m.UserID)
		}
		text := m.Text
		if resolver != nil {
			text = resolver.ResolveMentions(text)
		}
		msgs = append(msgs, map[string]interface{}{
			"timestamp":    m.Timestamp,
			"time":         m.Time,
			"user":         user,
			"text":         text,
			"thread_depth": m.ThreadDepth,
			"reactions":    m.Reactions,
		})
	}
	out := threadOutputJSON{
		ThreadTS:     permalink.ThreadTS,
		ChannelID:    permalink.ChannelID,
		Participants: participants,
		Messages:     msgs,
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// selectWorkspaceByURL selects a workspace matching the given URL, falling
// back to flagWorkspace selector or first-workspace logic.
func selectWorkspaceByURL(workspaces []tokens.Workspace, wsURL, selector string) (tokens.Workspace, error) {
	// First try exact URL match from the permalink
	for _, ws := range workspaces {
		if strings.HasPrefix(ws.URL, wsURL) || strings.HasPrefix(wsURL, ws.URL) {
			return ws, nil
		}
	}
	// Fall back to normal workspace selection
	return SelectWorkspace(workspaces, selector)
}

// defaultRunThread is the production implementation of threadRunFunc.
func defaultRunThread(
	ctx context.Context,
	workspace tokens.Workspace,
	channelID, threadTS string,
) ([]slack.Message, error) {
	c := slack.NewClientWithCache(workspace.Token, workspace.Cookie, nil, buildCacheStore(workspace), cache.WorkspaceKey(workspace.URL))
	return c.FetchThread(ctx, channelID, threadTS)
}

func init() {
	addThreadCmd(rootCmd, tokens.DefaultExtract, defaultRunThread)
}
