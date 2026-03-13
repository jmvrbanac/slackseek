// Package mcp exposes Slack operations as MCP (Model Context Protocol) tools
// over a stdio transport. It provides a long-running MCP server that reads
// Slack credentials from the locally installed Slack desktop application and
// makes them available to any MCP client (e.g., Claude Code) without requiring
// a registered Slack App or bot token.
//
// The server is started via the [Serve] function and reuses [internal/slack],
// [internal/cache], and [internal/tokens] packages directly — no subprocess
// overhead or shelling out to the slackseek CLI.
package mcp
