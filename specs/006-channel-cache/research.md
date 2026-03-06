# Research: 006 — Long-Term Day-History Cache

## Cache Keying Strategy

**Decision**: Use `kind = "history/" + channelID + "/" + date` with the existing `cache.Store`.

**Rationale**: `cache.Store.Save` calls `os.MkdirAll` on the parent directory before writing,
so nested `kind` paths like `history/C012345/2026-03-01` are fully supported today without
any structural change to the store. The resulting path is:

```
{cacheDir}/{wsKey}/history/{channelID}/{date}.json
```

**Alternatives considered**:
- Flat `kind = channelID + "_" + date` — avoids subdirs but collapses the namespace; harder
  to enumerate entries for a given channel.
- Separate `DayStore` type — cleaner interface but unnecessary given `Store` already has
  the needed write mechanics.

---

## TTL Bypass (LoadStable)

**Decision**: Add `LoadStable(key, kind string) ([]byte, bool, error)` to `cache.Store` that
skips the `time.Since(modTime) > ttl` check. `SaveStable` is an alias for `Save` (write
mechanics are identical; TTL only affects load).

**Rationale**: Past-day messages are immutable (barring deletion). A TTL-based expiry would
cause unnecessary re-fetches. The file's presence on disk is the only validity signal needed.
JSON validity check is retained to guard against truncated writes.

**Alternatives considered**:
- TTL set to `MaxInt64` — works but is semantically misleading; a reader would not know
  whether the long TTL is intentional.
- Separate file format with embedded `permanent: true` metadata — over-engineered for the
  current need.

---

## Shared FetchHistory Helper

**Decision**: Add `FetchHistoryCached` in a new `cmd/historycache.go` file.

**Signature**:
```go
func FetchHistoryCached(
    ctx context.Context,
    c *slack.Client,
    store *cache.Store,
    wsKey, channelID string,
    dr slack.DateRange,
    limit int,
    threads bool,
    noCache bool,
) ([]slack.Message, error)
```

**Rationale**: Centralises all cache decision logic (eligibility check, load, save) in one
place. Each `defaultRun*` function calls this instead of `c.FetchHistory` directly. The
injectable `*slack.Client` and `*cache.Store` allow the function to be unit-tested without
a live API.

**Alternatives considered**:
- Inline cache logic in each `defaultRun*` function — produces five copies of the same
  logic; violates DRY and makes the eligibility conditions easy to diverge.
- New `internal/historycache` package — adds a new internal package for what is essentially
  cmd-layer glue; `cmd/` is the right home.

---

## Cacheability Conditions

**Decision**: `cacheableDayKey(dr DateRange, fetchedCount, limit int) string` returns a
YYYY-MM-DD string when all four conditions hold, otherwise `""`.

Conditions:
1. `dr.From != nil && dr.To != nil`
2. Same calendar day in UTC: `From.Format("2006-01-02") == To.Format("2006-01-02")`
3. `dr.To.Before(time.Now().UTC())` (fully elapsed)
4. `limit == 0 || fetchedCount < limit` (not truncated)

**Rationale**: Condition 4 uses `fetchedCount < limit` (strict less-than). At exactly
`fetchedCount == limit` the result may be truncated, so we do not cache. This means a
`--limit 1000` run that happens to return exactly 1000 messages will not be cached — an
acceptable false negative to avoid serving incomplete data.

**Edge case**: Pre-fetch check (before API call) uses `limit == 0` only. The post-fetch
check uses the full four conditions with actual count.

---

## `--no-cache` Flag Placement

**Decision**: Per-command flag on `history`, `postmortem`, and `metrics`. Not a global flag.

**Rationale**: `digest` and `actions` do not cache in v1. A global flag would surface the
option on commands where it has no effect, creating confusion. Per-command flags make the
scope explicit and can be extended independently.

---

## Commands In / Out of Scope

| Command | Data Path | Cacheable (v1)? | Reason |
|---------|-----------|-----------------|--------|
| `history` | `FetchHistory(threads=configurable)` | YES (when threads=true) | Primary use case |
| `postmortem` | `FetchHistory(threads=true, limit=0)` | YES | Shares same API path |
| `metrics` | `FetchHistory(threads=true, limit=0)` | YES | Shares same API path |
| `actions` | `FetchHistory(threads=false, limit=0)` | NO | threads=false excluded per decision |
| `digest` | `GetUserMessages` → search API | NO | Different API; user-scoped, not channel-scoped |

`actions` and `digest` receive `--no-cache` flag as a no-op in v1, reserved for when they
gain caching in a future feature.

Actually: since `--no-cache` has no effect on `actions`/`digest`, omit it from those commands
in v1 to avoid misleading the user.

---

## Cache Invalidation

**Decision**: `slackseek cache clear` already removes the entire workspace subdirectory
via `ClearAll()` / `Clear(wsKey)`. Day-history entries are nested under the same directory,
so existing clear semantics cover them with no change.

**Future**: A targeted `slackseek cache clear --channel X --date Y` could be added later
without changing the storage layout.
