# Feature Specification: Resolve User IDs and Channel IDs in Output

**Feature Branch**: `003-resolve-ids-in-output`
**Created**: 2026-03-03
**Status**: In Progress (US4 pending)
**Input**: User description: "003 I would like for all user IDs and Channel IDs to get resolved in our output."

## User Scenarios & Testing *(mandatory)*

### User Story 1 — User IDs Replaced by Real Names in Message Output (Priority: P1)

A developer runs `slackseek history #general` and sees messages where the "User" column
shows a human-readable real name like `Alice Smith` instead of the raw Slack user ID
`U01234567`. The same applies to `slackseek messages <user>` and `slackseek search <query>`.
Inline `<@USERID>` mention tokens inside message text are also resolved to `@Real Name`.

**Why this priority**: Raw Slack user IDs (e.g. `U01234567`) are opaque and useless to
humans. Every command that shows messages displays these IDs. Resolving them to real names
is the highest-value change and delivers the core of the feature on its own.

**Independent Test**: Run `slackseek history #general` (with a populated user cache). Verify
that the User column/field contains a real name (e.g. `Alice Smith`) rather than a `U…` ID,
and that `<@USERID>` tokens in the message text are replaced with `@Real Name`.

**Acceptance Scenarios**:

1. **Given** the user cache is populated, **When** `history`, `messages`, or `search` output
   is rendered in text or table format, **Then** the UserID field is replaced by the user's
   real name (falling back to display name if real name is empty, then to raw ID).

2. **Given** a message body contains `<@USERID>` mention tokens, **When** output is rendered
   and the resolver is available, **Then** each token is replaced by `@Real Name` (falling
   back to `@USERID` for unresolved IDs).

3. **Given** a user ID is not present in the cache (e.g. a deleted or unknown user), **When**
   output is rendered, **Then** the raw user ID is shown as-is so output is never broken.

4. **Given** the user requests `--format json`, **When** output is rendered, **Then** the JSON
   payload includes both the original `user_id` and a new `user_display_name` field so
   downstream tools still have access to the raw ID. Inline mentions in `text` are also
   resolved in JSON output.

---

### User Story 2 — Channel IDs Replaced by Channel Names in History Output (Priority: P2)

A developer runs `slackseek history #general` or `slackseek messages alice` and sees the
channel column showing `#general` instead of `C01234567`. This is relevant primarily for
the `messages` command (which searches across channels) and the JSON output of all
message-bearing commands.

**Why this priority**: The history command already knows its channel and the search command
already receives the channel name from the API. The gap is the `messages` command, whose
output messages only carry a raw `ChannelID`. Resolving these IDs completes the picture.

**Independent Test**: Run `slackseek messages alice --format table`. Verify the Channel
column shows `#general` (or the channel name) rather than `C01234567`.

**Acceptance Scenarios**:

1. **Given** the channel cache is populated, **When** `messages` output is rendered and a
   message has a raw `ChannelID`, **Then** the channel column shows the resolved channel name.

2. **Given** a channel ID is not in the channel cache, **When** output is rendered, **Then**
   the raw channel ID is shown as a fallback.

3. **Given** the user requests `--format json`, **When** output is rendered for messages,
   **Then** the JSON payload includes both `channel_id` and a populated `channel_name` field.

---

### User Story 3 — Graceful Degradation When Cache Is Empty or Unavailable (Priority: P3)

A developer runs any command with `--no-cache`. Resolution silently falls back to raw IDs
so output is never broken and there are no unexpected API calls on account of ID resolution.

**Why this priority**: The resolution must not introduce new failure modes or silent API calls.
The degradation behaviour makes the feature safe to deploy even in environments with no cache.

**Independent Test**: Run `slackseek history #general --no-cache`. Verify output succeeds
and UserID/ChannelID fields contain raw IDs (no panic, no extra API call).

**Acceptance Scenarios**:

1. **Given** `--no-cache` is passed, **When** any command runs, **Then** output is rendered
   with raw IDs; no extra API call is made to resolve them.

2. **Given** the cache is stale and `--refresh-cache` is NOT passed, **When** output is
   rendered, **Then** the existing (potentially stale) cache is still used for resolution.

---

### User Story 4 — User Group `<!subteam^ID>` Resolved to Group Handle (Priority: P4)

