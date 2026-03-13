// Package cmd contains the cobra CLI commands for slackseek.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jmvrbanac/slackseek/internal/cache"
	"github.com/jmvrbanac/slackseek/internal/output"
	"github.com/jmvrbanac/slackseek/internal/slack"
	"github.com/jmvrbanac/slackseek/internal/tokens"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// Global flag values populated by cobra before PersistentPreRunE runs.
var (
	flagWorkspace    string
	flagFormat       string
	flagFrom         string
	flagTo           string
	flagCacheTTL     time.Duration
	flagRefreshCache bool
	flagNoCache      bool
	flagQuiet        bool
	flagSince        string
	flagUntil        string
	flagWidth        int
	flagEmojiEnabled bool
	flagNoEmoji      bool
)

// ParsedDateRange is set by PersistentPreRunE and available to all subcommands.
var ParsedDateRange slack.DateRange

// newRootCmdWithFlags builds a fresh root command.  Callers that want
// the canonical singleton should use Execute(); tests call NewRootCmd().
func buildRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:               "slackseek",
		Short:             "Query Slack workspaces using locally extracted credentials",
		SilenceUsage:      true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error { return globalPreRun() },
	}
	registerRootFlags(root)
	return root
}

// globalPreRun validates global flags and populates shared state.
func globalPreRun() error {
	if err := validateFormat(flagFormat); err != nil {
		return err
	}
	if flagSince != "" && flagFrom != "" {
		return fmt.Errorf("--since and --from are mutually exclusive: use one or the other")
	}
	if flagUntil != "" && flagTo != "" {
		return fmt.Errorf("--until and --to are mutually exclusive: use one or the other")
	}
	if flagEmojiEnabled && flagNoEmoji {
		return fmt.Errorf("--emoji and --no-emoji are mutually exclusive")
	}
	dr, err := parseDateRange()
	if err != nil {
		return err
	}
	ParsedDateRange = dr
	if flagCacheTTL < 0 {
		return fmt.Errorf("invalid --cache-ttl: duration must not be negative")
	}
	if flagRefreshCache && flagNoCache {
		return fmt.Errorf("--refresh-cache and --no-cache are mutually exclusive")
	}
	output.EmojiEnabled = resolveEmojiEnabled()
	output.WrapWidth = resolveWidth()
	return nil
}

// parseDateRange resolves --since/--until or --from/--to into a DateRange.
func parseDateRange() (slack.DateRange, error) {
	if flagSince != "" || flagUntil != "" {
		return slack.ParseRelativeDateRange(flagSince, flagUntil)
	}
	return slack.ParseDateRange(flagFrom, flagTo)
}

// registerRootFlags attaches all persistent flags to root.
func registerRootFlags(root *cobra.Command) {
	f := root.PersistentFlags()
	f.StringVarP(&flagWorkspace, "workspace", "w", "", "workspace name or base URL to target")
	f.StringVar(&flagFormat, "format", "text", "output format: text | table | json | markdown")
	f.StringVar(&flagFrom, "from", "", "start of date range: YYYY-MM-DD or RFC 3339")
	f.StringVar(&flagTo, "to", "", "end of date range: YYYY-MM-DD or RFC 3339")
	f.DurationVar(&flagCacheTTL, "cache-ttl", 24*time.Hour, "how long history cache entries remain valid (0 disables caching; does not affect entity/user/channel/group caches)")
	f.BoolVar(&flagRefreshCache, "refresh-cache", false, "force a fresh API fetch and overwrite the cached data")
	f.BoolVar(&flagNoCache, "no-cache", false, "bypass the cache entirely (read and write)")
	f.BoolVarP(&flagQuiet, "quiet", "q", false, "suppress progress and rate-limit notices from stderr")
	f.StringVar(&flagSince, "since", "", "start of range: ISO date, RFC 3339, or duration (30m, 4h, 7d, 2w)")
	f.StringVar(&flagUntil, "until", "", "end of range: ISO date, RFC 3339, or duration offset")
	f.IntVar(&flagWidth, "width", 0, "text wrap column width (0 = auto-detect tty, or 120 for pipes)")
	f.BoolVar(&flagEmojiEnabled, "emoji", false, "render :name: tokens as Unicode (default: on for tty)")
	f.BoolVar(&flagNoEmoji, "no-emoji", false, "disable emoji rendering")
}

