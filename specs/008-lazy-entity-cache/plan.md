# Implementation Plan: Lazy Entity Cache (Fetch-on-Miss)

**Branch**: `008-lazy-entity-cache` | **Date**: 2026-03-13 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/008-lazy-entity-cache/spec.md`

## Summary

Replace the TTL-based entity cache expiry with a fetch-on-miss strategy. Entity caches
(users, channels, user groups) are loaded without TTL checks using the existing
`LoadStable` API. When a Slack ID cannot be resolved from cache during output rendering,
a targeted single-entity API call is made (`users.info` or `conversations.info`), the
result is merged into the cache file, and the resolved name is used in the current
invocation's output. User group misses trigger a full `usergroups.list` refresh (no
per-group Slack API exists). Full list re-fetches only occur on cold start or
`--refresh-cache`.

## Technical Context

**Language/Version**: Go 1.24
**Primary Dependencies**: `github.com/slack-go/slack` (existing), `github.com/cenkalti/backoff/v4` (existing), `github.com/spf13/cobra` (existing)
**Storage**: File-based JSON cache under `os.UserCacheDir()/slackseek/` (existing `internal/cache` store, unchanged format)
**Testing**: `go test -race ./...` (mandatory); table-driven unit tests with injectable fns
**Target Platform**: Linux and macOS (existing cross-platform build)
**Project Type**: CLI tool
**Performance Goals**: Zero API calls on warm cache hit; single targeted API call per unique missed entity
**Constraints**: Functions ≤ 40 lines; no new external dependencies; backward-compatible cache file format

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|---|---|---|
| I. Clarity Over Cleverness | PASS | Callbacks close over ctx cleanly; no non-obvious constructs; all new functions ≤ 40 lines |
| II. Test-First (NON-NEGOTIABLE) | PASS | Every new exported function gets tests; fetch callbacks are injectable for testing |
| III. Single-Responsibility Packages | PASS | `internal/slack` owns entity fetch; `cmd/` owns wiring; no cross-package direction violations |
| IV. Actionable Error Handling | PASS | Targeted fetch failures silently fall back to raw ID (non-fatal); `buildResolver` full-fetch failures retain existing warning |
| V. Platform Isolation via Build Tags | PASS | No platform-specific code introduced |

**Post-design re-check**: No violations. New `FetchUser`/`FetchChannel` methods sit in
`internal/slack` (correct layer). Resolver callbacks are wired in `cmd/resolver.go`
(correct layer). Cache merge logic is in `internal/slack` per Single-Responsibility.

## Project Structure

### Documentation (this feature)

```text
specs/008-lazy-entity-cache/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── contracts/
│   └── cli-behavior.md  # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks — not yet created)
```

### Source Code (files changed by this feature)

```text
internal/slack/
├── client.go            # Add tier4 rate limiter field + init
├── users.go             # LoadStable; add FetchUser + mergeUser helpers
├── channels.go          # LoadStable; add FetchChannel + mergeChannel helpers
├── usergroups.go        # LoadStable
└── resolver.go          # NewResolverWithFetch; on-miss callback invocation

cmd/
└── resolver.go          # Wire FetchUser/FetchChannel/fetchGroups callbacks
```

**No new files required.** All changes are surgical additions to existing files.

**Structure Decision**: Single-project layout (existing). Changes are confined to
`internal/slack/` (business logic + API calls) and `cmd/resolver.go` (wiring). No new
packages are needed — fetch helpers are natural extensions of the existing `users.go`,
`channels.go`, and `usergroups.go` files.

## Implementation Phases

### Phase A — No-TTL entity load (foundation, independently shippable)

Switch `listUsersCached`, `listChannelsCached`, and `listUserGroupsCached` from
`store.Load` to `store.LoadStable`. This alone eliminates the daily re-fetch on the
common path. `--refresh-cache` continues to work because it clears the workspace dir
before `buildResolver` runs.

**Files**: `internal/slack/users.go`, `channels.go`, `usergroups.go`
**Tests**: Update existing `listUsersCached` / `listChannelsCached` tests to verify no
TTL miss occurs on aged-but-valid cache files.

---

### Phase B — Tier 4 rate limiter for users.info

Add `tier4 *rateLimiter` to `slack.Client` (90 calls/min). Initialize in `NewClient`.

**Files**: `internal/slack/client.go`
**Tests**: Update `client_test.go` constructor tests.

---

### Phase C — FetchUser and FetchChannel on Client

Add `FetchUser(ctx, id string) (User, error)` and `FetchChannel(ctx, id string) (Channel, error)` to `slack.Client`. Each method:
1. Calls the respective Slack API endpoint.
2. Calls an internal `mergeUser` / `mergeChannel` helper that loads the current cache
   via `LoadStable`, appends-or-replaces the entity, and saves via `Save`.
3. Returns the entity; cache write failures are non-fatal (printed to stderr, method
   still returns the entity).

**Files**: `internal/slack/users.go`, `internal/slack/channels.go`
**Tests**: New `TestFetchUser_*` and `TestFetchChannel_*` table-driven tests using the
injectable function pattern established by `listUsersCached`.

---

### Phase D — NewResolverWithFetch and on-miss callbacks in Resolver

Add `NewResolverWithFetch(users []User, channels []Channel, groups []UserGroup, fetchUser, fetchChannel func(string) (string, error), fetchGroups func() ([]UserGroup, error)) *Resolver`.

Update `UserDisplayName`, `ChannelName`, and the group resolution path in
`ResolveMentions` to invoke callbacks on miss and update internal maps. Add
`groupRefreshed bool` guard.

Keep `NewResolver` as: `return NewResolverWithFetch(users, channels, groups, nil, nil, nil)`.

**Files**: `internal/slack/resolver.go`
**Tests**: Extend `resolver_test.go` with tests for the on-miss path, callback
invocation count, and `groupRefreshed` guard.

---

### Phase E — Wire callbacks in buildResolver

Update `cmd/resolver.go` `buildResolver` to call `NewResolverWithFetch`, passing:
- `fetchUser`: closes over `ctx` and `c`; calls `c.FetchUser(ctx, id)`; returns display name.
- `fetchChannel`: closes over `ctx` and `c`; calls `c.FetchChannel(ctx, id)`; returns name.
- `fetchGroups`: closes over `ctx` and `c`; calls `c.ListUserGroups(ctx)` with cache
  bypassed (use `store.Clear` on user_groups key OR add a force-refresh path — see Note).

**Note on fetchGroups**: `ListUserGroups` uses `LoadStable` after Phase A, so a group
miss means the group genuinely isn't in cache. The `fetchGroups` callback should call a
new `c.RefreshUserGroups(ctx)` that bypasses `LoadStable` and forces a live fetch +
cache overwrite. Alternatively, keep it simple: `fetchGroups` deletes `user_groups.json`
then calls `c.ListUserGroups(ctx)` which cold-starts. The simple approach is preferred
per constitution Principle I.

**Files**: `cmd/resolver.go`
**Tests**: Update `cmd/resolver_test.go` (if it exists) or add one.

## Complexity Tracking

No constitution violations. No complexity justification required.
