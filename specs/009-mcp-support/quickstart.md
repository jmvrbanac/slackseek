# Quickstart: 009-mcp-support (MCP Server)

## Prerequisites

- `slackseek` binary built and on PATH
- Slack desktop app installed and at least one workspace logged in
- An MCP client (Claude Code recommended)

## Build

```sh
go build -o slackseek ./...
```

## Run the MCP Server

```sh
# Start the server (stdio transport — connect via MCP client)
slackseek mcp serve
```

The server reads JSON-RPC messages from stdin and writes responses to stdout. It is not interactive; connect it via an MCP client.

## Configure Claude Code

Add to `~/.claude/claude_desktop_config.json` (or equivalent MCP config):

```json
{
  "mcpServers": {
    "slackseek": {
      "command": "slackseek",
      "args": ["mcp", "serve"]
    }
  }
}
```

Restart Claude Code. The `slackseek` MCP server tools will appear in the tool list.

## Available Tools

| Tool | What it does |
|------|-------------|
| `slack_search` | Full-text search across workspace |
| `slack_history` | Channel message history |
| `slack_messages` | Messages from a specific user |
| `slack_thread` | All messages in a thread (by permalink) |
| `slack_channels` | List channels |
| `slack_users` | List users |
| `slack_digest` | Per-channel digest for a user |
| `slack_postmortem` | Incident postmortem from channel history |
| `slack_metrics` | Message activity metrics for a channel |
| `slack_actions` | Extract action items from channel history |

## Example Tool Calls (from Claude Code)

Search for messages about a deployment incident in the last week:
```
Use slack_search with query="deploy failed" since="7d"
```

Get the last 50 messages in #engineering:
```
Use slack_history with channel="engineering" limit=50
```

List all channels matching "incident":
```
Use slack_channels with filter="incident"
```

Get a thread by permalink:
```
Use slack_thread with url="https://acme.slack.com/archives/C01234567/p1700000000123456"
```

## Workspace Selection

All tools accept an optional `workspace` parameter. When omitted, the first discovered workspace is used. To target a specific workspace:

```
Use slack_search with query="foo" workspace="Acme Corp"
```

Pass either the workspace name (e.g., `"Acme Corp"`) or the base URL (e.g., `"https://acme.slack.com"`).

## Date Range Formats

`since` and `until` accept:
- ISO date: `2026-03-01`
- RFC 3339: `2026-03-01T09:00:00Z`
- Relative duration (for `since` only): `30m`, `4h`, `7d`, `2w`

## Troubleshooting

**"slack credentials unavailable"**: The Slack desktop app is not running, or you are not logged in. Start Slack, log in, then retry.

**"channel not found"**: Use `slack_channels` to list available channels and check the exact name or ID.

**"user not found"**: Use `slack_users` to list available users.

**Rate limit errors**: The server retries automatically. If errors persist, wait a minute and retry.
