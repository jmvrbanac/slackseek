# Tasks: Long-Term Day-History Cache

**Input**: Design documents from `specs/006-channel-cache/`
**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓, contracts/cli-flags.md ✓

**Tests**: MANDATORY per Constitution Principle II. All exported functions and CLI flags have
accompanying tests. TDD order: write failing test → implement → verify pass.

**Organization**: Tasks are grouped by user story to enable independent implementation and
testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: Which user story this task belongs to (US1–US4)

---

## Phase 1: Setup

**Purpose**: Create new file skeletons so test files can compile before implementation.

- [X] T001 Create `cmd/historycache.go` with package declaration, imports placeholder, and stub signatures for `cacheableDayKey` and `FetchHistoryCached`
- [X] T002 [P] Create `cmd/historycache_test.go` with package declaration and import block

**Checkpoint**: Both files compile (`go build ./...` passes); tests compile but are empty.

---

## Phase 2: Foundational — Cache Store Extensions

**Purpose**: `LoadStable` and `SaveStable` on `internal/cache.Store` must exist before any
command-layer work. All user stories depend on these methods.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T003 Add `LoadStable(key, kind string) ([]byte, bool, error)` to `internal/cache/store.go` — reads `{dir}/{key}/{kind}.json`, validates JSON, returns hit/miss without checking TTL (see data-model.md)
- [X] T004 [P] Add `SaveStable(key, kind string, data []byte) error` to `internal/cache/store.go` — delegates entirely to `Save` (TTL does not apply to stable entries)
- [X] T005 Add `TestLoadStable_BypassesTTL` in `internal/cache/store_test.go` — write entry with expired mod-time; assert `LoadStable` returns hit while `Load` returns miss
- [X] T006 [P] Add `TestLoadStable_MissingFile` and `TestLoadStable_InvalidJSON` in `internal/cache/store_test.go`
- [X] T007 [P] Add `TestSaveStable_RoundTrip` in `internal/cache/store_test.go` — `SaveStable` then `LoadStable` returns original data

**Checkpoint**: `go test -race ./internal/cache/...` passes with all new tests green.

---

## Phase 3: User Story 1 — Cache Hit Skips API for `history` (Priority: P0) 🎯 MVP

**Goal**: Running `history` for a completed past day fetches once then serves from disk.

**Independent Test**:
```sh
slackseek history general --from 2026-03-01 --to 2026-03-01   # fetches + writes cache
slackseek history general --from 2026-03-01 --to 2026-03-01   # returns from cache (no API call)
```

### Tests for User Story 1 ⚠️ Write these FIRST — they must FAIL before implementation

- [X] T008 [US1] Add table-driven `TestCacheableDayKey` in `cmd/historycache_test.go` covering: nil From, nil To, multi-day range, today, future, past full day (limit=0), past full day (count < limit), truncated (count == limit)
- [X] T009 [P] [US1] Add `TestFetchHistoryCached_Miss` in `cmd/historycache_test.go` — no cache entry, API called, entry written
- [X] T010 [P] [US1] Add `TestFetchHistoryCached_Hit` in `cmd/historycache_test.go` — cache entry present, API NOT called, cached messages returned
- [X] T011 [P] [US1] Add `TestFetchHistoryCached_Truncated` in `cmd/historycache_test.go` — count == limit, no cache entry written
- [X] T012 [P] [US1] Add `TestFetchHistoryCached_Today` in `cmd/historycache_test.go` — date range covers today, no cache entry written

### Implementation for User Story 1

- [X] T013 [US1] Implement `cacheableDayKey(dr slack.DateRange, fetchedCount, limit int) string` in `cmd/historycache.go` per data-model.md state table (all four conditions)
- [X] T014 [US1] Implement `FetchHistoryCached` in `cmd/historycache.go` — pre-fetch `LoadStable` check, call `c.FetchHistory`, post-fetch `cacheableDayKey` guard, `SaveStable` write; `noCache=false` path only
- [X] T015 [US1] Replace `c.FetchHistory(ctx, channelID, dr, limit, threads)` call in `defaultRunHistory` (`cmd/history.go`) with `FetchHistoryCached(..., noCache=false)`; pass `buildCacheStore(workspace)` and `cache.WorkspaceKey(workspace.URL)` through
- [X] T016 [US1] Add `TestDefaultRunHistory_CacheHit` and `TestDefaultRunHistory_CacheMiss` in `cmd/history_test.go` using the injectable `runFn` pattern

