# Quickstart: 006 — Long-Term Day-History Cache

## What's new

Day-history results for completed past days are now cached permanently on disk. Subsequent
requests for the same workspace + channel + day return instantly from cache without hitting
the Slack API.

## First use

No configuration needed. Caching is automatic.

```sh
# First call: fetches from Slack API and writes cache entry
slackseek history general --from 2026-03-01 --to 2026-03-01

# Second call: served from disk (<1ms overhead)
slackseek history general --from 2026-03-01 --to 2026-03-01
```

## When caching activates

A cache entry is written when **all** of the following are true:

1. `--from` and `--to` refer to the **same calendar day** in UTC.
2. That day has **fully elapsed** (i.e. `--to` is in the past).
3. The result was **not truncated** — either `--limit 0` (unlimited) or fewer messages were
   returned than the limit.
4. `--threads` is `true` (the default).

Today's messages and multi-day ranges are never cached.

## Force-refresh a cached day

Use `--no-cache` to bypass the cache and overwrite the entry with fresh data:

```sh
# Skip cache load, re-fetch from Slack, update cache entry
slackseek history general --from 2026-03-01 --to 2026-03-01 --no-cache
```

## Works across commands

The same cache entry is shared by `history`, `postmortem`, and `metrics`. Running any one
of them for a past day populates the cache for all three:

```sh
slackseek postmortem incidents --from 2026-03-01 --to 2026-03-01   # writes cache
slackseek metrics    incidents --from 2026-03-01 --to 2026-03-01   # cache hit
slackseek history    incidents --from 2026-03-01 --to 2026-03-01   # cache hit
```

## Cache location

```
~/.cache/slackseek/{workspaceKey}/history/{channelID}/{YYYY-MM-DD}.json
# macOS:
~/Library/Caches/slackseek/{workspaceKey}/history/{channelID}/{YYYY-MM-DD}.json
```

## Clear the cache

```sh
slackseek cache clear            # removes all cached data for the current workspace
```

Day-history entries are removed alongside channel/user entries.
