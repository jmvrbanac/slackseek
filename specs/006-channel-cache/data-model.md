# Data Model: 006 — Long-Term Day-History Cache

## Cache Entry

A day-history cache entry is a JSON-serialised `[]slack.Message` written by `cache.Store.SaveStable`
and read by `cache.Store.LoadStable`. The schema is **unchanged** from the existing API response —
no new wrapper type is introduced.

### File location

```
{os.UserCacheDir()}/slackseek/{workspaceKey}/history/{channelID}/{YYYY-MM-DD}.json
```

| Path segment | Source | Example |
|---|---|---|
| `os.UserCacheDir()` | stdlib (`~/.cache` on Linux, `~/Library/Caches` on macOS) | `/home/alice/.cache` |
| `slackseek` | constant | `slackseek` |
| `workspaceKey` | `cache.WorkspaceKey(workspace.URL)` — first 16 hex chars of SHA-256 | `a3f9c2e1b4d70812` |
| `history` | constant prefix | `history` |
| `channelID` | Slack channel ID returned by `ResolveChannel` | `C08AB1234XY` |
| `YYYY-MM-DD` | UTC date of `dr.From` | `2026-03-01` |

### Entry contents

```json
[
  {
    "ts": "1740844800.000100",
    "user": "U012AB3CD",
    "text": "Hello channel",
    "thread_ts": "",
    "reply_count": 0,
    "replies": null
  },
  ...
]
```

This is identical to what `c.FetchHistory` returns serialised via `json.Marshal`. No envelope,
no metadata field — the file is the array.

---

## New Store Methods

### `LoadStable(key, kind string) ([]byte, bool, error)`

- Reads `{dir}/{key}/{kind}.json`.
- Returns `(nil, false, nil)` if the file does not exist.
- Returns `(nil, false, nil)` if the file is not valid JSON.
- Returns `(data, true, nil)` on success.
- **Does not check TTL** — mod time is irrelevant for stable entries.

### `SaveStable(key, kind string, data []byte) error`

- Identical to `Save` — atomic write via tmp-file + rename.
- Cache write failure is logged to stderr and swallowed (same policy as `Save`).

---

## `cacheableDayKey` Function

```
func cacheableDayKey(dr slack.DateRange, fetchedCount, limit int) string
```

Returns the YYYY-MM-DD string when all conditions are met, `""` otherwise.

| Field | Type | Role |
|---|---|---|
| `dr.From` | `*time.Time` | Must be non-nil; provides the calendar date |
| `dr.To` | `*time.Time` | Must be non-nil; must be in the past; must share date with From |
| `fetchedCount` | `int` | Actual number of messages returned |
| `limit` | `int` | User's `--limit` value; `0` = unlimited |

State transition:

```
(dr.From=nil OR dr.To=nil)   →  "" (non-cacheable)
(From.date != To.date)       →  "" (multi-day range)
(To >= now)                  →  "" (today or future)
(limit > 0 AND count >= limit) →  "" (likely truncated)
else                         →  From.UTC().Format("2006-01-02")
```

---

## Cache Kind String

The `kind` parameter passed to `LoadStable`/`SaveStable`:

```
"history/" + channelID + "/" + date   →  e.g. "history/C08AB1234XY/2026-03-01"
```

Combined with the store path logic this produces:

```
{dir}/{wsKey}/history/C08AB1234XY/2026-03-01.json
```

`os.MkdirAll` in `SaveStable` creates the intermediate `history/` and `{channelID}/`
directories on first write.