**Checkpoint**: `go test -race ./cmd/...` passes. `history` for a past day writes to disk on first run, reads from disk on second run.

---

## Phase 4: User Story 2 — `--no-cache` Forces Fresh Fetch (Priority: P1)

**Goal**: `history --no-cache` bypasses cache load but still refreshes the cache entry.

**Independent Test**:
```sh
# Pre-populate cache, then force refresh:
slackseek history general --from 2026-03-01 --to 2026-03-01 --no-cache
# Cache file is updated; original API call was made.
```

### Tests for User Story 2 ⚠️ Write these FIRST

- [X] T017 [US2] Add `TestFetchHistoryCached_NoCache_SkipsLoad` in `cmd/historycache_test.go` — when `noCache=true`, API is always called even when cache entry exists
- [X] T018 [P] [US2] Add `TestFetchHistoryCached_NoCache_StillWrites` in `cmd/historycache_test.go` — after `noCache=true` fetch, `LoadStable` returns the freshly written entry
- [X] T019 [P] [US2] Add `TestHistoryCmd_NoCacheFlag` in `cmd/history_test.go` — `--no-cache` flag parsed correctly and forwarded to run function

### Implementation for User Story 2

- [X] T020 [US2] Update `FetchHistoryCached` in `cmd/historycache.go` to handle `noCache=true`: skip `LoadStable`, always call API, always write to cache on eligible day
- [X] T021 [US2] Add `--no-cache bool` flag (default `false`) to `history` command in `cmd/history.go`; thread `noCache` through `runHistoryE` → `defaultRunHistory` → `FetchHistoryCached`

**Checkpoint**: `go test -race ./cmd/...` passes. `--no-cache` skips disk read but still writes.

---

## Phase 5: User Story 3 — `postmortem` and `metrics` Share the Cache (Priority: P1)

**Goal**: All three channel-history commands (history, postmortem, metrics) read and write the
same cache entry for a given workspace + channel + day.

**Independent Test**:
```sh
slackseek postmortem incidents --from 2026-03-01 --to 2026-03-01  # writes cache
slackseek metrics    incidents --from 2026-03-01 --to 2026-03-01  # hits cache (no API)
slackseek history    incidents --from 2026-03-01 --to 2026-03-01  # hits cache (no API)
```

### Tests for User Story 3 ⚠️ Write these FIRST

- [X] T022 [US3] Add `TestDefaultRunPostmortem_CacheHit` and `TestDefaultRunPostmortem_CacheMiss` in `cmd/postmortem_test.go`
- [X] T023 [P] [US3] Add `TestDefaultRunMetrics_CacheHit` and `TestDefaultRunMetrics_CacheMiss` in `cmd/metrics_test.go`
- [X] T024 [P] [US3] Add `TestPostmortemCmd_NoCacheFlag` in `cmd/postmortem_test.go`
- [X] T025 [P] [US3] Add `TestMetricsCmd_NoCacheFlag` in `cmd/metrics_test.go`

### Implementation for User Story 3

- [X] T026 [US3] Replace `c.FetchHistory` call in `defaultRunPostmortem` (`cmd/postmortem.go`) with `FetchHistoryCached(..., threads=true, limit=0, noCache=false)`
- [X] T027 [P] [US3] Replace `c.FetchHistory` call in `defaultRunMetrics` (`cmd/metrics.go`) with `FetchHistoryCached(..., threads=true, limit=0, noCache=false)`
- [X] T028 [US3] Add `--no-cache bool` flag to `postmortem` command in `cmd/postmortem.go`; thread through `runPostmortemE` → `defaultRunPostmortem` → `FetchHistoryCached`
- [X] T029 [P] [US3] Add `--no-cache bool` flag to `metrics` command in `cmd/metrics.go`; thread through `runMetricsE` → `defaultRunMetrics` → `FetchHistoryCached`

