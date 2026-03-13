# Tasks: Lazy Entity Cache (Fetch-on-Miss)

**Input**: Design documents from `/specs/008-lazy-entity-cache/`
**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓, contracts/ ✓

**Tests**: Included per Constitution Principle II (Test-First is NON-NEGOTIABLE for this project).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: Which user story this task belongs to
- Exact file paths included in all descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Cross-cutting prerequisites that block multiple user stories.

- [x] T001 Add `tier4 *rateLimiter` field (90 calls/min) to `slack.Client` struct in `internal/slack/client.go` and initialize it in `NewClient`
- [x] T002 Update `client_test.go` in `internal/slack/client_test.go` to assert `tier4` is non-nil on a freshly constructed client

**Checkpoint**: `slack.Client` has a Tier 4 rate limiter; all existing tests pass.

---

## Phase 2: Foundational (Resolver Callback Infrastructure)

**Purpose**: `NewResolverWithFetch` enables all on-miss user stories (US2, US3, US4). Must be complete before Phases 5–7.

⚠️ **CRITICAL**: No fetch-on-miss user story work can begin until this phase is complete.

- [x] T003 Write failing unit tests for `NewResolverWithFetch` in `internal/slack/resolver_test.go`: verify fetchUser callback is invoked on user ID miss, result is cached in-memory, and callback is NOT invoked on a second reference to the same ID
- [x] T004 Write failing unit tests for the `groupRefreshed` guard in `internal/slack/resolver_test.go`: verify `fetchGroups` is called at most once per invocation regardless of how many group IDs are missing
- [x] T005 Add `fetchUser func(string) (string, error)`, `fetchChannel func(string) (string, error)`, `fetchGroups func() ([]UserGroup, error)`, and `groupRefreshed bool` fields to the `Resolver` struct in `internal/slack/resolver.go`
- [x] T006 Implement `NewResolverWithFetch` constructor in `internal/slack/resolver.go` accepting all callback parameters; update `NewResolver` to call it with all nil callbacks (preserving existing call sites)
- [x] T007 Update `UserDisplayName` in `internal/slack/resolver.go` to invoke `fetchUser` on miss, update `r.users[id]`, and return the resolved name (fall back to raw ID on callback error or nil callback)
- [x] T008 Update `ChannelName` in `internal/slack/resolver.go` to invoke `fetchChannel` on miss, update `r.channels[id]`, and return the resolved name (fall back to raw ID on callback error or nil callback)
- [x] T009 Update group resolution in `ResolveMentions` in `internal/slack/resolver.go` to invoke `fetchGroups` on first group miss (gated by `groupRefreshed`), rebuild `r.groups` from the returned slice, set `r.groupRefreshed = true`, and retry resolution

**Checkpoint**: All T003–T004 tests pass. `NewResolver` call sites are unchanged. `go test -race ./internal/slack/...` passes.

---

## Phase 3: User Stories 1 & 6 — Warm Cache Never Expires / Cold Start Unchanged (Priority: P1) 🎯 MVP

**Goal**: Entity caches load without any TTL check; cold start (no cache files) behavior is identical to today.

**Independent Test**: Populate entity cache files; advance mtime beyond 24 h; run any command referencing only cached IDs — verify zero list API calls and full name resolution. Also verify that removing cache files triggers a fresh full-list fetch.

- [x] T010 Write failing unit tests in `internal/slack/users_test.go` for `listUsersCached`: assert that a cache file older than 24 h is returned as a hit (no fall-through to `listFn`)
- [x] T011 [P] Write failing unit tests in `internal/slack/channels_test.go` for `listChannelsCached`: same TTL-bypass assertion
- [x] T012 [P] Write failing unit tests in `internal/slack/usergroups_test.go` for `listUserGroupsCached`: same TTL-bypass assertion
- [x] T013 [US1] Switch `store.Load` to `store.LoadStable` in `listUsersCached` in `internal/slack/users.go`
- [x] T014 [P] [US1] Switch `store.Load` to `store.LoadStable` in `listChannelsCached` in `internal/slack/channels.go`
- [x] T015 [P] [US1] Switch `store.Load` to `store.LoadStable` in `listUserGroupsCached` in `internal/slack/usergroups.go`

**Checkpoint**: T010–T012 tests pass. Existing cold-start tests continue to pass. `go test -race ./internal/slack/...` passes.

---

## Phase 4: User Story 2 — New User Resolves Transparently on First Miss (Priority: P1)

**Goal**: When a user ID is absent from cache, a single targeted `users.info` call is made; the result is shown in output and merged into the cache file.

**Independent Test**: Construct a `Client` with an injected `users.info` stub; call `FetchUser` for an unknown ID; verify the stub is called once, the returned `User` matches, and `LoadStable("users")` on the same cache key now includes the new entry.

