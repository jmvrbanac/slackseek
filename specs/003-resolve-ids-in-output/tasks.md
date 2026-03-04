# Tasks: Resolve User IDs and Channel IDs in Output

**Input**: Design documents from `/specs/003-resolve-ids-in-output/`
**Branch**: `003-resolve-ids-in-output`

**Organization**: Tasks are grouped by user story to enable independent implementation and testing.

> **CONSTITUTION NOTE (Principle II — NON-NEGOTIABLE)**: Tests MUST be written and confirmed
> failing BEFORE the implementation tasks that make them pass. This is enforced in every phase below.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)

---

## Phase 1: Setup

No project bootstrapping required — this is an existing Go project with all infrastructure in place.

---

## Phase 2: Foundational — Resolver Type

**Purpose**: The `slack.Resolver` struct is the core building block required by both US1
(user names) and US2 (channel names). It must be complete before either user story can proceed.

**⚠️ CRITICAL**: US1 and US2 cannot start until this phase is complete.

- [X] T001 Write unit tests for `NewResolver`, `UserDisplayName`, and `ChannelName` in `internal/slack/resolver_test.go`. Cover: normal lookup, real name preferred over display name, display name fallback when real name empty, both names empty falls back to raw ID, channel lookup, unknown channel ID returns raw ID, nil/empty slices produce a functional (non-panicking) resolver.

- [X] T002 Implement `Resolver` in `internal/slack/resolver.go`. Struct has two `map[string]string` fields (`users`, `channels`). `NewResolver(users []User, channels []Channel) *Resolver` builds maps in O(n); prefers `RealName` then `DisplayName` for users. `UserDisplayName(id string) string` returns name or `id`. `ChannelName(id string) string` returns name or `id`. No errors returned from any method. Run `go test ./internal/slack/...` — all T001 tests must pass.

**Checkpoint**: `go test -race ./internal/slack/...` passes. Resolver is ready for use in output and cmd layers.

---

## Phase 3: User Story 1 — User IDs Replaced by Display Names (Priority: P1) 🎯 MVP

**Goal**: All text/table/JSON output for `history`, `messages`, and `search` commands shows
human-readable user display names instead of raw `U…` Slack user IDs.

**Independent Test**: Run `slackseek history #general --format table` with a populated user cache.
Verify the "USER" column contains a display name (e.g. `alice`) instead of `U01234567`. Also run
`--format json` and confirm the output contains both `"user_id"` and `"user_display_name"` keys.

### Tests for User Story 1 ⚠️ WRITE THESE FIRST — CONFIRM THEY FAIL

- [X] T003 [US1] Add failing tests for user display name resolution to `internal/output/format_test.go`. Add tests: (a) `PrintMessages` text output uses display name not user ID when resolver is provided; (b) `PrintMessages` table output "USER" column contains display name; (c) `PrintMessages` JSON output includes `"user_display_name"` field; (d) `PrintMessages` with nil resolver falls back to raw user ID in all formats; (e) same four cases for `PrintSearchResults`. Tests should fail because `PrintMessages`/`PrintSearchResults` do not yet accept a resolver parameter.

### Implementation for User Story 1

- [X] T004 [US1] Add `UserDisplayName string \`json:"user_display_name"\`` field to `messageJSON` struct in `internal/output/format.go`. Add the same field to `searchResultJSON`. Field is an empty string when resolver is nil.

- [X] T005 [US1] Update `toMessageJSON` in `internal/output/format.go` to accept a `*slack.Resolver` parameter. When resolver is non-nil, populate `UserDisplayName` using `resolver.UserDisplayName(m.UserID)`. When nil, leave it empty.

- [X] T006 [US1] Update `PrintMessages` signature in `internal/output/format.go` to accept a `*slack.Resolver` parameter (after the `messages` slice). In the `FormatTable` case, change the "User" row value from `m.UserID` to `resolver.UserDisplayName(m.UserID)` (or `m.UserID` when resolver is nil). In the `default` (text) case, change the user field from `m.UserID` to the resolved name. Pass resolver through to `toMessageJSON`.

