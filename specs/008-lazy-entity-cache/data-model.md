# Data Model: Lazy Entity Cache (Fetch-on-Miss)

No new persistent types are introduced. All changes are to how existing types are loaded
and extended at runtime.

---

## Existing Types (unchanged on disk)

### `slack.User` — stored in `{cache-dir}/{workspace-key}/users.json` as `[]User`

```
id          string  — Slack user ID (immutable, e.g. "U01234567")
displayName string  — @-mentionable name (mutable)
realName    string  — full name (mutable)
email       string  — may be empty
isBot       bool
isDeleted   bool
```

**Invariants**: `id` is set and non-empty. IDs are immutable; names may change but
targeted fetch-on-miss will update stale names in the cache file.

---

### `slack.Channel` — stored in `{cache-dir}/{workspace-key}/channels.json` as `[]Channel`

```
id          string  — Slack channel ID (immutable, e.g. "C01234567")
name        string  — display name without '#' (mutable)
type        string  — "public_channel" | "private_channel" | "mpim" | "im"
memberCount int
topic       string
isArchived  bool
```

**Invariants**: `id` is set and non-empty.

---

### `slack.UserGroup` — stored in `{cache-dir}/{workspace-key}/user_groups.json` as `[]UserGroup`

```
id     string  — Slack group ID (immutable, e.g. "S01234567")
handle string  — without '@' (mutable)
name   string  — display name (mutable)
```

**Invariants**: `id` is set and non-empty.

---

## Cache File Lifecycle Changes

| Event | Current behavior | New behavior |
|---|---|---|
| Cold start (no file) | Fetch full list, save | Unchanged |
| Warm hit (file exists, in TTL) | Return cached data | Return cached data (LoadStable — no TTL check) |
| Stale hit (file exists, past TTL) | Cache miss → full re-fetch | Cache hit → return as-is (TTL ignored) |
| Single ID miss during resolution | N/A (not possible before) | Targeted fetch → merge entry → save |
| Group ID miss during resolution | N/A (not possible before) | Full group list refresh → save |
| `--refresh-cache` | Clear workspace dir → full re-fetch | Unchanged (Clear still removes all entity files) |
| `--no-cache` | Skip entity resolution | Unchanged |
| `cache clear` | Remove workspace dir | Unchanged |

---

## In-Memory Resolver Changes

### `slack.Resolver` (new fields)

```
users         map[string]string           — id → display name (unchanged)
channels      map[string]string           — id → channel name (unchanged)
groups        map[string]string           — id → handle (unchanged)
fetchUser     func(id string) (string, error)   — nil if no targeted fetch available
fetchChannel  func(id string) (string, error)   — nil if no targeted fetch available
fetchGroups   func() ([]UserGroup, error)        — nil if no targeted fetch available
groupRefreshed bool                             — prevents repeat group list calls
```

**State transitions for `groupRefreshed`**:
- Initial: `false`
- After first group miss that triggers fetch: `true`
- Resets at end of invocation (Resolver is discarded)

---

## New Client Methods

### `(*Client).FetchUser(ctx, id) (User, error)`

Calls `users.info` for a single user ID. On success:
1. Loads current `users.json` from cache via `LoadStable`.
2. Appends or replaces the entry for `id`.
3. Writes the updated slice back via `Save` (atomic).

Returns the `User` value regardless of cache write outcome (write failures are
non-fatal, printed to stderr).

---

### `(*Client).FetchChannel(ctx, id) (Channel, error)`

Calls `conversations.info` for a single channel ID. On success, merges into
`channels.json` via the same load-replace-save pattern.

---

## No Schema Migration Required

Cache files written by previous versions remain valid. `LoadStable` reads them normally.
Old TTL-based clients will still re-fetch on TTL expiry; new clients simply stop doing
so. Users with existing caches experience a seamless upgrade.
