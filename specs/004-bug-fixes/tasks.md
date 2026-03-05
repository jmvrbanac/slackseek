# Tasks: 004 Bug Fixes

**Input**: Design documents from `/specs/004-bug-fixes/`
**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓, contracts/ ✓, quickstart.md ✓

**Tests**: Included — Constitution Principle II (Test-First) is NON-NEGOTIABLE.
Write the failing test first; implement the minimum code to pass; then refactor.

**Organization**: Tasks are grouped by user story (one per bug fix) in priority order.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: Which fix this task belongs to (US1–US5 map to Fix 1–5)
- Exact file paths are included in every task description

## Path Conventions (single-project layout)

```
internal/slack/resolver.go        resolver_test.go
internal/output/format.go         format_test.go
cmd/root.go
```

---

## Phase 1: Setup

**Purpose**: Verify clean baseline before any changes.

- [X] T001 Confirm `go build ./...`, `go test -race ./...`, and `golangci-lint run` all pass on the current branch

---

## Phase 2: Foundational

*No shared infrastructure is required. All fixes work within existing packages.
User story phases may begin immediately after Phase 1.*

---

## Phase 3: User Story 1 — Fix 1: Mention regex (Priority: P1) 🎯 MVP

**Goal**: `<@USERID|label>` tokens in message text are resolved to display names
(or fall back to the embedded label) instead of being passed through verbatim.

**Independent Test**: Run `./slackseek search "please tag"` against a workspace
that has messages with embedded-label mentions. Confirm `<@U22JKTL6N|nmollenkopf>`
becomes `@Nick Mollenkopf` (or `@nmollenkopf` if the user is not cached).

### Tests — Fix 1 ⚠️ Write first, confirm they FAIL before implementing

- [X] T002 [US1] Add test cases for `<@USERID|label>` form in `internal/slack/resolver_test.go`: bare ID resolved, ID+label resolved via map, ID+label fallback to label when ID unknown, ID-only form still works

### Implementation — Fix 1

- [X] T003 [US1] Update `mentionPattern` to `<@([A-Z0-9]+)(?:\|([^>]+))?>` in `internal/slack/resolver.go`
- [X] T004 [US1] Rewrite the `mentionPattern.ReplaceAllStringFunc` handler in `ResolveMentions` (`internal/slack/resolver.go`) to use `FindStringSubmatch`: prefer resolved real name → embedded label → raw ID

**Checkpoint**: `go test -race ./internal/slack/...` passes. Mention tokens with labels now resolve correctly.

---

## Phase 4: User Story 2 — Fix 4: DM channel name resolution (Priority: P1)

**Goal**: When `messages <user>` returns results from DM conversations, the
channel column shows `@DisplayName` instead of a raw user ID like `U01ABCDEF`.

**Independent Test**: Run `./slackseek messages <user>` for a user known to
have DM history. Confirm the channel column shows `@Name` for DM results and
`#channel-name` for public channel results.

### Tests — Fix 4 ⚠️ Write first, confirm they FAIL before implementing

- [X] T005 [US2] Add `ResolveChannelDisplay` unit tests in `internal/slack/resolver_test.go`: user-ID name → `@DisplayName`, user-ID name not in map → `@rawID`, empty name → falls through to `ChannelName(id)`, regular channel name → unchanged
- [X] T006 [P] [US2] Add `resolveMessageFields` test in `internal/output/format_test.go` asserting that a `Message` with `ChannelName="U01ABCDEF"` renders as `@DisplayName` in all output formats

### Implementation — Fix 4

- [X] T007 [US2] Add `userIDPattern = regexp.MustCompile("^U[A-Z0-9]+$")` and `ResolveChannelDisplay(id, name string) string` method to `internal/slack/resolver.go`
- [X] T008 [US2] Update `resolveMessageFields` in `internal/output/format.go` to call `resolver.ResolveChannelDisplay(m.ChannelID, m.ChannelName)` instead of the current `channelDisplay == ""` conditional
- [X] T009 [US2] Update `toMessageJSON` in `internal/output/format.go` to use `resolver.ResolveChannelDisplay` for the `channelName` field

