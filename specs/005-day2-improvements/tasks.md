# Tasks: 005 Day 2 Improvements

**Input**: Design documents from `/specs/005-day2-improvements/`
**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓, contracts/ ✓, quickstart.md ✓

**Tests**: Included — constitution Principle II (Test-First) is NON-NEGOTIABLE for this project.
All test tasks must be written and confirmed RED before the corresponding implementation task.

**Organization**: Tasks are grouped by user story in priority order (P1 → P4).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no blocking dependencies)
- **[Story]**: Which user story this task belongs to (US1–US10)

## Path Conventions

Single project layout (Go). All paths are relative to repository root.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: One-time project changes required before any user story begins.

- [X] T001 Add `golang.org/x/term` to go.mod via `go get golang.org/x/term` and verify `go.sum` is updated
- [X] T002 [P] Create `internal/emoji/doc.go` with package comment: "Package emoji maps Slack :name: tokens to Unicode codepoints."
- [X] T003 Generate `internal/emoji/emoji-map.json` — a flat JSON object `{"name": "unicode-string"}` mapping common Slack emoji names to their Unicode equivalents; include at minimum: thumbsup, thumbsdown, fire, white_check_mark, x, heart, tada, rocket, wave, eyes, pray, clap, muscle, bulb, warning, rotating_light, heavy_check_mark, heavy_plus_sign, heavy_minus_sign, arrow_right, clock1 through clock12, and all 26 skin-tone variants of hand emojis

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Global flag additions to `cmd/root.go` that multiple user stories depend on.

**⚠️ CRITICAL**: No user story work can begin until these flag stubs are in place, as `PersistentPreRunE` must compile with all new flag variables declared.

- [X] T004 Declare new global flag variables in `cmd/root.go`: `flagQuiet bool`, `flagSince string`, `flagUntil string`, `flagWidth int`; register flags on the root command (`--quiet`/`-q`, `--since`, `--until`, `--width`); leave `PersistentPreRunE` wiring as stubs (no-op) until each story implements its logic
- [X] T005 Write tests in `cmd/root_test.go` for new flag presence: verify `--quiet`, `--since`, `--until`, `--width` are registered on the root command and have correct types/defaults; confirm `go test -race ./cmd/...` passes

**Checkpoint**: Foundation ready — all new flag variables compile; user story phases can begin.

---

## Phase 3: User Story 1 — `--quiet` Flag (Priority: P1) 🎯 MVP

**Goal**: Suppress stderr progress and rate-limit noise when `--quiet`/`-q` is passed; credential warnings remain visible.

**Independent Test**:
```sh
slackseek history general --format json --quiet 2>/tmp/quiet-stderr.txt
grep -c "fetching" /tmp/quiet-stderr.txt  # must be 0
```

### Tests for User Story 1 ⚠️ Write first — must FAIL before T007

- [X] T006 [US1] Write test in `cmd/history_test.go` that injects a fake `historyRunFunc` emitting >1 "page" callback and verifies that with `--quiet` no progress text is written to stderr; confirm test is RED before proceeding

### Implementation for User Story 1

- [X] T007 [US1] Gate `SetPageFetchedCallback` and `SetRateLimitCallback` calls in `defaultRunHistory` (`cmd/history.go`) behind `if !flagQuiet { ... }`; confirm T006 turns GREEN

**Checkpoint**: `slackseek history general --format json --quiet 2>/dev/null` produces clean JSON with zero progress lines.

---

## Phase 4: User Story 2 — `--since` / `--until` Relative Dates (Priority: P1)

**Goal**: Accept duration offsets (`30m`, `4h`, `7d`, `2w`) and ISO dates for `--since`/`--until`; mutually exclusive with `--from`/`--to`.

**Independent Test**:
```sh
slackseek history general --since 24h   # succeeds
slackseek history general --since 24h --from 2026-01-01  # errors
```

### Tests for User Story 2 ⚠️ Write first — must FAIL before T009/T011

- [X] T008 [P] [US2] Write unit tests in `internal/slack/daterange_test.go` for `parseDateOrOffset`: test `30m`, `4h`, `7d`, `2w` produce correct offsets from a fixed `now`; test ISO dates pass through; test unrecognised inputs return error; confirm RED
- [X] T009 [P] [US2] Write unit tests in `cmd/root_test.go` for `--since`/`--until` mutual exclusion: verify error when `--since` + `--from` both set, and when `--until` + `--to` both set; confirm RED

