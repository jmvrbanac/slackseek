# Feature Specification: Lazy Entity Cache (Fetch-on-Miss)

**Feature Branch**: `008-lazy-entity-cache`
**Created**: 2026-03-13
**Status**: Draft
**Input**: Reduce entity cache refresh: switch from TTL-based expiry to fetch-on-miss for users/channels/groups.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Warm Cache Stays Valid Indefinitely (Priority: P1)

A user runs slackseek commands repeatedly over days and weeks. Their local user and
channel cache is never automatically invalidated. As long as every user and channel
referenced in the output is already in the cache, no API calls are made to refresh
entity lists — commands start immediately and resolve all names correctly.

**Why this priority**: This is the common case (no new entities). It delivers the
primary benefit: eliminating unnecessary daily list re-fetches entirely.

**Independent Test**: Populate the entity cache, advance the system clock past 24 hours,
run a history or search command. Verify no `users.list` or `conversations.list` API
call is made and all names resolve correctly.

**Acceptance Scenarios**:

1. **Given** a populated entity cache older than 24 hours, **When** a command runs and all IDs in the output are present in the cache, **Then** no entity list API call is made and all names resolve correctly.
2. **Given** a populated entity cache of any age, **When** `--refresh-cache` is NOT passed, **Then** the cache file is not replaced unless a miss occurs.

---

### User Story 2 - New User Resolves Transparently on First Miss (Priority: P1)

A new colleague joins the Slack workspace. The next time a user runs a slackseek command
that includes a message from that new user, slackseek fetches only that user's profile
(not the entire user list), adds it to the local cache, and displays the correct name
in the output — all within the same command invocation.

**Why this priority**: Core correctness requirement. Without this, new workspace members
permanently show as raw IDs until `cache clear` is run manually.

**Independent Test**: Add a user ID to message output that is absent from the cache.
Verify a single targeted user lookup is made (not a full user list fetch), the name
resolves correctly in output, and the cache file is updated with the new entry.

**Acceptance Scenarios**:

1. **Given** an entity cache that does not contain user ID `UABC123`, **When** a command renders output that includes a message from `UABC123`, **Then** slackseek fetches only that user's profile, resolves the display name, and the output shows the real name.
2. **Given** the above scenario, **When** the same command runs again immediately after, **Then** no additional API call is made (the newly fetched user is now in cache).
3. **Given** a targeted user fetch that returns an error, **When** the ID cannot be resolved, **Then** the raw ID is shown in output and no error is surfaced to the user.

---

### User Story 3 - New Channel Resolves Transparently on First Miss (Priority: P2)

A user references a channel that was created after the entity cache was last populated.
slackseek fetches only that channel's info, merges it into the cache, and displays the
correct channel name — without re-fetching all channels.

**Why this priority**: Same correctness need as user miss, but channels change less
frequently than users. P2 because it is less common in practice.

**Independent Test**: Reference a channel ID absent from the cache. Verify a single
targeted channel lookup is made, the channel name resolves correctly, and the cache
file is updated.

**Acceptance Scenarios**:

1. **Given** an entity cache that does not contain channel ID `CABC123`, **When** output includes a reference to `CABC123`, **Then** slackseek fetches only that channel's info, resolves the name, and updates the cache.
2. **Given** the above scenario completed, **When** the channel ID appears again in subsequent output within the same invocation, **Then** the already-fetched name is reused without a second API call.

---

### User Story 4 - User Group Miss Triggers Full Group Refresh (Priority: P3)

A new Slack user group (`<!subteam^...>`) appears in message text. Because Slack has no
per-group lookup endpoint, slackseek refreshes the entire user groups list, updates the
cache, and resolves the group handle in the current invocation's output.

**Why this priority**: Group mentions are rare. The full list refresh is acceptable
given the API limitation.

**Independent Test**: Include a subteam token in output where the group ID is absent
from the cache. Verify the full groups list is fetched once, the group handle resolves,
and the cache is updated.

**Acceptance Scenarios**:

1. **Given** a cache missing group ID `SABC123`, **When** output contains a mention of `SABC123`, **Then** the full user group list is fetched once, the handle resolves, and the cache is updated.
2. **Given** multiple missing group IDs in the same invocation, **When** the first miss triggers a full refresh, **Then** no additional group list call is made for subsequent group misses in the same invocation.

---

### User Story 5 - Force Refresh with --refresh-cache (Priority: P2)

A user explicitly wants fresh entity data (e.g., a colleague changed their display name).
Passing `--refresh-cache` forces a full re-fetch of all entity lists and overwrites the
cache, regardless of its current state.

**Why this priority**: Escape hatch for cases where fetch-on-miss is insufficient (bulk
name changes, workspace reorganization).

**Independent Test**: Run any command with `--refresh-cache`. Verify all three entity
list fetches occur and the cache files are replaced.

**Acceptance Scenarios**:

