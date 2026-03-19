package cmd

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"runtime"
	"strings"

	"github.com/jmvrbanac/slackseek/internal/output"
	"github.com/jmvrbanac/slackseek/internal/tokens"
	"github.com/spf13/cobra"
)

// addAuthCmd attaches the auth command tree to parent.
// extractFn is used for normal extraction; diagFn for verbose/diagnose modes.
func addAuthCmd(
	parent *cobra.Command,
	extractFn func() (tokens.TokenExtractionResult, error),
	diagFn func(io.Writer) (tokens.TokenExtractionResult, error),
) {
	auth := &cobra.Command{
		Use:   "auth",
		Short: "Manage and verify local Slack workspace credentials",
	}

	auth.AddCommand(newAuthShowCmd(extractFn, diagFn))
	auth.AddCommand(newAuthExportCmd(extractFn))
	auth.AddCommand(newAuthDiagnoseCmd(diagFn))
	auth.AddCommand(newAuthDebugCookieCmd())
	parent.AddCommand(auth)
}

func newAuthShowCmd(
	extractFn func() (tokens.TokenExtractionResult, error),
	diagFn func(io.Writer) (tokens.TokenExtractionResult, error),
) *cobra.Command {
	var verbose bool
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Display discovered workspace credentials",
		RunE: func(cmd *cobra.Command, _ []string) error {
			var result tokens.TokenExtractionResult
			var err error
			if verbose {
				result, err = diagFn(cmd.ErrOrStderr())
			} else {
				result, err = extractFn()
			}
			if err != nil {
				return fmt.Errorf(
					"failed to extract Slack credentials: %w\n"+
						"Ensure the Slack desktop application is installed and you are logged in.\n"+
						"Run `slackseek auth show` to verify the extraction path.",
					err,
				)
			}
			for _, w := range result.Warnings {
				fmt.Fprintln(cmd.ErrOrStderr(), "Warning:", w)
			}
			format := output.Format(flagFormat)
			return output.PrintWorkspaces(cmd.OutOrStdout(), format, result.Workspaces)
		},
	}
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Print per-step diagnostic output to stderr")
	return cmd
}

func newAuthExportCmd(extractFn func() (tokens.TokenExtractionResult, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "export",
		Short: "Print shell export statements for all discovered workspace tokens",
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := extractFn()
			if err != nil {
				return fmt.Errorf(
					"failed to extract Slack credentials: %w\n"+
						"Ensure the Slack desktop application is installed and you are logged in.",
					err,
				)
			}
			for _, ws := range result.Workspaces {
				envName := toEnvName(ws.Name)
				fmt.Fprintf(cmd.OutOrStdout(), "export SLACK_TOKEN_%s=%s\n", envName, ws.Token)
				fmt.Fprintf(cmd.OutOrStdout(), "export SLACK_COOKIE_%s=%s\n", envName, ws.Cookie)
			}
			return nil
		},
	}
}

func newAuthDiagnoseCmd(diagFn func(io.Writer) (tokens.TokenExtractionResult, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "diagnose",
		Short: "Print a diagnostic report for credential extraction",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDiagnose(cmd.OutOrStdout(), diagFn)
		},
	}
}

func runDiagnose(out io.Writer, diagFn func(io.Writer) (tokens.TokenExtractionResult, error)) error {
	var diagBuf bytes.Buffer
	result, extractErr := diagFn(&diagBuf)

	paths := tokens.DefaultPaths()
	fmt.Fprintf(out, "Platform:     %s\n", runtime.GOOS)
	fmt.Fprintf(out, "LevelDB path: %s\n", paths.LevelDBPath())
	fmt.Fprintf(out, "Cookie path:  %s\n", paths.CookiePath())
	fmt.Fprintln(out)

	if extractErr != nil {
		fmt.Fprintf(out, "Workspaces: 0\n")
		fmt.Fprintf(out, "Error: %v\n", extractErr)
		if diagBuf.Len() > 0 {
			fmt.Fprintf(out, "\nDiagnostic trace:\n%s", diagBuf.String())
		}
		return extractErr
	}

	fmt.Fprintf(out, "Workspaces: %d\n", len(result.Workspaces))
	for _, ws := range result.Workspaces {
		fmt.Fprintf(out, "\n  %s\n", ws.Name)
		fmt.Fprintf(out, "    URL:    %s\n", ws.URL)
		fmt.Fprintf(out, "    Token:  %v\n", ws.Token != "")
		fmt.Fprintf(out, "    Cookie: %v\n", ws.Cookie != "")
	}
	if len(result.Warnings) > 0 {
		fmt.Fprintf(out, "\nWarnings:\n")
		for _, w := range result.Warnings {
			fmt.Fprintf(out, "  %s\n", w)
		}
	}
	if diagBuf.Len() > 0 {
		fmt.Fprintf(out, "\nDiagnostic trace:\n%s", diagBuf.String())
	}
	return nil
}

// toEnvName converts a workspace name to a valid uppercase environment variable
// suffix, e.g. "Acme Corp" → "ACME_CORP".
var nonAlphanumRE = regexp.MustCompile(`[^A-Z0-9]+`)

func toEnvName(name string) string {
	upper := strings.ToUpper(name)
	return nonAlphanumRE.ReplaceAllString(upper, "_")
}

func init() {
	addAuthCmd(rootCmd, tokens.DefaultExtract, tokens.DefaultExtractDiag)
}