### Implementation for User Story 2

- [X] T010 [US2] Implement `parseDateOrOffset(s string, now time.Time) (time.Time, error)` and `ParseRelativeDateRange(since, until string) (DateRange, error)` in `internal/slack/daterange.go`; confirm T008 turns GREEN
- [X] T011 [US2] Wire `--since`/`--until` into `PersistentPreRunE` in `cmd/root.go`: call `ParseRelativeDateRange` when either flag is non-empty; add mutual-exclusion guard against `--from`/`--to`; populate `ParsedDateRange`; confirm T009 turns GREEN

**Checkpoint**: `slackseek history general --since 24h` returns only messages from the last 24 hours; combined `--since`/`--from` returns a clear error.

---

## Phase 5: User Story 3 — `slackseek thread <url>` Command (Priority: P1)

**Goal**: Parse a Slack permalink, select the matching workspace, fetch the full thread, and print root + replies with a participant list.

**Independent Test**:
```sh
slackseek thread "https://ws.slack.com/archives/C01234/p1700000000123456" --format json
# → JSON with "participants" array and "messages" array
```

### Tests for User Story 3 ⚠️ Write first — must FAIL before T013/T015/T017

- [X] T012 [P] [US3] Write unit tests for `ParsePermalink` in `internal/slack/permalink_test.go`: test root permalink, reply permalink (with `?thread_ts=`), wrong scheme, missing `p` prefix, malformed path; confirm RED
- [X] T013 [P] [US3] Write unit tests for `FetchThread` in `internal/slack/messages_test.go` (or `client_test.go`): use a stub `conversations.replies` response; verify root is first element, replies follow in order; confirm RED

### Implementation for User Story 3

- [X] T014 [P] [US3] Implement `ThreadPermalink` struct and `ParsePermalink(url string) (ThreadPermalink, error)` in `internal/slack/permalink.go`; confirm T012 turns GREEN
- [X] T015 [P] [US3] Implement `FetchThread(ctx context.Context, channelID, threadTS string) ([]Message, error)` in `internal/slack/messages.go` using `conversations.replies` pagination; confirm T013 turns GREEN
- [X] T016 [US3] Implement `slackseek thread` command in `cmd/thread.go`: parse URL via `ParsePermalink`, select workspace by URL match, call `FetchThread`, collect unique participant display names, print via `output.PrintMessages` + participant footer; wire into `rootCmd` via `init()`
- [X] T017 [US3] Write command-level tests in `cmd/thread_test.go`: inject fake `FetchThread`; test text output includes participant list; test JSON output includes `participants` field; test unrecognised URL returns actionable error

**Checkpoint**: `slackseek thread <permalink>` prints the full thread and participant list; `--format json` includes `participants` field.

---

## Phase 6: User Story 4 — Line Wrapping in Text Output (Priority: P2)

**Goal**: Word-wrap the message text column in `--format text` output to terminal width (or 120 when piped). `--width 0` disables; `SLACKSEEK_WIDTH` and `--width N` override.

**Independent Test**:
```sh
slackseek history general --width 80 | awk '{print length}' | sort -n | tail -1
# → must be ≤ 80 (for lines that contain message content)
```

### Tests for User Story 4 ⚠️ Write first — must FAIL before T020/T022

- [X] T018 [P] [US4] Write unit tests for `WordWrap` in `internal/output/wrap_test.go`: test wrapping at exact boundary, word splitting, continuation indent, width=0 (no-op), single long word (no break), empty string; confirm RED
- [X] T019 [P] [US4] Write unit tests in `cmd/root_test.go` for width resolution: `SLACKSEEK_WIDTH=80` env sets width to 80; `--width 60` overrides env; `--width 0` disables; confirm RED

### Implementation for User Story 4

- [X] T020 [P] [US4] Implement `WordWrap(s string, width, indent int) string` in `internal/output/wrap.go`; confirm T018 turns GREEN
- [X] T021 [P] [US4] Add width-resolution helper (reads `SLACKSEEK_WIDTH` env, then `flagWidth`, then calls `term.GetSize`, defaults to 120) in `cmd/root.go`; export or thread resolved width into `output.PrintMessages`; confirm T019 turns GREEN
- [X] T022 [US4] Apply `WordWrap` to the message text field in `printMessagesText` in `internal/output/format.go`: compute prefix width from timestamp + channel + user columns, pass `remainingWidth = totalWidth − prefixWidth` and a `prefixWidth`-space indent to `WordWrap`

