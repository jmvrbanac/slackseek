# CLI Contract: 007 — Multi-Day History Cache

## Changed Behaviour (no new flags)

Feature 007 introduces **no new CLI flags**. The existing `--no-cache` flag (added in 006)
retains its documented semantics. The change is entirely in the cache strategy invoked
when the date range spans multiple days.

---

## `history` command

```
slackseek history <channel> [--from DATE] [--to DATE] [--since DURATION]
                             [--no-cache] [--limit N] [--threads]
```

| Flag | Type | Default | Semantics (unchanged) |
|---|---|---|---|
| `--no-cache` | bool | false | Bypass cache load; always fetch from API; write results back to cache |
| `--limit N` | int | 1000 | Maximum messages to return (0 = unlimited). Applied to the **merged** result after all gaps are fetched and cached. |
| `--threads` | bool | true | Include inline thread replies. Multi-day cache requires `threads=true` (default). |

### Behaviour change by date-range type

| Range type | 006 behaviour | 007 behaviour |
|---|---|---|
| Single past day | Cache hit → disk; miss → API + save | Unchanged |
| Multi-day, all cached | Always API fetch (no caching) | All past days from disk; today live |
| Multi-day, partial cache | Always API fetch | Cached days from disk; uncached gaps fetched from API |
| Open-ended (no From/To) | Always API fetch | Always API fetch (unchanged) |
| Includes today | Today not cached (single-day check) | Today always fetched live; past days may be cached |

---

## `postmortem` and `metrics` commands

No flag changes. Both commands call `FetchHistoryCached` with `limit=0` and
`threads=true`, which satisfies all multi-day cacheability conditions for past days.

---

## Cache file format (unchanged)

```
{os.UserCacheDir()}/slackseek/{wsKey}/history/{channelID}/{YYYY-MM-DD}.json
```

The `YYYY-MM-DD` is the UTC calendar day of the **root message**. This is identical to
the 006 single-day cache format. Files written by 007 are readable by the 006 cache
reader and vice versa.

---

## Error messages (non-fatal cache warnings, unchanged)

```
Warning: cache read failed: <err>
Warning: could not cache messages: <err>
```

These are written to stderr and do not affect the exit code.
