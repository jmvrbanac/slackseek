# Implementation Plan: Channel and User List Caching

**Branch**: `002-cache-channels-users` | **Date**: 2026-03-03 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/002-cache-channels-users/spec.md`

## Summary

Add a per-workspace, TTL-based, file-backed cache for `conversations.list` and
`users.list` API responses. The cache lives in `os.UserCacheDir()/slackseek/` and is
keyed by a short hex digest of the workspace URL. All commands that resolve channel
or user names benefit transparently. Three new root-level flags
(`--cache-ttl`, `--refresh-cache`, `--no-cache`) and a `slackseek cache clear`
subcommand give users full control. No new third-party dependencies are required.

## Technical Context

**Language/Version**: Go 1.24 (unchanged from feature 001)
**Primary Dependencies**: stdlib only — `crypto/sha256`, `encoding/json`, `os.UserCacheDir`
**Storage**: File-based JSON cache at `os.UserCacheDir()/slackseek/{workspaceKey}/`
**Testing**: `go test -race ./...`; `os.Chtimes` to simulate stale entries in unit tests; `t.TempDir()` for isolation; `INTEGRATION=1` guard for network tests
**Target Platform**: Linux and macOS (unchanged)
**Project Type**: CLI binary (unchanged)
**Performance Goals**: Cached resolution < 1 s; no list API calls after first run within TTL
**Constraints**: No new third-party modules; `--no-cache`/`--cache-ttl 0` must restore pre-feature behaviour exactly; `go test -race` must pass; cross-compile on both GOOS targets

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Evidence |
|---|---|---|
| I. Clarity Over Cleverness | ✅ Pass | `internal/cache` has a single, obvious API (`Load`/`Save`/`Clear`). Cache-check logic in `ListChannels`/`ListUsers` is ≤ 15 lines each. No clever tricks. |
| II. Test-First (NON-NEGOTIABLE) | ✅ Pass | `store_test.go` covers: hit, miss, stale, corrupt, unwritable dir. Client cache integration tested via injectable `listFn`. Tests authored before implementation. Race detector mandatory. |
| III. Single-Responsibility Packages | ✅ Pass | `internal/cache` stores bytes — knows nothing of Slack types. `internal/slack` marshals/unmarshals — knows nothing of file paths. `cmd/` constructs and injects the store. No circular imports. |
| IV. Actionable Error Handling | ✅ Pass | Cache failures degrade gracefully (warn to stderr, fall through to API). Every error site carries context: what failed, why, what to do. |
| V. Platform Isolation via Build Tags | ✅ Pass | `os.UserCacheDir()` handles platform differences natively — no build-tagged files needed for this feature. |

**Post-Phase-1 re-check**: All principles still pass. The `os.UserCacheDir` choice
eliminates any need for platform-specific code. Injectable `*cache.Store` keeps
`internal/slack` testable in isolation.

**Complexity Tracking**: No violations — no entry required.

## Project Structure

### Documentation (this feature)

```text
specs/002-cache-channels-users/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 — decisions and rationale
├── data-model.md        # Phase 1 — entities, state transitions, file layout
├── quickstart.md        # Phase 1 — developer and user quick reference
├── contracts/
│   └── cli-schema.md    # Phase 1 — new flags and cache clear command contract
└── tasks.md             # Phase 2 output (/speckit.tasks — not created here)
```

### Source Code (repository root)

```text
slackseek/
├── cmd/
│   ├── root.go          # Add --cache-ttl, --refresh-cache, --no-cache flags
│   ├── cache.go         # NEW: `cache clear` command + defaultRunCacheClear
│   ├── cache_test.go    # NEW: unit tests for cache clear command
│   ├── channels.go      # Pass *cache.Store through defaultRunChannels
│   └── users.go         # Pass *cache.Store through defaultRunUsers
│
└── internal/
    ├── cache/
    │   ├── doc.go        # NEW: package documentation
    │   ├── store.go      # NEW: CacheStore{dir, ttl} + Load/Save/Clear/ClearAll
    │   └── store_test.go # NEW: unit tests (temp dir, Chtimes for staleness)
    │
    └── slack/
        ├── client.go     # Add optional *cache.Store field; NewClientWithCache
        ├── channels.go   # ListChannels: cache check/write when store non-nil
        └── users.go      # ListUsers: cache check/write when store non-nil
```

**Structure Decision**: Single project layout unchanged. New `internal/cache` sub-package
follows the existing `internal/{tokens,slack,output}` pattern.