A developer runs any message command and sees `<!subteam^SSRHMQ1NC>` (a user-group mention
without an embedded label) replaced with `@eng-team` (the group's handle from Slack) instead
of the placeholder `@[group]`.

**Why this priority**: `<!subteam^ID>` tokens without an embedded label currently fall back
to `@[group]`, which is uninformative. Fetching user groups and caching them alongside users
and channels closes this gap. User groups are less critical than user/channel resolution (US1
and US2) so this is a lower-priority enhancement.

**Independent Test**: Run any message command against a channel containing a user-group
mention `<!subteam^KNOWN_ID>` with no embedded label. Verify the output shows `@handle`
(the group's Slack handle) instead of `@[group]`.

**Acceptance Scenarios**:

1. **Given** the user-groups cache is populated, **When** a message contains
   `<!subteam^ID>` with no embedded label, **Then** the output shows `@handle`
   (resolved from the groups cache), falling back to `@[group]` if the ID is unknown.

2. **Given** a message contains `<!subteam^ID|@label>` (embedded label present), **When**
   output is rendered, **Then** the embedded label is used regardless of the cache
   (existing behaviour preserved).

3. **Given** `--no-cache` is passed, **When** any command runs, **Then** user-group
   resolution is suppressed (nil resolver) and `<!subteam^ID>` without a label becomes
   `@[group]` as before — no extra API call.

4. **Given** `ListUserGroups` returns an error, **When** `buildResolver` runs, **Then** a
   warning is written to stderr and the resolver is built without group data; all other
   resolution (users, channels) still works.

---

### Edge Cases

- What if a user's real name is empty? Fall back to `display_name`; if that is also empty,
  fall back to the raw user ID.
- What about `<@USERID>` mention tokens? `Resolver.ResolveMentions` replaces them with
  `@Real Name`; unresolvable IDs become `@USERID`.
- What about `<!subteam^ID|handle>` user-group tokens? The embedded label is used when
  present (e.g. `@eng-team`); when the label is absent, the groups cache is consulted
  (US4); if still unresolved, the token becomes `@[group]`.
- What about `<!here>` / `<!channel>` / `<!everyone>`? Replaced with `@here`, `@channel`,
  `@everyone` respectively.
- What if the channel list does not include a DM/MPIM channel ID? Raw channel ID is shown.
- What happens when resolution adds latency? Resolution uses only the already-fetched,
  in-memory cached list — it is a map lookup, O(1) per message, with no network calls.
- What happens with the text format for messages that already have `ChannelName` set (search)?
  Prefer the existing non-empty `ChannelName`; only overwrite if it is empty.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: All text/table output for messages (history, messages, search) MUST display the
  user's real name (falling back to display name, then raw ID) instead of the raw Slack user ID.
- **FR-002**: All text/table output for messages MUST display the channel name instead of the
  raw Slack channel ID when a channel name is not already populated.
- **FR-003**: JSON output MUST include both the original `user_id` and a new
  `user_display_name` field in message and search-result payloads.
- **FR-004**: JSON output MUST include a populated `channel_name` field wherever a
  `channel_id` is present and can be resolved.
- **FR-005**: Resolution MUST use the same cached `channels` and `users` lists loaded by the
  existing `internal/cache` store; no additional API calls may be made solely for resolution.
- **FR-006**: If a user ID or channel ID cannot be resolved from the in-memory data, the raw
  ID MUST be shown as a fallback without error.
- **FR-007**: The `--no-cache` flag MUST suppress ID resolution (no API calls made) and raw
  IDs MUST appear in output as before.
- **FR-008**: Resolution logic MUST be encapsulated in `internal/slack` as a `Resolver` type
  and MUST NOT be duplicated across `cmd/` files.
- **FR-009**: Inline Slack mention tokens in message text MUST be resolved in all output
  formats when a resolver is available:
  - `<@USERID>` → `@Real Name` (falls back to `@USERID` if unresolved)
  - `<!subteam^ID|@handle>` → `@handle` (embedded label always used when present)
  - `<!subteam^ID>` → `@handle` from groups cache (see FR-010); falls back to `@[group]`
  - `<!here>`, `<!channel>`, `<!everyone>` → `@here`, `@channel`, `@everyone`
- **FR-010**: `buildResolver` MUST call `c.ListUserGroups(ctx)` and pass the resulting
  `[]UserGroup` to `NewResolver` as a third argument. `Resolver` MUST maintain a
  `groups map[string]string` (groupID → handle) and use it in `ResolveMentions` to resolve
  `<!subteam^ID>` tokens that lack an embedded label. On error, a warning is written to
  stderr, group resolution is skipped, and all other resolution continues normally.
  User groups MUST be cached under the key `"user_groups"` using the existing
  `internal/cache` store.

### Key Entities

- **UserGroup** (`internal/slack/types.go`): `ID string`, `Handle string`, `Name string`.
  Populated from `usergroups.list` Slack API endpoint.
- **Resolver**: Holds in-memory lookup maps (userID→realName, channelID→name,
  groupID→handle) built from `[]User`, `[]Channel`, and `[]UserGroup` slices. Constructor:
  `NewResolver(users []User, channels []Channel, groups []UserGroup) *Resolver`. Methods:
  `UserDisplayName(id string) string`, `ChannelName(id string) string`, and
  `ResolveMentions(text string) string`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `history`, `messages`, and `search` table/text output never contains bare
  `U…` or `C…` IDs when the user/channel is present in the cache.
- **SC-002**: Resolution adds zero API calls; confirmed by unit tests with mock data.
- **SC-003**: JSON output includes both `user_id` and `user_display_name` fields.
- **SC-004**: All existing `go test -race ./...` tests continue to pass unmodified.
- **SC-005**: `--no-cache` invocations produce raw IDs with no errors or panics.
- **SC-006**: All Slack mention tokens in message text are resolved in all output formats:
  `<@USERID>` → `@Real Name`; `<!subteam^…>` → label or `@[group]`;
  `<!here/channel/everyone>` → `@here/channel/everyone`.
- **SC-007**: `<!subteam^ID>` tokens without an embedded label resolve to `@handle` when
  the group ID is present in the user-groups cache; they fall back to `@[group]` otherwise.
