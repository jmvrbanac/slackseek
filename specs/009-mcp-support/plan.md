# Implementation Plan: 009-mcp-support

**Branch**: `009-mcp-support` | **Date**: 2026-03-13 | **Spec**: `specs/009-mcp-support/spec.md`
**Input**: Feature specification from `mcp-support.md` (copied to spec at plan time)

## Summary

Add a `slackseek mcp serve` subcommand that exposes all existing Slack CLI operations as an MCP server over stdio transport. Claude Code (and any other MCP client) can then query Slack without a registered Slack App or bot token — credentials come from the locally installed Slack desktop app exactly as the existing CLI commands use them.

The implementation adds one new external dependency (`github.com/mark3labs/mcp-go`), one new internal package (`internal/mcp`), and one new Cobra subcommand (`cmd/mcp.go`). No changes to existing packages are required.

## Technical Context

**Language/Version**: Go 1.24 (unchanged)
**Primary Dependencies**: `github.com/mark3labs/mcp-go` (new), `github.com/spf13/cobra` (existing), `github.com/jmvrbanac/slackseek/internal/slack` (existing), `internal/tokens` (existing), `internal/cache` (existing), `internal/output` (existing)
**Storage**: N/A — no new persistent storage; token cache is in-memory only
**Testing**: `go test -race ./...` — unit tests with mock `slackClient` interface and injectable `extractFn`
**Target Platform**: Linux + macOS (existing platform isolation; no new platform-specific code)
**Project Type**: CLI + MCP server (stdio transport)
**Performance Goals**: Tool response latency dominated by Slack API; token cache prevents re-extraction overhead
**Constraints**: Functions ≤ 40 lines; no panics; errors wrapped at every package boundary; no stderr output on stdio transport
**Scale/Scope**: Single user, local tool

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design.*

| Principle | Assessment | Notes |
|-----------|-----------|-------|
| I. Clarity Over Cleverness | ✅ PASS | New code follows existing patterns; handler functions are ≤ 40 lines by design; `tokenCache` logic is simple mutex + TTL |
| II. Test-First (NON-NEGOTIABLE) | ✅ PASS | `tokencache_test.go` and `tools_test.go` are explicitly planned; mock `slackClient` interface enables hermetic unit tests; all exported functions covered |
| III. Single-Responsibility Packages | ✅ PASS | `internal/mcp` has exactly one purpose: expose Slack ops as MCP tools. `cmd/mcp.go` only wires the Cobra subcommand. No circular imports |
| IV. Actionable Error Handling | ✅ PASS | All error messages include what failed, why, and what the user can do. Defined in `contracts/mcp-tools.md` |
| V. Platform Isolation via Build Tags | ✅ PASS | No new platform-specific code; token extraction is already platform-abstracted in `internal/tokens` |

**Post-design re-check**: All principles continue to hold. No new violations introduced in Phase 1 design.

## Project Structure

### Documentation (this feature)

```text
specs/009-mcp-support/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── mcp-tools.md     # MCP tool schemas and error contracts
└── tasks.md             # Phase 2 output (/speckit.tasks — NOT created here)
```

### Source Code (repository root)

```text
cmd/
└── mcp.go               # NEW — Cobra subcommand: "mcp serve" → mcp.Serve()

internal/mcp/
├── doc.go               # NEW — package comment (Principle III)
├── tokencache.go        # NEW — tokenCache struct, get(), refresh()
├── tokencache_test.go   # NEW — unit tests for TTL and 401 retry logic
├── tools.go             # NEW — one handler per MCP tool + slackClient interface
├── tools_test.go        # NEW — unit tests with mock slackClient
└── server.go            # NEW — NewMCPServer(), tool registration, Serve()

go.mod                   # MODIFIED — add github.com/mark3labs/mcp-go
go.sum                   # MODIFIED — updated by go mod tidy
```

**Structure Decision**: Single-project layout (Option 1). New code is isolated in `internal/mcp/` and `cmd/mcp.go` — no existing files modified beyond `go.mod`.

## Complexity Tracking

> No constitution violations — this table is empty.

## Phase 0 Research Findings

