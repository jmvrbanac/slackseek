# Quickstart: Channel and User List Caching

**Feature**: `002-cache-channels-users`
**Date**: 2026-03-03

---

## What This Feature Does

Running `slackseek history`, `slackseek messages`, or `slackseek search` with a channel
or user name forces a full paginated fetch of every channel and user in the workspace on
every single invocation. For large workspaces this can take tens of seconds and quickly
burns through Slack API rate limits.

This feature adds a simple file-based cache. The first time you run any name-resolving
command the lists are fetched and written to `~/.cache/slackseek/<workspace-key>/`.
All subsequent invocations within the TTL (default 24 hours) skip the API call
entirely and read from disk. The cache is completely transparent — no changes to
existing flags or output.

---

## Quick Usage

```bash
# First run — hits Slack API, saves cache
slackseek history general

# Second run — reads from disk, zero list API calls
slackseek history general

# Force a fresh fetch (e.g., after adding a new channel)
slackseek history general --refresh-cache

# Skip the cache for one invocation without writing to it
slackseek history general --no-cache

# Shorten the TTL to 1 hour for this invocation
slackseek history general --cache-ttl 1h

# Disable caching completely for this invocation
slackseek history general --cache-ttl 0

# Wipe the cache for the current workspace
slackseek cache clear

# Wipe all workspace caches
slackseek cache clear --all
```

---

## Where the Cache Lives

| Platform | Path |
|----------|------|
| Linux    | `$XDG_CACHE_HOME/slackseek/` (usually `~/.cache/slackseek/`) |
| macOS    | `~/Library/Caches/slackseek/` |

Each workspace gets its own subdirectory named after the first 16 hex characters of
`SHA-256(workspaceURL)`. Two files are stored per workspace:

```
~/.cache/slackseek/
└── a3f1b2c4d5e6f708/
    ├── channels.json   ← full channel list as JSON
    └── users.json      ← full user list as JSON
```

You can inspect, copy, or delete these files freely. `slackseek` will regenerate them
on the next run.

---

## Developer Setup

### Building and Testing

```bash
# Build the binary (no CGO, no changes to build flags)
go build -o slackseek ./...

# Run all unit tests (race detector required)
go test -race ./...

# Run integration tests (requires local Slack installation)
INTEGRATION=1 go test -race ./...

# Lint
golangci-lint run

# Cross-platform build check
GOOS=linux  go build ./...
GOOS=darwin go build ./...
```

### Testing Cache Behaviour Without Slack

The cache unit tests in `internal/cache/store_test.go` use a temporary directory and
manipulate file modification times via `os.Chtimes` to simulate stale entries. No
network connection or Slack credentials are required.

```bash
# Run only cache tests
go test -race ./internal/cache/...
```

### Inspecting the Cache in Integration Tests

When `INTEGRATION=1` is set, the slack package tests exercise the full
`ListChannels`/`ListUsers` → cache write → cache read path against a real Slack
workspace. The test helper creates a temp cache directory and cleans it up after each
run so the developer's real cache is never affected.

---

## Implementation Notes for Reviewers

- `internal/cache` is a new package with a single exported type (`Store`) and four
  methods (`Load`, `Save`, `Clear`, `ClearAll`). It depends only on the standard
  library.
- `internal/slack.Client` gains an optional `*cache.Store` field. When nil, behaviour
  is identical to today. Existing tests pass a nil cache; no test fixtures need changes.
- Three new persistent flags (`--cache-ttl`, `--refresh-cache`, `--no-cache`) are added
  to the root command and wired through `PersistentPreRunE`.
- `cmd/cache.go` adds the `cache clear` subcommand following the same injectable-runFn
  pattern used by all other commands.
- The cache key is `fmt.Sprintf("%x", sha256.Sum256([]byte(workspaceURL)))[:16]`.
  `crypto/sha256` is in the standard library; no new module entries in `go.mod`.