// buildCacheStore constructs a cache.Store for the given workspace, applying
// the global cache flags. Returns nil when caching is disabled.
func buildCacheStore(ws tokens.Workspace) *cache.Store {
	if flagNoCache || flagCacheTTL == 0 {
		return nil
	}
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not determine cache directory: %v\n", err)
		return nil
	}
	store := cache.NewStore(filepath.Join(userCacheDir, "slackseek"), flagCacheTTL)
	if flagRefreshCache {
		// --refresh-cache triggers a cold start for entity caches (users, channels,
		// groups) by removing the workspace directory. Subsequent listUsersCached,
		// listChannelsCached, and listUserGroupsCached calls will hit the API.
		_ = store.Clear(cache.WorkspaceKey(ws.URL))
	}
	return store
}

// resolveWidth returns the effective text wrap width, applying precedence:
// SLACKSEEK_WIDTH env > --width flag > terminal width (or 120 for pipes).
// A returned value of 0 means wrapping is disabled.
func resolveWidth() int {
	if env := os.Getenv("SLACKSEEK_WIDTH"); env != "" {
		var w int
		if _, err := fmt.Sscanf(env, "%d", &w); err == nil {
			return w
		}
	}
	if flagWidth != 0 {
		return flagWidth
	}
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 120
	}
	return w
}

// resolveEmojiEnabled returns true when emoji rendering should be active.
// Precedence: --no-emoji > --emoji > isatty(stdout).
func resolveEmojiEnabled() bool {
	if flagNoEmoji {
		return false
	}
	if flagEmojiEnabled {
		return true
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// validateFormat returns an error if f is not one of the accepted format values.
func validateFormat(f string) error {
	for _, valid := range output.ValidFormats {
		if output.Format(f) == valid {
			return nil
		}
	}
	return fmt.Errorf("invalid --format %q: must be one of text, table, json, markdown", f)
}

// rootCmd is the singleton used by main.go.
var rootCmd = buildRootCmd()

// NewRootCmd returns a fresh root command for use in tests.
// Each call returns an independent instance so tests do not share state.
func NewRootCmd() *cobra.Command {
	return buildRootCmd()
}

// Execute runs the root command with the given version string.
func Execute(version string) error {
	rootCmd.Version = version
	return rootCmd.Execute()
}

// SelectWorkspace picks a workspace from the extracted list based on the
// --workspace selector string.  When selector is empty the first workspace
// is returned and a notice is printed to stderr.  Returns an error when
// no workspaces are available or the selector matches nothing.
func SelectWorkspace(workspaces []tokens.Workspace, selector string) (tokens.Workspace, error) {
	if len(workspaces) == 0 {
		return tokens.Workspace{}, fmt.Errorf("no Slack workspaces found: run `slackseek auth show` to diagnose credential extraction")
	}

	if selector == "" {
		ws := workspaces[0]
		fmt.Fprintf(os.Stderr, "Using workspace: %s (%s)\n", ws.Name, ws.URL)
		if len(workspaces) > 1 {
			names := make([]string, len(workspaces)-1)
			for i, w := range workspaces[1:] {
				names[i] = w.Name
			}
			fmt.Fprintf(os.Stderr, "Other workspaces available: %s\n", strings.Join(names, ", "))
		}
		return ws, nil
	}

	lower := strings.ToLower(selector)
	for _, ws := range workspaces {
		if strings.ToLower(ws.Name) == lower || ws.URL == selector {
			return ws, nil
		}
	}

	names := make([]string, len(workspaces))
	for i, w := range workspaces {
		names[i] = w.Name
	}
	return tokens.Workspace{}, fmt.Errorf(
		"workspace %q not found: available workspaces are: %s",
		selector, strings.Join(names, ", "),
	)
}