**Checkpoint**: `go test -race ./internal/slack/... ./internal/output/...` passes. DM results show `@Name` in all output formats.

---

## Phase 5: User Story 3 — Fix 5: Table newline alignment (Priority: P1)

**Goal**: Messages containing `\n` characters display as a single collapsed
line within their table cell; no continuation lines bleed to column 0.

**Independent Test**: Run `./slackseek history <channel> --format table` on a
channel that has multi-line messages (e.g. pasted code blocks). Confirm every
row stays within the visual grid.

### Tests — Fix 5 ⚠️ Write first, confirm they FAIL before implementing

- [X] T010 [US3] Add `tableSafe` unit test in `internal/output/format_test.go`: input with `\n`, `\r\n`, tabs, and leading/trailing space all collapse to a single-space-separated string; result is truncated at the requested rune limit

### Implementation — Fix 5

- [X] T011 [US3] Add `tableSafe(s string, n int) string` helper to `internal/output/format.go` using `strings.Fields` + `strings.Join` then `truncate`
- [X] T012 [US3] Replace `truncate(text, N)` with `tableSafe(text, N)` in the `FormatTable` case of `PrintMessages` in `internal/output/format.go`
- [X] T013 [US3] Replace `truncate(text, N)` with `tableSafe(text, N)` in the `FormatTable` case of `PrintSearchResults` in `internal/output/format.go`

**Checkpoint**: `go test -race ./internal/output/...` passes. Multi-line messages produce single-line table cells.

---

## Phase 6: User Story 4 — Fix 2: Thread grouping (Priority: P2)

**Goal**: `history --threads` groups replies visually under their parent
message across all output formats (text, table, JSON).

**Independent Test**: Run `./slackseek history <channel> --threads --format text`
on a channel with known threads. Confirm replies appear indented with `  └─ `
under their parent, separated from the next root message by a blank line.
Run `./slackseek history <channel> --threads --format json | jq '.[0].replies'`
and confirm a non-null array.

### Tests — Fix 2 ⚠️ Write first, confirm they FAIL before implementing

- [X] T014 [US4] Add `groupByThread` unit tests in `internal/output/format_test.go`: empty input, all root messages (no threading), mixed roots and replies, reply whose parent is not in the slice (orphan — attaches to nearest root or is promoted)
- [X] T015 [P] [US4] Add `PrintMessages` text-format integration test in `internal/output/format_test.go`: mixed root+reply messages produce `└─` indented output with blank-line separators
- [X] T016 [P] [US4] Add `PrintMessages` JSON-format integration test in `internal/output/format_test.go`: replies nested under parent; flat top-level array excludes reply messages

### Implementation — Fix 2

- [X] T017 [US4] Add `Replies []messageJSON \`json:"replies,omitempty"\`` field to `messageJSON` struct in `internal/output/format.go`
- [X] T018 [US4] Add `groupByThread(msgs []slack.Message) (roots []slack.Message, replies map[string][]slack.Message)` helper in `internal/output/format.go` (single-pass O(n): root if `ThreadTS == ""` or `ThreadTS == Timestamp`)
- [X] T019 [US4] Update `FormatText` case of `PrintMessages` in `internal/output/format.go` to call `groupByThread` and print replies with `  └─ ` prefix and blank-line group separators
- [X] T020 [US4] Update `FormatTable` case of `PrintMessages` in `internal/output/format.go` to call `groupByThread` and emit reply rows with `  └─ ` prefix in the Text column (use `tableSafe`)
- [X] T021 [US4] Update `FormatJSON` case of `PrintMessages` in `internal/output/format.go` to call `groupByThread` and populate `messageJSON.Replies`; exclude reply messages from the top-level slice

**Checkpoint**: `go test -race ./internal/output/...` passes. All three format paths produce grouped thread output.

---

## Phase 7: User Story 5 — Fix 3: Markdown export (Priority: P3)

