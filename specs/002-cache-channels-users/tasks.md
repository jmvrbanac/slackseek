# Tasks: Channel and User List Caching

**Input**: Design documents from `/specs/002-cache-channels-users/`
**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓, contracts/cli-schema.md ✓

**Tests**: Test tasks are MANDATORY per Constitution Principle II (Test-First NON-NEGOTIABLE).
Write and confirm tests FAIL before writing implementation. `go test -race ./...` must pass
after each phase.

**Organization**: Tasks are grouped by user story to enable independent implementation
and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no shared dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)
- Exact file paths are included in every description

---

## Phase 1: Setup

**Purpose**: Create the new package skeleton so later tasks have a home.

- [X] T001 Create `internal/cache/doc.go` with package comment: `// Package cache provides a file-backed, TTL-based cache for Slack entity lists (channels and users). Each entry is stored as a JSON file under os.UserCacheDir()/slackseek/{workspaceKey}/.`

---

## Phase 2: Foundational — `internal/cache` Package

**Purpose**: Build the `internal/cache` package that all user stories depend on.
No user story work can begin until T002–T004 are complete.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T002 Write unit tests for `Store` and `WorkspaceKey` in `internal/cache/store_test.go`. Tests MUST cover: `WorkspaceKey` is deterministic and returns 16 lowercase hex chars; `Load` returns miss when file absent; `Load` returns hit when file is fresh; `Load` returns miss when file's ModTime is older than TTL (use `os.Chtimes` to backdate the file); `Load` returns miss (not error) when file contains invalid JSON; `Save` creates the workspace subdirectory if absent; `Save` writes a temp file then renames atomically (file always complete); `Save` returns nil and logs nothing when directory is unwritable (warn-only); `Clear` removes the workspace subdirectory; `ClearAll` removes the entire base directory.
- [X] T003 Implement `internal/cache/store.go`. Define `Store struct { dir string; ttl time.Duration }`. Implement: `NewStore(dir string, ttl time.Duration) *Store`; `WorkspaceKey(workspaceURL string) string` (returns `fmt.Sprintf("%x", sha256.Sum256([]byte(workspaceURL)))[:16]`); `Load(key, kind string) ([]byte, bool, error)` — returns `(nil, false, nil)` on `os.IsNotExist`; checks `time.Since(info.ModTime()) > s.ttl` for staleness; returns `(nil, false, nil)` on JSON unmarshal errors; `Save(key, kind string, data []byte) error` — creates `{dir}/{key}/` with `os.MkdirAll`; writes to `*.tmp` then renames for atomicity; on save error prints `"Warning: could not write cache: %v"` to stderr and returns nil; `Clear(key string) error` — calls `os.RemoveAll("{dir}/{key}")`; `ClearAll() error` — calls `os.RemoveAll(s.dir)`.
- [X] T004 [P] Add `json` struct tags to all fields of `Channel` and `User` in `internal/slack/types.go`. Tags must match the lowercase snake_case keys documented in `data-model.md`: `Channel` fields → `id`, `name`, `type`, `memberCount`, `topic`, `isArchived`; `User` fields → `id`, `displayName`, `realName`, `email`, `isBot`, `isDeleted`.

**Checkpoint**: `go test -race ./internal/cache/...` passes. Foundation ready.

---

## Phase 3: User Story 1 — Transparent Cache Hit (Priority: P1) 🎯 MVP

**Goal**: Any command that resolves channel or user names uses the on-disk cache on
second and subsequent invocations, producing zero `conversations.list` or `users.list`
API calls within the TTL window.

**Independent Test**: Run `slackseek channels list` twice against a real workspace.
First run writes `~/.cache/slackseek/{key}/channels.json`. Second run (within TTL)
completes in < 1 s with no API call for the list. Confirm via file mtime.

> **NOTE: Write T005–T006 tests FIRST and confirm they fail to compile before T007.**