**Checkpoint**: Long messages in `--format text` wrap cleanly at the configured or detected width; `--width 0` disables wrapping; piped output wraps at 120.

---

## Phase 7: User Story 5 — Multi-Channel Search (Priority: P2)

**Goal**: `--channel` is repeatable; results from all channels are merged, deduplicated by `Timestamp`, and sorted ascending by time.

**Independent Test**:
```sh
slackseek search "X" --channel general --channel random --format json | jq 'length'
# → ≥ results from either channel alone
```

### Tests for User Story 5 ⚠️ Write first — must FAIL before T025/T026

- [X] T023 [P] [US5] Write unit tests in `cmd/search_test.go` for multi-channel merge: inject two fake search results with overlapping timestamps; verify deduplication and ascending time sort; confirm RED
- [X] T024 [P] [US5] Write unit tests in `cmd/search_test.go` for parallelism bound: inject 5 channels with a mutex-guarded counter; verify at most 3 goroutines run concurrently; confirm RED

### Implementation for User Story 5

- [X] T025 [US5] Change `--channel` flag in `cmd/search.go` from `StringVarP` to `StringArrayVarP` (type `[]string channels`); update `searchRunFunc` type signature to accept `channels []string`; update all call sites in `cmd/search.go` and `cmd/search_test.go`
- [X] T026 [US5] Implement parallel multi-channel fetch in `defaultRunSearch` (`cmd/search.go`): for single channel use existing path; for multiple channels use `sync.WaitGroup` + semaphore channel (`make(chan struct{}, 3)`); merge results into a map keyed by `Timestamp` to deduplicate; sort by `Message.Time` ascending; confirm T023 and T024 turn GREEN

**Checkpoint**: `slackseek search "X" --channel A --channel B` returns merged, sorted, deduplicated results; single-channel path unchanged.

---

## Phase 8: User Story 6 — Emoji Rendering (Priority: P3)

**Goal**: `:name:` tokens in message text and reactions are replaced with Unicode equivalents when rendering in a tty. `--emoji`/`--no-emoji` flags override.

**Independent Test**:
```sh
echo ':thumbsup: done' | slackseek … # not directly testable via pipe
# Unit test: emoji.Render(":thumbsup: done") == "👍 done"
```

### Tests for User Story 6 ⚠️ Write first — must FAIL before T028/T030

- [X] T027 [P] [US6] Write unit tests in `internal/emoji/emoji_test.go`: `Render(":thumbsup: :fire:")` returns `"👍 🔥"`; `Render(":unknown:")` returns `":unknown:"`; `RenderName("thumbsup")` returns `"👍"`; multiple tokens in one string; empty string; confirm RED
- [X] T028 [P] [US6] Write unit tests in `internal/output/format_test.go` for emoji-in-reactions: `formatReactions` with emoji active returns `"👍×3"` not `"thumbsup×3"`; confirm RED

### Implementation for User Story 6

- [X] T029 [P] [US6] Implement `internal/emoji/emoji.go`: embed `emoji-map.json` via `//go:embed`; implement `mustLoad() map[string]string`; implement `Render(s string) string` using `regexp.MustCompile(`:([a-z0-9_+-]+):`)` + `ReplaceAllStringFunc`; implement `RenderName(name string) string`; confirm T027 turns GREEN
- [X] T030 [P] [US6] Add `--emoji` / `--no-emoji` bool flag to `cmd/root.go` with default tied to `isatty` check via `golang.org/x/term`; expose resolved `flagEmojiEnabled bool` for use in output layer
- [X] T031 [US6] Wire emoji rendering into `internal/output/format.go`: call `emoji.Render(text)` in `resolveMessageFields` when emoji is enabled; call `emoji.RenderName(r.Name)` in `formatReactions` when emoji is enabled; thread the enabled bool through `PrintMessages` / `PrintSearchResults` options or a package-level toggle; confirm T028 turns GREEN

**Checkpoint**: `slackseek history general` in a tty renders `:thumbsup:` as `👍`; piped output passes through; `--no-emoji` disables in tty.

---

## Phase 9: User Story 7 — `slackseek postmortem <channel>` (Priority: P4)

**Goal**: Produce a structured Markdown incident document with period, participants, and timeline table.

