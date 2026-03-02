// Package cmd contains the cobra CLI commands for slackseek.
package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "slackseek",
	Short: "Query Slack workspaces using locally extracted credentials",
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