**Goal**: `--format markdown` produces a well-structured Markdown document for
`history` and `search` output, suitable for saving as an incident post-mortem
or decision log.

**Independent Test**:
```sh
./slackseek history <channel> --format markdown > out.md
# open out.md — expect: # heading, ## per root message, > block-quoted replies
./slackseek search "deploy" --format markdown > search.md
# open search.md — expect: # Search results, ## per result, permalink link
```

### Tests — Fix 3 ⚠️ Write first, confirm they FAIL before implementing

- [X] T022 [US5] Add golden-output test for `PrintMessages` markdown format in `internal/output/format_test.go`: single-day heading `# #channel — YYYY-MM-DD`, `##` root headings, `> ` block-quoted replies, reactions, `---` separators
- [X] T023 [P] [US5] Add golden-output test for `PrintSearchResults` markdown format in `internal/output/format_test.go`: `# Search results` heading, `## date · channel · user`, message body, `[View in Slack](permalink)`, `---` separators
- [X] T024 [P] [US5] Add test in `internal/output/format_test.go` asserting that `--format markdown` for `PrintChannels`/`PrintUsers` falls through to text format (not an error, not a crash)

### Implementation — Fix 3

- [X] T025 [US5] Add `FormatMarkdown Format = "markdown"` constant and append to `ValidFormats` slice in `internal/output/format.go`
- [X] T026 [US5] Add `printMessagesMarkdown(w io.Writer, messages []slack.Message, resolver *slack.Resolver) error` helper in `internal/output/format.go`: derive channel name via `resolver.ChannelName(messages[0].ChannelID)`, call `groupByThread`, emit `#`/`##`/`>`/`---` structure per spec
- [X] T027 [US5] Add `printSearchResultsMarkdown(w io.Writer, results []slack.SearchResult, resolver *slack.Resolver) error` helper in `internal/output/format.go`: flat `# Search results` list with permalink
- [X] T028 [US5] Add `case FormatMarkdown:` to `PrintMessages` switch in `internal/output/format.go` calling `printMessagesMarkdown`
- [X] T029 [US5] Add `case FormatMarkdown:` to `PrintSearchResults` switch in `internal/output/format.go` calling `printSearchResultsMarkdown`
- [X] T030 [US5] Update `--format` flag description string in `cmd/root.go` from `text | table | json` to `text | table | json | markdown`

**Checkpoint**: `go test -race ./internal/output/... ./cmd/...` passes. Markdown output renders correctly for both history and search.

---

## Phase 8: User Story 6 — Fix 6: Proactive rate limiting (Priority: P1)

**Goal**: API calls are throttled to stay below Slack's tier limits before a 429 is
received, eliminating silent stalls during paginated operations.

### Tests — Fix 6 ⚠️ Write first, confirm they FAIL before implementing

- [X] T036 [US6] Add `rateLimiter` unit tests in `internal/slack/client_test.go`:
  first `Wait` returns immediately (≤ 5 ms); second `Wait` blocks for ~interval;
  cancelled context unblocks a waiting `Wait` with `context.Canceled` error

### Implementation — Fix 6

- [X] T037 [US6] Add `rateLimiter` struct and `newRateLimiter(perMinute int)` to
  `internal/slack/client.go`; add `tier2` and `tier3` fields to `Client`;
  initialize both in `NewClient` (tier2=18/min, tier3=48/min)
- [X] T038 [US6] Add `c.tier2.Wait(ctx)` before `callWithRetry` in the `ListChannels`
  pagination loop in `internal/slack/channels.go`
- [X] T039 [US6] Add `c.tier3.Wait(ctx)` before `callWithRetry` in `historyPageFetch`
  and `repliesPageFetch` in `internal/slack/channels.go`
- [X] T040 [US6] Add `c.tier2.Wait(ctx)` before `callWithRetry` in the `SearchMessages`
  pagination loop in `internal/slack/search.go`; add `c.tier2.Wait(ctx)` before
  `GetUsersContext` in `ListUsers` in `internal/slack/users.go`

