package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/jmvrbanac/slackseek/internal/output"
	"github.com/jmvrbanac/slackseek/internal/tokens"
	"github.com/spf13/cobra"
)

// addAuthCmd attaches the auth command tree to parent using extractFn for
// credential extraction.  This signature enables test injection.
func addAuthCmd(parent *cobra.Command, extractFn func() (tokens.TokenExtractionResult, error)) {
	auth := &cobra.Command{
		Use:   "auth",
		Short: "Manage and verify local Slack workspace credentials",
	}

	auth.AddCommand(newAuthShowCmd(extractFn))
	auth.AddCommand(newAuthExportCmd(extractFn))
	parent.AddCommand(auth)
}

func newAuthShowCmd(extractFn func() (tokens.TokenExtractionResult, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display discovered workspace credentials (tokens and cookies are truncated)",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := extractFn()
			if err != nil {
				return fmt.Errorf(
					"failed to extract Slack credentials: %w\n"+
						"Ensure the Slack desktop application is installed and you are logged in.\n"+
						"Run `slackseek auth show` to verify the extraction path.",
					err,
				)
			}
			for _, w := range result.Warnings {
				fmt.Fprintln(os.Stderr, "Warning:", w)
			}
			format := output.Format(flagFormat)
			return output.PrintWorkspaces(cmd.OutOrStdout(), format, result.Workspaces)
		},
	}
}

func newAuthExportCmd(extractFn func() (tokens.TokenExtractionResult, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "export",
		Short: "Print shell export statements for all discovered workspace tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
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
			}
			return nil
		},
	}
}

// toEnvName converts a workspace name to a valid uppercase environment variable
// suffix, e.g. "Acme Corp" → "ACME_CORP".
var nonAlphanumRE = regexp.MustCompile(`[^A-Z0-9]+`)

func toEnvName(name string) string {
	upper := strings.ToUpper(name)
	return nonAlphanumRE.ReplaceAllString(upper, "_")
}

func init() {
	addAuthCmd(rootCmd, tokens.DefaultExtract)
}
