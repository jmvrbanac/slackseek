package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmvrbanac/slackseek/internal/cache"
	"github.com/jmvrbanac/slackseek/internal/tokens"
)

// runCacheCmd builds a fresh root command with injectable cache dependencies,
// executes the provided args, and returns stdout, stderr, error.
func runCacheCmd(
	t *testing.T,
	extractFn func() (tokens.TokenExtractionResult, error),
	runFn cacheRunFunc,
	args ...string,
) (stdout, stderr string, err error) {
	t.Helper()
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	root := NewRootCmd()
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	addCacheCmd(root, extractFn, runFn)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

var defaultCacheExtractFn = func() (tokens.TokenExtractionResult, error) {
	return tokens.TokenExtractionResult{
		Workspaces: []tokens.Workspace{
			{Name: "TestWS", URL: "https://test.slack.com", Token: "xoxs-test"},
		},
	}, nil
}

// makeClearRunFn returns a cacheRunFunc that uses dir as the base cache directory
// instead of os.UserCacheDir(), mirroring the defaultRunCacheClear logic.
func makeClearRunFn(dir string) cacheRunFunc {
	return func(_ context.Context, w io.Writer, ws tokens.Workspace, all bool) error {
		store := cache.NewStore(dir, time.Hour)
		if all {
			n, err := countAllFilesIn(dir)
			if err != nil {
				return err
			}
			if err := store.ClearAll(); err != nil {
				return err
			}
			fmt.Fprintf(w, "Cache cleared for all workspaces (%d files removed).\n", n)
			return nil
		}
		key := cache.WorkspaceKey(ws.URL)
		wsDir := filepath.Join(dir, key)
		n, err := countFilesIn(wsDir)
		if err != nil {
			return err
		}
		if n == 0 {
			fmt.Fprintf(w, "No cache found for workspace %s.\n", ws.Name)
			return nil
		}
		if err := store.Clear(key); err != nil {
			return err
		}
		fmt.Fprintf(w, "Cache cleared for workspace %s (%d files removed).\n", ws.Name, n)
		return nil
	}
}

func TestCacheCmd_ClearWorkspace_RemovesFilesAndPrintsMessage(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, time.Hour)
	key := cache.WorkspaceKey("https://test.slack.com")
	if err := store.Save(key, "channels", []byte(`[]`)); err != nil {
		t.Fatalf("Save channels: %v", err)
	}
	if err := store.Save(key, "users", []byte(`[]`)); err != nil {
		t.Fatalf("Save users: %v", err)
	}

	stdout, _, err := runCacheCmd(t, defaultCacheExtractFn, makeClearRunFn(dir), "cache", "clear")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, key, "channels.json")); !os.IsNotExist(statErr) {
		t.Error("channels.json should be removed after cache clear")
	}
	if _, statErr := os.Stat(filepath.Join(dir, key, "users.json")); !os.IsNotExist(statErr) {
		t.Error("users.json should be removed after cache clear")
	}
	if !strings.Contains(stdout, "Cache cleared for workspace TestWS") {
		t.Errorf("expected 'Cache cleared for workspace TestWS' in output, got: %q", stdout)
	}
	if !strings.Contains(stdout, "2 files removed") {
		t.Errorf("expected '2 files removed' in output, got: %q", stdout)
	}
}

func TestCacheCmd_ClearWorkspace_NoCacheDir_PrintsNoFoundMessage(t *testing.T) {
	dir := t.TempDir() // empty — no cache files written
	stdout, _, err := runCacheCmd(t, defaultCacheExtractFn, makeClearRunFn(dir), "cache", "clear")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "No cache found for workspace TestWS") {
		t.Errorf("expected 'No cache found for workspace TestWS' in output, got: %q", stdout)
	}
}

func TestCacheCmd_AllFlag_ClearsAllAndReportsTotalCount(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, time.Hour)
	key1 := cache.WorkspaceKey("https://ws1.slack.com")
	key2 := cache.WorkspaceKey("https://ws2.slack.com")
	_ = store.Save(key1, "channels", []byte(`[]`))
	_ = store.Save(key1, "users", []byte(`[]`))
	_ = store.Save(key2, "channels", []byte(`[]`))

	stdout, _, err := runCacheCmd(t, defaultCacheExtractFn, makeClearRunFn(dir), "cache", "clear", "--all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
		t.Error("base cache dir should be removed after --all clear")
	}
	if !strings.Contains(stdout, "Cache cleared for all workspaces") {
		t.Errorf("expected 'Cache cleared for all workspaces' in output, got: %q", stdout)
	}
	if !strings.Contains(stdout, "3 files removed") {
		t.Errorf("expected '3 files removed' in output, got: %q", stdout)
	}
}

func TestCacheCmd_IOError_ReturnsNonZero(t *testing.T) {
	runFn := func(_ context.Context, _ io.Writer, _ tokens.Workspace, _ bool) error {
		return errors.New("simulated I/O error")
	}
	_, _, err := runCacheCmd(t, defaultCacheExtractFn, runFn, "cache", "clear")
	if err == nil {
		t.Fatal("expected non-zero exit on I/O error, got nil")
	}
}