- [X] T005 [P] [US1] Write tests for cache-aware `ListChannels` in `internal/slack/channels_test.go`. Add four sub-tests: (1) nil store → behaviour identical to today (API called, no cache); (2) store + fresh cache file present → API NOT called, cached data returned; (3) store + no cache file → API called, result marshalled and saved to cache; (4) store + stale cache file → API called, cache overwritten. Use the existing `listFn` injection pattern; supply a mock `*cache.Store` backed by `t.TempDir()`.
- [X] T006 [P] [US1] Write tests for cache-aware `ListUsers` in `internal/slack/users_test.go`. Mirror the four sub-tests from T005 but for the `users.list` code path. Use `t.TempDir()` for store isolation.
- [X] T007 [US1] Extend `Client` in `internal/slack/client.go`: add unexported fields `store *cache.Store` and `cacheKey string`; add `NewClientWithCache(token, cookie string, httpClient *http.Client, store *cache.Store, cacheKey string) *Client`; keep existing `NewClient` unchanged (it sets `store: nil`).
- [X] T008 [US1] Add cache check/write to `ListChannels` in `internal/slack/channels.go`. At the top of the method: if `c.store != nil && c.cacheKey != ""`, call `c.store.Load(c.cacheKey, "channels")`; on hit, `json.Unmarshal` into `[]Channel` and return. On miss, proceed with paginated fetch; on success, `json.Marshal` the result and call `c.store.Save(c.cacheKey, "channels", data)` — ignore save errors (already warns internally).
- [X] T009 [US1] Add cache check/write to `ListUsers` in `internal/slack/users.go`. Mirror the pattern from T008 but use kind `"users"` and marshal `[]User`.
- [X] T010 [US1] Add cache infrastructure to `cmd/root.go`: declare package-level flag vars `flagCacheTTL time.Duration`, `flagRefreshCache bool`, `flagNoCache bool`; register them as persistent flags with defaults `24h`, `false`, `false`; in `PersistentPreRunE` add validation: (a) negative TTL → `"invalid --cache-ttl: duration must not be negative"`; (b) `--refresh-cache` AND `--no-cache` both set → `"--refresh-cache and --no-cache are mutually exclusive"`; add helper `buildCacheStore(ws tokens.Workspace) *cache.Store` — returns nil when `flagNoCache` is true OR `flagCacheTTL == 0`; otherwise constructs `cache.NewStore(filepath.Join(userCacheDir, "slackseek"), flagCacheTTL)` where `userCacheDir` comes from `os.UserCacheDir()` (warn to stderr and return nil on error); when `flagRefreshCache` is true, calls `store.Clear(cache.WorkspaceKey(ws.URL))` before returning the store.
- [X] T011 [P] [US1] Update `defaultRunChannels` in `cmd/channels.go`: replace `slack.NewClient(...)` with `slack.NewClientWithCache(..., buildCacheStore(workspace), cache.WorkspaceKey(workspace.URL))`.
- [X] T012 [P] [US1] Update `defaultRunUsers` in `cmd/users.go`: same substitution as T011.
- [X] T013 [P] [US1] Update `defaultRunHistory` in `cmd/history.go`: same substitution as T011.
- [X] T014 [P] [US1] Update `defaultRunMessages` in `cmd/messages.go`: same substitution as T011.
- [X] T015 [P] [US1] Update `defaultRunSearch` in `cmd/search.go`: same substitution as T011.

**Checkpoint**: `go test -race ./...` passes. `slackseek channels list` run twice produces a cache file and the second run is noticeably faster.

---

## Phase 4: User Story 2 — Force Refresh When Workspace Changes (Priority: P2)

**Goal**: `--refresh-cache` forces a new API fetch and rewrites the cache. `--no-cache`
bypasses cache entirely. Both flags are validated for mutual exclusion.

**Independent Test**: Write a fresh cache file; run any command with `--refresh-cache`;
confirm the cache file's `ModTime` is updated. Run with `--no-cache`; confirm no cache
file is created.

> **NOTE: Write T016 tests FIRST. The production code for US2 is entirely in T010
> (already part of US1). T016 verifies the flag validation and bypass paths.**

- [X] T016 [P] [US2] Write tests for cache control flag validation in `cmd/root_test.go`. Cover: `--cache-ttl -1h` returns error containing "must not be negative"; `--refresh-cache --no-cache` together returns error containing "mutually exclusive"; `--cache-ttl 0` is accepted without error; `--no-cache` alone is accepted without error; `--refresh-cache` alone is accepted without error.

**Checkpoint**: `go test -race ./cmd/...` passes. Both flags work correctly.

---

## Phase 5: User Story 3 — Cache Clear Command (Priority: P3)

**Goal**: `slackseek cache clear` deletes cached files for the current workspace.
`slackseek cache clear --all` deletes all workspace caches.

**Independent Test**: Populate cache files, run `slackseek cache clear`, verify files
are gone and command exits 0. Run again (nothing to clear) and verify command still
exits 0.

> **NOTE: Write T017 tests FIRST.**

- [X] T017 [P] [US3] Write unit tests for `cache clear` command in `cmd/cache_test.go`. Use the injectable `runFn` pattern consistent with other command tests. Cover: workspace clear removes `{key}/channels.json` and `{key}/users.json`; outputs `"Cache cleared for workspace … (N files removed)."`; no-op when workspace dir absent outputs `"No cache found for workspace …"`; `--all` calls `ClearAll` and reports total file count; command returns non-zero on unexpected I/O error.
- [X] T018 [US3] Implement `cmd/cache.go`: define `cacheRunFunc` as `func(ctx context.Context, workspace tokens.Workspace, all bool) error`; implement `addCacheCmd(parent, extractFn, runFn)`, `newCacheClearCmd(extractFn, runFn)`, `runCacheClearE`; implement `defaultRunCacheClear` — constructs a `cache.Store` from `os.UserCacheDir()`; when `--all`, calls `store.ClearAll()` and prints total; otherwise, calls `store.Clear(cache.WorkspaceKey(ws.URL))` and prints workspace name and file count; register via `init()`.

