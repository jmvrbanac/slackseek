# Implementation Plan: 005 Day 2 Improvements

**Branch**: `005-day2-improvements` | **Date**: 2026-03-05 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/005-day2-improvements/spec.md`

## Summary

Ten independent improvements across ergonomics, output quality, and new
developer-workflow commands. All changes are additive: no existing API
signatures are removed. Ordered by implementation priority (P1 first).

1. **`--quiet` flag** — suppress stderr progress; thread through history callbacks.
2. **`--since` / `--until` flags** — relative-date offsets in `DateRange` parsing.
3. **`slackseek thread <url>`** — new command; permalink parser + `FetchThread` API.
4. **Line wrapping in text output** — `wrap.go` helper + `golang.org/x/term`.
5. **Multi-channel search** — `--channel` becomes repeatable; parallel fetch.
6. **Emoji rendering** — new `internal/emoji` package with embedded lookup table.
7. **`slackseek postmortem <channel>`** — incident timeline Markdown document.
8. **`slackseek digest --user`** — per-channel message digest.
9. **`slackseek metrics <channel>`** — aggregated channel statistics.
10. **`slackseek actions <channel>`** — commitment pattern extraction.

## Technical Context

**Language/Version**: Go 1.24
**Primary Dependencies**: `golang.org/x/term` (new); `github.com/olekukonko/tablewriter v1.1.3` (existing); stdlib `regexp`, `sync`, `embed`, `unicode/utf8`
**Storage**: N/A — no new persistent storage; existing file-backed cache unchanged
**Testing**: `go test -race ./...` (mandatory per constitution)
**Target Platform**: Linux + macOS
**Project Type**: CLI
**Performance Goals**: All new helpers are O(n); emoji lookup is O(1) map access; multi-channel search is bounded at 3 concurrent goroutines
**Constraints**: Functions ≤ 40 lines; no panics; new emoji-map.json ≤ 200 KB
**Scale/Scope**: 6 new files (`cmd/thread.go`, `cmd/postmortem.go`, `cmd/digest.go`, `cmd/metrics.go`, `cmd/actions.go`, `internal/emoji/`); 6 new output helpers; ~800 lines net new (including tests)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Clarity Over Cleverness | ✅ PASS | All new helpers are ≤ 40 lines with descriptive names. Emoji regex replacement mirrors the existing `mentionPattern` approach. Word-wrap algorithm is a greedy line-fill — no clever bit-twiddling. Permalink parser is a simple string split. |
| II. Test-First (NON-NEGOTIABLE) | ✅ PASS | Each item specifies test tasks. Tasks will enforce Red-Green-Refactor ordering. Emoji table is bundled so tests are offline-capable. |
| III. Single-Responsibility | ✅ PASS | `internal/emoji` has one purpose (name→Unicode). `internal/output/wrap.go` has one purpose (word-wrap). Four new output files each handle one command's formatting. `internal/slack/permalink.go` handles one concern (URL parsing). Cross-package deps flow downward only. |
| IV. Actionable Error Handling | ✅ PASS | `ParsePermalink` errors name the expected format. `ParseRelativeDateRange` errors state which field failed and the accepted formats. `FetchThread` errors wrap the API error with channel + thread context. |
| V. Platform Isolation | ✅ PASS | Terminal width detection uses `golang.org/x/term` (cross-platform). No platform-specific syscalls introduced. Emoji JSON is embedded (no file-system access at runtime). |

**No violations. Complexity Tracking table not required.**

## Project Structure

### Documentation (this feature)

```text
specs/005-day2-improvements/
├── plan.md              ← this file
├── spec.md              ← feature specification (source of truth)
├── research.md          ← design decisions + alternatives
├── data-model.md        ← new types + function signatures
├── quickstart.md        ← manual verification steps
├── contracts/
│   └── cli-flags.md     ← new/changed flags + JSON schemas
└── tasks.md             ← Phase 2 output (not yet created)
```

### Source Code (repository root)

```text
internal/slack/
├── permalink.go          # ThreadPermalink type + ParsePermalink
├── permalink_test.go     # ParsePermalink unit tests
├── daterange.go          # parseDateOrOffset + ParseRelativeDateRange (additive)
├── daterange_test.go     # ParseRelativeDateRange test cases (additive)
├── client.go             # FetchThread method (additive)
└── client_test.go        # FetchThread test cases (additive)

internal/emoji/
├── doc.go                # package comment
├── emoji.go              # Render + RenderName + embedded table
├── emoji_test.go         # table lookup + Render unit tests
└── emoji-map.json        # embedded name→Unicode map (~150 KB)

internal/output/
├── wrap.go               # WordWrap helper
├── wrap_test.go          # WordWrap edge cases
├── format.go             # wire wrapping + emoji into printMessagesText, formatReactions
├── format_test.go        # wrap + emoji integration (additive)
├── postmortem.go         # IncidentDoc + PrintPostmortem
├── postmortem_test.go
├── digest.go             # GroupByChannel + PrintDigest
├── digest_test.go
├── metrics.go            # ComputeMetrics + PrintMetrics
├── metrics_test.go
└── actions.go            # ExtractActions + PrintActions
   actions_test.go

cmd/
├── root.go               # --quiet, --since, --until, --width, --emoji flags
├── root_test.go          # flag mutual-exclusion tests (additive)
├── search.go             # --channel → StringArrayVarP + multi-channel run
├── search_test.go        # multi-channel merge + dedup tests (additive)
├── thread.go             # thread command
├── thread_test.go
├── postmortem.go         # postmortem command
├── postmortem_test.go
├── digest.go             # digest command
├── digest_test.go
├── metrics.go            # metrics command
├── metrics_test.go
├── actions.go            # actions command
└── actions_test.go
```

**Structure Decision**: Single-project layout. New packages are added under
`internal/emoji` and `internal/output` (new files). All new commands are in
`cmd/`. No cross-package dependency changes.

## Complexity Tracking

> **No violations — this table is intentionally empty.**
