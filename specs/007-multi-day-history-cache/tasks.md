# Tasks: Multi-Day History Cache

**Input**: Design documents from `/specs/007-multi-day-history-cache/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/cli-flags.md

**Tests**: Mandatory per constitution (Principle II — Test-First, NON-NEGOTIABLE).
Write each test task FIRST and confirm it fails before writing the implementation.

**Organization**: Tasks grouped by user story. All code changes are confined to
`cmd/historycache.go` and `cmd/historycache_test.go`. No other files change.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files or independent within same file)
- **[Story]**: User story label (US1–US6 from spec.md)
- Exact file paths included in every description

---

## Phase 1: Setup

**Purpose**: Confirm baseline and verify existing cache tests are green before
touching `cmd/historycache.go`.

- [X] T001 Run `go test -race ./cmd/...` and confirm all existing `historycache_test.go` tests pass
- [X] T002 Run `golangci-lint run ./cmd/...` and confirm zero lint violations

**Checkpoint**: Existing 006 cache tests green. Safe to extend `historycache.go`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Two small, independently-testable building blocks that every subsequent
phase depends on. Complete and test these before writing any multi-day logic.

**⚠️ CRITICAL**: No user-story work can begin until this phase is complete.

- [X] T003 Write failing tests for `isMultiDay` in `cmd/historycache_test.go` — cover: both nil, one nil, same-day range, multi-day range (From < To across midnight UTC)
- [X] T004 Implement `isMultiDay(dr slack.DateRange) bool` in `cmd/historycache.go` — returns true when both `From` and `To` are non-nil and their UTC dates differ; ≤ 10 lines
- [X] T005 Write failing tests for `enumeratePastDays` in `cmd/historycache_test.go` — cover: nil inputs, single-day range (returns nil), range ending today (today excluded), range entirely in past, multi-week range; use table-driven format
- [X] T006 Implement `enumeratePastDays(from, to time.Time) []string` in `cmd/historycache.go` — returns ascending YYYY-MM-DD strings for each complete UTC day strictly before today within [from, to); returns nil for single-day or open-ended; ≤ 20 lines

**Checkpoint**: `go test -race ./cmd/... -run TestIsMultiDay` and `TestEnumeratePastDays` both green.

---

## Phase 3: User Stories 1, 2, 3, 4 — Core Multi-Day Fetch & Cache (Priority: P0)

**Goal**: A multi-day `FetchHistoryCached` call makes one bulk API fetch per gap, buckets
results by day (with thread replies under their root's day), caches each complete past day,
and serves subsequent runs from disk. Overlapping windows reuse cached days automatically.

**Independent Test**: Call `fetchHistoryMultiDayCached` with a stub `fetchFn`, a pre-populated
in-memory store, and a two-week range. Assert: (a) `fetchFn` is called only for uncached
day ranges, (b) returned messages include both cached and fetched, (c) `SaveStable` is called
for each complete past day, (d) today's messages are present but not saved.

### Tests for Phase 3

> **Write these FIRST — confirm they FAIL before implementing**

- [X] T007 [US1] Write failing test `TestPartitionByDay_RootMessages` in `cmd/historycache_test.go` — root messages land in their own UTC day bucket
- [X] T008 [US4] Write failing test `TestPartitionByDay_RepliesInheritRootDay` in `cmd/historycache_test.go` — reply with ThreadDepth=1 at 00:10 UTC day+1 is bucketed under root's day (23:55 UTC day 0); assert reply is NOT in day+1 bucket
- [X] T009 [US1] Write failing test `TestBuildGapRanges_AllCached` in `cmd/historycache_test.go` — all past days in store → gaps contains only today's range; cached map contains all days
- [X] T010 [US1] Write failing test `TestBuildGapRanges_AllUncached` in `cmd/historycache_test.go` — no entries in store → gaps is one contiguous range covering all past days + today; cached map is empty
- [X] T011 [US2] Write failing test `TestBuildGapRanges_PartialCache` in `cmd/historycache_test.go` — days 1–3 cached, days 4–5 not → two gap ranges (4–5 + today); cached map has days 1–3
- [X] T012 [US1] Write failing test `TestFetchHistoryMultiDayCached_FirstRun` in `cmd/historycache_test.go` — cold cache; stub fetchFn returns known messages; assert SaveStable called once per past day; returned slice equals all messages sorted by timestamp
- [X] T013 [US2] Write failing test `TestFetchHistoryMultiDayCached_WarmCache` in `cmd/historycache_test.go` — all past days cached; fetchFn returns only today's messages; assert fetchFn called once (today only); returned slice = cached + today
- [X] T014 [US3] Write failing test `TestFetchHistoryMultiDayCached_OverlappingWindow` in `cmd/historycache_test.go` — pre-populate 14 days of cache; call with 7-day range; assert fetchFn called once (today only); all 7 past days served from cache
- [X] T015 [US4] Write failing test `TestFetchHistoryMultiDayCached_CrossDayThread` in `cmd/historycache_test.go` — root at 23:55 day 1, reply at 00:10 day 2; assert day 1 cache file contains both root and reply; day 2 cache file does not contain the reply

### Implementation for Phase 3

- [X] T016 [P] [US4] Implement `partitionByDay(msgs []slack.Message) map[string][]slack.Message` in `cmd/historycache.go` — two-pass algorithm: pass 1 builds `rootDay map[string]string` (Timestamp → YYYY-MM-DD) for ThreadDepth==0; pass 2 assigns each message to its day (roots: own day; replies: rootDay[ThreadTS]); ≤ 30 lines
- [X] T017 [P] [US1] Implement `buildGapRanges(pastDays []string, store *cache.Store, wsKey, channelID string, noCache bool) (cached map[string][]slack.Message, gaps []slack.DateRange, err error)` in `cmd/historycache.go` — iterate pastDays; LoadStable each; merge contiguous misses into DateRange spans; always append today gap `[todayMidnight, nil)`; ≤ 35 lines
- [X] T018 [US1] Implement `fetchHistoryMultiDayCached(ctx, fetchFn, store, wsKey, channelID, dr, limit, threads, noCache)` in `cmd/historycache.go` — orchestrate: enumeratePastDays → buildGapRanges → fetch each gap with limit=0 → partitionByDay → SaveStable past-day buckets → merge cached+fetched → sort → apply limit; ≤ 40 lines
- [X] T019 [US1] Update `FetchHistoryCached` in `cmd/historycache.go` to add `isMultiDay(dr)` routing: if true delegate to `fetchHistoryMultiDayCached`, otherwise call existing `fetchHistoryCachedInner`; ≤ 5 lines added
- [X] T020 [US1] Run `go test -race ./cmd/... -run "TestPartitionByDay|TestBuildGapRanges|TestFetchHistoryMultiDayCached"` — all Phase 3 tests must pass green

**Checkpoint**: Multi-day fetch, caching, and thread bucketing fully functional. US1–US4
independently verifiable via `TestFetchHistoryMultiDayCached_*` tests.

---

## Phase 4: User Story 5 — `--no-cache` Forces Full Re-fetch (Priority: P1)

**Goal**: `--no-cache` bypasses all `LoadStable` reads (all days treated as misses) and still
writes fresh results back to `SaveStable`, consistent with the 006 single-day behaviour.

**Independent Test**: Call `fetchHistoryMultiDayCached` with `noCache=true` and a fully
pre-populated cache store. Assert: `fetchFn` is called for the full range (not split into
cached+gap); every past day is overwritten in the store.

### Tests for Phase 4

> **Write these FIRST — confirm they FAIL before implementing**

- [X] T021 [US5] Write failing test `TestFetchHistoryMultiDayCached_NoCacheBypassesLoad` in `cmd/historycache_test.go` — all past days cached; noCache=true; assert fetchFn called for full past range (not just today); cached map empty; fetchFn call count > 1 (covers past + today)
- [X] T022 [US5] Write failing test `TestFetchHistoryMultiDayCached_NoCacheStillWrites` in `cmd/historycache_test.go` — noCache=true; assert SaveStable called for each complete past day (fresh data written back to cache)

### Implementation for Phase 4

- [X] T023 [US5] Verify `buildGapRanges` already respects `noCache bool` parameter (skips LoadStable when true, treating all days as misses) — no new code needed if T017 was implemented correctly; add a targeted assertion to T021/T022 if the path is missing
- [X] T024 [US5] Run `go test -race ./cmd/... -run "TestFetchHistoryMultiDayCached_NoCache"` — both tests green

**Checkpoint**: `--no-cache` multi-day behavior matches 006 single-day semantics.

---

## Phase 5: User Story 6 — `--limit` Applied After Merge (Priority: P1)

**Goal**: Internal gap fetches always use `limit=0`. The user's `--limit` value is applied
once to the final merged slice, ensuring cached day files are always complete.

**Independent Test**: Call `fetchHistoryMultiDayCached` with `limit=3` and a stub that
returns 10 messages spread across 3 days. Assert: (a) `fetchFn` is called with `limit=0`,
(b) each past-day SaveStable call receives the full day's messages (not 3), (c) returned
slice has exactly 3 messages.

### Tests for Phase 5

> **Write these FIRST — confirm they FAIL before implementing**

- [X] T025 [US6] Write failing test `TestFetchHistoryMultiDayCached_LimitAppliedAtMerge` in `cmd/historycache_test.go` — stub fetchFn captures the `limit` argument; assert it equals 0 regardless of user limit; assert SaveStable gets full day buckets; assert return length == user limit
- [X] T026 [US6] Write failing test `TestFetchHistoryMultiDayCached_LimitZeroUnlimited` in `cmd/historycache_test.go` — limit=0 (unlimited); assert all messages returned; all past days cached in full

### Implementation for Phase 5

- [X] T027 [US6] Verify `fetchHistoryMultiDayCached` calls `fetchFn` with `limit=0` and applies the user limit only on the final merged slice — no new code needed if T018 was implemented correctly; inspect the implementation and add the limit trim if missing
- [X] T028 [US6] Run `go test -race ./cmd/... -run "TestFetchHistoryMultiDayCached_Limit"` — both tests green

**Checkpoint**: Limit semantics correct. Cached day files are always complete regardless
of `--limit` value.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Lint, race detector, cross-platform build, function-length gate.

- [X] T029 [P] Run `golangci-lint run ./cmd/...` — zero violations; fix any `funlen` violations by extracting sub-functions if any new function exceeds 40 lines
- [X] T030 [P] Run `go test -race ./...` — full test suite green with race detector
- [X] T031 [P] Run `GOOS=linux go build ./...` and `GOOS=darwin go build ./...` — both succeed
- [X] T032 Verify all new exported and inner functions in `cmd/historycache.go` have a one-line doc comment
- [ ] T033 Run the quickstart scenario from `specs/007-multi-day-history-cache/quickstart.md` manually against a real workspace (or skip with `INTEGRATION=1 go test -race ./...` if integration tests cover it)

**Checkpoint**: All quality gates pass. Feature ready for PR.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — run immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 — BLOCKS all user-story phases
- **Phase 3 (US1–US4)**: Depends on Phase 2 (needs `isMultiDay`, `enumeratePastDays`)
- **Phase 4 (US5)**: Depends on Phase 3 (`fetchHistoryMultiDayCached` must exist)
- **Phase 5 (US6)**: Depends on Phase 3 (same reason; can run in parallel with Phase 4)
- **Phase 6 (Polish)**: Depends on Phases 3–5 all complete

### User Story Dependencies

- **US1, US2, US3, US4**: All implemented together in Phase 3 — `partitionByDay` and
  `fetchHistoryMultiDayCached` satisfy all four in one coherent implementation
- **US5**: Depends on Phase 3 (tests the `noCache` path of `fetchHistoryMultiDayCached`)
- **US6**: Depends on Phase 3 (tests the limit path); independent of US5

### Within Each Phase

1. Write test → confirm FAIL → implement → confirm PASS (Red-Green-Refactor)
2. `[P]` tasks in the same phase with no file conflicts can run concurrently
3. Tests within Phase 3 marked `[P]` can be written in parallel (all target `historycache_test.go` but are independent table-driven cases)

---

## Parallel Opportunities

### Phase 2

```
# Can run in parallel (independent logic, same file — no conflict if separate functions):
T003 + T005   # write both test stubs first
T004 + T006   # implement both after tests fail
```

### Phase 3

```
# Write all failing tests first (all in historycache_test.go, independent functions):
T007, T008, T009, T010, T011, T012, T013, T014, T015

