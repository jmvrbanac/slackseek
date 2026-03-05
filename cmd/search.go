package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/jmvrbanac/slackseek/internal/cache"
	"github.com/jmvrbanac/slackseek/internal/output"
	"github.com/jmvrbanac/slackseek/internal/slack"
	"github.com/jmvrbanac/slackseek/internal/tokens"
	"github.com/spf13/cobra"
)

// searchRunFunc is the injectable search pipeline for testing. It receives the
// resolved workspace, the raw query, optional channels and user arguments, the
// parsed date range, and the limit, then returns matching search results.
type searchRunFunc func(
	ctx context.Context,
	workspace tokens.Workspace,
	query string,
	channels []string,
	userArg string,
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
	channels []string,
	userArg string,
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
	results, err := runFn(cmd.Context(), ws, query, channels, userArg, ParsedDateRange, limit)
	if err != nil {
		return fmt.Errorf(
			"search %q failed: %w\n"+
				"Check your Slack token with `slackseek auth show` or try a simpler query.",
			query, err,
		)
	}
	resolver := buildResolver(cmd.Context(), ws)
	return output.PrintSearchResults(cmd.OutOrStdout(), output.Format(flagFormat), results, resolver)
}

func newSearchCmd(
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn searchRunFunc,
) *cobra.Command {
	var (
		channels []string
		userArg  string
		limit    int
	)
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search messages across the workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearchE(cmd, args, extractFn, runFn, channels, userArg, limit)
		},
	}
	cmd.Flags().StringArrayVarP(&channels, "channel", "c", nil, "limit results to this channel (repeatable)")
	cmd.Flags().StringVarP(&userArg, "user", "u", "", "limit results to this user (display name, real name, or Slack ID)")
	cmd.Flags().IntVarP(&limit, "limit", "n", 100, "maximum number of results to return (0 = unlimited)")
	return cmd
}

// defaultRunSearch is the production implementation of searchRunFunc.
// For multiple channels, results are fetched in parallel (max 3 goroutines),
// deduplicated by Timestamp, and sorted ascending by time.
func defaultRunSearch(
	ctx context.Context,
	workspace tokens.Workspace,
	query string,
	channels []string,
	userArg string,
	dr slack.DateRange,
	limit int,
) ([]slack.SearchResult, error) {
	c := slack.NewClientWithCache(workspace.Token, workspace.Cookie, nil, buildCacheStore(workspace), cache.WorkspaceKey(workspace.URL))

	var userID string
	if userArg != "" {
		var err error
		userID, err = c.ResolveUser(ctx, userArg)
		if err != nil {
			return nil, fmt.Errorf("resolve user %q: %w", userArg, err)
		}
	}

	if len(channels) <= 1 {
		ch := ""
		if len(channels) == 1 {
			ch = channels[0]
		}
		q := slack.BuildSearchQuery(query, ch, userID, dr)
		return c.SearchMessages(ctx, q, limit)
	}

	return fetchMultiChannel(ctx, c, query, channels, userID, dr, limit)
}

type channelSearchResult struct {
	results []slack.SearchResult
	err     error
}

// fetchMultiChannel fetches results for multiple channels in parallel (max 3
// concurrent goroutines), deduplicates by Timestamp, and sorts ascending.
func fetchMultiChannel(
	ctx context.Context,
	c *slack.Client,
	query string,
	channels []string,
	userID string,
	dr slack.DateRange,
	limit int,
) ([]slack.SearchResult, error) {
	resultsCh := launchChannelSearches(ctx, c, query, channels, userID, dr, limit)
	return mergeChannelResults(resultsCh)
}

// launchChannelSearches fans out search goroutines and returns a result channel.
func launchChannelSearches(
	ctx context.Context,
	c *slack.Client,
	query string,
	channels []string,
	userID string,
	dr slack.DateRange,
	limit int,
) <-chan channelSearchResult {
	sem := make(chan struct{}, 3)
	resultsCh := make(chan channelSearchResult, len(channels))
	var wg sync.WaitGroup
	for _, ch := range channels {
		ch := ch
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			q := slack.BuildSearchQuery(query, ch, userID, dr)
			res, err := c.SearchMessages(ctx, q, limit)
			resultsCh <- channelSearchResult{results: res, err: err}
		}()
	}
	go func() { wg.Wait(); close(resultsCh) }()
	return resultsCh
}

// mergeChannelResults deduplicates and sorts results from multiple channels.
func mergeChannelResults(resultsCh <-chan channelSearchResult) ([]slack.SearchResult, error) {
	seen := make(map[string]slack.SearchResult)
	var firstErr error
	for r := range resultsCh {
		if r.err != nil && firstErr == nil {
			firstErr = r.err
		}
		for _, sr := range r.results {
			key := sr.ChannelID + "/" + sr.Timestamp
			if _, ok := seen[key]; !ok {
				seen[key] = sr
			}
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	merged := make([]slack.SearchResult, 0, len(seen))
	for _, sr := range seen {
		merged = append(merged, sr)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Time.Before(merged[j].Time)
	})
	return merged, nil
}

func init() {
	addSearchCmd(rootCmd, tokens.DefaultExtract, defaultRunSearch)
}
