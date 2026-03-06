# Feature Specification: Day 2 Improvements

**Feature Branch**: `005-day2-improvements`
**Created**: 2026-03-05
**Status**: Draft
**Input**: User description: "005 @day2_improvements.md"

## User Scenarios & Testing *(mandatory)*

### User Story 1 — `--quiet` flag suppresses stderr progress (Priority: P1)

An AI agent (Claude Code) pipes `slackseek history` output into a JSON
processor. The `\rfetching channels: N fetched...` progress lines clutter
stdout capture and confuse line-oriented parsers.

**Why this priority**: Zero-effort fix that immediately unblocks every
programmatic use case. Single flag, no new logic.

**Independent Test**: `slackseek history general --format json --quiet 2>/dev/null`
should produce clean JSON on stdout with no progress noise.

**Acceptance Scenarios**:

1. **Given** a channel with >1 page of history, **When** `--quiet` is passed,
   **Then** no progress text appears on stderr.
2. **Given** a workspace warning (expired token), **When** `--quiet` is passed,
   **Then** the `Warning: …` line is still emitted to stderr.
3. **Given** `--quiet` is not passed, **When** the command runs, **Then**
   existing progress output is unchanged.

---

### User Story 2 — Relative date flags `--since` / `--until` (Priority: P1)

A user wants to fetch the last 24 hours of a channel without computing exact
ISO dates.

**Why this priority**: Ergonomic upgrade used in every time-bounded query.
Pure parse-layer change; no API impact.

**Independent Test**: `slackseek history general --since 24h` and
`slackseek search "deploy" --since 7d --until 1d` produce correct
time-bounded results.

**Acceptance Scenarios**:

1. **Given** `--since 24h`, **When** the command runs, **Then** `DateRange.From`
   is set to `now − 24 hours`.
2. **Given** `--since 7d --until 1d`, **When** the command runs, **Then**
   `DateRange.From = now − 7d`, `DateRange.To = now − 1d`.
3. **Given** `--since 2026-01-15` (ISO date), **When** the command runs,
   **Then** it is accepted (existing `--from` behaviour preserved).
4. **Given** `--since 2h --until 24h`, **When** the command runs, **Then**
   an error is returned (`since` is after `until`).
5. **Given** both `--since` and legacy `--from` are provided, **Then** an
   error is returned (mutually exclusive).

---

### User Story 3 — `slackseek thread <permalink-url>` command (Priority: P1)

Claude Code finds a Slack permalink in a GitHub PR description and wants to
fetch the full thread for context.

**Why this priority**: New command with clear, bounded scope. Enables AI
agents to act on Slack links found in code review artifacts.

**Independent Test**: `slackseek thread https://<ws>.slack.com/archives/C123/p1700000000123456`
prints the complete thread (root + replies) in the requested format.

**Acceptance Scenarios**:

1. **Given** a thread root permalink, **When** the command runs, **Then**
   the root message and all replies are printed, followed by a participant list.
2. **Given** a reply permalink (not root), **When** the command runs, **Then**
   the full thread is fetched from the root so complete context is returned.
3. **Given** an unrecognised URL format, **When** the command runs, **Then**
   a clear error explains the expected format.
4. **Given** `--format json`, **When** the command runs, **Then** the thread
   is output as a JSON array with a top-level `participants` array.
5. **Given** multiple workspaces, **When** the permalink's subdomain matches
   one workspace URL, **Then** that workspace is auto-selected.

---

### User Story 4 — Line wrapping in `history` / `search` text output (Priority: P2)

A developer runs `slackseek history incidents` in a terminal and wants
messages to wrap cleanly at the terminal edge rather than spilling to a second
line mid-column.

**Why this priority**: Readability fix for the default output format. Pure
post-processing; no API changes.

**Independent Test**: `slackseek history general` on a 80-column terminal
wraps long messages so continuation lines are indented under the message
column.

