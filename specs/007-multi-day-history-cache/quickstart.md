# Quickstart: 007 — Multi-Day History Cache

## What Changes

Before 007, running `slackseek history #general --since 2w` always made a full Slack
API fetch regardless of how many times the command had been run. After 007, each
complete past day is cached to disk on first fetch. Subsequent runs load past days
from disk and only hit the API for today's messages.

## User-Facing Behaviour

### First run (cold cache)

```sh
$ slackseek history general --since 2w
fetching channels: 42 fetched — done
# API pages fetched for the full 2-week range (same speed as before)
# Results printed
```

### Second run (warm cache)

```sh
$ slackseek history general --since 2w
fetching channels: 42 fetched — done
# Only today's messages fetched from API
# Past 13 days loaded from disk — near-instant
# Results printed
```

### Overlapping window

```sh
$ slackseek history general --since 1w
# All 6 past days already cached from the 2-week run
# Only today fetched from API
```

### Force refresh

```sh
$ slackseek history general --since 2w --no-cache
# All days re-fetched from API; cache entries overwritten
```

## Cache Location

```
~/.cache/slackseek/<workspace-hash>/history/<channel-id>/YYYY-MM-DD.json
```

One file per channel per day. Files are permanent (no TTL). Thread replies are stored
with their root message's day, not their own posting day.

## Implementation Entry Points

| File | Role |
|---|---|
| `cmd/historycache.go` | `FetchHistoryCached` routing; `fetchHistoryMultiDayCached`; `enumeratePastDays`; `buildGapRanges`; `partitionByDay` |
| `cmd/historycache_test.go` | Table-driven tests for all new functions |

No changes to `internal/cache` (feature 006 already added `LoadStable`/`SaveStable`).
No changes to `internal/slack`. No new dependencies.
