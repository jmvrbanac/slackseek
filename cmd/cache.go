package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jmvrbanac/slackseek/internal/cache"
	"github.com/jmvrbanac/slackseek/internal/tokens"
	"github.com/spf13/cobra"
)

// cacheRunFunc is the injectable cache-clear pipeline for testing.
type cacheRunFunc func(ctx context.Context, w io.Writer, workspace tokens.Workspace, all bool) error

// addCacheCmd attaches the cache command to parent using the given injectable
// dependencies. This signature enables test injection.
func addCacheCmd(
	parent *cobra.Command,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn cacheRunFunc,
) {
	parent.AddCommand(newCacheCmd(extractFn, runFn))
}

func newCacheCmd(
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn cacheRunFunc,
) *cobra.Command {
	cacheCmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage the local cache",
	}
	cacheCmd.AddCommand(newCacheClearCmd(extractFn, runFn))
	return cacheCmd
}

func runCacheClearE(
	cmd *cobra.Command,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn cacheRunFunc,
	all bool,
) error {
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
	return runFn(cmd.Context(), cmd.OutOrStdout(), ws, all)
}

func newCacheClearCmd(
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn cacheRunFunc,
) *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Remove cached channel and user lists",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCacheClearE(cmd, extractFn, runFn, all)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "remove cached data for all workspaces")
	return cmd
}

// countFilesIn returns the number of non-directory entries in dir.
// Returns 0 without error when dir does not exist.
func countFilesIn(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			n++
		}
	}
	return n, nil
}

// countAllFilesIn recursively counts non-directory entries under dir.
// Returns 0 without error when dir does not exist.
func countAllFilesIn(dir string) (int, error) {
	n := 0
	err := filepath.WalkDir(dir, func(_ string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if !d.IsDir() {
			n++
		}
		return nil
	})
	return n, err
}

// defaultRunCacheClear is the production implementation of cacheRunFunc.
func defaultRunCacheClear(_ context.Context, w io.Writer, ws tokens.Workspace, all bool) error {
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("could not determine cache directory: %w", err)
	}
	dir := filepath.Join(userCacheDir, "slackseek")
	store := cache.NewStore(dir, 0)

	if all {
		n, err := countAllFilesIn(dir)
		if err != nil {
			return fmt.Errorf("cache clear --all: %w", err)
		}
		if clearErr := store.ClearAll(); clearErr != nil {
			return fmt.Errorf("cache clear --all: %w", clearErr)
		}
		fmt.Fprintf(w, "Cache cleared for all workspaces (%d files removed).\n", n)
		return nil
	}

	key := cache.WorkspaceKey(ws.URL)
	wsDir := filepath.Join(dir, key)
	n, err := countFilesIn(wsDir)
	if err != nil {
		return fmt.Errorf("cache clear: %w", err)
	}
	if n == 0 {
		fmt.Fprintf(w, "No cache found for workspace %s.\n", ws.Name)
		return nil
	}
	if clearErr := store.Clear(key); clearErr != nil {
		return fmt.Errorf("cache clear: %w", clearErr)
	}
	fmt.Fprintf(w, "Cache cleared for workspace %s (%d files removed).\n", ws.Name, n)
	return nil
}

func init() {
	addCacheCmd(rootCmd, tokens.DefaultExtract, defaultRunCacheClear)
}