- [X] T007 [P] [US1] Update `toSearchResultJSON` and `PrintSearchResults` in `internal/output/format.go` identically to T005/T006 — accept `*slack.Resolver`, populate `UserDisplayName`, use resolved name in table "USER" column and text output. These changes are in the same file so coordinate with T006 but touch distinct functions.

- [X] T008 [US1] Add a `buildResolver(ctx context.Context, ws tokens.Workspace) *slack.Resolver` helper function in `cmd/root.go` (or a new `cmd/resolver.go`). It creates a `slack.NewClientWithCache` with the workspace credentials, calls `c.ListUsers(ctx)` and `c.ListChannels(ctx, nil, false)`, builds `slack.NewResolver(users, channels)`, and returns it. On any error it writes `"Warning: could not resolve IDs: <err>"` to stderr and returns nil (graceful degradation).

- [X] T009 [US1] Update `runHistoryE` in `cmd/history.go` to call `buildResolver(cmd.Context(), ws)` after the `runFn` call succeeds, then pass the resolver to `output.PrintMessages`. Update `addHistoryCmd`, `runHistoryE` and their test helpers in `cmd/history_test.go` to compile after the `PrintMessages` signature change (pass `nil` resolver in test stubs).

- [X] T010 [P] [US1] Update `runMessagesE` in `cmd/messages.go` identically to T009: call `buildResolver`, pass resolver to `output.PrintMessages`. Update `cmd/messages_test.go` to pass `nil` resolver in test stubs.

- [X] T011 [P] [US1] Update `runSearchE` in `cmd/search.go` identically to T009: call `buildResolver`, pass resolver to `output.PrintSearchResults`. Update `cmd/search_test.go` to pass `nil` resolver in test stubs. Read `cmd/search.go` first to understand its structure.

**Checkpoint**: `go test -race ./...` passes. Run `slackseek history #general --format table` — User
column shows display names. Run with `--format json` — output contains `user_display_name`. US1 is
fully functional and independently verified.

---

## Phase 4: User Story 2 — Channel IDs Replaced by Channel Names (Priority: P2)

**Goal**: Text/table/JSON output for the `messages` command (and JSON output for `history`) shows
resolved channel names instead of raw `C…` channel IDs.

**Independent Test**: Run `slackseek messages <user> --format table` with a populated channel cache.
Verify the channel column shows `general` not `C01234567`. Run with `--format json` and confirm
`channel_name` is populated.

### Tests for User Story 2 ⚠️ WRITE THESE FIRST — CONFIRM THEY FAIL

- [X] T012 [US2] Add failing tests for channel name resolution to `internal/output/format_test.go`. Add tests: (a) `PrintMessages` text output uses channel name not channel ID when resolver provided and `ChannelName` is empty on message; (b) `PrintMessages` table output "CHANNEL" column contains resolved name; (c) `PrintMessages` JSON output has `channel_name` populated; (d) when `Message.ChannelName` is already set (search context), the existing name is preserved and not overwritten by resolver; (e) nil resolver leaves `channel_name` empty in JSON (history context).

### Implementation for User Story 2

- [X] T013 [US2] Update `toMessageJSON` in `internal/output/format.go` to also resolve channel names: when `m.ChannelName` is empty and resolver is non-nil, set `ChannelName` to `resolver.ChannelName(m.ChannelID)`. When `m.ChannelName` is already non-empty (e.g. search results), preserve the existing value. This populates `channel_name` in JSON output for history/messages commands.

- [X] T014 [US2] Update `PrintMessages` in `internal/output/format.go` to use resolved channel names in text and table output. The `messages` command table currently shows `m.UserID` in the User column (addressed in US1) and needs a Channel column showing the resolved name. Add "Channel" to the table header and row data; use `resolver.ChannelName(m.ChannelID)` when `m.ChannelName` is empty, otherwise use the existing `ChannelName`. In the text (default) format, include the resolved channel name.

**Checkpoint**: `go test -race ./...` passes. Run `slackseek messages <user> --format table` — Channel
column shows `general` not `C01234567`. Run `--format json` — `channel_name` is populated. US2 is
fully functional and independently verified.

---

## Phase 5: User Story 3 — Graceful Degradation (Priority: P3)

**Goal**: `--no-cache` and resolver failures never break output; raw IDs are shown as fallback
with no panics and no extra API calls.