- [x] T016 [US2] Write failing unit tests for `Client.FetchUser` in `internal/slack/users_test.go`: inject a mock `GetUserInfoContext`, verify single API call, cache merge, and graceful error handling (API error → returns error, cache unchanged)
- [x] T017 [US2] Implement `mergeUser(store *cache.Store, key string, u User) error` helper in `internal/slack/users.go`: load current `users.json` via `LoadStable`, replace-or-append the entry for `u.ID`, write back via `Save`
- [x] T018 [US2] Implement `(*Client).FetchUser(ctx context.Context, id string) (User, error)` in `internal/slack/users.go`: call `users.info` via `tier4` limiter, call `mergeUser` on success (non-fatal failure), return the `User`
- [x] T019 [US2] Add `fetchUser` closure to `buildResolver` in `cmd/resolver.go` and pass it to `NewResolverWithFetch`: closure captures `ctx` and `c`, calls `c.FetchUser(ctx, id)`, returns the display name string

**Checkpoint**: T016 tests pass. A command referencing a new user ID resolves the name in the same invocation and the cache file contains the new entry. `go test -race ./...` passes.

---

## Phase 5: User Story 3 — New Channel Resolves Transparently on First Miss (Priority: P2)

**Goal**: When a channel ID is absent from cache, a single targeted `conversations.info` call is made; the result is shown in output and merged into the cache file.

**Independent Test**: Same pattern as Phase 4 but for channels: inject `GetConversationInfoContext` stub, verify single call, cache merge, graceful error.

- [x] T020 [P] [US3] Write failing unit tests for `Client.FetchChannel` in `internal/slack/channels_test.go`: inject mock `GetConversationInfoContext`, verify single API call, cache merge, and error handling
- [x] T021 [P] [US3] Implement `mergeChannel(store *cache.Store, key string, ch Channel) error` helper in `internal/slack/channels.go`: same load-replace-append-save pattern as `mergeUser`
- [x] T022 [P] [US3] Implement `(*Client).FetchChannel(ctx context.Context, id string) (Channel, error)` in `internal/slack/channels.go`: call `conversations.info` via `tier3` limiter, call `mergeChannel` on success, return the `Channel`
- [x] T023 [US3] Add `fetchChannel` closure to `buildResolver` in `cmd/resolver.go` and pass it to `NewResolverWithFetch`: closure captures `ctx` and `c`, calls `c.FetchChannel(ctx, id)`, returns the channel name string

**Checkpoint**: T020 tests pass. A command referencing a new channel ID resolves the name in the same invocation. `go test -race ./...` passes.

---

## Phase 6: User Story 4 — User Group Miss Triggers Full Group Refresh (Priority: P3)

**Goal**: When a group ID is absent from cache, `usergroups.list` is called once, the cache is updated, and all group mentions in the current invocation resolve correctly.

**Independent Test**: In `resolver_test.go`, supply a `fetchGroups` stub returning a new group; verify it is called exactly once for two group misses in the same `ResolveMentions` call, and both group mentions resolve.

- [ ] T024 [US4] Implement `(*Client).ForceRefreshUserGroups(ctx context.Context) ([]UserGroup, error)` in `internal/slack/usergroups.go`: calls `listUserGroupsCached` with a `nil` store (bypasses LoadStable, hits API directly), then saves the result to cache via `c.store.Save` if store is non-nil
- [ ] T025 [US4] Write failing unit tests for `ForceRefreshUserGroups` in `internal/slack/usergroups_test.go`: verify the list API is called even when a valid cache file exists, and the cache file is updated with the fresh data
- [ ] T026 [US4] Add `fetchGroups` closure to `buildResolver` in `cmd/resolver.go` and pass it to `NewResolverWithFetch`: closure captures `ctx` and `c`, calls `c.ForceRefreshUserGroups(ctx)`, returns the `[]UserGroup` slice

**Checkpoint**: T004 and T025 tests pass. A command with group mentions absent from cache resolves all handles with a single `usergroups.list` call. `go test -race ./...` passes.

---

## Phase 7: User Story 5 — Force Refresh with --refresh-cache (Priority: P2)

**Goal**: Confirm `--refresh-cache` still triggers a full entity list re-fetch after the LoadStable changes. No new production code — only verification.

**Independent Test**: Run any command with `--refresh-cache`; verify all three entity list API calls are made and cache files are replaced.