**Independent Test**:
```sh
slackseek postmortem general --since 7d | head -5
# → "# Incident: general"
slackseek postmortem general --since 7d --format json | jq 'has("participants")'
# → true
```

### Tests for User Story 7 ⚠️ Write first — must FAIL before T034/T036

- [X] T032 [P] [US7] Write unit tests for `BuildIncidentDoc` and `PrintPostmortem` in `internal/output/postmortem_test.go`: given a slice of messages, verify `IncidentDoc.Participants` is alphabetically sorted and deduplicated; verify `Timeline` rows are in chronological order; verify thread roots with replies show `(N replies)` in event text; confirm RED
- [X] T033 [P] [US7] Write command tests in `cmd/postmortem_test.go`: inject fake history; verify default format is `markdown`; verify JSON output contains `period`, `participants`, `timeline` keys; confirm RED

### Implementation for User Story 7

- [X] T034 [P] [US7] Implement `IncidentDoc`, `TimelineRow`, `BuildIncidentDoc(messages []slack.Message, resolver *slack.Resolver) IncidentDoc`, and `PrintPostmortem(w io.Writer, fmt Format, doc IncidentDoc) error` in `internal/output/postmortem.go`; confirm T032 turns GREEN
- [X] T035 [US7] Implement `slackseek postmortem` command in `cmd/postmortem.go`: fetch history with threads, call `output.BuildIncidentDoc`, call `output.PrintPostmortem`; default `--format markdown`; wire into `rootCmd` via `init()`; confirm T033 turns GREEN

**Checkpoint**: `slackseek postmortem <channel> --since 7d` produces a complete Markdown incident document.

---

## Phase 10: User Story 8 — `slackseek digest --user` (Priority: P4)

**Goal**: Per-channel message digest for a user, grouped and sorted by message count descending.

**Independent Test**:
```sh
slackseek digest --user alice --since 7d | head -3
# → "## #<channel> (N messages)"
```

### Tests for User Story 8 ⚠️ Write first — must FAIL before T038/T040

- [X] T036 [P] [US8] Write unit tests for `GroupByChannel` and `PrintDigest` in `internal/output/digest_test.go`: verify channels sorted descending by message count; verify first-line preview truncates at 80 chars; verify JSON output schema; confirm RED
- [X] T037 [P] [US8] Write command tests in `cmd/digest_test.go`: require `--user` flag (error when absent); inject fake `GetUserMessages`; verify output groups by channel; confirm RED

### Implementation for User Story 8

- [X] T038 [P] [US8] Implement `ChannelDigest`, `GroupByChannel(messages []slack.Message) []ChannelDigest`, and `PrintDigest(w io.Writer, fmt Format, groups []ChannelDigest, resolver *slack.Resolver) error` in `internal/output/digest.go`; confirm T036 turns GREEN
- [X] T039 [US8] Implement `slackseek digest` command in `cmd/digest.go`: require `--user`/`-u` flag; resolve user ID via `c.ResolveUser`; call `GetUserMessages`; call `output.GroupByChannel` + `output.PrintDigest`; wire into `rootCmd` via `init()`; confirm T037 turns GREEN

**Checkpoint**: `slackseek digest --user alice --since 7d` prints channels grouped by count with message previews.

---

## Phase 11: User Story 9 — `slackseek metrics <channel>` (Priority: P4)

**Goal**: Per-user message counts, thread stats, top 5 reactions, and hourly UTC distribution for a channel.

**Independent Test**:
```sh
slackseek metrics general --since 7d | grep -q "Message counts"
slackseek metrics general --since 7d --format json | jq 'has("hourly")'
# → true
```

### Tests for User Story 9 ⚠️ Write first — must FAIL before T043/T045

- [X] T040 [P] [US9] Write unit tests for `ComputeMetrics` in `internal/output/metrics_test.go`: given a fixed message slice with known authors/reactions/times, verify `UserCounts` sorted descending, `ThreadCount`, `AvgReplyDepth`, `TopReactions` (top 5), and `HourlyDist` array values; confirm RED
- [X] T041 [P] [US9] Write command tests in `cmd/metrics_test.go`: inject fake history; verify text output contains "Message counts", "Thread stats", "Top reactions", "Messages by hour" sections; verify JSON has `users`, `threads`, `top_reactions`, `hourly` keys; confirm RED

### Implementation for User Story 9

