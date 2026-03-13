package mcp

import (
	"context"

	"github.com/jmvrbanac/slackseek/internal/tokens"
	mcplib "github.com/mark3labs/mcp-go/mcp"
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
	registerSearchTool(s, tc)
	registerHistoryTool(s, tc)
	registerMessagesTool(s, tc)
	registerThreadTool(s, tc)
	registerChannelsTool(s, tc)
	registerUsersTool(s, tc)
	registerDigestTool(s, tc)
	registerPostmortemTool(s, tc)
	registerMetricsTool(s, tc)
	registerActionsTool(s, tc)
}

func registerSearchTool(s *server.MCPServer, tc *tokenCache) {
	tool := mcplib.NewTool("slack_search",
		mcplib.WithDescription("Search Slack messages across the workspace using a query string, optionally filtered by channel(s), user, and date range."),
		mcplib.WithString("query", mcplib.Required(), mcplib.Description("Search query")),
		mcplib.WithArray("channels", mcplib.Description("Limit to channels (names or IDs)")),
		mcplib.WithString("user", mcplib.Description("Limit to user (display name or ID)")),
		mcplib.WithString("since", mcplib.Description("Start: ISO date, RFC 3339, or duration (e.g. 7d)")),
		mcplib.WithString("until", mcplib.Description("End: ISO date, RFC 3339, or duration")),
		mcplib.WithNumber("limit", mcplib.DefaultNumber(100), mcplib.Description("Max results (0 = unlimited)")),
		mcplib.WithString("workspace", mcplib.Description("Workspace name or URL")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		return handleSlackSearch(ctx, req, tc, buildMCPClient)
	})
}

func registerHistoryTool(s *server.MCPServer, tc *tokenCache) {
	tool := mcplib.NewTool("slack_history",
		mcplib.WithDescription("Retrieve the message history of a channel within a date range."),
		mcplib.WithString("channel", mcplib.Required(), mcplib.Description("Channel name or ID")),
		mcplib.WithString("since", mcplib.Description("Start of range")),
		mcplib.WithString("until", mcplib.Description("End of range")),
		mcplib.WithNumber("limit", mcplib.DefaultNumber(100), mcplib.Description("Max messages")),
		mcplib.WithBoolean("threads", mcplib.DefaultBool(false), mcplib.Description("Include thread replies")),
		mcplib.WithString("workspace", mcplib.Description("Workspace name or URL")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		return handleSlackHistory(ctx, req, tc, buildMCPClient)
	})
}

func registerMessagesTool(s *server.MCPServer, tc *tokenCache) {
	tool := mcplib.NewTool("slack_messages",
		mcplib.WithDescription("Retrieve messages sent by a specific user across all channels."),
		mcplib.WithString("user", mcplib.Required(), mcplib.Description("User display name or ID")),
		mcplib.WithString("since", mcplib.Description("Start of range")),
		mcplib.WithString("until", mcplib.Description("End of range")),
		mcplib.WithNumber("limit", mcplib.DefaultNumber(100), mcplib.Description("Max messages")),
		mcplib.WithString("workspace", mcplib.Description("Workspace name or URL")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		return handleSlackMessages(ctx, req, tc, buildMCPClient)
	})
}

func registerThreadTool(s *server.MCPServer, tc *tokenCache) {
	tool := mcplib.NewTool("slack_thread",
		mcplib.WithDescription("Retrieve all messages in a thread by permalink URL."),
		mcplib.WithString("url", mcplib.Required(), mcplib.Description("Slack permalink URL")),
		mcplib.WithString("workspace", mcplib.Description("Workspace name or URL")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		return handleSlackThread(ctx, req, tc, buildMCPClient)
	})
}

func registerChannelsTool(s *server.MCPServer, tc *tokenCache) {
	tool := mcplib.NewTool("slack_channels",
		mcplib.WithDescription("List channels in the workspace, optionally filtered by name."),
		mcplib.WithString("filter", mcplib.Description("Substring filter on channel name")),
		mcplib.WithBoolean("include_archived", mcplib.DefaultBool(false), mcplib.Description("Include archived channels")),
		mcplib.WithString("workspace", mcplib.Description("Workspace name or URL")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		return handleSlackChannels(ctx, req, tc, buildMCPClient)
	})
}

func registerUsersTool(s *server.MCPServer, tc *tokenCache) {
	tool := mcplib.NewTool("slack_users",
		mcplib.WithDescription("List workspace members, optionally filtered by name or email."),
		mcplib.WithString("filter", mcplib.Description("Substring filter on display name, real name, or email")),
		mcplib.WithString("workspace", mcplib.Description("Workspace name or URL")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		return handleSlackUsers(ctx, req, tc, buildMCPClient)
	})
}

func registerDigestTool(s *server.MCPServer, tc *tokenCache) {
	tool := mcplib.NewTool("slack_digest",
		mcplib.WithDescription("Show a per-channel message digest for a user over a date range."),
		mcplib.WithString("user", mcplib.Required(), mcplib.Description("User display name or ID")),
		mcplib.WithString("since", mcplib.Description("Start of range")),
		mcplib.WithString("until", mcplib.Description("End of range")),
		mcplib.WithString("workspace", mcplib.Description("Workspace name or URL")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		return handleSlackDigest(ctx, req, tc, buildMCPClient)
	})
}

func registerPostmortemTool(s *server.MCPServer, tc *tokenCache) {
	tool := mcplib.NewTool("slack_postmortem",
		mcplib.WithDescription("Generate an incident postmortem document from a channel's message history."),
		mcplib.WithString("channel", mcplib.Required(), mcplib.Description("Channel name or ID")),
		mcplib.WithString("since", mcplib.Description("Start of range")),
		mcplib.WithString("until", mcplib.Description("End of range")),
		mcplib.WithString("workspace", mcplib.Description("Workspace name or URL")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		return handleSlackPostmortem(ctx, req, tc, buildMCPClient)
	})
}

func registerMetricsTool(s *server.MCPServer, tc *tokenCache) {
	tool := mcplib.NewTool("slack_metrics",
		mcplib.WithDescription("Compute message activity metrics for a channel over a date range."),
		mcplib.WithString("channel", mcplib.Required(), mcplib.Description("Channel name or ID")),
		mcplib.WithString("since", mcplib.Description("Start of range")),
		mcplib.WithString("until", mcplib.Description("End of range")),
		mcplib.WithString("workspace", mcplib.Description("Workspace name or URL")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		return handleSlackMetrics(ctx, req, tc, buildMCPClient)
	})
}

func registerActionsTool(s *server.MCPServer, tc *tokenCache) {
	tool := mcplib.NewTool("slack_actions",
		mcplib.WithDescription("Extract action items (TODO/follow-up) from a channel's message history."),
		mcplib.WithString("channel", mcplib.Required(), mcplib.Description("Channel name or ID")),
		mcplib.WithString("since", mcplib.Description("Start of range")),
		mcplib.WithString("until", mcplib.Description("End of range")),
		mcplib.WithString("workspace", mcplib.Description("Workspace name or URL")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		return handleSlackActions(ctx, req, tc, buildMCPClient)
	})
}
