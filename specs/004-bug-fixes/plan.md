# Implementation Plan: 004 Bug Fixes

**Branch**: `004-bug-fixes` | **Date**: 2026-03-04 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/004-bug-fixes/spec.md`

## Summary

Five targeted bug fixes across `internal/slack/resolver.go` and
`internal/output/format.go`. No new dependencies, no API changes, no new
commands. All fixes are additive or corrective changes to existing functions.

1. **Fix 1** — Extend `mentionPattern` to handle `<@USERID|label>` tokens.
2. **Fix 2** — Group thread replies under their parent message in all output formats.
3. **Fix 3** — Add `--format markdown` for `history` and `search` output.
4. **Fix 4** — Resolve DM channel names to user display names via `ResolveChannelDisplay`.
5. **Fix 5** — Collapse embedded newlines in table cells via `tableSafe`.

## Technical Context

**Language/Version**: Go 1.24
**Primary Dependencies**: `github.com/olekukonko/tablewriter v1.1.3` (existing), `regexp` stdlib (existing)
**Storage**: N/A — no new storage; existing file-backed cache unchanged
**Testing**: `go test -race ./...` (mandatory per constitution)
**Target Platform**: Linux + macOS
**Project Type**: CLI
**Performance Goals**: No new performance requirements; all helpers are O(n)
**Constraints**: Functions ≤ 40 lines; no new external dependencies
**Scale/Scope**: 5 files touched; ~150 lines net new (mostly tests)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Clarity Over Cleverness | ✅ PASS | All new helpers (`groupByThread`, `tableSafe`, `ResolveChannelDisplay`) are ≤ 15 lines with descriptive names. `ReplaceAllStringFunc` with `FindStringSubmatch` mirrors the existing `subteamPattern` pattern — no novel cleverness. |
| II. Test-First (NON-NEGOTIABLE) | ✅ PASS | Each fix specifies test additions. Tasks will enforce Red-Green-Refactor ordering. |
| III. Single-Responsibility | ✅ PASS | Changes stay within existing package boundaries. `resolver.go` gains one method; `format.go` gains helpers within its existing responsibility. No cross-package dependency changes. |
| IV. Actionable Error Handling | ✅ PASS | No new error paths introduced. |
| V. Platform Isolation | ✅ PASS | No platform-specific code introduced. All changes are cross-platform. |

**No violations. Complexity Tracking table not required.**

## Project Structure

### Documentation (this feature)

```text
specs/004-bug-fixes/
├── plan.md          ← this file
├── spec.md          ← fix-feature.md (source of truth)
├── research.md      ← design decisions + alternatives
├── data-model.md    ← type changes + new helpers
├── quickstart.md    ← manual verification steps
├── contracts/
│   └── cli-flags.md ← --format change + Resolver API + JSON schema note
└── tasks.md         ← Phase 2 output (not yet created)
```

### Source Code (repository root)

```text
internal/slack/
├── resolver.go         # Fix 1: mentionPattern + ResolveMentions
│                       # Fix 4: userIDPattern + ResolveChannelDisplay
└── resolver_test.go    # Tests for Fix 1 + Fix 4

internal/output/
├── format.go           # Fix 2: groupByThread, messageJSON.Replies
│                       # Fix 3: FormatMarkdown, printMessagesMarkdown,
│                       #        printSearchResultsMarkdown
│                       # Fix 4: resolveMessageFields + toMessageJSON use
│                       #        ResolveChannelDisplay
│                       # Fix 5: tableSafe
└── format_test.go      # Tests for Fix 2, 3, 5

cmd/
└── root.go             # Fix 3: --format flag description string only
```

**Structure Decision**: Single-project layout. All changes are within the
existing `internal/slack` and `internal/output` packages. No new packages.
