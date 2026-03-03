// Package cmd contains the cobra CLI commands for slackseek.
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/jmvrbanac/slackseek/internal/output"
	"github.com/jmvrbanac/slackseek/internal/slack"
	"github.com/jmvrbanac/slackseek/internal/tokens"
	"github.com/spf13/cobra"
)

// Global flag values populated by cobra before PersistentPreRunE runs.
var (
	flagWorkspace string
	flagFormat    string
	flagFrom      string
	flagTo        string
)

// ParsedDateRange is set by PersistentPreRunE and available to all subcommands.
var ParsedDateRange slack.DateRange

// newRootCmdWithFlags builds a fresh root command.  Callers that want
// the canonical singleton should use Execute(); tests call NewRootCmd().
func buildRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "slackseek",
		Short:        "Query Slack workspaces using locally extracted credentials",
		SilenceUsage: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if err := validateFormat(flagFormat); err != nil {
				return err
			}
			dr, err := slack.ParseDateRange(flagFrom, flagTo)
			if err != nil {
				return err
			}
			ParsedDateRange = dr
			return nil
		},
	}

	root.PersistentFlags().StringVarP(&flagWorkspace, "workspace", "w", "", "workspace name or base URL to target")
	root.PersistentFlags().StringVar(&flagFormat, "format", "text", "output format: text | table | json")
	root.PersistentFlags().StringVar(&flagFrom, "from", "", "start of date range: YYYY-MM-DD or RFC 3339")
	root.PersistentFlags().StringVar(&flagTo, "to", "", "end of date range: YYYY-MM-DD or RFC 3339")

	return root
}

// validateFormat returns an error if f is not one of the accepted format values.
func validateFormat(f string) error {
	for _, valid := range output.ValidFormats {
		if output.Format(f) == valid {
			return nil
		}
	}
	return fmt.Errorf("invalid --format %q: must be one of text, table, json", f)
}

// rootCmd is the singleton used by main.go.
var rootCmd = buildRootCmd()

// NewRootCmd returns a fresh root command for use in tests.
// Each call returns an independent instance so tests do not share state.
func NewRootCmd() *cobra.Command {
	return buildRootCmd()
}

// Execute runs the root command.
func Execute() error {
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