**Checkpoint**: `go test -race ./cmd/...` passes. Writing cache via `postmortem` and reading via `metrics` (or vice versa) works correctly.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Quality gates, documentation, and spec consistency.

- [X] T030 Run `go test -race ./...` and fix any failures (full suite including new tests)
- [X] T031 [P] Run `go vet ./...` and fix any issues
- [ ] T032 [P] Run `golangci-lint run` and fix any lint issues
- [X] T033 [P] Run `GOOS=linux go build ./...` and `GOOS=darwin go build ./...` to confirm cross-platform build
- [X] T034 Update `CLAUDE.md` active technologies section to include 006-channel-cache entry

**Checkpoint**: All quality gates pass. Feature is merge-ready.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — **BLOCKS all user stories**
- **US1 (Phase 3)**: Depends on Phase 2 — primary cache path for `history`
- **US2 (Phase 4)**: Depends on Phase 3 — adds `--no-cache` to `history`
- **US3 (Phase 5)**: Depends on Phase 2 (foundational), can start parallel to Phase 3 after T014 is done
- **Polish (Phase 6)**: Depends on all user story phases

### User Story Dependencies

- **US1 (P0)**: Depends only on Phase 2 (cache store methods)
- **US2 (P1)**: Depends on US1 (`FetchHistoryCached` signature established)
- **US3 (P1)**: Depends on Phase 2 + US1's `FetchHistoryCached` being complete (T014)
- **US4 (P0)**: Persistence is a property of `LoadStable`/`SaveStable` design — covered by Phase 2

### Within Each Phase

- Tests MUST be written (and confirmed to fail) before the corresponding implementation tasks
- Stub in T001 allows test files to compile from the start
- T003 and T004 can be written in a single edit pass since `SaveStable` delegates to `Save`

---

## Parallel Execution Examples

### Phase 2: Foundational (parallel within phase)

```
T003 Add LoadStable to store.go          ─┐
T004 Add SaveStable to store.go          ─┤─ same file, do sequentially or one edit
T005 TestLoadStable_BypassesTTL          ─┐
T006 TestLoadStable_MissingFile/JSON     ─┤─ same file, do sequentially
T007 TestSaveStable_RoundTrip            ─┘
```

### Phase 3: US1 (parallel test writing)

```
T008 TestCacheableDayKey (table-driven)  ─┐
T009 TestFetchHistoryCached_Miss         ─┤
T010 TestFetchHistoryCached_Hit          ─┤─ all in historycache_test.go, write sequentially
T011 TestFetchHistoryCached_Truncated    ─┤
T012 TestFetchHistoryCached_Today        ─┘

T013 Implement cacheableDayKey           → then
T014 Implement FetchHistoryCached        → then
T015 Wire into defaultRunHistory         → then
T016 TestDefaultRunHistory_Cache*        ─ cmd/history_test.go (different file, parallel ok)
```

### Phase 5: US3 (parallel across commands)

```
T026 Wire postmortem  ─┐
T027 Wire metrics     ─┘  parallel (different files)

T028 --no-cache postmortem  ─┐
T029 --no-cache metrics     ─┘  parallel (different files)
```

---

## Implementation Strategy

### MVP (User Story 1 only — Phases 1–3)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (cache store extensions)
3. Complete Phase 3: US1 (`history` caching)
4. **STOP and VALIDATE**: `history` for a past day should read/write cache
5. Demo: run `history` twice, confirm second call has no `fetching channels` progress output

### Incremental Delivery

1. Phases 1–3 → `history` caches past days (MVP)
2. Phase 4 → `--no-cache` flag on `history`
3. Phase 5 → `postmortem` + `metrics` share the cache
4. Phase 6 → all quality gates pass, ready to merge

### Key Implementation Notes

- `FetchHistoryCached` in `cmd/historycache.go` is the single decision point; keep it ≤ 40 lines
- `SaveStable` calls `Save` directly — do not duplicate write logic
- The `cacheableDayKey` pre-fetch check uses `limit == 0` only (no count yet); the post-fetch check uses the full four conditions
- `--no-cache` always **writes** to cache (even when true) — this is the refresh semantic
- `actions` and `digest` are intentionally excluded from this feature