**Acceptance Scenarios**:

1. **Given** stdout is a tty, **When** a message exceeds remaining column
   width, **Then** it is word-wrapped with continuation lines indented.
2. **Given** stdout is piped (not a tty), **When** a message exceeds 120
   chars, **Then** it is wrapped at 120 characters.
3. **Given** `--width 0`, **When** any message is output, **Then** no
   wrapping is applied.
4. **Given** `SLACKSEEK_WIDTH=200` env var, **When** the command runs,
   **Then** wrapping occurs at 200 characters.
5. **Given** `--width N` flag, **When** the command runs, **Then** it
   overrides the env var and tty-detected width.

---

### User Story 5 — Multi-channel search (Priority: P2)

A user wants to search for "incident" across three channels at once:
`slackseek search "incident" --channel ic-5697 --channel ic-5698 --channel ic-5699`.

**Why this priority**: Common power-user pattern; enables incident
retrospectives without running three separate commands.

**Independent Test**: `slackseek search "deploy" --channel general --channel deploys`
returns merged, deduplicated, time-sorted results from both channels.

**Acceptance Scenarios**:

1. **Given** two `--channel` flags, **When** the command runs, **Then**
   one `SearchMessages` call is issued per channel, results are merged and
   sorted by time.
2. **Given** a duplicate message (same `Timestamp`) appears in both result
   sets, **When** merged, **Then** only one copy appears.
3. **Given** one of the channels does not exist, **When** the command runs,
   **Then** an error names the missing channel; results from valid channels
   are discarded (fail-fast).
4. **Given** more than 3 channels, **When** the command runs, **Then** at
   most 3 goroutines run concurrently.

---

### User Story 6 — Emoji rendering in message text (Priority: P3)

A user reading `--format text` output wants `:white_check_mark:` to render
as `✅` for quick visual scanning.

**Why this priority**: Visual polish; does not affect correctness. Can be
shipped independently.

**Independent Test**: `slackseek history general` in a tty renders known
emoji names as Unicode; unknown names like `:company-logo:` pass through.

**Acceptance Scenarios**:

1. **Given** stdout is a tty (default on), **When** a message contains
   `:thumbsup:`, **Then** the output shows `👍`.
2. **Given** stdout is piped (default off), **When** a message contains
   `:fire:`, **Then** `:fire:` is printed unchanged.
3. **Given** `--emoji` flag, **When** stdout is piped, **Then** emoji are
   still rendered.
4. **Given** `--no-emoji` flag, **When** stdout is a tty, **Then** emoji
   names are printed as-is.
5. **Given** an unrecognised name `:company-logo:`, **When** rendering,
   **Then** `:company-logo:` is preserved unchanged.
6. **Given** emoji in reaction names, **When** rendered, **Then**
   `👍×3 ❤️×2` format is used.

---

### User Story 7 — `slackseek postmortem <channel>` (Priority: P4)

A manager wants a structured incident timeline document from a Slack channel
to attach to a postmortem ticket.

**Why this priority**: High-value workflow but depends on `thread` command
foundations. Delivers a turnkey postmortem document.

**Independent Test**: `slackseek postmortem ic-5697` prints a Markdown
document with period, participants, and a chronological timeline of
significant events.

**Acceptance Scenarios**:

1. **Given** a channel name, **When** the command runs, **Then** a Markdown
   document is produced with `# Incident: <channel>`, period, participant
   list, and per-entry timeline blocks (not a table).
2. **Given** the channel contains casual conversation, **When** the command
   runs, **Then** only significant messages are included: those with thread
   replies, at least one reaction, or text matching incident keywords
   (`deploy`, `rollback`, `alert`, `paged`, `escalated`, `identified`,
   `mitigated`, `resolved`, `outage`, `degraded`, `restored`, `fixed`,
   `root cause`, `postmortem`, `on-call`, `sev[0-9]`, etc.).
