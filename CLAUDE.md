# slackseek Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-02

## Active Technologies
- Go 1.24 (unchanged from feature 001) + stdlib only — `crypto/sha256`, `encoding/json`, `os.UserCacheDir` (002-cache-channels-users)
- File-based JSON cache at `os.UserCacheDir()/slackseek/{workspaceKey}/` (002-cache-channels-users)

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
- 002-cache-channels-users: Added Go 1.24 (unchanged from feature 001) + stdlib only — `crypto/sha256`, `encoding/json`, `os.UserCacheDir`

- 001-slackseek-cli: Added Go 1.24 (`go 1.24` in `go.mod`; floor set by modernc.org/sqlite)

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