1. **Given** a populated entity cache, **When** any command runs with `--refresh-cache`, **Then** all three entity lists are fetched fresh and the cache is overwritten.
2. **Given** a populated entity cache, **When** any command runs without `--refresh-cache`, **Then** no full list fetch occurs (only targeted miss fetches if needed).

---

### User Story 6 - Cold Start Populates Cache Normally (Priority: P1)

A user runs slackseek for the first time, or after `cache clear`. The entity cache files
are absent. slackseek performs the full initial population fetch (users, channels, groups)
exactly as before, and subsequent invocations use the lazy strategy.

**Why this priority**: Baseline correctness — the feature must not regress the initial
setup experience.

**Independent Test**: Remove cache files. Run any command. Verify all three entity lists
are fetched, cache files are created, and output resolves names correctly.

**Acceptance Scenarios**:

1. **Given** no entity cache files exist, **When** any command runs, **Then** all three entity lists are fetched, cache files are created, and all names resolve.
2. **Given** cache files were cleared with `cache clear`, **When** the next command runs, **Then** fresh entity data is fetched and cached.

---

### Edge Cases

- What happens when a targeted user or channel fetch fails mid-output rendering? The raw ID is shown; no error is surfaced to the user; the miss is not persisted to cache.
- What if the same unknown ID appears multiple times in the same output? Only one API call is made; the result is reused within the invocation.
- What if `--no-cache` is set? Entity resolution is skipped entirely (existing behaviour preserved).
- What if the cache file is corrupt or contains invalid JSON? Treat as a cold start; re-fetch the full list for that entity type.
- What if a targeted user fetch returns "user not found" (deleted/deactivated user)? Show the raw ID; do not write a miss entry to cache.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The entity cache (users, channels, user groups) MUST NOT expire automatically based on age; no TTL governs entity cache validity.
- **FR-002**: On a user ID miss, the system MUST fetch only that single user's profile and merge it into the cache within the current invocation.
- **FR-003**: On a channel ID miss, the system MUST fetch only that single channel's info and merge it into the cache within the current invocation.
- **FR-004**: On a user group ID miss, the system MUST fetch the full user group list, update the cache, and not repeat the fetch within the same invocation regardless of how many group misses occur.
- **FR-005**: The `--refresh-cache` flag MUST cause a full re-fetch of all entity lists, overwriting the cache, in any command that uses entity resolution.
- **FR-006**: When no entity cache files exist, the system MUST perform a full initial population fetch for all entity types before resolving IDs.
- **FR-007**: If a targeted single-entity fetch fails, the system MUST fall back to displaying the raw ID without surfacing an error to the user.
- **FR-008**: Within a single command invocation, repeated references to the same previously-unknown ID MUST result in at most one API call for that ID.
- **FR-009**: The `--no-cache` flag MUST continue to bypass entity resolution entirely (existing behaviour preserved).
- **FR-010**: The `cache clear` command MUST continue to remove all entity cache files, triggering a fresh full fetch on the next invocation.
- **FR-011**: The `--cache-ttl` flag MUST be retained for backward compatibility but has no effect on entity (user/channel/group) cache behaviour under this design.

### Key Entities

- **User entity**: A workspace member identified by a Slack user ID. Attributes: ID, display name, real name. Accumulated in the local users cache.
- **Channel entity**: A workspace channel or conversation identified by a Slack channel ID. Attributes: ID, name, type. Accumulated in the local channels cache.
- **User group entity**: A Slack subteam identified by a group ID. Attributes: ID, handle, name. Stored in the local user groups cache (always replaced in full on refresh).
- **Entity cache**: A local file store holding the accumulated set of resolved entities per workspace. Grows incrementally via targeted fetches; replaced only on full refresh or cold start.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Commands that reference only already-cached entities make zero entity list API calls, regardless of cache age.
- **SC-002**: Commands that encounter one previously-unseen user make exactly one targeted user lookup — not a full user list fetch — and display the resolved name in the same invocation.
- **SC-003**: Commands that encounter one previously-unseen channel make exactly one targeted channel lookup — not a full channel list fetch — and display the resolved name in the same invocation.
- **SC-004**: After any targeted miss fetch, the same ID resolves from cache on the next invocation with no API call.
- **SC-005**: The `--refresh-cache` flag triggers exactly one full list fetch per entity type (users, channels, groups) per invocation and no targeted fetches.
- **SC-006**: Cold-start behaviour (no cache files present) is functionally identical to the existing implementation.

## Assumptions

- Slack user and channel IDs are immutable; only names/handles can change. Targeted fetches are therefore safe — an ID fetched once is valid forever.
- The Slack API's single-entity lookup endpoints are available with the token types already supported by slackseek (xoxc, xoxp).
- There is no Slack API endpoint for fetching a single user group by ID; the full group list is the only option.
- "Merge into cache" means: load the current cache list from disk, append or replace the entry for the fetched ID, and write the updated list back to disk atomically.
- The `--cache-ttl` flag becomes a no-op for entity caches under this design. It is retained for backward compatibility but has no effect on users/channels/groups.
