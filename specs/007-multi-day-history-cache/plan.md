# Implementation Plan: Multi-Day History Cache

**Branch**: `007-multi-day-history-cache` | **Date**: 2026-03-09 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/007-multi-day-history-cache/spec.md`

## Summary

Extend the existing per-day history cache (006) to cover multi-day date ranges such as
`--since 2w`. Instead of making N per-day API calls (which would burn Tier 3 rate budget),
a single bulk API call is made for each contiguous uncached gap range — identical in cost
to the current non-caching path. The bulk result is then partitioned client-side into
per-day buckets, each complete past day is cached with `SaveStable`, and today's messages
are always fetched live. Thread replies are bucketed with their root message's day (not
their own posting day) to keep threads intact across cache loads.

## Technical Context

**Language/Version**: Go 1.24
**Primary Dependencies**: `github.com/olekukonko/tablewriter v1.1.3` (existing); stdlib only for new code
**Storage**: File-based JSON under `os.UserCacheDir()/slackseek/` (existing `internal/cache` store, unchanged)
**Testing**: `go test -race ./...`; table-driven unit tests; injectable `historyFetchFunc` for all new inner functions
**Target Platform**: Linux + macOS (no platform-specific code in this feature)
**Project Type**: CLI tool
**Performance Goals**: First run: same API call count as current; subsequent runs: O(1) disk reads for past days
**Constraints**: No new dependencies; no panics; cache failures must not fail the command; ≤ 40 lines per function
**Scale/Scope**: Per-user disk cache; typical day entry < 1MB JSON; up to ~90 days in common usage

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|---|---|---|
| I. Clarity Over Cleverness | PASS | Each new function has a single responsibility and a descriptive name; two-pass bucketing algorithm is documented in data-model.md |
| II. Test-First (NON-NEGOTIABLE) | PASS | All new exported and inner functions have table-driven tests specified before implementation |
| III. Single-Responsibility Packages | PASS | All new code lives in `cmd/historycache.go`; no new packages; `internal/cache` is unchanged |
| IV. Actionable Error Handling | PASS | Cache failures warn to stderr (non-fatal); no new user-facing error paths |
| V. Platform Isolation via Build Tags | PASS | No platform-specific code; `os.UserCacheDir` is stdlib cross-platform |

## Project Structure

### Documentation (this feature)

```text
specs/007-multi-day-history-cache/
├── plan.md              # This file
├── spec.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── cli-flags.md
└── tasks.md             # /speckit.tasks output (not yet created)
```

### Source Code

```text
cmd/
├── historycache.go      # +enumeratePastDays, +buildGapRanges, +partitionByDay,
│                        #  +fetchHistoryMultiDayCached, updated FetchHistoryCached routing
└── historycache_test.go # +TestEnumeratePastDays, +TestBuildGapRanges,
                         #  +TestPartitionByDay, +TestFetchHistoryMultiDayCached_*
```

No changes to `internal/cache`, `internal/slack`, or any other `cmd/` files.
Call sites (`history.go`, `postmortem.go`, `metrics.go`) are unchanged — they already
call `FetchHistoryCached`, which gains the multi-day routing internally.

**Structure Decision**: Single project (default). All changes are additive within the
existing `cmd/historycache.go` file. No new packages required.

## Complexity Tracking

No constitution violations. All changes are additive within existing files.

---

## Phase 0: Research

*Completed. See [research.md](research.md).*

Key findings:

1. **Fetch strategy**: Bulk fetch per gap range (not per day) — identical API call count
   to the current non-caching path; zero extra rate-limit pressure.
2. **Thread bucketing**: Two-pass algorithm (root-day index → reply inheritance via
   `ThreadTS`) stores replies with their root's day, matching 006 single-day behaviour.
3. **Gap construction**: Enumerate past days, check `LoadStable` per day, merge contiguous
   misses into gap ranges, always append today as a live gap.
4. **`--limit` semantics**: Applied to the final merged slice only; internal fetches use
   `limit=0` to ensure complete, cacheable day buckets.
5. **`--no-cache`**: Skips `LoadStable` reads; still writes back via `SaveStable` (consistent
   with 006 documented behaviour).
6. **Routing**: `FetchHistoryCached` detects multi-day via `isMultiDay(dr)` and delegates
   to `fetchHistoryMultiDayCached`; single-day path unchanged.

---

## Phase 1: Design & Contracts

*Completed. See [data-model.md](data-model.md), [contracts/cli-flags.md](contracts/cli-flags.md), [quickstart.md](quickstart.md).*

### New functions in `cmd/historycache.go`

```go
// enumeratePastDays returns YYYY-MM-DD strings for each complete UTC day
// strictly before today within [from, to). Returns nil for single-day or
// open-ended ranges (caller falls through to existing path).
func enumeratePastDays(from, to time.Time) []string

// buildGapRanges checks the cache for each past day, loads hits into cached,
// and merges contiguous misses into gap DateRanges. Today is always appended
// as the final gap.
func buildGapRanges(
    pastDays  []string,
    store     *cache.Store,
    wsKey     string,
    channelID string,
    noCache   bool,
) (cached map[string][]slack.Message, gaps []slack.DateRange, err error)

// partitionByDay buckets a flat []Message by UTC calendar day using a two-pass
// algorithm: pass 1 builds a root-day index; pass 2 assigns replies to their
// root's day via ThreadTS.
func partitionByDay(msgs []slack.Message) map[string][]slack.Message

// fetchHistoryMultiDayCached is the testable inner implementation for multi-day
// ranges. fetchFn is injectable for unit tests.
func fetchHistoryMultiDayCached(
    ctx       context.Context,
    fetchFn   historyFetchFunc,
    store     *cache.Store,
    wsKey     string,
    channelID string,
    dr        slack.DateRange,
    limit     int,
    threads   bool,
    noCache   bool,
) ([]slack.Message, error)
```

### Updated routing in `FetchHistoryCached`

```go
func FetchHistoryCached(...) ([]slack.Message, error) {
    return fetchHistoryCachedInner(ctx, c.FetchHistory, ...)  // existing
}
// becomes:
func FetchHistoryCached(...) ([]slack.Message, error) {
    if isMultiDay(dr) {
        return fetchHistoryMultiDayCached(ctx, c.FetchHistory, ...)
    }
    return fetchHistoryCachedInner(ctx, c.FetchHistory, ...)
}
```

`isMultiDay(dr)` — both `From` and `To` non-nil, and their UTC dates differ.

---

## Post-Design Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. Clarity | PASS | Each function is ≤ 40 lines; two-pass bucketing documented; routing is explicit |
| II. Test-First | PASS | Tests specified for every new function and error path |
| III. Single-Responsibility | PASS | All new code in `cmd/historycache.go`; no cross-package entanglement |
| IV. Error Handling | PASS | Cache failures are non-fatal stderr warnings; gap fetch errors propagate with `%w` |
| V. Platform Tags | PASS | No platform-specific code |
