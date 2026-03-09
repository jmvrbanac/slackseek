# Research: 007 — Multi-Day History Cache

## Fetch Strategy: Bulk Fetch vs. Per-Day Calls

**Decision**: Make one bulk API call per gap range (same as the existing non-caching path),
then partition the results client-side. Do NOT split into N per-day API calls.

**Rationale**: `FetchHistory` uses `conversations.history` (Tier 3, 48 calls/min). With
`threads=true`, each threaded root message additionally triggers a `conversations.replies`
call. A 2-week window with 14 per-day calls would consume at minimum 14 Tier 3 slots just
for history pages (before reply fetches), taking ~18 seconds in rate-limit spacing alone.
The bulk fetch makes the same number of page calls as today — one per 200 messages over
the full range — with zero extra rate-limit pressure.

**Alternatives considered**:
- Per-day API calls in parallel — reduces wall time but risks concurrent 429 responses;
  the shared `tier3` rate limiter is not goroutine-safe, so this would require a mutex or
  channel-based semaphore. Not worth the complexity given bulk fetch is equally fast.
- Per-day API calls sequentially — strictly worse than bulk: same number of history pages
  plus 14× the per-call overhead.

---

## Thread Reply Bucketing

**Decision**: Two-pass algorithm. Pass 1 maps each root message's `Timestamp → YYYY-MM-DD`.
Pass 2 assigns each reply to its root's day via `msg.ThreadTS`.

**Rationale**: `buildMsgList` in `internal/slack/channels.go` fetches all replies for each
root regardless of reply timestamp. A reply posted at 00:10 UTC on day 2 to a root from
23:55 UTC on day 1 must land in day 1's cache file — otherwise it is orphaned when day 1
is loaded from cache in isolation. This matches the behaviour of the existing single-day
cache (which also stores all replies with their root regardless of reply date).

Edge case: a reply whose `ThreadTS` refers to a root outside the fetched window. This
cannot occur because `conversations.history` only returns roots within the requested
`oldest`/`latest` bounds, and `collectReplies` is called only for those roots.

**Alternatives considered**:
- Bucket by each message's own `msg.Time` — simple but orphans cross-day replies.
- Bucket all thread messages under root day using `ThreadTS` without a pre-pass — requires
  scanning root messages first anyway; two-pass is cleaner.

---

## Gap Range Construction

**Decision**: Enumerate all complete past UTC days in the requested range. For each day,
check `LoadStable`. Merge contiguous uncached days into a single `DateRange`. Append today
as a final gap (always fetched live). Ranges with nil `From` or `To` fall through to the
existing live-fetch path (no caching).

**Rationale**: Merging contiguous uncached days minimises API calls. If days 3–7 of a
10-day window are cached but days 1–2 and 8–10 are not, two gap ranges are constructed
(one for each uncached block) rather than five per-day calls.

**Gap range bounds**:
- Gap start: `midnight UTC` of the first uncached day in the block.
- Gap end: `midnight UTC` of the day after the last uncached day in the block (exclusive
  upper bound — aligns with how `conversations.history` treats `latest`).
- Today gap: `midnight UTC of today → nil` (no upper bound, i.e. live messages).

---

## `FetchHistoryCached` Routing

**Decision**: Extend the existing `FetchHistoryCached` entry point with a routing layer.
If the range covers multiple calendar days (or one day is today), delegate to a new
`fetchHistoryMultiDayCached` inner function. If the range is a single complete past day,
use the existing `fetchHistoryCachedInner`. If `From`/`To` are nil (open-ended), fall
through to a live fetch.

**Rationale**: All callers (`defaultRunHistory`, `defaultRunPostmortem`, `defaultRunMetrics`)
already call `FetchHistoryCached` — changing the routing inside that function requires zero
changes at call sites. The existing single-day path is preserved without modification,
keeping 006 behaviour intact.

**Alternatives considered**:
- New top-level function `FetchHistoryRangeCached` — requires updating three call sites;
  no benefit over routing inside the existing function.
- Merge multi-day logic into `fetchHistoryCachedInner` — makes that function too long
  (constitution: ≤ 40 lines); splitting is cleaner.

---

## `--limit` Semantics

**Decision**: All internal gap fetches use `limit=0`. The `--limit` value is applied once
to the final merged `[]slack.Message` slice before returning.

**Rationale**: Per-gap limit would produce truncated, incomplete day caches. Caching a
partial day defeats the purpose of the cache — the next run for a wider window would still
find incomplete data and re-fetch. Applying limit at merge time ensures every cached day
file is complete and reusable by any future request.

---

## Today's Messages

**Decision**: Today is always fetched live. Today's messages are returned but never written
to `SaveStable`.

**Rationale**: Today's message set is still accumulating. Caching a partial day and
returning stale results would be confusing. The cost of fetching today live is low (one
partial day's worth of API pages). Short-TTL caching of today is deferred to a future
feature if needed.

---

## `--no-cache` Behaviour

**Decision**: `--no-cache` skips all `LoadStable` reads (all days treated as cache misses)
and all `SaveStable` writes (results are not re-cached after fetch).

**Rationale**: The user explicitly wants fresh data. Writing fresh results back to cache
after a `--no-cache` fetch would be confusing — if the intent is "do not use cache", it
should also mean "do not write cache". This differs from the 006 single-day behaviour
(which does overwrite the cache on `--no-cache`). Revisit if users request "refresh and
cache" semantics.

**Update**: On reflection, writing back to cache after `--no-cache` is consistent with
006's documented behaviour ("bypass cache and force a fresh API fetch" then save). Align
007 with 006: `--no-cache` skips load but still writes. This allows the user to refresh
stale cache entries.

---

## Scope

| Command | Multi-day cache (007)? |
|---------|------------------------|
| `history` | YES |
| `postmortem` | YES (calls `FetchHistoryCached` with `limit=0, threads=true`) |
| `metrics` | YES (calls `FetchHistoryCached` with `limit=0, threads=true`) |
| `actions` | NO (threads=false; out of scope) |
| `digest` | NO (uses search API, not `FetchHistory`) |
