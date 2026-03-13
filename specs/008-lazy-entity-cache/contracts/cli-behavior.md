# CLI Behavior Contract: Lazy Entity Cache

This document defines observable behavior changes for slackseek commands after
implementing the lazy entity cache feature.

---

## Flag Behavior Changes

### `--cache-ttl <duration>` (global flag)

| Aspect | Before | After |
|---|---|---|
| Effect on users/channels/groups cache | Controls expiry; default 24h | No effect (entity caches never auto-expire) |
| Effect on history cache | Controls expiry | Unchanged |
| Valid values | Any non-negative duration | Unchanged |
| Error on negative value | Yes | Unchanged |
| `--cache-ttl 0` (disables caching) | Disables all caching | Unchanged (entity resolution also skipped) |

**Note**: The flag is retained for backward compatibility. Passing `--cache-ttl 12h`
produces no error but has no effect on when users/channels/groups are refreshed.

---

### `--refresh-cache` (global flag)

| Aspect | Before | After |
|---|---|---|
| Clears entity cache files | Yes (via workspace dir clear) | Unchanged |
| Triggers full entity list re-fetch | Yes (implicit, on next load) | Unchanged |
| Clears history cache | Yes (workspace dir clear removes all files) | Unchanged |

---

### `--no-cache` (global flag)

No behavior change. Entity resolution is still skipped when this flag is set.

---

## Entity Resolution Behavior

### When output contains a user ID already in cache

- **Before**: Resolved from cache if within TTL; full re-fetch if TTL expired.
- **After**: Resolved from cache always (no TTL check). No API call made.

### When output contains a user ID NOT in cache

- **Before**: Not possible in the same way (full list was fetched upfront).
- **After**: A single `users.info` call is made for that ID. If successful, the resolved
  name is shown in output and the cache is updated. If the call fails, the raw ID
  (e.g., `UABC123`) is shown — no error is printed to stdout or stderr.

### When output contains a channel ID NOT in cache

Same as user miss above, using `conversations.info`.

### When output contains a group mention NOT in cache

A full `usergroups.list` call is made once per invocation. All group misses in the same
invocation are resolved from the result. Cache is updated. If the call fails, raw group
IDs are shown.

---

## `slackseek cache clear`

No behavior change. Removes entity and history cache files. Next command re-fetches all
entity lists from scratch.

---

## Stdout / Stderr

- Entity resolution failures (targeted fetch errors) are **silent** — no output on
  stdout or stderr. The raw ID is used instead.
- Existing `Warning: could not resolve IDs` stderr message (for failures in
  `buildResolver`) is preserved for the cold-start / full-list fetch path.

---

## Performance Expectations

| Scenario | Before | After |
|---|---|---|
| Repeat invocations, no new entities | Brief pause for TTL check every 24h | No pause; cache read is instant |
| First invocation / cold start | Full list fetch | Unchanged |
| Invocation encounters 1 new user | N/A (full list fetched on TTL) | One `users.info` call (~milliseconds) |
| Invocation encounters many new users | N/A | One `users.info` call per unique new user |
