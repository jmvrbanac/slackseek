# Implementation Plan: Resolve User IDs and Channel IDs in Output

**Branch**: `003-resolve-ids-in-output` | **Date**: 2026-03-03 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/003-resolve-ids-in-output/spec.md`

## Summary

All message-bearing commands (`history`, `messages`, `search`) currently display raw Slack
user IDs (e.g. `U01234567`) and channel IDs (e.g. `C01234567`) in their output. This
feature adds a lightweight `Resolver` type to `internal/slack` that builds in-memory lookup
maps from the cached `[]User` and `[]Channel` slices and supplies human-readable names to
the output layer — with zero additional API calls and graceful fallback when data is absent.

## Technical Context

**Language/Version**: Go 1.24 (unchanged)
**Primary Dependencies**: stdlib only for Resolver; existing `internal/slack`, `internal/cache`, `internal/output` packages
**Storage**: Uses existing `internal/cache` file-backed store (no new storage)
**Testing**: `go test -race ./...`; table-driven unit tests with mock data
**Target Platform**: Linux + macOS (unchanged)
**Project Type**: CLI
**Performance Goals**: Resolution is O(n) map build + O(1) lookup per message; no network calls
**Constraints**: Zero extra API calls for resolution; `--no-cache` suppresses resolution entirely
**Scale/Scope**: Workspaces with up to tens of thousands of users and channels

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Clarity Over Cleverness | ✅ PASS | `Resolver` is a simple struct with two lookup maps; methods are one-liners |
| II. Test-First | ✅ PASS | Unit tests for `Resolver`, updated output tests, updated cmd integration tests required before merge |
| III. Single-Responsibility | ✅ PASS | `Resolver` lives in `internal/slack` (entity domain); output formatting stays in `internal/output` |
| IV. Actionable Error Handling | ✅ PASS | No new errors surface to the user; fallback to raw ID is silent and safe |
| V. Platform Isolation | ✅ PASS | No platform-specific code introduced |

**Pre-Phase 0 gate**: All principles pass. No violations to justify.

## Project Structure

### Documentation (this feature)

```text
specs/003-resolve-ids-in-output/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
internal/slack/
├── resolver.go          # NEW: Resolver struct, NewResolver, UserDisplayName, ChannelName
├── resolver_test.go     # NEW: unit tests for Resolver
├── types.go             # MODIFY: no struct changes needed (Resolver is separate)
├── channels.go          # unchanged
└── users.go             # unchanged

internal/output/
├── format.go            # MODIFY: PrintMessages, PrintSearchResults accept optional *slack.Resolver
└── format_test.go       # MODIFY: add tests for resolved output

cmd/
├── history.go           # MODIFY: create Resolver from client, pass to output
├── messages.go          # MODIFY: create Resolver from client, pass to output
└── search.go            # MODIFY: create Resolver from client, pass to output
```

**Structure Decision**: Single project structure retained. Resolver is a new file in
`internal/slack/` to honour the single-responsibility principle (ID resolution is a Slack
domain concern). Output formatting continues to live in `internal/output/`.

## Complexity Tracking

> No constitution violations detected; table intentionally empty.
