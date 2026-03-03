# CLI Contract: Channel and User List Caching

**Feature**: `002-cache-channels-users`
**Date**: 2026-03-03

---

## New Global Flags (added to root command)

These flags apply to every `slackseek` subcommand that makes API calls.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--cache-ttl` | `duration` | `24h` | Maximum age of a cached channel or user list. Use Go duration syntax: `1h`, `30m`, `48h`, `0` (disables cache). |
| `--refresh-cache` | `bool` | `false` | Ignore the on-disk cache, fetch fresh data, and overwrite the cache entry. Mutually exclusive with `--no-cache`. |
| `--no-cache` | `bool` | `false` | Bypass the cache entirely for this invocation — neither read nor write. Mutually exclusive with `--refresh-cache`. |

### Validation

- `--cache-ttl` with a negative value → error: `"invalid --cache-ttl: duration must not be negative"`.
- `--refresh-cache` and `--no-cache` both set → error: `"--refresh-cache and --no-cache are mutually exclusive"`.
- `--cache-ttl 0` is equivalent to `--no-cache` (disables caching).

---

## New Command: `cache`

```
slackseek cache <subcommand>
```

Top-level noun grouping for cache management operations.

---

### `slackseek cache clear`

Deletes cached channel and user lists for the currently selected workspace.

```
slackseek cache clear [flags]
```

**Flags**:

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--all` | `bool` | `false` | Clear cache for all workspaces, not just the currently selected one. |

**Exit codes**:

| Code | Meaning |
|------|---------|
| 0 | Cache cleared successfully (or nothing to clear). |
| 1 | Unexpected error (e.g., permission denied on cache directory). |

**Stdout (text format, nothing to clear)**:
```
No cache found for workspace "myworkspace".
```

**Stdout (text format, cleared)**:
```
Cache cleared for workspace "myworkspace" (2 files removed).
```

**Stdout (with --all)**:
```
Cache cleared for all workspaces (5 files removed).
```

**Stderr on non-fatal warning**:
```
Warning: could not remove /home/user/.cache/slackseek/a3f1b2c4/users.json: permission denied
```

---

## Behaviour Changes to Existing Commands

### All name-resolving commands (`history`, `messages`, `search`)

These commands call `ResolveChannel` or `ResolveUser` internally. With caching enabled,
the first call per workspace per TTL window is the only call that touches the Slack API
for list data. Subsequent calls within the TTL return immediately from disk.

**Observable change**: First run may print a cache-write notice to stderr when
`--verbose` is added in a future feature; currently silent.

**No change** to stdout format, flags, or exit codes.

### `channels list`

- First run: fetches from API, writes cache, returns results.
- Subsequent runs within TTL: reads from cache, skips API call.
- `--refresh-cache`: fetches from API regardless of cache age.
- `--no-cache`: fetches from API, does not write cache.

### `users list`

Same caching behaviour as `channels list`.

---

## Cache File Paths (informational)

| Platform | Base path |
|----------|-----------|
| Linux | `$XDG_CACHE_HOME/slackseek/` (defaults to `~/.cache/slackseek/`) |
| macOS | `~/Library/Caches/slackseek/` |

Workspace subdirectory: 16-character lowercase hex prefix of `SHA-256(workspaceURL)`.

Example (Linux, workspace URL `https://myteam.slack.com`):
```
~/.cache/slackseek/a3f1b2c4d5e6f708/channels.json
~/.cache/slackseek/a3f1b2c4d5e6f708/users.json
```
