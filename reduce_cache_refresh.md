# Ideation: Reduce Entity Cache Refresh

## Problem

The current entity cache (users, channels, user groups) uses a hard 24-hour TTL
(`--cache-ttl`, default `24*time.Hour`). Once expired, the very next command invocation
triggers a full re-fetch of all three lists — even when no IDs in that command's output
are actually unresolved. This is wasteful because:

- Slack user/channel IDs are **immutable**. Once an ID maps to a name, that mapping is
  permanent. Only the name can change (e.g., a channel rename or display name update),
  and even that is rare.
- Fetching all users + all channels + all groups on a daily basis incurs unnecessary API
  calls, rate-limit pressure, and startup latency.
- Most invocations don't encounter new or renamed entities since the last fetch.

## Key Observation: `LoadStable` Already Exists

`cache.Store` already has a `LoadStable` / `SaveStable` pair that reads cache files
without any TTL check (`store.go:82-102`). The infrastructure for "never expire" is
already present — it just isn't wired into the entity-resolution path.

## Proposed Strategy: Fetch-on-Empty, Re-fetch-on-Miss

Replace the TTL-gated load in `listUsersCached` and `listChannelsCached` with
`LoadStable`. Introduce a miss-detection mechanism that triggers a cache refresh only
when a lookup actually fails.

### Refresh triggers (replacing the daily TTL)

| Trigger | When | Action |
|---|---|---|
| Cache file absent | First run or after `cache clear` | Fetch from API, save to cache |
| `--refresh-cache` flag | User explicitly requests fresh data | Bypass cache, fetch, overwrite |
| Unresolved ID detected | An ID came back as a raw `UXXXXXX`/`CXXXXXX` | Invalidate + re-fetch for next invocation (or immediately, two-pass) |

### Option A — Async invalidation (simplest, one-pass)

After a command completes, if `Resolver.HasMisses()` is true, delete the stale cache
file(s). The **current** command shows raw IDs (acceptable for the rare new-user/
new-channel case). The **next** command fetches fresh data and resolves correctly.

- `Resolver` gains a `MissCount() int` (or `HasMisses() bool`) that counts how many
  `UserDisplayName` / `ChannelName` lookups fell back to the raw ID.
- After `buildResolver` / command output is written, `cmd/resolver.go` checks for misses
  and calls `store.Clear(key)` on the relevant kind(s).
- No second API call in the same invocation. Startup latency stays low.

### Option B — Synchronous two-pass (accurate, slightly slower on miss)

When `Resolver.HasMisses()` is true after rendering:
1. Force-re-fetch the affected list(s) with `--refresh-cache` semantics.
2. Re-build the resolver from fresh data.
3. Re-render output.

This guarantees the current invocation shows resolved names, at the cost of one extra
API round-trip on the rare occasion a new entity appears. For most commands (history,
search, digest) the output is already computed so re-rendering is straightforward.

### Option C — Targeted fetch on miss (recommended)

- Use `LoadStable` (no TTL) for the resolver path.
- On a user ID miss: call `users.info` for that single ID (Tier 4), add to in-memory
  map and update the cache file. Resolution succeeds in the same invocation.
- On a channel ID miss: call `conversations.info` for that single ID (Tier 3), same
  treatment.
- On a user group miss: do a full `usergroups.list` refresh and update the cache.
  Groups are referenced infrequently so this is acceptable.
- No second pass. No full list re-fetch unless it's a group miss or cold start.

This keeps the common case (no new entities) fast and the rare case (new user joins,
new channel created) transparent to the user.

## Targeted Fetch Feasibility (Slack API)

The slack-go library (`github.com/slack-go/slack`) exposes single-entity endpoints:

| Entity | Endpoint | slack-go method | Tier |
|---|---|---|---|
| User | `users.info` | `GetUserInfoContext(ctx, userID)` | Tier 4 (100+/min) |
| Channel | `conversations.info` | `GetConversationInfoContext(ctx, params)` | Tier 3 (50+/min) |
| User group | *(none)* | — | — |