- [ ] T027 [US5] Write or extend tests in `cmd/resolver_test.go` to verify that when `flagRefreshCache` is true, `buildCacheStore` clears the workspace cache dir, and the subsequent `listUsersCached` / `listChannelsCached` / `listUserGroupsCached` calls hit the API (cold start after clear)
- [ ] T028 [US5] Confirm `buildCacheStore` in `cmd/root.go` requires no code change: `store.Clear(key)` before resolver construction already handles the force-refresh path; add a comment documenting that `--refresh-cache` triggers cold-start for entity caches by clearing files

**Checkpoint**: `--refresh-cache` produces a full entity re-fetch. Existing flag exclusion tests (`--refresh-cache` + `--no-cache`) still pass.

---

## Phase 8: Polish & Cross-Cutting Concerns

- [ ] T029 Update the `--cache-ttl` flag description string in `cmd/root.go` to note it no longer controls entity (user/channel/group) cache expiry; controls history cache only
- [ ] T030 [P] Update `specs/008-lazy-entity-cache/contracts/cli-behavior.md` to mark as implemented if any observable behavior differs from the contract
- [ ] T031 Run `golangci-lint run` and resolve any `funlen`, `cyclop`, or other violations introduced by new functions
- [ ] T032 Run `go test -race ./...` and confirm zero failures and zero races across the full test suite
- [ ] T033 [P] Run `GOOS=linux go build ./...` and `GOOS=darwin go build ./...` to confirm cross-platform build passes

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Foundational)**: No dependencies on Phase 1 — can start immediately in parallel
- **Phase 3 (US1+US6)**: No dependencies on Phase 2 — can start immediately in parallel with Phase 2
- **Phase 4 (US2)**: Depends on Phase 1 (tier4 limiter) and Phase 2 (NewResolverWithFetch)
- **Phase 5 (US3)**: Depends on Phase 2; T020–T022 are independent of Phase 4; T023 (cmd/resolver.go) depends on T019 (same file, sequential)
- **Phase 6 (US4)**: Depends on Phase 2; T026 (cmd/resolver.go) depends on T023
- **Phase 7 (US5)**: Depends on Phase 3 (LoadStable switch) being complete
- **Phase 8 (Polish)**: Depends on all desired stories complete

### User Story Dependencies

- **US1+US6 (Phase 3)**: Independent — only needs LoadStable switch, no callbacks
- **US2 (Phase 4)**: Needs Phase 1 + Phase 2 complete
- **US3 (Phase 5)**: T020–T022 independent of US2; T023 sequential after T019 (same file)
- **US4 (Phase 6)**: T026 sequential after T023 (same file)
- **US5 (Phase 7)**: Independent of US2/US3/US4

### Critical Sequential Chain

```
Phase 1 + Phase 2 → Phase 4 (US2) → Phase 5 T023 → Phase 6 T026
```

Everything else in Phases 3, 5 (T020–T022), and 7 can run in parallel with the above chain.

---

## Parallel Opportunities

### Phases 1, 2, 3 can all start simultaneously:

```
Task: T001–T002  (Phase 1: rate limiter)
Task: T003–T009  (Phase 2: Resolver infrastructure)
Task: T010–T015  (Phase 3: LoadStable switch)
```

### Within Phase 5 (US3), implementation tasks parallel with Phase 4:

```
Task: T020 Write failing FetchChannel tests     [different file from T016]
Task: T021 Implement mergeChannel helper         [different file from T017]
Task: T022 Implement FetchChannel               [different file from T018]
```
*(T023 wiring must follow T019)*

### Phase 8 polish tasks T029, T031, T032, T033 can run together after implementation complete.

---

## Implementation Strategy

### MVP First (User Stories 1 & 6 — zero API calls on warm cache)

1. Complete Phase 1 and Phase 3 (LoadStable switch)
2. **STOP and VALIDATE**: Run commands against a populated cache with artificially aged files; confirm no list API calls.
3. This alone eliminates the daily re-fetch and is a shippable improvement.

### Incremental Delivery

1. Phase 1 + Phase 2 + Phase 3 → warm cache never expires (MVP)
2. Add Phase 4 → new users resolve inline
3. Add Phase 5 → new channels resolve inline
4. Add Phase 6 → group misses resolve inline
5. Phase 7 + Phase 8 → verification and polish

---

## Notes

- Constitution Principle II (Test-First) is enforced: failing tests are written **before** each implementation step.
- All new functions must be ≤ 40 lines (Principle I); `mergeUser` and `mergeChannel` are good candidates to stay short by delegating to a shared `mergeEntity` helper if both grow beyond ~20 lines.
- Cache write failures in `FetchUser`/`FetchChannel`/`ForceRefreshUserGroups` are **non-fatal** — print to stderr, return the entity value anyway.
- `[P]` tasks touch different files and have no incomplete task dependencies.
- `cmd/resolver.go` wiring tasks (T019, T023, T026) are sequential because they all modify `buildResolver` in the same file.
