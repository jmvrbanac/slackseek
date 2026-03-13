# Research: 009-mcp-support

## Decision: MCP Go SDK — `github.com/mark3labs/mcp-go`

**Decision**: Use `github.com/mark3labs/mcp-go` as the sole new external dependency.

**Rationale**: The spec explicitly calls this out. It provides stdio transport, tool registration with typed parameters, and a clean `CallToolRequest`/`CallToolResult` handler contract. No other SDK is needed — Go stdlib covers the rest (sync, context, encoding/json, fmt).

**Alternatives considered**: Writing a raw JSON-RPC server from scratch (too much protocol code), or using the official Anthropic MCP SDK (Go SDK not available).

---

## Decision: Token Refresh Strategy — In-Memory Cache + 401 Retry (Option B)

**Decision**: `tokenCache` struct with a 5-minute TTL and a 401-triggered immediate re-extract.

**Rationale**: The MCP server is long-lived; Slack rotates tokens. TTL guards against silent degradation; 401 retry handles the common rotation case with minimal overhead.

**Alternatives considered**: Option A (re-extract on every call) — unnecessary network overhead for stable sessions. Option C (no re-extract, hard fail on 401) — bad UX for long-running sessions.

---

## Decision: Tool Output Format — JSON via `encoding/json`

**Decision**: All tool handlers marshal their results to JSON using `encoding/json` and return a single `mcp.TextContent` with `Type: "text"` carrying the JSON string.

**Rationale**: MCP clients (Claude Code) can parse JSON tool output. The existing `internal/output` package supports JSON format via `output.Format("json")` and `io.Writer`, but using it here would require capturing stdout into a buffer. Direct `json.Marshal` of the result structs is simpler and more reliable for programmatic consumption.

**Alternatives considered**: Using `output.PrintXxx` with a `bytes.Buffer` writer — valid but couples MCP to output formatting details (column widths, emoji, etc.) that are irrelevant for tool consumption.

---

## Decision: Workspace Resolution in MCP Tools

**Decision**: Optional `workspace` parameter passed to each tool. When empty, use first workspace from extracted credentials (same as CLI default). Use `SelectWorkspace`-equivalent logic inline (without printing to stderr — instead return error as MCP tool error).

**Rationale**: The MCP server must not print to stderr (stdio is the MCP transport). Workspace selection logic in `cmd.SelectWorkspace` prints to stderr which would corrupt the JSON-RPC stream.

**Alternatives considered**: Reusing `cmd.SelectWorkspace` — not viable due to stderr pollution on the stdio transport.

---

## Decision: `internal/mcp` Package — New Sub-Package

**Decision**: New `internal/mcp/` package containing `server.go`, `tools.go`, `tokencache.go`. New `cmd/mcp.go` for the Cobra subcommand.

**Rationale**: Follows the existing project pattern (each feature in its own internal package). The MCP package has a single purpose: expose Slack operations as MCP tools.

**Alternatives considered**: Adding MCP logic directly to `cmd/` — violates Single-Responsibility (Principle III); `cmd/` should only wire CLI flags to internal operations.

---

## Decision: `doc.go` for `internal/mcp`

**Decision**: Add `internal/mcp/doc.go` with package-level comment documenting the package purpose.

**Rationale**: Principle III mandates a `doc.go` or package comment for every `internal/` package.

---

## Decision: Test Approach — Mock `extractFn` and `slackClient` interface

**Decision**: Define a `slackClient` interface in `internal/mcp` with the methods used by tools. Pass a mock implementation in tests. `extractFn` is injectable (same pattern as `cmd/` tests).

**Rationale**: Principle II mandates tests before merge. The `*slack.Client` concrete type is not mockable without an interface; defining a narrow interface enables fast, hermetic unit tests.

**Alternatives considered**: Integration tests only — too slow; require a running Slack instance. Using `httptest` to stub Slack API — more complex than a simple interface mock.

---

## Decision: Cobra Subcommand Structure — `mcp` parent + `serve` child

**Decision**: `slackseek mcp serve` mirrors the existing `slackseek channels list` / `slackseek users list` pattern.

**Rationale**: Keeps the CLI hierarchy consistent. Leaves room for future `mcp` subcommands (e.g., `mcp status`) without a breaking flag change.

**Alternatives considered**: Flat `slackseek mcp-serve` command — inconsistent with existing CLI patterns.

---

## Decision: Output Type for Digest, Actions, Metrics, Postmortem

**Decision**: Return the raw Go struct slices from each tool handler, then marshal to JSON directly.

- `slack_digest`: returns `[]output.ChannelDigest` (produced by `slack.GetUserMessages` + digest grouping via `cmd.defaultRunDigest`-equivalent logic)
- `slack_actions`: returns `[]output.ActionItem`
- `slack_metrics`: returns `output.ChannelMetrics`
- `slack_postmortem`: returns `output.IncidentDoc`

**Rationale**: The `cmd/` layer calls `defaultRunXxx` functions that return these structs. MCP handlers can call the same functions and marshal directly, avoiding any output formatting dependency.

---

## Resolution: `cmd.buildResolver` for MCP

**Decision**: The MCP `tools.go` builds its own resolver using `slack.NewResolverWithFetch` directly (same logic as `cmd/resolver.go:buildResolver` but without the global flag dependencies and without stderr output).

**Rationale**: `cmd.buildResolver` reads global flags (`flagNoCache`, `flagCacheTTL`) and writes to `os.Stderr`. The MCP server must avoid both. A new private `buildMCPResolver` function in `internal/mcp/tools.go` replicates the logic with a fixed 24h TTL.

---

## mcp-go API Patterns (confirmed from source)

```go
// Server creation
s := server.NewMCPServer("slackseek", version, server.WithToolCapabilities(true))

// Tool registration
tool := mcp.NewTool("slack_search",
    mcp.WithDescription("..."),
    mcp.WithString("query", mcp.Required(), mcp.Description("...")),
    mcp.WithString("workspace", mcp.Description("...")),
)
s.AddTool(tool, handlerFunc)

// Handler signature
func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)

// Parameter extraction
query := mcp.ParseString(req, "query", "")

// Result construction
return &mcp.CallToolResult{
    Content: []mcp.Content{mcp.TextContent{Type: "text", Text: jsonStr}},
}, nil

// Error result
return &mcp.CallToolResult{
    Content: []mcp.Content{mcp.TextContent{Type: "text", Text: msg}},
    IsError: true,
}, nil

// Stdio transport
server.ServeStdio(s)
```