`usergroups.info` does not exist in the Slack API. The only option for groups is the
full `usergroups.list`. This means the targeted-fetch strategy is **asymmetric**:

- **Users**: fetch exactly the one missing user by ID — fast, Tier 4, no list needed.
- **Channels**: fetch exactly the one missing channel by ID — Tier 3.
- **User groups**: on a group miss, must do a full `usergroups.list` and update the
  whole cache. Fortunately group references (`<!subteam^...>`) are rare and groups
  change even less frequently than users.

### Cache update on targeted fetch

The current cache stores the full list as a single JSON file (`users.json`,
`channels.json`). On a targeted fetch, the flow would be:

1. Load current cache into memory (it may be stale but mostly complete).
2. Fetch the single missing entity via `users.info` / `conversations.info`.
3. Append the new entity to the in-memory slice.
4. Write the updated slice back to the cache file.

This means the cache grows incrementally — new users/channels accumulate without
ever requiring a full re-fetch. A full re-fetch only occurs on first run, `--refresh-cache`,
or a group miss.

### Rate limit improvement

Because targeted fetches hit Tier 3/4 endpoints (instead of the Tier 2 list endpoints),
they are subject to significantly higher rate limits. This is strictly better than the
current approach of re-fetching the full lists on a daily basis.

## Code Changes Required

### `internal/slack/users.go` and `channels.go`

Switch `listUsersCached` and `listChannelsCached` from `store.Load` (TTL-aware) to
`store.LoadStable` (no TTL). These functions already accept the store as a parameter, so
the change is localized to two call sites.

### `internal/slack/usergroups.go`

Same: switch from `Load` to `LoadStable` in `listUserGroupsCached` (if it exists) or
equivalent.

### `internal/slack/resolver.go`

Add miss tracking to `Resolver`:

```go
type Resolver struct {
    users    map[string]string
    channels map[string]string
    groups   map[string]string
    misses   map[string]struct{} // IDs that weren't resolved
}

func (r *Resolver) HasMisses() bool { return len(r.misses) > 0 }
```

`UserDisplayName`, `ChannelName`, etc. populate `r.misses` on a cache miss before
returning the raw ID.

### `cmd/resolver.go` — `buildResolver`

After building the resolver, if the chosen strategy is Option A, register a cleanup
function (or just call it inline after the command runs) to invalidate stale cache kinds.

For Option C, `buildResolver` becomes a two-phase function:
1. Load from stable cache → build resolver
2. Expose the resolver to the command
3. After output, check `r.HasMisses()` → if true, re-fetch + re-save + update resolver maps

### `cmd/root.go`

The `--cache-ttl` flag becomes less meaningful for the entity cache. It could be
repurposed as a "minimum age before a miss-triggered re-fetch is allowed" guard
(to prevent thrashing if many unknown IDs appear), or removed from the entity-cache
path entirely. The `--refresh-cache` flag remains useful for manual override.

## Trade-offs

| | Current (TTL) | Proposed (Fetch-on-miss) |
|---|---|---|
| Startup cost after TTL | Always re-fetch all lists | Only re-fetch when a new entity appears |
| Accuracy | Fresh within TTL window | Accurate; re-fetches on actual staleness |
| Stale name risk | Low (24h max lag) | Low (re-fetches on first miss) |
| API call frequency | Daily regardless | Only when entities genuinely change |
| Complexity | Low | Moderate (miss tracking + conditional re-fetch) |

## Open Questions

1. **Partial re-fetch**: Should a user miss trigger only a user list re-fetch, or refresh
   all three lists together? Users and groups are related (group memberships), but
   channels are independent.

2. **Miss threshold**: Should a single unresolved ID trigger a re-fetch, or wait until N
   misses accumulate? A threshold prevents unnecessary fetches from corrupted/test data.

3. **`channels list` / `users list` commands**: These user-facing commands currently
   benefit from the TTL (they show "fresh-ish" data). With `LoadStable` they'd never
   auto-refresh. Should they keep TTL semantics independently of the resolver path,
   or always require `--refresh-cache` for fresh output?

4. **`--cache-ttl 0` behavior**: Currently disables caching. This semantic should be
   preserved.
