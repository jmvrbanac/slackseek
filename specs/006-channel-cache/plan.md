# Implementation Plan: Long-Term Day-History Cache

**Branch**: `006-channel-cache` | **Date**: 2026-03-06 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/006-channel-cache/spec.md`

## Summary

Add a permanent (no-TTL) on-disk cache for complete past-day channel history. When
`history`, `postmortem`, or `metrics` is run for a single completed past day with
`threads=true` and a non-truncating limit, the result is saved to
`{cacheDir}/{wsKey}/history/{channelID}/{date}.json`. Subsequent requests for the same
day return from disk without any Slack API call. A `--no-cache` flag forces a refresh.

## Technical Context

**Language/Version**: Go 1.24
**Primary Dependencies**: `github.com/olekukonko/tablewriter v1.1.3` (existing); stdlib only for new code
**Storage**: File-based JSON under `os.UserCacheDir()/slackseek/` (existing `internal/cache` store)
**Testing**: `go test -race ./...`; table-driven unit tests; no new integration test requirements
**Target Platform**: Linux + macOS (no platform-specific code in this feature)
**Project Type**: CLI tool
**Performance Goals**: Cache load < 1ms; no regression on cache-miss path
**Constraints**: No new dependencies; no panics; cache failures must not fail the command
**Scale/Scope**: Per-user disk cache; typical entry < 1MB JSON

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|---|---|---|
| I. Clarity Over Cleverness | PASS | `cacheableDayKey` and `FetchHistoryCached` have narrow, descriptive signatures |
| II. Test-First (NON-NEGOTIABLE) | PASS | All new exported functions and CLI flags will have tests before implementation |
| III. Single-Responsibility Packages | PASS | New `historycache.go` lives in `cmd/`; `LoadStable`/`SaveStable` extend `internal/cache` only |
| IV. Actionable Error Handling | PASS | Cache write failures warn to stderr (non-fatal); no new user-facing errors added |
| V. Platform Isolation via Build Tags | PASS | No platform-specific code in this feature; `os.UserCacheDir` is stdlib cross-platform |

## Project Structure

### Documentation (this feature)

```text
specs/006-channel-cache/
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
internal/cache/
├── store.go             # +LoadStable, +SaveStable
└── store_test.go        # +TestLoadStable_BypassesTTL, +TestSaveStable_*

cmd/
├── historycache.go      # NEW: FetchHistoryCached, cacheableDayKey
├── historycache_test.go # NEW: table-driven tests for all cache paths
├── history.go           # wire FetchHistoryCached; add --no-cache flag
├── history_test.go      # add --no-cache + cache hit/miss tests
├── postmortem.go        # wire FetchHistoryCached; add --no-cache flag
├── postmortem_test.go   # add --no-cache + cache hit/miss tests
├── metrics.go           # wire FetchHistoryCached; add --no-cache flag
└── metrics_test.go      # add --no-cache + cache hit/miss tests
```

**Structure Decision**: Single project (default). No new packages. New code extends
existing `internal/cache` and `cmd/` packages.

## Complexity Tracking

No constitution violations. All changes are additive within existing packages.

---

## Phase 0: Research

*Completed. See [research.md](research.md).*

Key findings:

1. **Cache key**: `kind = "history/" + channelID + "/" + date` on existing `Store` —
   `os.MkdirAll` already handles nested paths.
2. **TTL bypass**: Add `LoadStable` that omits the mod-time check; `SaveStable` is
   an alias for `Save`.
3. **Shared helper**: `FetchHistoryCached` in `cmd/historycache.go` replaces direct
   `c.FetchHistory` calls in `defaultRunHistory`, `defaultRunPostmortem`, `defaultRunMetrics`.
4. **Scope**: `digest` (search API, user-scoped) and `actions` (threads=false) are out
   of scope for v1. `--no-cache` is not added to those commands.

---

## Phase 1: Design & Contracts

*Completed. See [data-model.md](data-model.md), [contracts/cli-flags.md](contracts/cli-flags.md), [quickstart.md](quickstart.md).*

### `internal/cache/store.go` additions

```go
// LoadStable reads key/kind without checking the TTL.
// Returns (data, true, nil) on a valid JSON hit, (nil, false, nil) on any miss.
func (s *Store) LoadStable(key, kind string) ([]byte, bool, error) {
    path := filepath.Join(s.dir, key, kind+".json")
    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, false, nil
        }
        return nil, false, fmt.Errorf("cache read %s: %w", path, err)
    }
    if !json.Valid(data) {
        return nil, false, nil
    }
    return data, true, nil
}

// SaveStable writes data to {dir}/{key}/{kind}.json with no TTL semantics.
// Write failures are printed to stderr and nil is returned (non-fatal).
func (s *Store) SaveStable(key, kind string, data []byte) error {
    return s.Save(key, kind, data) // identical mechanics
}
```

### `cmd/historycache.go` additions

```go
// cacheableDayKey returns the YYYY-MM-DD cache key when the DateRange and
// fetch result qualify for permanent caching. Returns "" otherwise.
func cacheableDayKey(dr slack.DateRange, fetchedCount, limit int) string

// FetchHistoryCached checks the day cache before calling FetchHistory and
// writes to cache on a miss (or when noCache=true).
func FetchHistoryCached(
    ctx context.Context,
    c *slack.Client,
    store *cache.Store,
    wsKey, channelID string,
    dr slack.DateRange,
    limit int,
    threads bool,
    noCache bool,
) ([]slack.Message, error)
```

### CLI changes

`--no-cache bool` (default false) added to `history`, `postmortem`, `metrics`.

---

## Post-Design Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. Clarity | PASS | Helper functions have single responsibilities and descriptive names |
| II. Test-First | PASS | Tests specified for all paths in tasks.md |
| III. Single-Responsibility | PASS | `historycache.go` has one purpose; `LoadStable` extends cache cleanly |
| IV. Error Handling | PASS | Cache failures are non-fatal warnings; no new user-facing error paths |
| V. Platform Tags | PASS | No platform-specific code |
