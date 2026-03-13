# MCP Server Support

Add a `mcp serve` subcommand to the existing `slackseek` binary that exposes Slack functionality
as a local MCP server over stdio. This allows Claude Code (and other MCP clients) to query Slack
without requiring official Slack API tokens or OAuth apps — credentials are extracted from the
local Slack desktop application the same way the existing CLI commands work.

## Motivation

Company policy may prevent direct connection to the Slack MCP server (which requires a bot token
from a registered Slack App). `slackseek` sidesteps this by reading credentials from the locally
installed Slack desktop app. Wrapping `slackseek` internals as an MCP server gives Claude
structured, first-class Slack tool access with no additional credential setup.

## Usage

```sh
# Start the MCP server (stdio transport)
slackseek mcp serve

# Claude Code MCP config (~/.claude/claude_desktop_config.json)
{
  "mcpServers": {
    "slackseek": {
      "command": "slackseek",
      "args": ["mcp", "serve"]
    }
  }
}
```

## Architecture

The MCP server lives entirely within the existing binary as a new Cobra subcommand. It reuses
`internal/slack`, `internal/cache`, and `internal/tokens` directly — no subprocess overhead,
no shelling out to `slackseek` CLI.

```
cmd/mcp.go          ← new Cobra subcommand: "mcp serve"
internal/mcp/       ← MCP server logic, tool handlers, token cache
  server.go         ← server setup, tool registration
  tools.go          ← one handler function per tool
  tokencache.go     ← token refresh logic (see below)
```

**New dependency:** `github.com/mark3labs/mcp-go` — Go MCP SDK (stdio transport).

## Token Refresh Strategy

The MCP server is a long-running process; Slack rotates tokens periodically. The strategy is
**Option B: in-memory cache + 401 retry**, with a short TTL as a proactive safety net.

```go
// internal/mcp/tokencache.go
type tokenCache struct {
    mu         sync.Mutex
    workspaces []tokens.Workspace
    fetchedAt  time.Time
}

const tokenTTL = 5 * time.Minute
```

Refresh logic per tool call:

1. Acquire lock; check if `fetchedAt` is older than TTL → if so, re-extract.
2. Release lock; proceed with cached workspace credentials.
3. If Slack API returns HTTP 401 → re-extract immediately, retry the call once.
4. If retry also returns 401 → return error to MCP client.

This means at most one redundant API call per token rotation event. The TTL ensures stale
credentials are not held indefinitely even if no 401 is observed (e.g., a Slack endpoint that
degrades silently rather than returning 401).

## Exposed Tools

All tools accept an optional `workspace` parameter (workspace name or base URL) that maps to
the existing `--workspace` flag. When omitted, the first discovered workspace is used.

| Tool | Parameters | Maps to |
|------|-----------|---------|
| `slack_search` | `query`, `channels[]`, `user`, `since`, `until`, `limit` | `slackseek search` |
| `slack_history` | `channel`, `since`, `until`, `limit`, `threads` | `slackseek history` |
| `slack_messages` | `user`, `since`, `until`, `limit` | `slackseek messages` |
| `slack_thread` | `url` | `slackseek thread` |
| `slack_channels` | `filter`, `include_archived` | `slackseek channels list` |
| `slack_users` | `filter` | `slackseek users list` |
| `slack_digest` | `user`, `since`, `until` | `slackseek digest` |
| `slack_postmortem` | `channel`, `since`, `until` | `slackseek postmortem` |
| `slack_metrics` | `channel`, `since`, `until` | `slackseek metrics` |
| `slack_actions` | `channel`, `since`, `until` | `slackseek actions` |

All tools return JSON output (equivalent to `--format json`). This is more reliable for Claude
to reason about than formatted text or tables.

## Date Range Handling

Tools accept `since`/`until` parameters that delegate to the existing
`slack.ParseRelativeDateRange` / `slack.ParseDateRange` functions. Supported formats:

- ISO date: `2026-03-01`
- RFC 3339: `2026-03-01T09:00:00Z`
- Relative duration: `30m`, `4h`, `7d`, `2w` (for `since` only)

## Error Handling

Tool handlers return structured MCP errors:

- **Auth failure** (after retry): `"slack credentials unavailable: <reason> — ensure the Slack desktop app is running and you are logged in"`
- **Channel/user not found**: `"channel <name> not found — use slack_channels to list available channels"`
- **Rate limited**: `"slack rate limit exceeded — retry after <N>s"`
- All other errors: wrapped with context, e.g. `"search <query> failed: <upstream error>"`

## Code Style Constraints

Per project guidelines:
- Functions ≤ 40 lines
- Errors wrapped with `fmt.Errorf("context: %w", err)` at every package boundary
- No panics in production paths
- Stdlib only for new code beyond the `mcp-go` dependency
- Platform-specific files use `_linux.go` / `_darwin.go` naming with `//go:build` constraints (not needed here — token extraction is already abstracted)

## Implementation Order

1. Add `github.com/mark3labs/mcp-go` to `go.mod`
2. `internal/mcp/tokencache.go` — `tokenCache` struct with `get()` and `refresh()` methods
3. `internal/mcp/tools.go` — one handler per tool, each calling existing `internal/slack` functions
4. `internal/mcp/server.go` — server init, tool registration, stdio serve loop
5. `cmd/mcp.go` — Cobra subcommand wiring `mcp serve` → `mcp.Serve()`
6. Tests: `internal/mcp/tokencache_test.go`, `internal/mcp/tools_test.go` (inject mock slack client and mock `extractFn`)

## Out of Scope

- SSE / HTTP transport (stdio is sufficient for Claude Code local use)
- MCP resources or prompts (tools only)
- `cache clear` tool (users can run `slackseek cache clear` directly)
- Authentication management tools (users can run `slackseek auth show` directly)
