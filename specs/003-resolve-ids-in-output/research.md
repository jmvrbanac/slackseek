# Research: Resolve User IDs and Channel IDs in Output

**Feature**: 003-resolve-ids-in-output
**Date**: 2026-03-03

---

## Decision 1: Where does resolution logic live?

**Decision**: A new `Resolver` struct in `internal/slack/resolver.go`.

**Rationale**:
- The cached `[]User` and `[]Channel` slices are already produced and owned by `internal/slack`
  (via `ListUsers` and `ListChannels`). Resolution is conceptually part of the Slack entity
  domain, not a formatting concern.
- Placing it in `internal/slack` keeps the `output` package as a pure formatter: it receives
  already-resolved names (or falls back to raw IDs) via the `Resolver`, rather than knowing
  how Slack IDs work.
- The constitution (Principle III) mandates single-responsibility packages. The `output`
  package's responsibility is formatting; it should not contain Slack domain logic.

**Alternatives considered**:
- *Resolve in `cmd/` layer* — would require duplicating resolver construction in `history.go`,
  `messages.go`, and `search.go`. Rejected because it violates DRY and creates drift risk.
- *Mutate `Message` struct with `UserDisplayName` field* — the `Message` type is a wire-level
  representation; enriching it with resolved names blurs the distinction between raw API data
  and presentation data. Rejected in favour of a separate resolver.
- *Resolve in `internal/output`* — the output package already imports `internal/slack` for
  types, but making it call `ListUsers`/`ListChannels` would invert the data-flow dependency
  and couple formatting to API/cache behaviour. Rejected.

---

## Decision 2: How does `Resolver` receive its data?

**Decision**: `NewResolver(users []User, channels []Channel) *Resolver` — callers pass the
already-fetched slices. The resolver builds O(n) lookup maps at construction time.

**Rationale**:
- The calling `cmd/` code already has a `*Client` and will have called `ListUsers` and
  `ListChannels` to perform its primary operation (or will call them once for this purpose).
  The cache from feature 002 means these calls are typically free (disk reads).
- Passing slices (not a `*Client`) keeps `Resolver` stateless and trivially testable — unit
  tests can construct a `Resolver` with hand-crafted `[]User` / `[]Channel` without a Slack
  client or HTTP server.

**Alternatives considered**:
- *Pass `*Client` to `Resolver`* — would make `Resolver` call `ListUsers`/`ListChannels`
  internally. This is fine for production but makes unit tests heavier. Rejected in favour of
  simpler slice-based construction.

---

## Decision 3: How does the output layer receive the Resolver?

**Decision**: `PrintMessages` and `PrintSearchResults` accept an optional `*slack.Resolver`
(may be `nil`). When `nil`, output falls back to raw IDs, preserving exact backwards
compatibility. The `PrintUsers` and `PrintChannels` functions do not need a resolver (they
already display full entity data).

**Rationale**:
- Backwards-compatible signature change (nil = no resolution) preserves existing test
  compatibility and the `--no-cache` path.
- Adding the parameter explicitly (rather than relying on a package-level global or context
  value) is idiomatic Go and keeps the function signature self-documenting.

**Alternatives considered**:
- *Context-based resolver injection* — idiomatic for middleware stacks but unusual for pure
  output formatters. Rejected as over-engineering for a CLI tool.
- *Separate `PrintMessagesResolved` function* — would duplicate the entire function body.
  Rejected.

---

## Decision 4: What name is shown for a user?

**Decision**: Prefer `User.DisplayName`; fall back to `User.RealName`; if both empty, use raw
user ID.

**Rationale**:
- Slack's `display_name` is the `@`-mentionable name and is most recognisable in chat context.
- Some bot accounts and very old accounts have an empty `display_name` but a non-empty
  `real_name`.
- Raw ID fallback ensures output is never broken.

---

## Decision 5: JSON output — new field or replace?

**Decision**: Add `user_display_name string` to `messageJSON` and `searchResultJSON`. Keep
`user_id`. For channel name, `channel_name` already exists in both structs; populate it from
the resolver when it is currently empty (history context).

**Rationale**:
- Downstream tools (scripts, jq pipelines) may depend on `user_id`. Removing it would be a
  breaking change. Adding `user_display_name` is additive and non-breaking.
- `channel_name` already exists in the JSON structs for search results. Extending the same
  field to history results is consistent.

---

## Decision 6: Resolver construction in cmd/ layer

**Decision**: Each affected `cmd/` run function (history, messages, search) calls
`c.ListUsers(ctx)` and `c.ListChannels(ctx, nil, false)` after its primary fetch, constructs
`slack.NewResolver(users, channels)`, and passes the resolver to the output function. If
either list call fails, it logs a warning to stderr and passes `nil` (raw IDs fallback).

**Rationale**:
- Both lists are typically served from the 002 cache (zero API cost). A failure should not
  abort an otherwise-successful command.
- Attempting resolution on a best-effort basis is consistent with the existing cache-failure
  policy (FR-007 of feature 002: graceful degradation).

**Alternatives considered**:
- *Fail hard if list calls fail* — would make `--no-cache` paths abort on resolution. Rejected.
- *Add a `BuildResolver` method to `*Client`* — convenience wrapper; deferred as unnecessary
  complexity since the two calls are already straightforward.
