# Research: Channel and User List Caching

**Feature**: `002-cache-channels-users`
**Date**: 2026-03-03

## Decision Log

---

### Decision 1: Cache Storage Format — JSON Files

**Decision**: Use JSON files with `encoding/json` from the standard library.

**Rationale**:
- Zero new dependencies — standard library only.
- Human-readable and debuggable (`cat ~/.cache/slackseek/…/channels.json`).
- Already used everywhere in the codebase for `--format json` output.
- Performance is irrelevant: the bottleneck is the Slack API, not local disk I/O.

**Alternatives considered**:
- `encoding/gob` — binary, slightly faster, but not debuggable and creates lock-in.
- SQLite / BoltDB — overkill; single-table KV store is a sledgehammer here.
- Pickle / msgpack — not idiomatic Go; would add a dependency.

---

### Decision 2: Cache TTL Mechanism — File Modification Time

**Decision**: Determine staleness by comparing `time.Now()` to the file's `ModTime()`
(via `os.Stat`). No embedded timestamp is written into the JSON payload.

**Rationale**:
- No custom metadata format needed.
- `os.Chtimes` can be used in tests to simulate stale files without waiting.
- Consistent with how HTTP caches, package managers, and shell tools typically track
  file freshness.

**Alternatives considered**:
- Embed `CachedAt time.Time` in the JSON envelope — adds a struct wrapper around every
  payload and complicates the Load/Save API.
- External `.meta` sidecar file — doubles the number of files with no benefit.

---

### Decision 3: Cache Directory — `os.UserCacheDir()`

**Decision**: Resolve the base directory via `os.UserCacheDir()` (returns
`$XDG_CACHE_HOME` or `~/.cache` on Linux; `~/Library/Caches` on macOS) and append
`/slackseek`.

**Rationale**:
- Standard Go stdlib function; already handles both Linux XDG and macOS conventions.
- No platform-specific code required — `os.UserCacheDir` is cross-platform.
- Cache data is not sensitive (channel/user metadata only), so the default user cache
  directory is appropriate.
- Following XDG and macOS conventions means OS tools (Disk Utility, `du`, `find`) can
  locate and manage the cache naturally.

**Alternatives considered**:
- `os.UserHomeDir()` + `.slackseek/cache` — non-standard; mixes config and cache.
- Hard-coded `~/.cache/slackseek` — would silently ignore `$XDG_CACHE_HOME`.
- `os.TempDir()` — lost on reboot; provides no persistent benefit.

---

### Decision 4: Workspace Cache Key — SHA-256 of Workspace URL (8-char hex prefix)

**Decision**: Key each workspace's cache subdirectory using the first 16 hex characters
of `sha256.Sum256([]byte(workspaceURL))`. Example:
`~/.cache/slackseek/a3f1b2c4d5e6f708/channels.json`.

**Rationale**:
- Workspace URLs contain slashes, dots, and colons — not safe as directory names.
- A deterministic hash is stable across invocations and collisions are negligible
  (2^64 space for 16 hex chars).
- `crypto/sha256` is in the standard library — no new dependency.
- 16 chars is short enough to be readable in `ls` output.

**Alternatives considered**:
- URL-encode the workspace URL — produces long, ugly directory names with `%3A` etc.
- MD5 hash — deprecated in security contexts even though this is non-security use.
  Using SHA-256 is consistent with the rest of the Go ecosystem.
- Sequential numeric IDs — not deterministic across machines or reinstalls.

---

### Decision 5: Package Boundary — `internal/cache` Independent, `internal/slack` Import-Free

**Decision**: Create a new `internal/cache` package that stores and loads raw `[]byte`.
The `internal/slack` package imports `internal/cache` and handles JSON marshalling of
`Channel` and `User` slices. The `cmd/` layer constructs the `*cache.Store` and injects
it into the Slack client.

**Rationale**:
- Avoids a circular import: `internal/cache` stores bytes, knows nothing of Slack types.
- `internal/slack` already owns `Channel` and `User` — marshalling belongs there.
- `cmd/` is the composition root and is the right place to construct and inject the
  cache.
- Constitution Principle III (Single-Responsibility Packages): each package has one job.

**Alternatives considered**:
- `internal/cache` stores typed structs — would need to import `internal/slack`,
  creating a circular dependency with `internal/slack` importing `internal/cache`.
- Cache entirely in `cmd/` — would require duplicating list-fetch logic or adding a
  cache-aware wrapper around each runFn; bloats `cmd/` with infrastructure concerns.
- Wrap `Client` with a `CachingClient` struct in `internal/slack` — elegant but adds
  an extra type when embedding the cache directly in `Client` achieves the same result
  with less boilerplate.

---

### Decision 6: New Flags — `--cache-ttl`, `--refresh-cache`, `--no-cache`

**Decision**: Add three persistent flags to the root command:
- `--cache-ttl duration` (default `24h`) — TTL for cached lists.
- `--refresh-cache` (bool) — bypass read, force fresh fetch, overwrite cache.
- `--no-cache` (bool) — bypass read and write entirely.

**Rationale**:
- All commands that resolve names benefit from caching; the flags belong at the root
  so they work with every subcommand without repetition.
- Separate `--refresh-cache` and `--no-cache` serve distinct needs: refresh updates the
  cache for next time; no-cache leaves the cache untouched (useful for debugging).
- `duration` type reuses Cobra's built-in `DurationVar` — no parsing code required.

**Alternatives considered**:
- Single `--cache` flag with values `on|off|refresh` — strings are less composable
  with shell scripts than boolean flags.
- Per-command flags — inconsistent UX; every command would need separate plumbing.

---

### Decision 7: `slackseek cache clear` Command

**Decision**: Add a new top-level `cache` command with a `clear` subcommand.
`clear` accepts `--all` to wipe all workspace subdirectories.

**Rationale**:
- Follows the same `<noun> <verb>` pattern as `auth show`, `channels list`, `users list`.
- `--all` flag is explicit and prevents accidental deletion of other workspaces' caches.

**Alternatives considered**:
- `--clear-cache` flag on the root — mixes infrastructure operations with data commands.
- Shell script / manual deletion — acceptable but not discoverable by users.

---

### Decision 8: Race-Safety Approach — Last-Writer-Wins, No Locking

**Decision**: No file locking. If two concurrent `slackseek` invocations both miss the
cache simultaneously, both fetch from the API and write the file. The last writer wins.

**Rationale**:
- `slackseek` is a single-user CLI tool; true concurrency is rare and accidental.
- File content is idempotent (both writers would fetch the same Slack data).
- Adding `flock` / `os.File.Lock` would be a platform-specific API surface and would
  complicate tests significantly.
- Constitution Principle V (Platform Isolation) and Principle I (Clarity) argue against
  adding locking complexity for a non-problem.

**Alternatives considered**:
- Advisory `flock` locking — Linux-specific; violates Principle V.
- Atomic rename via temp file — already the write strategy to avoid partial reads.

---

### Summary of New Dependencies

| Dependency | Source | Reason |
|------------|--------|--------|
| `crypto/sha256` | stdlib | Workspace cache key derivation |
| `encoding/json` | stdlib | Cache file serialisation |
| `os.UserCacheDir` | stdlib | Platform-correct cache base path |

**No new third-party modules required.**