3. **Given** a message contains HTML entities (`&gt;`, `&lt;`, `&amp;`),
   **When** rendered in the timeline, **Then** they are decoded to their
   literal characters.
4. **Given** a message contains newlines, **When** rendered in the timeline,
   **Then** they are preserved (block format allows multi-line content).
5. **Given** `--since` / `--until` flags, **When** the command runs, **Then**
   the timeline is scoped to that window.
6. **Given** `--format json`, **When** the command runs, **Then** structured
   JSON with `period`, `participants`, and `timeline` arrays is emitted.

---

### User Story 8 — `slackseek digest --user @alice --since 7d` (Priority: P4)

Before a 1:1, a manager wants a quick list of what @alice has been saying in
the last week, grouped by channel.

**Why this priority**: Reuses existing `GetUserMessages`; wrapper command
only.

**Independent Test**: `slackseek digest --user @alice --since 7d` prints
channels grouped by message count with a one-line preview per message.

**Acceptance Scenarios**:

1. **Given** a user display name and duration, **When** the command runs,
   **Then** channels are listed descending by message count with first-line
   preview.
2. **Given** `--format json`, **When** the command runs, **Then** full
   messages grouped by channel are emitted.

---

### User Story 9 — `slackseek metrics <channel>` (Priority: P4)

A manager wants per-user message counts, thread stats, and busiest hour for
an incident channel.

**Why this priority**: Pure post-processing over `FetchHistory`; no new
API calls.

**Independent Test**: `slackseek metrics ic-5697 --since 7d` prints a
table of user message counts, thread stats, and ASCII bar chart of
messages-per-hour.

**Acceptance Scenarios**:

1. **Given** a channel, **When** the command runs, **Then** per-user message
   counts (sorted descending), thread count and average reply depth, and
   top-5 reacted messages are displayed.
2. **Given** `--format json`, **When** the command runs, **Then**
   `{"users":[...],"threads":{...},"top_reactions":[...],"hourly":[...]}` is
   emitted.

---

### User Story 10 — `slackseek actions <channel>` (Priority: P4)

A manager wants to extract action items from a channel to a checklist without
re-reading every thread.

**Why this priority**: Heuristic regex post-processing; zero new API calls.

**Independent Test**: `slackseek actions incidents --since 7d` prints a
checklist of messages matching commitment patterns.

**Acceptance Scenarios**:

1. **Given** a channel, **When** the command runs, **Then** messages matching
   `I'll`, `will do`, `action item`, `TODO`, `follow up`, `@mention … can you`
   patterns are printed as a checklist.
2. **Given** no matches, **When** the command runs, **Then** an empty
   checklist with a summary line is printed.

---

### Edge Cases

- Terminal width detection fails (non-standard TIOCGWINSZ) → fall back to 120.
- Slack permalink uses a non-standard workspace URL (vanity domain) → match
  against stored workspace URL field, not just subdomain.
- Multi-channel search with 0 results from all channels → empty output, no error.
- Emoji lookup table missing an entry → pass-through (`:name:` preserved).
- `--since` / `--until` with nanosecond precision duration strings (e.g. `1h30m`) → supported.
- `slackseek thread` on a deleted message → Slack API returns error; surface
  with actionable message.
- `--from 2026-03-05 --to 2026-03-05` (same-day range) → `--to` is interpreted
  as `2026-03-05T23:59:59.999999999Z` so the full day is included. The same
  applies to `--until` when an ISO date is given (not an offset).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `--quiet` / `-q` global flag MUST suppress all `\r…` progress
  lines and rate-limit wait notices from stderr; credential warnings (`Warning:`)
  MUST still be printed.
- **FR-002**: `--since` / `--until` flags MUST accept ISO dates (`2026-01-15`)
  and duration offsets (`30m`, `4h`, `7d`, `2w`). Duration offsets MUST be
  relative to `time.Now()`.
