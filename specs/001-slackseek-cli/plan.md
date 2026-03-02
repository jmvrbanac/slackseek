# Implementation Plan: slackseek CLI

**Branch**: `001-slackseek-cli` | **Date**: 2026-03-02 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/001-slackseek-cli/spec.md`

## Summary

Build `slackseek` — a stateless Go CLI tool that extracts authentication tokens
from a locally installed Slack desktop application and uses them to query the
Slack API for message search, channel history, user message aggregation, and
workspace resource discovery. Supports Linux and macOS; outputs text, table, or
JSON. All commands re-extract credentials on each invocation (no credential
caching). Platform-specific code (keyring access, file paths) is isolated behind
Go build tags.

## Technical Context

**Language/Version**: Go 1.24 (`go 1.24` in `go.mod`; floor set by modernc.org/sqlite)
**Primary Dependencies**:
- `github.com/spf13/cobra` — CLI framework
- `github.com/syndtr/goleveldb` — Read Slack's LevelDB Local Storage (read-only)
- `modernc.org/sqlite` — Pure-Go SQLite (no CGO); reads Slack's Chromium cookie DB
- `github.com/godbus/dbus/v5` — Linux D-Bus SecretService keyring access
- `github.com/keybase/go-keychain` — macOS Keychain access
- `github.com/slack-go/slack` — Slack API client (pin stable tagged release ≤ Go 1.24 requirement)
- `github.com/olekukonko/tablewriter` — Aligned ASCII table output
- `github.com/cenkalti/backoff/v4` — Retry loop with Retry-After support

**Storage**: None (stateless). Reads Slack's LevelDB and SQLite stores via temp copies.
**Testing**: `go test -race ./...`; interface mocking for platform-specific code;
`t.Skip()` + `INTEGRATION=1` env guard for real-OS integration tests.
**Target Platform**: Linux (amd64, arm64) + macOS (amd64, arm64). No Windows.
**Project Type**: CLI binary.
**Performance Goals**: Auth verification < 5 s; search (100 results) < 30 s on broadband.
**Constraints**: No CGO; single statically-linked binary; no credential persistence;
`go test -race` must pass; cross-compiles cleanly between Linux and macOS.
**Scale/Scope**: Single-user CLI; no concurrent users; single binary with no runtime deps.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Evidence |
|---|---|---|
| I. Clarity Over Cleverness | ✅ Pass | Each package has a single clear purpose; packages are small by design (< 5 files each). Function-length targets enforced by `golangci-lint`. |
| II. Test-First (NON-NEGOTIABLE) | ✅ Pass | All exported functions must have tests before merge. Integration tests gated with `INTEGRATION=1` env var and `t.Skip()`. Race detector mandatory. Test tasks MUST be written before implementation tasks in `tasks.md`. |
| III. Single-Responsibility Packages | ✅ Pass | `internal/tokens` (extraction only), `internal/slack` (API only), `internal/output` (formatting only). `cmd/` never imported by `internal/`. |
| IV. Actionable Error Handling | ✅ Pass | FR-014 mandates three-part error messages. Each error site must include: what failed, why, what to do. `fmt.Errorf("…: %w", err)` at all package boundaries. |
| V. Platform Isolation via Build Tags | ✅ Pass | `keyring_linux.go`, `keyring_darwin.go`, `paths_linux.go`, `paths_darwin.go` with `//go:build` constraints. Cross-platform core compiles without platform tags. |

**Post-Phase-1 re-check**: All principles still pass. The `modernc.org/sqlite`
choice removes CGO (simplifies Principle V). Interface injection design enforces
Principle III boundaries.

**Complexity Tracking**: No violations — no entry required.

## Project Structure

### Documentation (this feature)

```text
specs/001-slackseek-cli/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   ├── cli-schema.md    # Phase 1 output — command/flag contract
│   └── json-schema.md   # Phase 1 output — JSON output schema
└── tasks.md             # Phase 2 output (/speckit.tasks — not created here)
```

### Source Code (repository root)

