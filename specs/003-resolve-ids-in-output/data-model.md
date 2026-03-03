# Data Model: Resolve User IDs and Channel IDs in Output

**Feature**: 003-resolve-ids-in-output

---

## Entities

### Resolver (new — `internal/slack/resolver.go`)

A value type that holds pre-built lookup maps for user and channel name resolution.
Constructed once per command invocation from already-fetched entity lists.

| Field | Type | Description |
|-------|------|-------------|
| `users` | `map[string]string` | userID → display name (or real name fallback) |
| `channels` | `map[string]string` | channelID → channel name |

**Constructor**: `NewResolver(users []User, channels []Channel) *Resolver`
- Iterates `users` once: prefers `DisplayName`, falls back to `RealName`, skips if both empty.
- Iterates `channels` once: maps `ID` → `Name`.
- Returns a pointer (nil-safe call sites use `if resolver != nil { … }`).

**Methods**:
- `UserDisplayName(id string) string` — returns resolved name or `id` if not found.
- `ChannelName(id string) string` — returns resolved name or `id` if not found.

**Validation rules**:
- `NewResolver` never panics; empty slices produce an empty (but functional) resolver.
- Neither method returns an error; missing-key fallback is the raw ID string.

---

### messageJSON (modified — `internal/output/format.go`)

Extended to carry resolved display names alongside raw IDs.

| Field | Type | JSON key | Change |
|-------|------|----------|--------|
| `Timestamp` | `string` | `timestamp` | unchanged |
| `SlackTS` | `string` | `slack_ts` | unchanged |
| `UserID` | `string` | `user_id` | unchanged |
| `UserDisplayName` | `string` | `user_display_name` | **NEW** — empty string if resolver is nil |
| `Text` | `string` | `text` | unchanged |
| `ChannelID` | `string` | `channel_id` | unchanged |
| `ChannelName` | `string` | `channel_name,omitempty` | **extended** — now populated from resolver in history context |
| `ThreadTS` | `string` | `thread_ts` | unchanged |
| `ThreadDepth` | `int` | `thread_depth` | unchanged |
| `Reactions` | `[]reactionJSON` | `reactions` | unchanged |

---

### searchResultJSON (modified — `internal/output/format.go`)

Same additions as `messageJSON` (it embeds the same fields).

| Field | Type | JSON key | Change |
|-------|------|----------|--------|
| `UserDisplayName` | `string` | `user_display_name` | **NEW** |
| `ChannelName` | `string` | `channel_name,omitempty` | already populated by API; resolver fills gaps only |
| *(all other fields)* | — | — | unchanged |

---

## State Transitions

No persistent state is introduced. The `Resolver` is constructed per-invocation and discarded
after the output function returns. There are no state transitions.

---

## Relationships

```
cmd/history.go ──calls──► c.ListUsers(ctx)      ──returns──► []slack.User
                ──calls──► c.ListChannels(ctx)   ──returns──► []slack.Channel
                ──builds──► slack.NewResolver(users, channels) ──► *slack.Resolver
                ──passes──► output.PrintMessages(w, fmt, msgs, resolver)

internal/slack/resolver.go
  Resolver.UserDisplayName(id) ──lookup──► map[userID]displayName
  Resolver.ChannelName(id)     ──lookup──► map[channelID]name

internal/output/format.go
  PrintMessages       accepts *slack.Resolver (nil = raw IDs)
  PrintSearchResults  accepts *slack.Resolver (nil = raw IDs)
  PrintUsers          unchanged (no resolver needed)
  PrintChannels       unchanged (no resolver needed)
```
