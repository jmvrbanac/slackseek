# Feature Specification: slackseek CLI

**Feature Branch**: `001-slackseek-cli`
**Created**: 2026-03-02
**Status**: Draft
**Input**: User description: "./spec.md"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Verify Local Slack Session (Priority: P1)

A developer or power user wants to confirm that their locally installed Slack
desktop application's authentication credentials are accessible before running
any other commands. They run a verification command that displays discovered
workspaces, token identifiers, and session cookie snippets — all without making
any network calls.

**Why this priority**: Every other command depends on local credential access.
If tokens cannot be discovered, no other feature provides value. This story is
the minimum viable foundation.

**Independent Test**: Can be fully tested by opening a terminal on a machine
with Slack installed and logged in, running the auth verification command, and
confirming that at least one workspace name, a truncated token, and a truncated
cookie value appear in the output.

**Acceptance Scenarios**:

1. **Given** Slack is installed and the user is logged in to one or more
   workspaces, **When** the user runs the auth verification command,
   **Then** the tool displays each workspace name, URL, a partial token (not
   the full secret), and a partial session cookie snippet.

2. **Given** the user wants to script credential access, **When** the user runs
   the auth export command, **Then** the tool prints shell-compatible variable
   assignment statements (one per workspace) suitable for use with `eval`.

3. **Given** Slack is not installed or the user is not logged in,
   **When** the user runs any auth command, **Then** the tool exits with a
   non-zero status code and prints an actionable error message explaining what
   is missing and how to resolve it.

---

### User Story 2 - Search Workspace Messages (Priority: P2)

A developer or analyst wants to search across all Slack messages they have
access to using keywords, optionally narrowed by date range, channel, or
message author. Results are returned in a scriptable format.

**Why this priority**: Full-text search is the highest-value read operation.
It allows users to locate specific conversations without needing to know the
exact channel or time, making it immediately useful for auditing, research, or
reference.

**Independent Test**: Can be fully tested by running a search for a known
keyword that appears in at least one message, with and without date range
constraints, and verifying that matching messages are returned with timestamp,
channel name, author, and text visible.

**Acceptance Scenarios**:

1. **Given** the user provides a search query, **When** the search command runs,
   **Then** results include the message text, channel, author, and timestamp for
   each match, paginating automatically until the requested limit is reached.

2. **Given** the user provides `--from` and `--to` date flags in addition to
   a query, **When** the search command runs, **Then** only messages within that
   date range are returned.

3. **Given** the user provides a `--channel` or `--user` filter alongside the
   query, **When** the search command runs, **Then** results are narrowed to
   messages from that channel or author only.

4. **Given** the user specifies `--format json`, **When** the search command
   runs, **Then** output is a JSON array of result objects suitable for
   piping into other tools.

---

### User Story 3 - Retrieve Channel Message History (Priority: P3)

A developer or analyst wants to pull the full message history of a specific
Slack channel — including threaded replies — over a given date range, for
archival, auditing, or analysis purposes.

**Why this priority**: Channel history retrieval is a high-value capability for
compliance, retrospectives, and team analytics. It builds directly on the auth
foundation.

**Independent Test**: Can be fully tested by retrieving history for a known
channel with a bounded date range, verifying that messages appear in
chronological order and that threaded replies are inlined beneath their parent
messages.

**Acceptance Scenarios**:

1. **Given** a valid channel name or ID and optional date bounds,
   **When** the history command runs, **Then** all messages in the date range
   appear in chronological order with timestamp, author, and text.

2. **Given** the `--threads` flag is active (default on), **When** a message
   has replies, **Then** replies appear annotated with depth directly after the
   parent message.

3. **Given** a `--limit` value is set, **When** the result set exceeds the
   limit, **Then** the tool returns exactly that many messages and stops.

4. **Given** an invalid channel name is provided, **When** the history command
   runs, **Then** the tool exits with a non-zero status and an error message
   identifying the unresolvable channel.

---

### User Story 4 - Aggregate Messages by User (Priority: P3)

A developer or analyst wants to see all messages sent by a specific Slack user
across all accessible channels and optionally within a specific date range or
channel.

**Why this priority**: Per-user message aggregation supports team analytics,
onboarding retrospectives, and compliance reviews. It shares infrastructure with
search but provides a targeted, user-centric view.

**Independent Test**: Can be fully tested by querying messages for a known
active user and confirming that results include messages from multiple channels,
with timestamp, channel, and text visible.

**Acceptance Scenarios**:

1. **Given** a valid user display name, real name, or ID,
   **When** the messages command runs, **Then** all accessible messages from
   that user appear with timestamp, channel name, and text.

2. **Given** a `--channel` flag is provided, **When** the messages command
   runs, **Then** only that user's messages within the specified channel are
   returned.

3. **Given** `--from`/`--to` date flags are provided, **When** the messages
   command runs, **Then** only messages within the date range are included.

---

### User Story 5 - Browse Workspace Resources (Priority: P4)

A developer wants to list all channels and users in a workspace to discover
IDs, types, and metadata before running more targeted commands.

**Why this priority**: Discovery commands make other commands more useful by
providing the IDs and names needed for filtering. They are lower priority
because they complement rather than deliver standalone value.