- [X] T042 [P] [US9] Implement `ChannelMetrics`, `UserCount`, `ReactionCount`, `ComputeMetrics(messages []slack.Message, resolver *slack.Resolver) ChannelMetrics`, and `PrintMetrics(w io.Writer, fmt Format, m ChannelMetrics) error` (including ASCII bar chart for hourly dist) in `internal/output/metrics.go`; confirm T040 turns GREEN
- [X] T043 [US9] Implement `slackseek metrics` command in `cmd/metrics.go`: fetch history with threads, call `output.ComputeMetrics`, call `output.PrintMetrics`; wire into `rootCmd` via `init()`; confirm T041 turns GREEN

**Checkpoint**: `slackseek metrics <channel> --since 7d` prints user counts, thread stats, top reactions, and hourly bar chart.

---

## Phase 12: User Story 10 — `slackseek actions <channel>` (Priority: P4)

**Goal**: Extract messages matching commitment patterns into a checklist.

**Independent Test**:
```sh
slackseek actions general --since 7d | grep -c "\[ \]"
# → N ≥ 0 (checklist items)
```

### Tests for User Story 10 ⚠️ Write first — must FAIL before T047/T049

- [X] T044 [P] [US10] Write unit tests for `ExtractActions` in `internal/output/actions_test.go`: test each regex pattern independently (`I'll`, `I will`, `will do`, `on it`, `action item`, `TODO`, `follow up`, `@mention … can you`, `@mention … please`); test case-insensitivity; test non-matching message returns empty slice; confirm RED
- [X] T045 [P] [US10] Write command tests in `cmd/actions_test.go`: inject fake history with known matching + non-matching messages; verify text output checklist format `[ ] @user  <text>  <timestamp>`; verify JSON output schema; verify empty result prints summary line; confirm RED

### Implementation for User Story 10

- [X] T046 [P] [US10] Implement `ActionItem`, commitment pattern regexes, `ExtractActions(messages []slack.Message, resolver *slack.Resolver) []ActionItem`, and `PrintActions(w io.Writer, fmt Format, items []ActionItem) error` in `internal/output/actions.go`; confirm T044 turns GREEN
- [X] T047 [US10] Implement `slackseek actions` command in `cmd/actions.go`: fetch history (threads=false, root messages only), call `output.ExtractActions`, call `output.PrintActions`; wire into `rootCmd` via `init()`; confirm T045 turns GREEN

**Checkpoint**: `slackseek actions <channel> --since 7d` prints a checklist of commitment messages; empty result prints a summary.

---

## Phase 9c: Same-Day Date Range Fix (FR-014)

**Problem**: `--from 2026-03-05 --to 2026-03-05` returns nothing because both
dates parse to `00:00:00 UTC`, making the range a zero-width instant. A
`YYYY-MM-DD` `--to`/`--until` should cover through end of day.

- [X] T055 [P] [US2] Write unit tests in `internal/slack/daterange_test.go` for the end-of-day behaviour: `ParseDateRange("2026-03-05", "2026-03-05")` returns `To == 2026-03-05T23:59:59.999999999Z`; RFC 3339 `--to` is unchanged; duration offset `--until` is unchanged; confirm RED
- [X] T056 [US2] In `internal/slack/daterange.go`, add `parseDateStringEndOfDay(s string) (time.Time, error)` that parses `YYYY-MM-DD` as `23:59:59.999999999 UTC` and falls back to RFC 3339 as-is; use it for the `to` field in `ParseDateRange` and for the ISO-date branch of `parseDateOrOffset` when called for `until` in `ParseRelativeDateRange`; confirm T055 turns GREEN

---

## Phase 9b: Postmortem Quality Improvements (Post-Initial Implementation)

**Purpose**: Improve postmortem output quality based on real-world output review.

- [X] T052 [US7] Replace postmortem table format with per-entry block format in `printPostmortemMarkdown`: each timeline entry rendered as `---\n**timestamp UTC** · Who _(N replies)_\n\nevent text\n`; remove fixed-width table columns
- [X] T053 [US7] Add significance filter to `buildTimeline` in `internal/output/postmortem.go`: skip root messages that have no replies, no reactions, and no incident keyword match; implement `isSignificant(m slack.Message, replyCount int) bool` and `incidentKeywords` regexp covering deploy, rollback, hotfix, alert, paged, escalated, identified, mitigated, resolved, outage, degraded, down, restored, fixed, root cause, postmortem, on-call, sev[0-9]
- [X] T054 [US7] Add `unescapeHTML(s string) string` helper in `internal/output/postmortem.go` decoding `&amp;`, `&lt;`, `&gt;`, `&quot;`, `&#39;`; apply to event text before rendering in markdown output