**Checkpoint**: `go test -race ./internal/slack/...` passes.

---

## Phase 9: Polish & Cross-Cutting Concerns

- [X] T031 Run `go vet ./...` and confirm zero issues
- [X] T032 Run `golangci-lint run` and fix any lint findings
- [X] T033 Run `go test -race ./...` (full suite) and confirm zero failures and zero races
- [X] T034 [P] Run `GOOS=linux go build ./...` and `GOOS=darwin go build ./...` — both must succeed
- [X] T035 Walk through `specs/004-bug-fixes/quickstart.md` manual verification steps for all five fixes

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Foundational)**: Skipped — no shared infrastructure needed
- **Phases 3–7 (User Stories)**: Depend on Phase 1 completion
  - US1 (Fix 1) and US2 (Fix 4) both touch `resolver.go` — run **sequentially**
  - US3 (Fix 5) touches only `format.go` — can run in parallel with US1/US2 if on different branches, otherwise run after US2
  - US4 (Fix 2) touches `format.go` — run after US3 (avoids file conflicts)
  - US5 (Fix 3) depends on US4 (reuses `groupByThread`) — run after US4
- **Phase 8 (Polish)**: Run after all user stories are complete

### User Story Dependencies

| Story | Fix | Depends on | Blocks |
|-------|-----|-----------|--------|
| US1 | Fix 1 — mention regex | Phase 1 | — |
| US2 | Fix 4 — DM channels | US1 (same file: `resolver.go`) | — |
| US3 | Fix 5 — table newlines | Phase 1 | — |
| US4 | Fix 2 — thread grouping | US3 (same file: `format.go`) | US5 |
| US5 | Fix 3 — markdown export | US4 (`groupByThread` required) | — |

### Within Each User Story

1. Write failing test(s) first — confirm RED
2. Implement minimum code — confirm GREEN
3. Refactor if needed — confirm still GREEN
4. Commit the story before moving to the next

---

## Parallel Opportunities

### If working solo (recommended order)

```
T001 → T002→T003→T004 → T005→T006→T007→T008→T009 → T010→T011→T012→T013
     → T014→T015→T016→T017→T018→T019→T020→T021 → T022→T023→T024→T025→T026→T027→T028→T029→T030
     → T031→T032→T033→T034→T035
```

### Parallel opportunities within stories

```
# US4 (Fix 2) — tests can be written in parallel (different test scenarios):
T014 (groupByThread unit tests)
T015 (text format integration test)   ← [P] with T016
T016 (JSON format integration test)   ← [P] with T015

# US5 (Fix 3) — tests can be written in parallel:
T022 (PrintMessages markdown test)
T023 (PrintSearchResults markdown test)  ← [P] with T022, T024
T024 (fallthrough test)                  ← [P] with T022, T023
```

---

## Implementation Strategy

### MVP (Fixes 1, 4, 5 — all P1 display bugs)

1. Complete Phase 1 (T001)
2. Complete Phase 3 — Fix 1 (T002–T004)
3. Complete Phase 4 — Fix 4 (T005–T009)
4. Complete Phase 5 — Fix 5 (T010–T013)
5. **STOP and VALIDATE**: Run `go test -race ./...`, check output manually
6. These three fixes can ship independently — no breaking changes

### Full delivery

Continue with Phase 6 (Fix 2) → Phase 7 (Fix 3) → Phase 8 (Polish).

> ⚠️ **Breaking change notice**: Fix 2 (Phase 6) changes the JSON schema for
> `history --threads --format json`. The `replies` field moves replies out of
> the top-level array. Communicate to any JSON consumers before shipping.

---

## Notes

- `[P]` = different files or independent test scenarios — safe to parallelize
- Constitution Principle II is enforced: every task that adds production code
  is preceded by a failing test task
- `go test -race ./...` must pass after each phase checkpoint before advancing
- Commit after each complete phase or logical group (not individual tasks)
