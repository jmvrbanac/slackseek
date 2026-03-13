# slackseek Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-02

## Active Technologies
- Go 1.24 (unchanged from feature 001) + stdlib only — `crypto/sha256`, `encoding/json`, `os.UserCacheDir` (002-cache-channels-users)
- File-based JSON cache at `os.UserCacheDir()/slackseek/{workspaceKey}/` (002-cache-channels-users)
- Go 1.24 (unchanged) + stdlib only for Resolver; existing `internal/slack`, `internal/cache`, `internal/output` packages (003-resolve-ids-in-output)
- Uses existing `internal/cache` file-backed store (no new storage) (003-resolve-ids-in-output)
- Go 1.24 + `github.com/olekukonko/tablewriter v1.1.3` (existing), `regexp` stdlib (existing) (004-bug-fixes)
- N/A — no new storage; existing file-backed cache unchanged (004-bug-fixes)
- Go 1.24 + `golang.org/x/term` (new); `github.com/olekukonko/tablewriter v1.1.3` (existing); stdlib `regexp`, `sync`, `embed`, `unicode/utf8` (005-day2-improvements)
- N/A — no new persistent storage; existing file-backed cache unchanged (005-day2-improvements)
- Go 1.24 + `github.com/olekukonko/tablewriter v1.1.3` (existing); stdlib only for new code (006-channel-cache)
- File-based JSON under `os.UserCacheDir()/slackseek/` (existing `internal/cache` store) (006-channel-cache)
- File-based JSON under `os.UserCacheDir()/slackseek/` (existing `internal/cache` store, unchanged) (007-multi-day-history-cache)
- Go 1.24 + `github.com/slack-go/slack` (existing), `github.com/cenkalti/backoff/v4` (existing), `github.com/spf13/cobra` (existing) (008-lazy-entity-cache)
- File-based JSON cache under `os.UserCacheDir()/slackseek/` (existing `internal/cache` store, unchanged format) (008-lazy-entity-cache)
- Go 1.24 (unchanged) + `github.com/mark3labs/mcp-go` (new), `github.com/spf13/cobra` (existing), `github.com/jmvrbanac/slackseek/internal/slack` (existing), `internal/tokens` (existing), `internal/cache` (existing), `internal/output` (existing) (009-mcp-support)
- N/A — no new persistent storage; token cache is in-memory only (009-mcp-support)

- Go 1.24 (`go 1.24` in `go.mod`; floor set by modernc.org/sqlite) (001-slackseek-cli)

## Project Structure

```text
cmd/             # Cobra subcommands (root, auth, channels, history, messages, search, users)
internal/tokens/ # Local credential extraction (LevelDB + SQLite + keyring)
internal/slack/  # Slack API client, pagination, entity resolution
internal/output/ # text / table / JSON formatters
main.go
go.mod           # go 1.24; module github.com/jmvrbanac/slackseek
.golangci.yml    # Linter config
specs/           # Feature specs, plans, contracts (speckit workflow)
```

Platform-specific files use `_linux.go` / `_darwin.go` naming with `//go:build` constraints.

## Commands

```sh
go build -o slackseek ./...           # Build binary (no CGO required)
go test -race ./...                   # Unit tests with race detector (mandatory)
INTEGRATION=1 go test -race ./...     # Include integration tests (requires Slack running)
golangci-lint run                     # Linting (mandatory before merge)
GOOS=linux  go build ./...            # Cross-platform build check
GOOS=darwin go build ./...            # Cross-platform build check
```

## Code Style

Go 1.24: idiomatic Go only. Functions ≤ 40 lines. Descriptive names. Errors wrapped with
`fmt.Errorf("context: %w", err)` at every package boundary. No panics in production paths.
See `.specify/memory/constitution.md` for full coding principles.

## Recent Changes
- 009-mcp-support: Added Go 1.24 (unchanged) + `github.com/mark3labs/mcp-go` (new), `github.com/spf13/cobra` (existing), `github.com/jmvrbanac/slackseek/internal/slack` (existing), `internal/tokens` (existing), `internal/cache` (existing), `internal/output` (existing)
- 008-lazy-entity-cache: Added Go 1.24 + `github.com/slack-go/slack` (existing), `github.com/cenkalti/backoff/v4` (existing), `github.com/spf13/cobra` (existing)
- 007-multi-day-history-cache: Added Go 1.24 + `github.com/olekukonko/tablewriter v1.1.3` (existing); stdlib only for new code


<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
