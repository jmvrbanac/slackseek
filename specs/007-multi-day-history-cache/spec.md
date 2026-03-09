# Feature Specification: Multi-Day History Cache

**Feature Branch**: `007-multi-day-history-cache`
**Created**: 2026-03-09
**Status**: Draft
**Input**: Ideation document `longer-history-fetch.md` + user decisions

## User Scenarios & Testing *(mandatory)*

### User Story 1 — First multi-day fetch caches each complete past day (Priority: P0)

A user runs `slackseek history general --since 2w` for the first time. The command makes
one bulk API call (paginated at 200 messages per page, same as today), then stores each
complete past day to disk. The response is returned in full.

**Why this priority**: The core value proposition — large historical fetches are expensive;
caching eliminates the cost on every subsequent run.

**Independent Test**: Mock `FetchHistory` to return a known multi-day message set; assert
that `SaveStable` is called once per complete past day and that the returned slice equals
all messages.

**Acceptance Scenarios**:

1. **Given** a multi-day range with no cache entries, **When** `history --since 2w` runs,
   **Then** a single bulk API fetch is made (one call per 200-message page), each complete
   past day is cached to disk, and all messages are returned.
2. **Given** some days in the range are already cached and others are not, **When** `history
   --since 2w` runs, **Then** only the uncached days form gap ranges for API calls; cached
   days are served from disk.
3. **Given** the range spans only one calendar day (existing single-day path), **When**
   `history` runs, **Then** the existing `fetchHistoryCachedInner` path is used unchanged.

---

### User Story 2 — Second run is near-instant for past days (Priority: P0)

A user repeats the same `--since 2w` fetch. All 13 complete past days load from disk
instantly; only today's messages are fetched from the API.

**Acceptance Scenarios**:

1. **Given** all past days are cached, **When** `history --since 2w` runs again,
   **Then** `FetchHistory` is called once (for today only) and all past days are served
   from disk.
2. **Given** today is the only uncached gap, **When** history runs, **Then** no cached
   day triggers an API call.

---

### User Story 3 — Overlapping windows reuse cache (Priority: P1)

A user runs `--since 2w` and later `--since 1w`. All 7 days of the shorter window are
already cached from the longer fetch; the second run only fetches today.

**Acceptance Scenarios**:

1. **Given** `--since 2w` was previously run, **When** `--since 1w` runs, **Then** all
   7 overlapping past days are served from cache.

---

### User Story 4 — Thread replies are cached with their root's day (Priority: P0)

A thread root message posted at 23:55 UTC on day 1 has a reply at 00:10 UTC on day 2.
Both the root and the reply are cached in day 1's cache file, not split across two days.

**Acceptance Scenarios**:

1. **Given** a root message on day 1 with a reply on day 2, **When** a multi-day fetch
   runs and results are bucketed, **Then** the reply is stored in day 1's cache file.
2. **Given** day 1 is loaded from cache, **When** results are returned, **Then** both
   the root and its cross-day reply are present in the output.

---

### User Story 5 — `--no-cache` forces full re-fetch (Priority: P1)

A user passes `--no-cache` to bypass cached days and refresh stale data.

**Acceptance Scenarios**:

1. **Given** all days are cached, **When** `--no-cache` is passed, **Then** all days
   are re-fetched from the API and all past-day caches are overwritten.
2. **Given** no cache exists, **When** `--no-cache` is passed, **Then** behaviour is
   identical to a normal first run.

---

### User Story 6 — `--limit` is applied after merging (Priority: P1)

A user passes `--limit 100 --since 2w`. The internal fetches and cache reads are
unlimited; the final merged slice is trimmed to 100 before returning.

**Acceptance Scenarios**:

1. **Given** `--limit 100` and a 2-week window with 500 messages, **When** history runs,
   **Then** each gap is fetched with `limit=0`, the merged result is trimmed to 100
   at return time, and each complete past day is cached in full (not truncated).

---

## Requirements

### Functional

- R1: When `dr` spans multiple calendar days, decompose into per-day cache lookups and
  gap ranges. Single-day ranges use the existing `fetchHistoryCachedInner` path unchanged.
- R2: Gap ranges are built by merging contiguous uncached past days. Today is always a gap.
- R3: Each gap is fetched with one paginated API call (`limit=0` internally).
- R4: After fetching a gap, partition messages into per-day buckets using a two-pass
  algorithm: (pass 1) map each root message's `Timestamp` to its UTC day; (pass 2) assign
  replies to their root's day via `msg.ThreadTS`.
- R5: Each complete past-day bucket is saved with `SaveStable`. Today's bucket is
  returned but never cached.
- R6: `--limit` is applied to the final merged `[]slack.Message` slice after all gaps
  are fetched and cached days are merged. Internal fetches always use `limit=0`.
- R7: `--no-cache` skips all `LoadStable` reads and `SaveStable` writes; every gap
  includes all days (no cached days are reused).
- R8: Open-ended ranges (nil `From` or nil `To`) fall through to a live API fetch
  with no caching attempt (existing behaviour preserved).
- R9: Cache failures (disk full, permission error) MUST NOT fail the command — warn
  to stderr.
- R10: `postmortem` and `metrics` benefit automatically; they call `FetchHistoryCached`
  with `limit=0` and `threads=true`, which satisfies all cacheability conditions for past
  days.

### Non-Functional

- N1: No new external dependencies; stdlib only for new code.
- N2: All new functions are unit-testable without a live Slack connection (injectable
  fetch function).
- N3: First run API call count is identical to the current non-caching path (one page
  fetch per 200 messages over the full range).
- N4: Cache entries are valid JSON (`[]slack.Message`), identical in shape to existing
  day-cache entries written by feature 006.
- N5: `go test -race ./...` passes with no data races on all new code.