```text
slackseek/
├── cmd/
│   ├── root.go          # Root cobra command; global flags; workspace selection
│   ├── auth.go          # auth show / auth export
│   ├── channels.go      # channels list
│   ├── history.go       # history <channel>
│   ├── messages.go      # messages <user>
│   ├── search.go        # search <query>
│   └── users.go         # users list
├── internal/
│   ├── tokens/
│   │   ├── doc.go                  # Package documentation
│   │   ├── extractor.go            # TokenExtractionResult, Workspace types + top-level Extract()
│   │   ├── leveldb.go              # LevelDB workspace token extraction (cross-platform)
│   │   ├── cookie.go               # SQLite cookie decryption (cross-platform)
│   │   ├── keyring.go              # KeyringReader interface definition
│   │   ├── keyring_linux.go        # //go:build linux — D-Bus SecretService impl
│   │   ├── keyring_darwin.go       # //go:build darwin — macOS Keychain impl
│   │   ├── paths.go                # CookiePathProvider interface definition
│   │   ├── paths_linux.go          # //go:build linux — ~/.config/Slack/…
│   │   └── paths_darwin.go         # //go:build darwin — ~/Library/Application Support/Slack/…
│   ├── slack/
│   │   ├── doc.go                  # Package documentation
│   │   ├── client.go               # Authenticated client wrapper; rate-limit retry loop
│   │   ├── channels.go             # Channel listing; channel-name → ID resolution
│   │   ├── history.go              # conversations.history pagination + thread merging
│   │   ├── messages.go             # Per-user search.messages aggregation
│   │   ├── search.go               # Full-text search; query composition
│   │   └── users.go                # users.list pagination; user-name → ID resolution
│   └── output/
│       ├── doc.go                  # Package documentation
│       └── format.go               # text / table / JSON formatters for all entity types
├── main.go
├── go.mod                          # go 1.24; module github.com/jmvrbanac/slackseek
├── go.sum
└── .golangci.yml                   # Linter configuration
```

**Structure Decision**: Single project layout (no monorepo, no workspace files).
All platform-specific files use the `_os.go` naming convention, which Go's build
system recognises automatically (supplemented by explicit `//go:build` constraints
for clarity and tooling compatibility).

## Phase 0: Research — Completed

See [research.md](./research.md) for full findings. Key decisions:

| Topic | Decision | Rationale |
|---|---|---|
| LevelDB | `syndtr/goleveldb` | Only production-grade pure-Go reader; read-only use mitigates maintenance risk |
| SQLite | `modernc.org/sqlite` | No CGO; enables single static binary; sets Go 1.24 floor |
| Go version | 1.24 | Driven by modernc.org/sqlite |
| Rate limiting | Honor `Retry-After` for 429; exponential backoff for 5xx | Slack API sends exact wait time on 429 |
| Testing strategy | Interface injection + `t.Skip(INTEGRATION)` + build tags | Idiomatic Go; fast hermetic unit tests |
| slack-go | Pin stable tagged release | Master branch temporarily requires Go 1.25 (pre-release) |

## Phase 1: Design — Completed

Artifacts produced:

| Artifact | Path |
|---|---|
| Data model | [data-model.md](./data-model.md) |
| CLI contract | [contracts/cli-schema.md](./contracts/cli-schema.md) |
| JSON contract | [contracts/json-schema.md](./contracts/json-schema.md) |
| Quickstart | [quickstart.md](./quickstart.md) |

### Key design decisions

**Interface boundaries for testability** (Principle II + III):

```
KeyringReader interface {
    ReadPassword(service, account string) ([]byte, error)
}
CookiePathProvider interface {
    LevelDBPath() string
    CookiePath() string
}
```

Platform implementations (`keyring_linux.go`, `keyring_darwin.go`,
`paths_linux.go`, `paths_darwin.go`) satisfy these interfaces. Unit tests inject
mocks. Integration tests use `INTEGRATION=1` guard.

**Rate-limit retry** (research finding — refines FR-013):

```
On HTTP 429:
  1. Parse Retry-After header (seconds).
  2. Sleep exactly that duration.
  3. Retry; max 3 attempts.
On HTTP 5xx:
  1. Exponential backoff with jitter via cenkalti/backoff/v4.
  2. Max 3 attempts.
```

**Token copy strategy**: LevelDB and SQLite stores are copied to `os.MkdirTemp()`
before opening. Temp dirs are removed with `defer os.RemoveAll(dir)`.

**Output routing**: All data → stdout. All errors + notices → stderr.
`--format` flag is validated in `root.go` before any subcommand runs.

**Non-marketplace rate limit warning**: When paginating `conversations.history`
and a 429 with `Retry-After > 30s` is received, print a progress warning to
stderr: `rate limited — waiting Xs (message N of approx M)`.