---

## Phase 13: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, build validation, and linter clean-up.

- [X] T048 [P] Create `docs/claude-usage.md`: document all commands with recommended flags for programmatic use (`--format json --quiet`), channel resolution pattern (`channels list --format json` first), example CLAUDE.md snippet
- [X] T049 [P] Run `go vet ./...` and `golangci-lint run`; fix all new issues introduced by this feature (must reach zero)
- [X] T050 [P] Verify cross-platform build: `GOOS=linux go build ./...` and `GOOS=darwin go build ./...` both succeed
- [X] T051 Run through `specs/005-day2-improvements/quickstart.md` manually and confirm each scenario passes end-to-end

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion — BLOCKS all user story phases
- **US1–US5 (Phases 3–7)**: Depend on Phase 2; can proceed in parallel after foundation
- **US6 (Phase 8)**: Depends on Phase 1 (emoji-map.json in T003); independent of US1–US5
- **US7–US10 (Phases 9–12)**: Depend on Phase 2; fully independent of each other and of US1–US6
- **Polish (Phase 13)**: Depends on all desired stories being complete

### User Story Dependencies

| Story | Depends On | Can Run In Parallel With |
|-------|-----------|--------------------------|
| US1 (quiet) | Phase 2 | US2, US3 |
| US2 (dates) | Phase 2 | US1, US3 |
| US3 (thread) | Phase 2 | US1, US2 |
| US4 (wrap) | Phase 2 + T001 (x/term) | US5, US6 |
| US5 (multi-ch) | Phase 2 | US4, US6 |
| US6 (emoji) | T001 + T002 + T003 (emoji-map.json) | US4, US5 |
| US7 (postmortem) | Phase 2 | US8, US9, US10 |
| US8 (digest) | Phase 2 | US7, US9, US10 |
| US9 (metrics) | Phase 2 | US7, US8, US10 |
| US10 (actions) | Phase 2 | US7, US8, US9 |

### Within Each User Story

1. Write tests → confirm RED
2. Implement model/helper → confirm GREEN
3. Implement command or integration → confirm GREEN
4. All tests pass with `go test -race ./...`

---

## Parallel Execution Examples

### P1 Stories (after Phase 2)

```
Parallel batch A (different files, no shared dependencies):
  Task: T006+T007  (US1 — cmd/history.go)
  Task: T008+T009  (US2 — internal/slack/daterange.go)
  Task: T012+T013  (US3 — internal/slack/permalink.go)
```

### P4 Output Helpers (after Phase 2)

```
Parallel batch B (all different output files):
  Task: T032+T034  (US7 — internal/output/postmortem.go)
  Task: T036+T038  (US8 — internal/output/digest.go)
  Task: T040+T042  (US9 — internal/output/metrics.go)
  Task: T044+T046  (US10 — internal/output/actions.go)
```

---

## Implementation Strategy

### MVP First (P1 stories only — 3 features)

1. Complete Phase 1: Setup (T001–T003)
2. Complete Phase 2: Foundational (T004–T005)
3. Complete Phase 3: US1 `--quiet` (T006–T007)
4. Complete Phase 4: US2 `--since`/`--until` (T008–T011)
5. Complete Phase 5: US3 `thread` command (T012–T017)
6. **STOP and VALIDATE**: run quickstart.md sections 1–3

### Incremental Delivery

1. Setup + Foundational → compiler passes
2. US1 → quiet AI-agent-friendly output
3. US2 → ergonomic time-bounded queries
4. US3 → thread permalink command
5. US4 → clean terminal wrapping
6. US5 → multi-channel search
7. US6 → emoji polish
8. US7–US10 → dev-manager workflows (independent of each other)

---

## Notes

- **Test-First is non-negotiable** (constitution Principle II): every test task must be confirmed RED before its paired implementation task
- `[P]` marks tasks that touch different files with no blocking dependencies — safe to run concurrently with other `[P]` tasks in the same phase
- Each story phase ends with a checkpoint — validate the story before moving to the next priority
- `go test -race ./...` must pass after every phase
- `golangci-lint run` must pass before Polish phase is considered complete
- Total task count: **56 tasks** across 15 phases (T052–T056 added post-initial implementation)