**Independent Test**: Run `slackseek history #general --no-cache`. Output succeeds; User column
shows raw `U…` IDs. No panic. No extra API calls beyond the history fetch itself.

### Tests for User Story 3 ⚠️ WRITE THESE FIRST — CONFIRM THEY FAIL (or verify they already pass from US1/US2)

- [X] T015 [US3] Verify/add tests in `cmd/history_test.go` for the nil-resolver path: when `buildResolver` would return nil (simulate by passing nil in test), output still succeeds and contains the raw user ID `U123` in the User column. Add equivalent tests in `cmd/messages_test.go` and `cmd/search_test.go`. If these already pass from T003, mark as confirmed-passing.

- [X] T016 [P] [US3] Add a test in `internal/output/format_test.go` for `NewResolver` called with nil slices — verify it does not panic, and that `UserDisplayName`/`ChannelName` return the raw ID. Add a test that `PrintMessages` with a `Resolver` built from empty slices returns raw IDs (not empty strings).

### Implementation for User Story 3

- [X] T017 [US3] Review `buildResolver` in `cmd/root.go` (or `cmd/resolver.go`) created in T008. Confirm that when `--no-cache` is set (i.e. `flagNoCache` is true on the root command), `buildResolver` returns `nil` immediately without calling `ListUsers` or `ListChannels`. Read `cmd/root.go` to locate the `flagNoCache` global. If `--no-cache` suppression is not yet implemented, add a guard: `if flagNoCache { return nil }`.

**Checkpoint**: `go test -race ./...` passes. `slackseek history #general --no-cache` outputs raw IDs
without panics. US3 is fully functional.

---

## Phase 6: Post-Implementation Refinements

### Name preference correction

- [X] T021 Swap name-resolution preference in `NewResolver`: prefer `RealName` over `DisplayName` (real names like `Alice Smith` are more human-readable than Slack handle-style display names like `alice.smith`). Update resolver_test.go to assert `RealName` is preferred, and update format_test.go fixture expectations accordingly.

### Inline mention resolution

- [X] T022 Add `ResolveMentions(text string) string` method to `Resolver` in `internal/slack/resolver.go`. Handles three token types: (1) `<@USERID>` → `@name` (via `UserDisplayName`, falls back to `@USERID`); (2) `<!subteam^ID|label>` → `label`, `<!subteam^ID>` → `@[group]`; (3) `<!here>`, `<!channel>`, `<!everyone>` → `@here`, `@channel`, `@everyone`. Uses three compiled package-level regexps. Tests cover all token types.

- [X] T023 Rename `resolveMessageNames` helper in `internal/output/format.go` to `resolveMessageFields`, adding a `text string` return value. When resolver is non-nil, apply `resolver.ResolveMentions(m.Text)` to produce the resolved text. Update all call sites in `PrintMessages` (table and text format) to use the resolved text.

- [X] T024 Apply `resolver.ResolveMentions` to the `Text` field inside `toMessageJSON` so JSON output also has inline mentions resolved.

- [X] T025 Update `PrintSearchResults` table and text paths to use `resolveMessageFields(sr.Message, resolver)` for consistent mention resolution (replaces the inline `userDisplay`/`sr.Text` pattern with the shared helper).

**Checkpoint**: `go test -race ./...` passes. `golangci-lint run` passes. Messages containing `<@USERID>` tokens show resolved names in all output formats.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T018 Run `golangci-lint run` and fix any linting issues across all modified files (`internal/slack/resolver.go`, `internal/slack/resolver_test.go`, `internal/output/format.go`, `internal/output/format_test.go`, `cmd/history.go`, `cmd/messages.go`, `cmd/search.go`, `cmd/root.go` or `cmd/resolver.go`).

- [X] T019 [P] Run `GOOS=linux go build ./...` and `GOOS=darwin go build ./...` to confirm cross-platform build succeeds with no new CGO or platform-specific dependencies.

- [X] T020 [P] Run `go test -race ./...` one final time to confirm the full suite is green with no race conditions. Capture and fix any remaining failures.

---

## Phase 8: User Story 4 — User Group Resolution (Priority: P4)

