# Data Model: 007 — Multi-Day History Cache

## Cache Entry (unchanged from 006)

Day-history cache entries written by this feature are **identical in format** to those
written by feature 006. No new file format or wrapper type is introduced.

### File location

```
{os.UserCacheDir()}/slackseek/{workspaceKey}/history/{channelID}/{YYYY-MM-DD}.json
```

| Path segment | Source | Example |
|---|---|---|
| `os.UserCacheDir()` | stdlib | `/home/alice/.cache` |
| `slackseek` | constant | `slackseek` |
| `workspaceKey` | `cache.WorkspaceKey(workspace.URL)` | `a3f9c2e1b4d70812` |
| `history` | constant prefix | `history` |
| `channelID` | Slack channel ID | `C08AB1234XY` |
| `YYYY-MM-DD` | UTC calendar day | `2026-02-23` |

The `YYYY-MM-DD` key is the UTC day of the **root message** for that bucket. Replies
whose own timestamp falls on a different day are stored under their root's day.

### Entry contents

```json
[
  {
    "ts": "1740844800.000100",
    "user": "U012AB3CD",
    "text": "Hello channel",
    "thread_ts": "",
    "reply_count": 2
  },
  {
    "ts": "1740844900.000200",
    "user": "U012AB3CD",
    "text": "A reply",
    "thread_ts": "1740844800.000100"
  }
]
```

Root messages and their replies are stored together in the same day file. The array is
the direct output of `json.Marshal([]slack.Message{...})`.

---

## New Functions in `cmd/historycache.go`

### `enumeratePastDays(from, to time.Time) []string`

Returns a slice of `YYYY-MM-DD` strings for each complete UTC calendar day strictly
before today that falls within `[from, to)`. Days are returned in ascending order.

| Condition | Result |
|---|---|
| `from` or `to` is zero | `nil` (caller falls through to live fetch) |
| `from.UTC().Date() == to.UTC().Date()` | `nil` (single-day range; use existing path) |
| `to` is today or future | `to` is clamped to yesterday; today is always a live gap |

### `buildGapRanges(pastDays []string, store *cache.Store, wsKey, channelID string, noCache bool) (cached map[string][]slack.Message, gaps []slack.DateRange, err error)`

Iterates `pastDays`. For each day:
- Attempts `store.LoadStable(wsKey, cacheKind(channelID, day))` (unless `noCache`).
- On hit: stores messages in `cached[day]`.
- On miss: extends the current open gap range or starts a new one.

Returns `cached` (pre-loaded messages keyed by date) and `gaps` (contiguous uncached
date ranges ready to pass to `FetchHistory`). Today's gap `[todayMidnight, nil)` is
always appended as the final element of `gaps`.

### `partitionByDay(msgs []slack.Message) map[string][]slack.Message`

Two-pass bucketing of a flat `[]slack.Message`:

**Pass 1** — build root-day index:
```
rootDay map[string]string   // Timestamp → YYYY-MM-DD
for msg where ThreadDepth == 0:
    rootDay[msg.Timestamp] = msg.Time.UTC().Format("2006-01-02")
```

**Pass 2** — assign to buckets:
```
for each msg:
    if ThreadDepth == 0: day = msg.Time.UTC().Format("2006-01-02")
    else:                day = rootDay[msg.ThreadTS]
    buckets[day] = append(buckets[day], msg)
```

Returns `map[string][]slack.Message` keyed by `YYYY-MM-DD`.

### `fetchHistoryMultiDayCached` (inner, testable)

```go
func fetchHistoryMultiDayCached(
    ctx      context.Context,
    fetchFn  historyFetchFunc,
    store    *cache.Store,
    wsKey    string,
    channelID string,
    dr       slack.DateRange,
    limit    int,
    threads  bool,
    noCache  bool,
) ([]slack.Message, error)
```

Orchestrates the full multi-day flow:
1. `enumeratePastDays(dr.From, dr.To)` → past day list.
2. `buildGapRanges(...)` → `cached` map + `gaps` slice.
3. For each gap: `fetchFn(ctx, channelID, gap, 0, threads)` (limit=0 internally).
4. For each gap result: `partitionByDay(msgs)` → bucket; for each past-day bucket call
   `store.SaveStable`; accumulate today's bucket without saving.
5. Merge `cached` + all fetched buckets into a single chronologically-sorted slice.
6. Apply `limit` to the merged slice.

### Updated `FetchHistoryCached` routing

```go
func FetchHistoryCached(...) ([]slack.Message, error) {
    // open-ended or same-day → existing path
    if isMultiDay(dr) {
        return fetchHistoryMultiDayCachedInner(ctx, c.FetchHistory, store, ...)
    }
    return fetchHistoryCachedInner(ctx, c.FetchHistory, store, ...)
}
```

`isMultiDay(dr)` returns true when both `From` and `To` are non-nil and their UTC dates
differ.

---

## State Transitions for a Single Day Within a Multi-Day Fetch

```
Request arrives for range [from, to]
          │
          ▼
  isMultiDay? ──No──► fetchHistoryCachedInner  (006 path, unchanged)
          │
         Yes
          │
          ▼
 enumeratePastDays
          │
  ┌───────┴──────────┐
  │ for each past day│
  │  LoadStable hit? │──Yes──► add to cached map
  │                  │
  │         No       │──────► extend current gap range
  └──────────────────┘
          │
          ▼
  append today gap
          │
          ▼
  for each gap: FetchHistory(limit=0)
          │
          ▼
  partitionByDay (two-pass)
          │
          ▼
  for past-day buckets: SaveStable
          │
          ▼
  merge cached + fetched → sort → apply limit → return
```
