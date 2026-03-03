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

- [X] T001 Write unit tests for `NewResolver`, `UserDisplayName`, and `ChannelName` in `internal/slack/resolver_test.go`. Cover: normal lookup, display name fallback to real name, both names empty falls back to raw ID, channel lookup, unknown channel ID returns raw ID, nil/empty slices produce a functional (non-panicking) resolver.

- [X] T002 Implement `Resolver` in `internal/slack/resolver.go`. Struct has two `map[string]string` fields (`users`, `channels`). `NewResolver(users []User, channels []Channel) *Resolver` builds maps in O(n); prefers `DisplayName` then `RealName` for users. `UserDisplayName(id string) string` returns name or `id`. `ChannelName(id string) string` returns name or `id`. No errors returned from any method. Run `go test ./internal/slack/...` — all T001 tests must pass.

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

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T018 Run `golangci-lint run` and fix any linting issues across all modified files (`internal/slack/resolver.go`, `internal/slack/resolver_test.go`, `internal/output/format.go`, `internal/output/format_test.go`, `cmd/history.go`, `cmd/messages.go`, `cmd/search.go`, `cmd/root.go` or `cmd/resolver.go`).

- [X] T019 [P] Run `GOOS=linux go build ./...` and `GOOS=darwin go build ./...` to confirm cross-platform build succeeds with no new CGO or platform-specific dependencies.

- [X] T020 [P] Run `go test -race ./...` one final time to confirm the full suite is green with no race conditions. Capture and fix any remaining failures.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 2 (Foundational)**: No dependencies — start immediately
- **Phase 3 (US1)**: Depends on Phase 2 completion (Resolver must exist)
- **Phase 4 (US2)**: Depends on Phase 3 completion (reuses output/cmd changes)
- **Phase 5 (US3)**: Depends on Phase 3 completion; some tests may already pass
- **Phase 6 (Polish)**: Depends on all user story phases

### User Story Dependencies

- **US1 (P1)**: Requires Foundational (T001–T002). Can start after Phase 2.
- **US2 (P2)**: Requires US1 completion (shares `PrintMessages` signature change).
- **US3 (P3)**: Requires US1 completion (verifies nil-resolver path).

### Within Each Phase

- T001 before T002 (tests first, then implement)
- T003 before T004–T007 (failing tests before implementation)
- T004 before T005 and T006 (struct field before usage)
- T005 before T006 (helper before callers)
- T008 before T009–T011 (`buildResolver` helper before cmds use it)
- T012 before T013–T014 (failing tests before implementation)
- T015–T016 before T017 (failing tests before implementation)

### Parallel Opportunities

- T006 and T007 touch distinct functions in the same file — coordinate but can be drafted together
- T009, T010, T011 touch three different `cmd/` files — can proceed in parallel after T008
- T018, T019, T020 (Polish) can run concurrently

---

## Parallel Execution Example: User Story 1

```text
After T008 (buildResolver) is complete, launch in parallel:
  T009 — update cmd/history.go + cmd/history_test.go
  T010 — update cmd/messages.go + cmd/messages_test.go
  T011 — update cmd/search.go + cmd/search_test.go
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

---

## Notes

- **Test-first is mandatory** (Constitution Principle II). Never skip the "write failing tests" steps.
- `go test -race ./...` must pass after every phase — not just at the end.
- The `Resolver` is nil-safe at every call site; `if resolver != nil { … }` before any method call.
- `buildResolver` returns `nil` (not an error) on failure — output gracefully degrades to raw IDs.
- `MessageJSON.channel_name` is already present in the struct; this feature populates it for history context (previously empty).
- Do not add `user_display_name` to `channelJSON` or `userJSON` — those types already show names.
