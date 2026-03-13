package mcp

import (
	"github.com/jmvrbanac/slackseek/internal/tokens"
	"github.com/mark3labs/mcp-go/server"
)

// version is the server version string reported during MCP handshake.
// It is intentionally kept as a simple constant; the build tag approach used
// by the CLI binary is not needed here because the MCP server version does
// not surface in the CLI help text.
const version = "0.1.0"

// Serve initialises the MCP server, registers all Slack tools, and starts the
// stdio transport loop. It blocks until the client disconnects or a signal is
// received. Returns any error from the transport layer.
func Serve(extractFn func() (tokens.TokenExtractionResult, error)) error {
	tc := &tokenCache{extractFn: extractFn}

	s := server.NewMCPServer(
		"slackseek",
		version,
		server.WithToolCapabilities(true),
	)

	registerTools(s, tc)

	return server.ServeStdio(s)
}

// registerTools adds all Slack MCP tools to the server. It is called once
// during Serve and is separated for testability.
func registerTools(s *server.MCPServer, tc *tokenCache) {
	// Tools are registered in phases as they are implemented.
	// Phase 4 (US2): slack_search, slack_history, slack_messages, slack_thread
	// Phase 5 (US3): slack_channels, slack_users
	// Phase 6 (US4): slack_digest, slack_postmortem, slack_metrics, slack_actions
	_ = tc // suppress unused warning until tool handlers are wired
}
