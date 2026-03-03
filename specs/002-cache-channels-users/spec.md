# Feature Specification: Channel and User List Caching

**Feature Branch**: `002-cache-channels-users`
**Created**: 2026-03-03
**Status**: Draft
**Input**: User description: "002 We need to cache the channel and user lists. They're too big to fetch and they keep getting rate limited"

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Transparent Cache Hit Speeds Up Commands (Priority: P1)

A developer runs `slackseek history #general` or `slackseek messages alice` for the
first time. On first run the tool fetches channels/users from the Slack API and stores
them on disk. On subsequent runs within the cache TTL the tool reads from disk and
never touches the API for those two lists, so the command completes dramatically faster
and never triggers a rate limit on account of list fetches.

**Why this priority**: Every command that resolves a channel name or user name incurs a
full paginated `conversations.list` or `users.list` call. For large workspaces these
calls dominate total runtime and exhaust the Slack API tier-2 rate limit. Removing this
bottleneck unlocks all downstream commands.

**Independent Test**: Run `slackseek channels list` twice. First run hits the network;
second run reads from disk. Confirm second run completes faster and the cache file
exists at the expected path.

**Acceptance Scenarios**:

1. **Given** no cache exists, **When** a name-resolving command runs,
   **Then** the tool fetches from the API, writes a cache file, and completes normally.

2. **Given** a fresh cache file exists, **When** the same command runs again within
   the TTL, **Then** the tool reads from disk, makes zero API calls for channels/users,
   and produces identical output.

3. **Given** the cache file is older than the TTL, **When** any command runs,
   **Then** the tool refetches from the API, overwrites the stale cache file, and
   completes normally.

---

### User Story 2 — Force Refresh When Workspace Changes (Priority: P2)

A developer adds a new channel or user, or switches workspaces, and wants to ensure the
tool reflects the latest state. They either wait for the TTL to expire or explicitly
pass `--refresh-cache` to bypass the cache for that single invocation.

**Why this priority**: Without a manual bypass, users would have to wait for the TTL to
see new channels or users. This is high friction when channels are added frequently.

**Independent Test**: Populate the cache. Run any command with `--refresh-cache`. Verify
a new network call was made (visible via a rate-limit notice or timing) and the cache
file modification time is updated.

**Acceptance Scenarios**:

1. **Given** a valid cache exists, **When** the user passes `--refresh-cache`,
   **Then** the tool ignores the on-disk cache, fetches fresh data, and rewrites the
   cache file.

2. **Given** `--no-cache` is passed, **When** the command runs,
   **Then** no cache is read and no cache is written; the raw API results are used.

---

### User Story 3 — Cache Clear Command (Priority: P3)

A developer wants to wipe all cached data for a workspace (or all workspaces) without
running a read command. They run `slackseek cache clear`.

**Why this priority**: A dedicated housekeeping command is useful for scripted
environments, debugging, or when switching Slack accounts entirely.

**Independent Test**: Populate cache files, run `slackseek cache clear`, verify files
are gone.

**Acceptance Scenarios**:

1. **Given** cache files exist, **When** the user runs `slackseek cache clear`,
   **Then** all cache files for the selected workspace are deleted.

2. **Given** no cache files exist, **When** the user runs `slackseek cache clear`,
   **Then** the command succeeds with a message indicating nothing was cleared.

---

### Edge Cases

- What happens when the cache directory is unwritable? The tool logs a warning and
  falls through to the API; the cache miss is transparent to the user.
- What happens when the cache file is corrupt (invalid JSON)? The tool treats it as a
  cache miss, refetches from the API, and overwrites the corrupt file.
- What happens when multiple concurrent invocations of `slackseek` run simultaneously?
  Both may fetch from the API and write the cache; the last writer wins. This is
  acceptable since the tool is a single-user CLI and Slack data is idempotent within a
  short window.
- What happens when `--cache-ttl 0` is passed? Zero duration disables caching entirely
  (equivalent to `--no-cache`).
- What happens when the workspace URL changes (e.g., workspace renamed)? The old cache
  directory is orphaned; `slackseek cache clear --all` removes all workspace caches.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The tool MUST cache the results of `conversations.list` and `users.list`
  to a per-workspace directory on disk, using JSON serialisation.
- **FR-002**: The cache directory MUST be `$XDG_CACHE_HOME/slackseek` on Linux and
  `~/Library/Caches/slackseek` on macOS, resolved via `os.UserCacheDir()`.
- **FR-003**: Cached data MUST be namespaced per workspace using a deterministic,
  filesystem-safe key derived from the workspace URL.
- **FR-004**: The cache TTL MUST default to 24 hours and MUST be overridable via a
  global `--cache-ttl` flag accepting Go duration syntax (e.g., `1h`, `30m`, `0`).
- **FR-005**: A `--refresh-cache` flag MUST bypass the on-disk cache and force a new
  API fetch, then overwrite the cache with the fresh result.
- **FR-006**: A `--no-cache` flag MUST bypass the cache entirely for that invocation
  (neither read nor write).
- **FR-007**: The tool MUST gracefully handle cache read/write failures by falling
  through to the API and printing a warning to stderr.
- **FR-008**: Corrupt cache files (unparseable JSON) MUST be treated as cache misses;
  the tool MUST silently overwrite them.
- **FR-009**: A `slackseek cache clear` command MUST delete all cache files for the
  currently selected workspace (or `--all` for all workspaces).
- **FR-010**: All existing commands that resolve channel names or user names MUST
  benefit from the cache transparently, with no change to their flags or output.

### Key Entities

- **CacheStore**: On-disk storage for serialised entity lists. Attributes: base
  directory, TTL, workspace key (hex digest of workspace URL).
- **CacheEntry**: A single cached list file. Attributes: workspace key, entity type
  (`channels` | `users`), serialised JSON payload, file modification time (TTL source).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Second and subsequent invocations of name-resolving commands complete in
  under 1 second for the channel/user resolution step (vs. up to 30 s on a large
  workspace).
- **SC-002**: Zero API calls are made for `conversations.list` or `users.list` when the
  cache is valid, confirmed by integration test with a mock Slack server.
- **SC-003**: All existing `go test -race ./...` tests continue to pass with no
  modifications to existing test fixtures.
- **SC-004**: The cache gracefully degrades: if the cache directory is unwritable or
  the file is corrupt, the tool still succeeds (falls back to API).
- **SC-005**: `slackseek cache clear` deletes all cache files and exits with code 0.