**Goal**: `<!subteam^ID>` tokens without an embedded label resolve to `@handle` using a
cached user-groups list from the Slack `usergroups.list` API. Tokens with an embedded label
continue to use the label as before. `@[group]` appears only when the ID is unknown.

**Independent Test**: Run any message command against a channel containing
`<!subteam^KNOWN_ID>` with no embedded label. Verify output shows `@handle` rather than
`@[group]`. Confirm that `--no-cache` still produces `@[group]` (nil resolver path).

**⚠️ CRITICAL**: T026 (API tests) and T029 (resolver tests) MUST be written and confirmed
failing BEFORE their respective implementation tasks.

### Tests for User Story 4 ⚠️ WRITE THESE FIRST — CONFIRM THEY FAIL

- [X] T026 [US4] Write failing tests for `ListUserGroups` in a new file
  `internal/slack/usergroups_test.go`. Use a local `httptest.NewServer` to mock the
  `usergroups.list` response. Tests: (a) successful response returns a slice of `UserGroup`
  with correct `ID`, `Handle`, `Name` fields; (b) response with `"ok": false` returns an
  error; (c) empty `usergroups` array returns an empty slice without error. Tests should fail
  because `ListUserGroups` does not yet exist.

- [X] T029 [US4] Add failing resolver tests to `internal/slack/resolver_test.go` for group
  resolution. Tests: (a) `<!subteam^KNOWN_ID>` with no label resolves to `@handle` when
  group is in resolver; (b) `<!subteam^UNKNOWN_ID>` with no label falls back to `@[group]`;
  (c) `<!subteam^ID|@label>` still uses the embedded label even when group is in resolver
  (label wins); (d) `NewResolver` with nil groups slice does not panic. Tests should fail
  because `NewResolver` does not yet accept a groups parameter.

### Implementation for User Story 4

- [X] T027 [US4] Add `UserGroup` struct to `internal/slack/types.go`:
  `type UserGroup struct { ID string; Handle string; Name string }`. No other changes.

- [X] T028 [US4] Create `internal/slack/usergroups.go`. Implement:
  `listUserGroupsCached(ctx, client, cacheStore, workspaceKey) ([]UserGroup, error)` — calls
  `usergroups.list` API (param `include_disabled=false`), caches result under key
  `"user_groups"` using the existing `internal/cache` store pattern (same as channels/users).
  `ListUserGroups(ctx context.Context) ([]UserGroup, error)` method on `Client` — delegates
  to `listUserGroupsCached`. Parse `usergroups` JSON array from response. No pagination
  needed (Slack returns all groups in one call). Run `go test ./internal/slack/...` — T026
  tests must pass.

- [X] T030 [US4] Update `NewResolver` signature in `internal/slack/resolver.go` to accept a
  third parameter `groups []UserGroup`. Add `groups map[string]string` field to `Resolver`.
  Populate from the groups slice: key = `g.ID`, value = `g.Handle`. Nil slice is safe (empty
  map). Run `go test ./internal/slack/...` — T029 resolver tests must pass.

- [X] T031 [US4] Update `ResolveMentions` in `internal/slack/resolver.go`: in the subteam
  handler, after checking for an embedded label, look up `r.groups[id]` where `id` is the
  group ID extracted from the token. If found, return `"@" + handle`; otherwise return
  `"@[group]"`. The embedded label path is unchanged (label wins when present).

- [X] T032 [US4] Update `buildResolver` in `cmd/resolver.go` to call
  `c.ListUserGroups(ctx)` after fetching users and channels. On error, write
  `"Warning: could not resolve user groups: <err>"` to stderr and pass `nil` for groups
  (resolver still built with users and channels). Pass groups slice as third argument to
  `slack.NewResolver(users, channels, groups)`.

- [X] T033 [P] [US4] Update all existing `NewResolver(users, channels)` call sites to pass
  the groups argument. Affected files: `internal/output/format_test.go` (fixtureResolver and
  any direct NewResolver calls — pass `nil`), `internal/slack/resolver_test.go` (all
  `NewResolver(…)` calls — pass `nil` for the groups arg). Compile-check with
  `go build ./...`.

