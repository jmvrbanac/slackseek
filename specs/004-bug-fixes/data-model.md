# Data Model: 004 Bug Fixes

No new persistent entities are introduced. This document captures the
type-level changes to existing in-memory structures.

---

## Modified: `messageJSON` (`internal/output/format.go`)

Added field for Fix 2 (thread grouping in JSON output):

```go
type messageJSON struct {
    Timestamp       string         `json:"timestamp"`
    SlackTS         string         `json:"slack_ts"`
    UserID          string         `json:"user_id"`
    UserDisplayName string         `json:"user_display_name"`
    Text            string         `json:"text"`
    ChannelID       string         `json:"channel_id"`
    ChannelName     string         `json:"channel_name,omitempty"`
    ThreadTS        string         `json:"thread_ts"`
    ThreadDepth     int            `json:"thread_depth"`
    Reactions       []reactionJSON `json:"reactions"`
    Replies         []messageJSON  `json:"replies,omitempty"`  // NEW — Fix 2
}
```

**Constraint:** `Replies` is `omitempty` — flat output (no threads) and search
results are unaffected. The field is only populated when thread grouping is
active and a root message has at least one reply.

---

## New: `threadGroup` (`internal/output/format.go`, unexported)

Intermediate grouping type used by `groupByThread`. Not serialised.

```go
type threadGroup struct {
    Root    slack.Message
    Replies []slack.Message
}
```

**Lifecycle:** Created during `groupByThread`, consumed by print helpers,
discarded after output is written.

---

## Modified: `slack.Resolver` (`internal/slack/resolver.go`)

New method and supporting regex for Fix 4:

```go
// userIDPattern matches bare Slack user IDs as returned by the search API
// for DM channel names (e.g. "U01ABCDEF").
var userIDPattern = regexp.MustCompile(`^U[A-Z0-9]+$`)

// ResolveChannelDisplay resolves a channel name for display purposes.
// When name matches a Slack user ID pattern (DM channels), it is resolved
// via the users map and prefixed with '@'. Otherwise the channel ID is
// resolved normally via the channels map.
func (r *Resolver) ResolveChannelDisplay(id, name string) string
```

**Invariants:**
- When `name` is a user ID and the user is in the resolver map → returns `@DisplayName`.
- When `name` is a user ID and the user is not in the map → returns `@name` (raw ID with @ prefix, better than bare ID).
- When `name` is a non-empty regular channel name → returns `name` unchanged.
- When `name` is empty → falls through to `ChannelName(id)`.

---

## New: `output.FormatMarkdown` (`internal/output/format.go`)

New constant for Fix 3:

```go
const FormatMarkdown Format = "markdown"
```

Added to `ValidFormats`. No schema impact — this is a presentation-layer
constant only.

---

## New: `tableSafe` (`internal/output/format.go`, unexported)

New helper for Fix 5:

```go
// tableSafe collapses all whitespace (including newlines) to single spaces
// then truncates to n runes. Use in FormatTable cases to prevent cell
// misalignment caused by embedded newlines.
func tableSafe(s string, n int) string
```

No state; pure function.