See `research.md` for full decisions. Key resolutions:

1. **mcp-go API** — confirmed: `server.NewMCPServer` + `s.AddTool` + `server.ServeStdio`. Handler signature: `func(ctx, mcp.CallToolRequest) (*mcp.CallToolResult, error)`.
2. **Token refresh** — Option B (in-memory cache + 401 retry, 5-minute TTL). See `tokenCache`.
3. **Output format** — direct `json.Marshal` of result structs (not `output.PrintXxx`); avoids output formatting side-effects on stdio transport.
4. **Workspace selection** — `cmd.SelectWorkspace` cannot be reused (writes to stderr). MCP tools do their own workspace selection silently.
5. **Resolver** — inline `buildMCPResolver` in `tools.go`, mirrors `cmd.buildResolver` without global flags or stderr writes.
6. **Testing** — narrow `slackClient` interface enables mock injection; all handlers and the token cache are unit-testable without a live Slack instance.

## Phase 1 Design Decisions

See `data-model.md` and `contracts/mcp-tools.md` for full details. Key decisions:

### `internal/mcp/tokencache.go`
- `tokenCache` struct: `sync.Mutex`, `[]tokens.Workspace`, `time.Time`, injectable `extractFn`
- `get()`: TTL check → re-extract if stale → return workspaces
- `refresh()`: always re-extract, update cache

### `internal/mcp/tools.go`
- `slackClient` interface: 12 methods matching `*slack.Client`
- `buildMCPClient(ws tokens.Workspace) slackClient`: creates `*slack.Client` with 24h TTL cache store
- `buildMCPResolver(ctx, ws, c) *slack.Resolver`: inline resolver construction (no stderr, no global flags)
- `parseDateRange(since, until string) (slack.DateRange, error)`: delegates to `slack.ParseRelativeDateRange` if either param contains a duration suffix, else `slack.ParseDateRange`
- `selectWorkspace(workspaces []tokens.Workspace, selector string) (tokens.Workspace, error)`: silent workspace selection (no stderr)
- One handler per tool (10 total), each ≤ 40 lines

### `internal/mcp/server.go`
- `Serve(extractFn func() (tokens.TokenExtractionResult, error)) error`
- Creates `tokenCache`, registers all 10 tools, calls `server.ServeStdio`
- Version passed via `buildinfo` or a package-level constant

### `cmd/mcp.go`
- Cobra command `mcp` with subcommand `serve`
- `serve` calls `mcp.Serve(tokens.DefaultExtract)` and propagates any error
- Registered via `init()` into `rootCmd`

### Error Handling Strategy
- 401 from Slack API → call `tokenCache.refresh()` → retry once → if still 401, return auth error
- Channel/user not found → return structured error message with remediation hint
- All errors wrapped with `fmt.Errorf("context: %w", err)` before returning to MCP client
- `CallToolResult.IsError = true` for all error returns

### Testing Strategy
- `tokencache_test.go`: test TTL expiry (cache hit within TTL, miss after TTL), refresh on 401, concurrent access (race detector)
- `tools_test.go`: test each handler with a mock `slackClient`; verify JSON output shape, error propagation, workspace selection, date range parsing
- Build constraint: no `INTEGRATION=1` guard needed (mock-based, no OS resources)

## Implementation Order

1. `go get github.com/mark3labs/mcp-go` → update `go.mod` + `go.sum`
2. `internal/mcp/doc.go` — package comment
3. `internal/mcp/tokencache.go` — `tokenCache` struct + `get()` + `refresh()`
4. `internal/mcp/tokencache_test.go` — unit tests (write first per Principle II)
5. `internal/mcp/tools.go` — `slackClient` interface + helper functions + 10 tool handlers
6. `internal/mcp/tools_test.go` — unit tests for all handlers
7. `internal/mcp/server.go` — `Serve()` function, tool registration
8. `cmd/mcp.go` — Cobra subcommand wiring
9. `go test -race ./...` — verify all tests pass
10. `golangci-lint run` — verify linting passes
11. `GOOS=linux go build ./...` + `GOOS=darwin go build ./...` — cross-platform check