**Independent Test**: Can be fully tested by listing channels and users for a
logged-in workspace and verifying that at least the expected channel names and
user display names appear in the output.

**Acceptance Scenarios**:

1. **Given** the user runs the channel listing command, **Then** each channel's
   ID, name, type (public/private/group DM/direct message), member count, and
   topic appear in the output.

2. **Given** the user runs the user listing command, **Then** each user's ID,
   display name, real name, and bot/active status appear in the output.

3. **Given** filter flags (by channel type, archived status, deleted users, or
   bot accounts) are provided, **When** a list command runs, **Then** only
   matching records are returned.

---

### Edge Cases

- What happens when multiple workspaces are detected and no workspace selector
  is provided? The tool selects the first discovered workspace, prints a
  one-line notice identifying it, and notes that others are available.
- What happens when the start date is later than the end date? The tool exits
  immediately with a validation error before making any external requests.
- What happens when the workspace API returns a rate-limit response? The tool
  retries automatically with exponential backoff up to three times before
  surfacing an error to the user.
- What happens when Slack is running and holds a lock on the local credential
  store? The tool works around the lock by reading from a temporary copy of the
  store, so the running Slack application does not block the tool.
- What happens when the auth command is run on an unsupported operating system?
  The tool exits with a clear message stating which operating systems are
  supported.
- What happens when a channel name resolves to multiple channels? The tool
  reports the ambiguity and asks the user to use the channel ID instead.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The tool MUST extract workspace tokens and session cookies from
  the locally installed Slack desktop application without requiring any
  cooperation from Slack or a network connection.
- **FR-002**: The tool MUST support Linux and macOS as target platforms; each
  platform's credential storage location MUST be resolved automatically based
  on the host operating system.
- **FR-003**: The tool MUST expose a workspace selector flag accepting either a
  workspace name or URL; when absent, the first discovered workspace MUST be
  used and identified in a one-line notice.
- **FR-004**: Every command MUST support three output formats — human-readable
  text, aligned table, and JSON array — selectable via a global format flag.
- **FR-005**: Every read command MUST support start and end date range flags
  accepting both `YYYY-MM-DD` and full RFC 3339 timestamps; when a time
  component is absent, start-of-day UTC MUST be assumed.
- **FR-006**: The search command MUST support full-text keyword search across
  all accessible messages, with optional narrowing by channel and author.
- **FR-007**: The history command MUST retrieve messages for a named or
  ID-identified channel, with inline thread replies toggled on by default and
  a configurable message limit.
- **FR-008**: The messages command MUST aggregate all messages from a named or
  ID-identified user across all accessible channels, with optional channel and
  date constraints.
- **FR-009**: The channels command MUST list accessible channels supporting
  type filtering (public, private, group DM, direct message) and an archived
  inclusion toggle.
- **FR-010**: The users command MUST list workspace members with optional
  inclusion of deactivated accounts and bot accounts.
- **FR-011**: The auth command MUST provide a `show` subcommand displaying
  workspace and credential summaries, and an `export` subcommand emitting
  shell-compatible variable assignment statements.
- **FR-012**: The tool MUST NOT persist credentials to disk; tokens MUST be
  re-extracted from local storage on every invocation.
- **FR-013**: When the workspace API returns a rate-limit response, the tool
  MUST retry with exponential backoff for up to three attempts before surfacing
  an error.
- **FR-014**: All error messages MUST identify what failed, why it likely
  failed, and what the user can do to resolve the situation.
- **FR-015**: When start date is later than end date, the tool MUST exit with
  a validation error before making any external requests.

### Key Entities

- **Workspace**: A Slack workspace the user is authenticated to. Attributes:
  name, URL, authentication token (never displayed in full), session cookie.
- **Channel**: A Slack conversation space. Attributes: ID, name, type
  (public/private/group DM/direct message), member count, topic, archived
  status.
- **Message**: A single Slack message. Attributes: timestamp, author user ID,
  text content, thread parent reference, thread depth, reactions, channel ID.
- **User**: A Slack workspace member. Attributes: ID, display name, real name,
  email address, bot flag, active/deleted status.
- **SearchResult**: A message returned by full-text search. Attributes: all
  Message attributes plus permalink URL.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user with Slack installed and a valid session can discover their
  workspace credentials and confirm access in under 5 seconds on first run.
- **SC-002**: A full-text search returning up to 100 results completes and
  displays output in under 30 seconds on a standard broadband connection.
- **SC-003**: Channel history retrieval for a 30-day range with up to 1,000
  messages (including thread replies) completes without manual intervention.
- **SC-004**: All commands produce valid, parseable JSON output when the JSON
  format is selected, enabling downstream processing by standard data tools.
- **SC-005**: The tool runs correctly on both Linux and macOS without requiring
  the user to configure platform-specific paths or install additional software
  beyond the binary itself.
- **SC-006**: All error conditions produce a non-zero exit code and a message
  that a non-expert user can act upon without consulting documentation.
- **SC-007**: The date range validation rejects invalid ranges before any
  external call is made, preventing unnecessary network requests.