# Then implement (T016 and T017 are independent — different function bodies):
T016 (partitionByDay) || T017 (buildGapRanges)

# T018 depends on T016 + T017; T019 depends on T018
T018 → T019 → T020
```

### Phase 4 and Phase 5

```
# Can run in parallel after Phase 3:
Phase 4 (T021–T024) || Phase 5 (T025–T028)
```

---

## Implementation Strategy

### MVP (Phase 1 + 2 + 3 only)

1. Phase 1: Confirm baseline green
2. Phase 2: `isMultiDay` + `enumeratePastDays` with tests
3. Phase 3: `partitionByDay` + `buildGapRanges` + `fetchHistoryMultiDayCached` + routing
4. **STOP and VALIDATE**: `go test -race ./cmd/...` — US1–US4 all green
5. Multi-day fetch caching is fully functional for the common case

### Full Delivery

After MVP: add Phase 4 (US5) and Phase 5 (US6) in parallel, then Phase 6 polish.

---

## Notes

- All changes are in `cmd/historycache.go` and `cmd/historycache_test.go` only
- `internal/cache` is **unchanged** — `LoadStable`/`SaveStable` already exist from 006
- `internal/slack` is **unchanged** — `FetchHistory`, `Message`, `DateRange` used as-is
- No new CLI flags — the existing `--no-cache` and `--limit` flags cover all new behaviour
- `historyFetchFunc` (already defined in `historycache.go`) is the injectable type for tests
