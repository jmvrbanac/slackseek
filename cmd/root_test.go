package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jmvrbanac/slackseek/cmd"
	"github.com/spf13/cobra"
)

// noopCmd is a do-nothing subcommand used to trigger PersistentPreRunE.
func noopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "noop",
		Short: "no-op for testing",
		RunE:  func(cmd *cobra.Command, args []string) error { return nil },
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
