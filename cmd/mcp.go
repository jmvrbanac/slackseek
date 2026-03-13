package cmd

import (
	"github.com/jmvrbanac/slackseek/internal/mcp"
	"github.com/jmvrbanac/slackseek/internal/tokens"
	"github.com/spf13/cobra"
)

// addMCPCmd attaches the "mcp" command tree to parent using the given serve
// function. The serveFn is injectable so tests can replace it without starting
// a real stdio MCP server.
func addMCPCmd(parent *cobra.Command, serveFn func() error) {
	parent.AddCommand(newMCPCmd(serveFn))
}

// newMCPCmd builds the "mcp" parent command with a "serve" child.
func newMCPCmd(serveFn func() error) *cobra.Command {
	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server commands",
		Long: `Commands for running slackseek as a Model Context Protocol (MCP) server.

The MCP server exposes Slack functionality as tools that can be used by any
MCP client (e.g. Claude Code) without requiring a registered Slack App or
bot token — credentials are read from the local Slack desktop application.`,
	}
	mcpCmd.AddCommand(newServeCmd(serveFn))
	return mcpCmd
}

// newServeCmd builds the "serve" subcommand that starts the stdio MCP server.
func newServeCmd(serveFn func() error) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server over stdio",
		Long: `Start slackseek as an MCP server using stdio transport.

Connect an MCP client (e.g. Claude Code) to this server to query Slack without
a registered Slack App. Add to ~/.claude/claude_desktop_config.json:

  {
    "mcpServers": {
      "slackseek": {
        "command": "slackseek",
        "args": ["mcp", "serve"]
      }
    }
  }`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return serveFn()
		},
	}
}

func init() {
	addMCPCmd(rootCmd, func() error {
		return mcp.Serve(tokens.DefaultExtract)
	})
}
