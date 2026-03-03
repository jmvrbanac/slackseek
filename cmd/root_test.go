package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jmvrbanac/slackseek/cmd"
	"github.com/spf13/cobra"
)

// --- Cache flag validation tests (T016) ---

func TestCacheTTLNegativeReturnsError(t *testing.T) {
	_, _, err := runRoot("--cache-ttl=-1h", "noop")
	if err == nil {
		t.Fatal("expected error for negative --cache-ttl, got nil")
	}
	if !strings.Contains(err.Error(), "must not be negative") {
		t.Errorf("expected 'must not be negative' in error, got: %v", err)
	}
}

func TestCacheTTLZeroIsAccepted(t *testing.T) {
	_, _, err := runRoot("--cache-ttl=0", "noop")
	if err != nil {
		t.Errorf("expected no error for --cache-ttl 0, got: %v", err)
	}
}

func TestRefreshCacheAndNoCacheMutuallyExclusive(t *testing.T) {
	_, _, err := runRoot("--refresh-cache", "--no-cache", "noop")
	if err == nil {
		t.Fatal("expected error for --refresh-cache + --no-cache, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' in error, got: %v", err)
	}
}

func TestNoCacheAloneIsAccepted(t *testing.T) {
	_, _, err := runRoot("--no-cache", "noop")
	if err != nil {
		t.Errorf("expected no error for --no-cache alone, got: %v", err)
	}
}

func TestRefreshCacheAloneIsAccepted(t *testing.T) {
	_, _, err := runRoot("--refresh-cache", "noop")
	if err != nil {
		t.Errorf("expected no error for --refresh-cache alone, got: %v", err)
	}
}

// noopCmd is a do-nothing subcommand used to trigger PersistentPreRunE.
func noopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "noop",
		Short: "no-op for testing",
		RunE:  func(_ *cobra.Command, _ []string) error { return nil },
	}
}

// runRoot creates a fresh root command with a noop subcommand and executes it.
func runRoot(args ...string) (stdout, stderr string, err error) {
	root := cmd.NewRootCmd()
	root.AddCommand(noopCmd())

	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

func TestFormatFlagInvalid(t *testing.T) {
	_, _, err := runRoot("--format", "xyz", "noop")
	if err == nil {
		t.Error("expected error for invalid --format value, got nil")
	}
}

func TestFormatFlagValidJSON(t *testing.T) {
	_, _, err := runRoot("--format", "json", "noop")
	if err != nil {
		t.Errorf("expected no error for --format json, got: %v", err)
	}
}

func TestFormatFlagValidText(t *testing.T) {
	_, _, err := runRoot("--format", "text", "noop")
	if err != nil {
		t.Errorf("expected no error for --format text, got: %v", err)
	}
}

func TestFormatFlagValidTable(t *testing.T) {
	_, _, err := runRoot("--format", "table", "noop")
	if err != nil {
		t.Errorf("expected no error for --format table, got: %v", err)
	}
}

func TestFormatFlagInvalidStderrMessage(t *testing.T) {
	root := cmd.NewRootCmd()
	root.AddCommand(noopCmd())

	errBuf := new(bytes.Buffer)
	root.SetErr(errBuf)
	root.SetArgs([]string{"--format", "invalid", "noop"})
	root.Execute() //nolint:errcheck

	errOutput := errBuf.String()
	if errOutput == "" {
		t.Error("expected error message on stderr for invalid --format, got empty")
	}
}

func TestDateRangeFromAfterToExitsWithError(t *testing.T) {
	_, _, err := runRoot("--from", "2025-02-01", "--to", "2025-01-01", "noop")
	if err == nil {
		t.Error("expected error when --from > --to, got nil")
	}
}

func TestDateRangeFromAfterToNoAPICall(t *testing.T) {
	// Verify the error comes from validation (stderr contains date-range mention)
	root := cmd.NewRootCmd()
	root.AddCommand(noopCmd())

	errBuf := new(bytes.Buffer)
	root.SetErr(errBuf)
	root.SetArgs([]string{"--from", "2025-02-01", "--to", "2025-01-01", "noop"})
	root.Execute() //nolint:errcheck

	if !strings.Contains(errBuf.String(), "from") && !strings.Contains(errBuf.String(), "before") {
		t.Errorf("expected date-range error message, got: %q", errBuf.String())
	}
}
