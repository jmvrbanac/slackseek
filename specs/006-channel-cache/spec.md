# Feature Specification: Long-Term Day-History Cache

**Feature Branch**: `006-channel-cache`
**Created**: 2026-03-06
**Status**: Draft
**Input**: Ideation document `006-channel-cache.md` + user decisions

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Cache hit skips Slack API for completed past days (Priority: P0)

A user runs `slackseek history general --from 2026-03-01 --to 2026-03-01` on 2026-03-06.
The first call fetches from the API. The second call returns instantly from disk without
any HTTP request.

**Why this priority**: Core value proposition — every repeated past-day query costs zero
API calls and responds in milliseconds instead of seconds.

**Independent Test**: Run history twice for a past day; assert zero API calls on second run.

**Acceptance Scenarios**:

1. **Given** a completed past day with no cache entry, **When** `history` runs,
   **Then** messages are fetched from Slack, saved to disk, and returned.
2. **Given** the same day is requested again, **When** `history` runs,
   **Then** messages are returned from disk with no API call.
3. **Given** the day is today (not yet complete), **When** `history` runs,
   **Then** no cache entry is written and every run fetches from the API.
4. **Given** `--limit 100` truncates the result (there are more messages than the limit),
   **When** `history` runs, **Then** no cache entry is written.
5. **Given** `--limit 0` (unlimited) or `len(fetched) < limit`,
   **When** `history` runs for a past day, **Then** a cache entry is written.

---

### User Story 2 — `--no-cache` forces a fresh API fetch and refreshes entry (Priority: P1)

A user knows some messages were deleted or the cache is stale and wants to re-fetch.

**Acceptance Scenarios**:

1. **Given** a cached past-day entry, **When** `--no-cache` is passed,
   **Then** the API is called and the cache entry is overwritten with fresh data.
2. **Given** no cache entry, **When** `--no-cache` is passed,
   **Then** the command behaves identically to the no-cache-flag case (API fetch + write).
3. **Given** `--no-cache` is NOT passed, **When** a cache hit exists,
   **Then** no API call is made.

---

### User Story 3 — `postmortem` and `metrics` also benefit transparently (Priority: P1)

A user generates a postmortem for `#incidents` on 2026-03-01, then runs `metrics` on the
same channel and date. The second command (and all subsequent) serve from cache.

**Acceptance Scenarios**:

1. **Given** a `postmortem` run that writes a cache entry, **When** `metrics` runs
   on the same channel and date, **Then** it reads from cache (no second API call).
2. **Given** `postmortem --no-cache` is passed, **Then** the cache entry is refreshed.
3. **Given** `metrics --no-cache` is passed, **Then** the cache entry is refreshed.

---

### User Story 4 — Cache survives across `slackseek` invocations (Priority: P0)

Cache entries persist on disk under `os.UserCacheDir()/slackseek/{wsKey}/history/{channelID}/{date}.json`.

**Acceptance Scenarios**:

1. **Given** a cache entry written in one session, **When** the binary is invoked fresh,
   **Then** the cache entry is found and served.
2. **Given** `slackseek cache clear`, **When** run, **Then** all day-history entries
   are removed (the whole workspace subdirectory is deleted, as today).

---

## Requirements

### Functional

- R1: Cache key = `workspaceKey / "history" / channelID / YYYY-MM-DD`
- R2: A day is cacheable when: (a) From and To are both set, (b) same calendar day,
  (c) To < now, (d) result not truncated by limit.
- R3: Caching applies only when `threads=true` (default). threads=false requests bypass cache.
- R4: `--no-cache` flag on `history`, `postmortem`, `metrics` skips load, always writes.
- R5: `digest` and `actions` are out of scope for v1 (different data paths).
- R6: Cache store uses a new `LoadStable`/`SaveStable` pair that bypasses TTL.
- R7: Cache failures (disk full, permission error) MUST NOT fail the command — warn to stderr.

### Non-Functional

- N1: Cache load adds <1ms overhead on a hit.
- N2: No new dependencies; stdlib only.
- N3: All caching logic is unit-testable without a Slack connection.
- N4: Cache entries are valid JSON (`[]slack.Message`), identical in shape to the API response.
