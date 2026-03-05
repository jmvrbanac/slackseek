# Data Model: 005 Day 2 Improvements

## New Types

### `internal/slack/permalink.go`

```go
// ThreadPermalink holds the parsed components of a Slack permalink URL.
type ThreadPermalink struct {
    WorkspaceURL string // e.g. "https://acme.slack.com"
    ChannelID    string // e.g. "C01234567"
    ThreadTS     string // Slack ts, e.g. "1700000000.123456"
}
```

**Validation rules:**
- `ChannelID` must match `^[CDGW][A-Z0-9]{5,}$` (same regex as search).
- `ThreadTS` must be non-empty after the `p`-prefix strip.
- Unparseable URLs return an error with the expected format.

---

### `internal/slack/daterange.go` additions

```go
// parseDateOrOffset resolves either an ISO date string or a duration offset
// of the form `\d+[mhdw]` relative to `now`.
func parseDateOrOffset(s string, now time.Time) (time.Time, error)

// ParseRelativeDateRange parses --since / --until style flag strings.
// Each string may be empty, an ISO date, RFC 3339, or a duration offset.
// Returns an error if the resolved From is after the resolved To.
func ParseRelativeDateRange(since, until string) (DateRange, error)
```

---

### `internal/slack/client.go` additions

```go
// FetchThread returns the root message and all replies for the given thread.
// The first element is always the root message; subsequent elements are replies
// in chronological order.
func (c *Client) FetchThread(ctx context.Context, channelID, threadTS string) ([]Message, error)
```

---

### `internal/emoji/` (new package)

```go
// Package emoji maps Slack :name: tokens to Unicode equivalents.
package emoji

// Render replaces all :name: tokens in s with their Unicode equivalents.
// Tokens with no mapping are left unchanged.
func Render(s string) string

// RenderName returns the Unicode equivalent of a single emoji name
// (without colons). Returns the name unchanged if not found.
func RenderName(name string) string
```

**Embedded file:** `internal/emoji/emoji-map.json` — flat JSON object:
```json
{"thumbsup": "👍", "fire": "🔥", "white_check_mark": "✅", ...}
```

---

### `internal/output/wrap.go` (new file)

```go
// WordWrap wraps s at word boundaries so no line exceeds width runes.
// Continuation lines are indented by indent spaces.
// A width of 0 disables wrapping and returns s unchanged.
func WordWrap(s string, width, indent int) string
```

---

### `internal/output/postmortem.go` (new file)

```go
// IncidentDoc is the structured postmortem document.
type IncidentDoc struct {
    Channel      string
    PeriodFrom   time.Time
    PeriodTo     time.Time
    Participants []string
    Timeline     []TimelineRow
}

// TimelineRow is one row in the incident timeline.
type TimelineRow struct {
    Time    time.Time
    Who     string
    Event   string
    Replies int // > 0 if this was a thread root with replies
}

// PrintPostmortem formats an IncidentDoc to w.
func PrintPostmortem(w io.Writer, fmt Format, doc IncidentDoc) error
```

---

### `internal/output/metrics.go` (new file)

```go
// ChannelMetrics is the aggregated metrics for a channel.
type ChannelMetrics struct {
    UserCounts    []UserCount     // sorted descending by Count
    ThreadCount   int
    AvgReplyDepth float64
    TopReactions  []ReactionCount // top 5 sorted descending
    HourlyDist    [24]int         // UTC hour → message count
}

type UserCount struct {
    DisplayName string
    Count       int
}

type ReactionCount struct {
    Name  string
    Total int
}

// ComputeMetrics aggregates messages into ChannelMetrics.
func ComputeMetrics(messages []slack.Message, resolver *slack.Resolver) ChannelMetrics

// PrintMetrics formats ChannelMetrics to w.
func PrintMetrics(w io.Writer, fmt Format, m ChannelMetrics) error
```

---

### `internal/output/actions.go` (new file)

```go
// ActionItem is a message that matched a commitment pattern.
type ActionItem struct {
    Who       string
    Text      string
    Timestamp time.Time
}

// ExtractActions scans messages for commitment patterns and returns matches.
func ExtractActions(messages []slack.Message, resolver *slack.Resolver) []ActionItem

// PrintActions formats ActionItems to w.
func PrintActions(w io.Writer, fmt Format, items []ActionItem) error
```

---

### `internal/output/digest.go` (new file)

```go
// ChannelDigest groups messages by channel for digest output.
type ChannelDigest struct {
    ChannelName string
    Messages    []slack.Message
}

// GroupByChannel groups messages by ChannelName, sorted descending by count.
func GroupByChannel(messages []slack.Message) []ChannelDigest

// PrintDigest formats ChannelDigest slices to w.
func PrintDigest(w io.Writer, fmt Format, groups []ChannelDigest, resolver *slack.Resolver) error
```

---

## Modified Types / Functions

### `cmd/root.go`

New global flags:
```go
var (
    flagQuiet bool    // --quiet / -q
    flagSince string  // --since (duration or date)
    flagUntil string  // --until (duration or date)
    flagWidth int     // --width (0 = auto)
    flagEmoji *bool   // --emoji / --no-emoji (nil = auto-detect tty)
)
```

`PersistentPreRunE` additions:
- Mutual exclusion: error if both `--from` and `--since` are set, or both
  `--to` and `--until` are set.
- If `--since`/`--until` are set, call `ParseRelativeDateRange` to populate
  `ParsedDateRange`.

### `cmd/search.go`

```go
// channel flag changes from single string to string slice
var channels []string
cmd.Flags().StringArrayVarP(&channels, "channel", "c", nil, "limit to channel (repeatable)")
```

`searchRunFunc` signature changes:
```go
type searchRunFunc func(
    ctx context.Context,
    workspace tokens.Workspace,
    query string,
    channels []string,  // was: channel string
    userArg string,
    dr slack.DateRange,
    limit int,
) ([]slack.SearchResult, error)
```

### `internal/output/format.go`

- `printMessagesText`: message field is wrapped via `WordWrap` before printing.
- `formatReactions`: calls `emoji.RenderName` when emoji rendering is active.
- `resolveMessageFields`: calls `emoji.Render` on `Text` when emoji rendering
  is active.

## State Transitions

None — all entities are value types passed through the pipeline. No persistent
state changes.

## Relationships

```
cmd/thread.go
  → slack.ParsePermalink(url) → ThreadPermalink
  → slack.SelectWorkspaceByURL(workspaces, wpURL) → Workspace
  → slack.Client.FetchThread(channelID, threadTS) → []Message
  → output.PrintMessages (with participant post-processing)

cmd/postmortem.go
  → slack.Client.FetchHistory(channelID, dr, 0, true) → []Message
  → output.BuildIncidentDoc(messages, resolver) → IncidentDoc
  → output.PrintPostmortem(w, format, doc)

cmd/digest.go
  → slack.Client.GetUserMessages(ctx, userID, "", dr, 0) → []Message
  → output.GroupByChannel(messages) → []ChannelDigest
  → output.PrintDigest(w, format, groups, resolver)

cmd/metrics.go
  → slack.Client.FetchHistory(channelID, dr, 0, true) → []Message
  → output.ComputeMetrics(messages, resolver) → ChannelMetrics
  → output.PrintMetrics(w, format, metrics)

cmd/actions.go
  → slack.Client.FetchHistory(channelID, dr, 0, false) → []Message
  → output.ExtractActions(messages, resolver) → []ActionItem
  → output.PrintActions(w, format, items)
```