- **FR-003**: `--since` and `--until` MUST be mutually exclusive with the
  existing `--from` / `--to` flags.
- **FR-004**: `slackseek thread <url>` MUST parse a Slack permalink, resolve
  the workspace, fetch the thread, and print root + replies plus a participant list.
- **FR-005**: Text output MUST word-wrap the message text column to the
  detected or configured width. Continuation lines MUST be indented to align
  with the start of the message column.
- **FR-006**: `--width N` flag (global) and `SLACKSEEK_WIDTH` env var MUST
  override auto-detected terminal width. `--width 0` MUST disable wrapping.
- **FR-007**: `--channel` on `search` MUST accept multiple values
  (repeatable flag). Results MUST be merged, deduplicated by `Timestamp`,
  and sorted ascending by time.
- **FR-008**: Emoji rendering MUST use a bundled lookup table embedded via
  `//go:embed`. Custom workspace emoji not in the table MUST pass through.
- **FR-009**: `--emoji` / `--no-emoji` flag MUST control emoji rendering.
  Default MUST be on when `os.Stdout` is a tty, off otherwise.
- **FR-010**: `slackseek postmortem <channel>` MUST produce a Markdown
  incident document with period, participants, and a per-entry timeline.
  Timeline MUST include only significant messages: those with thread replies,
  at least one reaction, or text matching incident keywords. HTML entities
  (`&gt;`, `&lt;`, `&amp;`, `&quot;`, `&#39;`) MUST be decoded before
  rendering.
- **FR-011**: `slackseek digest --user <name> --since <duration>` MUST
  produce per-channel message summaries using `GetUserMessages`.
- **FR-012**: `slackseek metrics <channel>` MUST compute per-user counts,
  thread stats, top reactions, and hourly distribution from `FetchHistory`.
- **FR-013**: `slackseek actions <channel>` MUST scan messages for commitment
  patterns and emit a checklist.
- **FR-014**: When `--to` or `--until` is given as a `YYYY-MM-DD` date string
  (no time component), it MUST be resolved to `23:59:59.999999999 UTC` of that
  day so that a same-day `--from`/`--to` range covers the entire day. RFC 3339
  inputs and duration offsets MUST be used as-is.

### Key Entities

- **DateOffset**: a duration string (`30m`, `4h`, `7d`, `2w`) that resolves
  to a `time.Time` relative to `time.Now()`.
- **ThreadPermalink**: parsed form of a Slack permalink URL with `WorkspaceURL`,
  `ChannelID`, and `ThreadTS` fields.
- **EmojiTable**: embedded name→Unicode map loaded once at startup.
- **ActionItem**: a `Message` matched by a commitment pattern heuristic.
- **IncidentDoc**: structured postmortem document with `Period`,
  `Participants []string`, and `Timeline []TimelineRow`.
- **ChannelMetrics**: per-channel aggregation: `UserCounts`, `ThreadStats`,
  `TopReactions`, `HourlyDist`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `slackseek history general --format json --quiet 2>&1 | grep -c "fetching"` returns 0.
- **SC-002**: `slackseek history general --since 24h` returns only messages
  from the last 24 hours (verifiable by checking the earliest message timestamp).
- **SC-003**: `slackseek thread <url>` outputs the root message plus at least
  one reply for a known thread permalink.
- **SC-004**: Long messages (>80 chars) in text output do not exceed the
  detected terminal width (verifiable via `fold -w $COLUMNS`).
- **SC-005**: `slackseek search "X" --channel A --channel B` returns a superset
  of `slackseek search "X" --channel A` for the same query.
- **SC-006**: Known emoji names (`:thumbsup:`, `:fire:`, `:white_check_mark:`)
  are replaced with the correct Unicode codepoints in tty output.
- **SC-007**: All new commands pass `go test -race ./...` with zero failures.
- **SC-008**: `golangci-lint run` passes with zero new issues.
