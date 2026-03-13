# Research: Lazy Entity Cache (Fetch-on-Miss)

## Decision 1: Single-entity lookup availability in the Slack API

**Decision**: Use `users.info` for user misses and `conversations.info` for channel misses.
No equivalent exists for user groups — `usergroups.list` is the only option.

**Rationale**: The slack-go library exposes `GetUserInfoContext(ctx, userID)` (Tier 4, ~100
calls/min) and `GetConversationInfoContext(ctx, params)` (Tier 3, ~50 calls/min). Both
exist today on the `*slackgo.Client` embedded in our `slack.Client`. No new dependency is
required. Tier 4 is significantly less rate-limited than the Tier 2 list endpoints, making
targeted fetches strictly better than a full re-fetch for common-case single misses.

**Alternatives considered**:
- Full list re-fetch on any miss — rejected; wastes API quota and increases latency.
- Slack webhook/event subscription for real-time updates — out of scope for a CLI tool.

---

## Decision 2: How to eliminate TTL-based expiry for entity caches

**Decision**: Switch `listUsersCached`, `listChannelsCached`, and `listUserGroupsCached`
from `store.Load` (TTL-aware) to `store.LoadStable` (no TTL). The `LoadStable` method
already exists in `internal/cache/store.go`; no new infrastructure is needed.

**Rationale**: The history cache (`historycache.go`) still uses `store.Load` (TTL-aware)
and must continue to do so. Because `store.Load` vs `store.LoadStable` is selected at
the call site, changing only the entity functions is safe and surgical.

**Effect on `--cache-ttl` flag**: The flag is still passed to `cache.NewStore` (which
the history cache uses), so history cache TTL behavior is unchanged. For entity caches,
the TTL value is simply never consulted. The flag is retained for backward compatibility
and documented as a no-op for entity caches.

**Alternatives considered**:
- Remove `--cache-ttl` flag entirely — rejected; breaks existing scripts/aliases.
- Introduce a separate `--entity-cache-ttl` flag — rejected; adds complexity with no
  benefit given the fetch-on-miss design supersedes TTL.

---

## Decision 3: Inline vs. post-pass targeted fetch

**Decision**: Inline targeted fetch — when a resolution miss occurs during output
rendering, the fetch happens immediately (same invocation, same goroutine), and the
resolved name is used in the output.

**Rationale**: A two-pass approach (render → collect misses → re-fetch → re-render)
requires buffering the entire output and re-running formatting logic. Inline resolution
is simpler: the Resolver receives fetch callbacks that close over the context and client,
and calls them on miss. For a CLI tool with a short-lived Resolver per invocation,
storing context in a closure is idiomatic and safe.

**Alternatives considered**:
- Post-pass re-render — rejected; more complex, requires buffering, and the extra
  rendering pass adds latency with no correctness advantage.
- Background async fetch (show raw ID now, refresh cache for next time) — rejected;
  user sees raw IDs on the current invocation, which is a worse UX than a brief
  targeted API call.

---

## Decision 4: How to thread context into Resolver resolution methods

**Decision**: Fetch callbacks close over the `context.Context` captured in
`buildResolver`. The `Resolver` itself holds `func(id string) (string, error)` callbacks
— no context parameter is added to `UserDisplayName`, `ChannelName`, etc.

**Rationale**: Adding `ctx` to every resolution method would require updating all call
sites in `cmd/` and `internal/output/`. Since the `Resolver` is constructed once per
command invocation, capturing `ctx` in a closure at construction time is equivalent and
less invasive. If the context is cancelled during output rendering, the callback returns
an error and the raw ID is used — a safe degradation path.

**Alternatives considered**:
- Store `ctx` in the `Resolver` struct — equivalent but considered a worse pattern; a
  closure makes the dependency explicit.
- Add `ctx` to every resolution method — rejected; high churn across call sites with no
  behavioral benefit.

---

## Decision 5: Cache merge strategy for targeted fetches

**Decision**: On a targeted user or channel fetch: load the current cache file via
`LoadStable`, unmarshal the slice, append or replace the entry for the fetched ID,
re-marshal, and write back via `Save` (atomic rename). Group misses replace the whole
file (same as current full-refresh behavior).

**Rationale**: The cache file format (`[]User`, `[]Channel`, `[]UserGroup` JSON arrays)
is unchanged. Merge-and-save is O(n) where n is the number of cached entities — for
users/channels in a typical workspace (hundreds to low thousands), this is negligible.
Atomicity is guaranteed by the existing `tmp → rename` pattern in `store.Save`.

**Alternatives considered**:
- Separate per-entity cache files (one JSON file per user/channel) — rejected; more
  filesystem churn, more complex listing/clearing, no performance benefit at this scale.
- Append-only log with compaction — rejected; over-engineered for the entity counts
  involved.

---

## Decision 6: New constructor vs. modifying NewResolver

**Decision**: Add `NewResolverWithFetch` as a new constructor that accepts optional
fetch callbacks. Keep `NewResolver` as a thin wrapper calling
`NewResolverWithFetch(users, channels, groups, nil, nil, nil)`. All existing call sites
remain unchanged except `buildResolver` (which switches to `NewResolverWithFetch`).

**Rationale**: Keeps backward compatibility for tests that call `NewResolver` directly.
Avoids breaking the existing test suite. The naming is explicit about the capability
difference.

**Alternatives considered**:
- Functional options pattern (`WithFetchUser(fn)`, etc.) — valid but adds indirection
  for two optional fields; the direct constructor is simpler given the constitution's
  preference for clarity.
- Modify `NewResolver` in place (add parameters) — requires updating all test call
  sites; rejected to minimise churn.

---

## Decision 7: Rate limiter for users.info (Tier 4)

**Decision**: Add a `tier4` rate limiter to `slack.Client` at 90 calls/min (10% margin
on Slack's ~100/min Tier 4 limit). Use `tier4` for `FetchUser`; use existing `tier3`
for `FetchChannel` (conversations.info is Tier 3).

**Rationale**: Targeted fetches on miss are rare and happen one-at-a-time (not in
batch), so even Tier 3 headroom is more than sufficient. Adding `tier4` keeps the rate
limiter semantics consistent with the existing tier naming convention.

**Alternatives considered**:
- Reuse `tier2` for users.info — incorrect; Tier 4 is more permissive, not less.
- No rate limiter for targeted fetches — rejected; rate limiters are a project-wide
  safety net, not an optimization.

---

## Decision 8: Guard against redundant group list refreshes

**Decision**: `Resolver` stores a `groupRefreshed bool` flag. Once a group miss triggers
`usergroups.list`, the flag is set and no further group list calls are made within the
same invocation, regardless of how many additional group misses occur.

**Rationale**: Prevents O(n) API calls when a message contains many group mentions that
are all absent from cache (e.g., first run after cache clear with group-heavy messages).

**Alternatives considered**:
- Always refresh groups on every miss — rejected; could cause rate-limit pressure in
  pathological cases.