**Checkpoint**: `go test -race ./...` passes. `<!subteam^ID>` tokens with no embedded label
resolve to `@handle` when the group is cached. `--no-cache` still produces `@[group]`.
US4 is fully functional and independently verified.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 2 (Foundational)**: No dependencies — start immediately
- **Phase 3 (US1)**: Depends on Phase 2 completion (Resolver must exist)
- **Phase 4 (US2)**: Depends on Phase 3 completion (reuses output/cmd changes)
- **Phase 5 (US3)**: Depends on Phase 3 completion; some tests may already pass
- **Phase 6 (Polish)**: Depends on all user story phases
- **Phase 8 (US4)**: Depends on Phase 7 completion (builds on existing Resolver/buildResolver)

### User Story Dependencies

- **US1 (P1)**: Requires Foundational (T001–T002). Can start after Phase 2.
- **US2 (P2)**: Requires US1 completion (shares `PrintMessages` signature change).
- **US3 (P3)**: Requires US1 completion (verifies nil-resolver path).
- **US4 (P4)**: Requires Phase 7 completion. T026–T027 can start in parallel; T028 requires T026; T029 requires T027; T030 requires T029; T031 requires T030; T032 requires T030; T033 requires T030 (compile-fix).

### Within Each Phase

- T001 before T002 (tests first, then implement)
- T003 before T004–T007 (failing tests before implementation)
- T004 before T005 and T006 (struct field before usage)
- T005 before T006 (helper before callers)
- T008 before T009–T011 (`buildResolver` helper before cmds use it)
- T012 before T013–T014 (failing tests before implementation)
- T015–T016 before T017 (failing tests before implementation)
- T026 before T028; T029 before T030–T033; T027 before T029; T030 before T031, T032, T033

### Parallel Opportunities

- T006 and T007 touch distinct functions in the same file — coordinate but can be drafted together
- T009, T010, T011 touch three different `cmd/` files — can proceed in parallel after T008
- T018, T019, T020 (Polish) can run concurrently
- T026 and T027 (US4 setup) touch different files — can run in parallel
- T031 and T032 and T033 all depend on T030 but touch different files — can run in parallel

---

## Parallel Execution Example: User Story 1

```text
After T008 (buildResolver) is complete, launch in parallel:
  T009 — update cmd/history.go + cmd/history_test.go
  T010 — update cmd/messages.go + cmd/messages_test.go
  T011 — update cmd/search.go + cmd/search_test.go
```

## Parallel Execution Example: User Story 4

```text
Start in parallel:
  T026 — write ListUserGroups API tests (internal/slack/usergroups_test.go)
  T027 — add UserGroup type to internal/slack/types.go

After T026 passes: T028 — implement ListUserGroups
After T027 passes: T029 — write resolver group tests

After T028 and T029 pass: T030 — update NewResolver signature

After T030 passes, launch in parallel:
  T031 — update ResolveMentions subteam path
  T032 — update buildResolver in cmd/resolver.go
  T033 — update all NewResolver call sites (compile-fix)
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 2: Foundational (T001–T002)
2. Complete Phase 3: US1 (T003–T011)
3. **STOP and VALIDATE**: `slackseek history #general` shows display names; `--format json` shows `user_display_name`
4. Demo/ship — channel IDs can follow in a second increment

### Incremental Delivery

1. Phase 2 → Phase 3 (US1): users resolved in all message commands
2. Phase 4 (US2): channels resolved in messages/history JSON
3. Phase 5 (US3): degradation verified; `--no-cache` safe
4. Phase 6 (Polish): lint + build checks; PR-ready
5. Phase 8 (US4): user groups cached and resolved in `<!subteam^ID>` tokens

---

## Notes

- **Test-first is mandatory** (Constitution Principle II). Never skip the "write failing tests" steps.
- `go test -race ./...` must pass after every phase — not just at the end.
- The `Resolver` is nil-safe at every call site; `if resolver != nil { … }` before any method call.
- `buildResolver` returns `nil` (not an error) on failure — output gracefully degrades to raw IDs.
- `MessageJSON.channel_name` is already present in the struct; this feature populates it for history context (previously empty).
- Do not add `user_display_name` to `channelJSON` or `userJSON` — those types already show names.
- `ListUserGroups` does not require pagination — Slack returns all user groups in a single call.
- T033 is a compile-fix only; it passes `nil` for the new groups arg at existing call sites that do not need group resolution.