**Checkpoint**: `go test -race ./cmd/...` passes. `slackseek cache clear` works end-to-end.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T019 [P] Run `go vet ./...` and resolve any reported issues
- [X] T020 [P] Run `golangci-lint run` and resolve any lint issues (function length, error wrapping, unused imports)
- [X] T021 [P] Run `go test -race ./...` and confirm zero failures and zero race conditions
- [X] T022 [P] Run `GOOS=linux go build ./...` and `GOOS=darwin go build ./...` and confirm both succeed with no errors

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — blocks all user stories
- **US1 (Phase 3)**: Depends on Phase 2 — core feature; builds client cache integration
- **US2 (Phase 4)**: Depends on Phase 2 and T010 from US1 — tests only (production code in T010)
- **US3 (Phase 5)**: Depends on Phase 2 only — independent of US1/US2 except for `cache.Store`
- **Polish (Phase 6)**: Depends on all story phases complete

### User Story Dependencies

- **US1 (P1)**: Requires Phase 2 complete. No other story dependency. This is the MVP.
- **US2 (P2)**: Requires T010 (adds the flags + `buildCacheStore`). US2 is tested-only; implementation is woven into US1.
- **US3 (P3)**: Requires Phase 2 complete (`cache.Store`). Independent of US1 and US2.

### Within Each Phase

```
Phase 2:  T002 → T003  (tests before implementation; T004 [P] any time in phase)
Phase 3:  T005 [P], T006 [P]   (write tests first — expect compile failure)
          → T007                (add store field to Client)
          → T008, T009          (wire cache into ListChannels/ListUsers)
          → T010                (add flags + buildCacheStore)
          → T011–T015 [P]       (update all defaultRun* functions; parallel)
Phase 4:  T016 [P]              (flag validation tests — independent)
Phase 5:  T017 [P]              (write tests first)
          → T018                (implement cache clear command)
```

### Parallel Opportunities

All tasks labelled `[P]` within the same phase can run concurrently:
- **Phase 2**: T004 can run in parallel with T002–T003
- **Phase 3**: T005 and T006 (test writing) can be written in parallel; T011–T015 (cmd wiring) can all run in parallel after T007–T010
- **Phase 4**: T016 is independent and can run in parallel with Phase 3 after T010 exists
- **Phase 5**: T017 can run in parallel with Phase 3/4 after Phase 2 completes
- **Phase 6**: All polish tasks are parallel

---

## Parallel Example: User Story 1

```bash
# Step 1 — Write tests in parallel (both fail to compile until T007):
Task T005: "Write cache-aware ListChannels tests in internal/slack/channels_test.go"
Task T006: "Write cache-aware ListUsers tests in internal/slack/users_test.go"

# Step 2 — Implement client+cache integration (sequential):
Task T007: "Add store field and NewClientWithCache to internal/slack/client.go"
Task T008: "Add cache check/write to ListChannels in internal/slack/channels.go"
Task T009: "Add cache check/write to ListUsers in internal/slack/users.go"
Task T010: "Add cache flags and buildCacheStore helper to cmd/root.go"

# Step 3 — Wire all defaultRun* functions in parallel:
Task T011: "Update defaultRunChannels in cmd/channels.go"
Task T012: "Update defaultRunUsers in cmd/users.go"
Task T013: "Update defaultRunHistory in cmd/history.go"
Task T014: "Update defaultRunMessages in cmd/messages.go"
Task T015: "Update defaultRunSearch in cmd/search.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001)
2. Complete Phase 2: Foundational (T002–T004)
3. Complete Phase 3: US1 (T005–T015)
4. **STOP and VALIDATE**: Run `slackseek channels list` twice; confirm cache file written and second run is fast
5. Run `go test -race ./...` — all tests must pass

### Incremental Delivery

1. Phases 1–2 + US1 → **MVP**: cache transparently speeds up all commands
2. Add US2 (T016 tests) → validate flag bypass behavior
3. Add US3 (T017–T018) → `cache clear` command usable
4. Phase 6 Polish → ready to merge

### Task Count Summary

| Phase | Tasks | Parallel opportunities |
|-------|-------|----------------------|
| 1 Setup | 1 | 0 |
| 2 Foundational | 3 | 1 (T004) |
| 3 US1 | 11 | 7 (T005, T006, T011–T015) |
| 4 US2 | 1 | 1 (T016) |
| 5 US3 | 2 | 1 (T017) |
| 6 Polish | 4 | 4 (all) |
| **Total** | **22** | **14** |

---

## Notes

- `[P]` tasks modify different files and have no dependency on each other
- `[Story]` label maps each task to a specific user story for traceability
- Constitution Principle II requires tests written BEFORE implementation — do not skip
- Use `t.TempDir()` for all cache store tests (automatic cleanup, no side effects)
- Use `os.Chtimes` to simulate stale cache entries without sleeping in tests
- `NewClient` in `internal/slack/client.go` is unchanged — no breaking change for existing tests
- `buildCacheStore` returning `nil` restores exact pre-feature behaviour (nil store = no cache)
- Commit after each checkpoint; stop at any checkpoint to validate the story independently
