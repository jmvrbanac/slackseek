# CLI Contract Changes: 004 Bug Fixes

## `--format` flag (all commands)

**Before:**
```
--format string   output format: text | table | json (default "text")
```

**After:**
```
--format string   output format: text | table | json | markdown (default "text")
```

**Scope of `markdown`:**
- Accepted by: all commands (flag validation is global in `cmd/root.go`)
- Produces useful output for: `history`, `messages`, `search`
- For `channels`, `users`, `auth show`: the flag is accepted without error
  but the formatters for those commands do not implement a `FormatMarkdown`
  case. They fall through to the `default` (text) path. This is intentional
  — see research.md §Fix 3 for rationale.

**Validation:** `validateFormat` in `cmd/root.go` iterates `output.ValidFormats`;
no code change required there beyond adding `FormatMarkdown` to `ValidFormats`.

---

## `slack.Resolver` public API

New exported method (Fix 4):

```go
// ResolveChannelDisplay(id, name string) string
//
// Resolves a channel name for display. Handles DM channels where the Slack
// search API returns the other user's ID as the channel name.
//
// Parameters:
//   id   – Slack channel ID (e.g. "D01ABCDEF")
//   name – raw channel name as returned by the API (may be a user ID for DMs)
//
// Returns:
//   "@DisplayName"  when name is a user ID and user is resolved
//   "@<id>"         when name is a user ID but user is not in resolver
//   name            when name is a non-empty regular channel name
//   ChannelName(id) when name is empty
func (r *Resolver) ResolveChannelDisplay(id, name string) string
```

**Backwards compatibility:** Additive only. No existing method signatures
change.

---

## JSON output schema (`--format json`)

`PrintMessages` JSON output gains an optional `replies` field on root messages
(Fix 2). Existing consumers that ignore unknown fields are unaffected.

```
messageJSON.replies  []messageJSON  omitempty
```

Replies in a thread are **removed from the top-level array** and nested under
their parent. Consumers that currently iterate the flat array will see fewer
top-level items when thread data is present. This is a **breaking change for
JSON consumers** of `history --threads --format json`.

> **Migration:** Consumers must handle the nested `replies` array. A flat
> view can be reconstructed by iterating each root's `replies` array.
