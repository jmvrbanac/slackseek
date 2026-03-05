# Bug Fixes

## 1. Inline @mentions with embedded labels not resolved

**Symptom:** Messages containing `<@U22JKTL6N|nmollenkopf>` are printed verbatim
instead of being replaced with a human-readable name. Only the bare `<@USERID>`
form is matched.

**Root cause:** `mentionPattern` in `internal/slack/resolver.go` is:

```go
var mentionPattern = regexp.MustCompile(`<@([A-Z0-9]+)>`)
```

It does not account for the `<@USERID|username>` variant that Slack emits when
the client already knows the username at message-send time.

**Fix:** Update `mentionPattern` to optionally capture the embedded label:

```go
var mentionPattern = regexp.MustCompile(`<@([A-Z0-9]+)(?:\|([^>]+))?>`)
```

In `ResolveMentions`, use `FindStringSubmatch` (like `subteamPattern` already
does) so the handler can:
1. Resolve by user ID from the cache/API (preferred — gives real name).
2. Fall back to the embedded label (e.g. `nmollenkopf`) if the ID is not in
   the resolver map.
3. Fall back to the raw ID only if neither is available.

**Affected files:**
- `internal/slack/resolver.go` — fix regex + mention handler
- `internal/slack/resolver_test.go` — add test cases for `<@ID|label>` form

**Scope:** All output paths that call `resolver.ResolveMentions` are affected:
`PrintMessages` (text/table/JSON) and `PrintSearchResults` (text/table/JSON)
in `internal/output/format.go`.

## 2. Threaded messages lack visual grouping

**Symptom:** `history --threads` outputs thread replies in chronological order
mixed with root messages, making it impossible to follow a conversation thread.
`ThreadDepth` and `ThreadTS` fields exist on `Message` but are unused by the
formatters.

**Current data shape:** `Message.ThreadDepth` is 0 for root messages and 1 for
direct replies; `Message.ThreadTS` holds the parent message's Slack timestamp,
which is the stable grouping key.

**Fix:** Add a threaded display mode to `internal/output/format.go`.

### Text / table formats

Group replies under their parent before printing:

1. Group messages by `ThreadTS`: root messages (empty `ThreadTS` or
   `ThreadTS == Timestamp`) anchor each group; replies with a matching
   `ThreadTS` belong to that group.
2. Print each root message normally.
3. Indent each reply with a visible prefix (e.g. `  └─ `) and include the
   reply author and timestamp.
4. Separate thread groups with a blank line for readability.

Example text output:

```
2024-01-15T10:00:00Z  #incidents  Alice     The deploy is failing.
  └─ 2024-01-15T10:02:00Z  Bob       Investigating now.
  └─ 2024-01-15T10:05:00Z  Alice     Found it — bad env var. Fixing.

2024-01-15T10:10:00Z  #incidents  Carol     Unrelated root message.
```

### JSON format

Add a `replies` array to each root message object instead of emitting flat
rows. Replies are omitted from the top-level array and nested under their
parent:

```json
[
  {
    "timestamp": "...",
    "user_display_name": "Alice",
    "text": "The deploy is failing.",
    "replies": [
      { "timestamp": "...", "user_display_name": "Bob",   "text": "Investigating now." },
      { "timestamp": "...", "user_display_name": "Alice", "text": "Found it — bad env var." }
    ]
  }
]
```

### Affected files

- `internal/output/format.go` — add `groupByThread` helper; update
  `PrintMessages` text/table/JSON paths; extend `messageJSON` with a
  `replies []messageJSON` field (omitempty).
- `internal/output/format_test.go` — add tests for grouped output with mixed
  root + reply messages.

### Non-goals

- `PrintSearchResults` is intentionally excluded: search results are
  cross-channel snippets where thread context is less meaningful. Raw
  chronological order is fine there.
- No new flags are added. Grouping is applied whenever `--threads` produces
  messages that have non-empty `ThreadTS` values (i.e. the data drives the
  behaviour, not a separate flag).

## 3. Markdown export for history

**Motivation:** Users want to save channel history as a readable document
(e.g. incident post-mortems, decision logs) that renders nicely in GitHub,
Notion, or any Markdown viewer.

**Change:** Add `markdown` as a new accepted value for `--format`.

### Wire-up

- `output.ValidFormats` gains `FormatMarkdown Format = "markdown"`.
- `validateFormat` in `cmd/root.go` already iterates `ValidFormats`, so it
  picks up the new value automatically. The `--format` help string changes
  from `text | table | json` to `text | table | json | markdown`.
- `PrintMessages` gains a `FormatMarkdown` case.
- `PrintSearchResults` gains a `FormatMarkdown` case (useful for archiving
  search hits alongside history exports).

### Document structure (`PrintMessages`)

Thread-grouped output (applying the same grouping logic from item 2):

```markdown
# #incidents — 2024-01-15

## 10:00 · Alice
The deploy is failing.

> **10:02 · Bob**
> Investigating now.

> **10:05 · Alice**
> Found it — bad env var. Fixing.
> :white_check_mark:×1

---

## 10:10 · Carol
Unrelated root message.
```

Rules:
- Document heading: `# #{channel} — {date}` (date from the first message;
  omitted when messages span multiple days — use `# #{channel}` instead).
- Each root message is an `##` heading with `HH:MM · {user}`.
- Thread replies are block-quoted (`> `) with bold `**HH:MM · {user}**` on
  the first line of the quote.
- Reactions appended as `:name:×count` on the last line of the block.
- A `---` rule separates root-message groups.
- Message text is used as-is (already resolved through `ResolveMentions`);
  no additional escaping beyond what Markdown naturally handles.

### Search results (`PrintSearchResults`)

Simpler flat list without thread grouping:

```markdown
# Search results

## 2024-01-15 10:00 · #incidents · Alice
The deploy is failing.
[View in Slack](https://…)

---
```

### Scope: markdown is intentionally limited to message/search output

`--format markdown` is only wired into `PrintMessages` and
`PrintSearchResults`. It is **not** added to `PrintChannels`, `PrintUsers`,
or `PrintWorkspaces`:

- `PrintChannels` / `PrintUsers` — purely tabular data; a Markdown table
  would be functionally identical to `--format table` and adds no value.
- `PrintWorkspaces` — contains sensitive credential fragments; a Markdown
  document of those is an odd artifact with no clear use case.

If a channels-as-Markdown-table or user-roster use case emerges later, it
can be added as a separate, explicitly motivated feature.

### Affected files

- `internal/output/format.go` — add `FormatMarkdown` constant; add
  `printMessagesMarkdown` and `printSearchResultsMarkdown` helpers called
  from the existing switch statements.
- `internal/output/format_test.go` — add golden-output tests for the new
  format.
- `cmd/root.go` — update `--format` flag description string.

## 4. DM channel names not resolved to user display names

**Symptom:** When running `messages <user>`, results that came from a DM
conversation show the raw other-user ID (e.g. `U01ABCDEF`) as the channel
name instead of a human-readable name like `@Alice`.

**Root cause:** Two compounding issues:

1. The Slack search API returns `m.Channel.Name` as the other user's Slack ID
   for DM channels (IDs starting with `D`). `search.go:73` stores this
   directly into `Message.ChannelName`, so `ChannelName` is already non-empty.

2. In `resolveMessageFields` (`internal/output/format.go`), the resolver is
   only called when `channelDisplay == ""`. Since `ChannelName` is set (to
   a user ID string), the resolver is never consulted. Even if it were,
   `resolver.ChannelName` only checks the channels map — it has no awareness
   of user IDs.

**Fix:** Extend `Resolver` with a `ResolveChannelDisplay` method that handles
DM names:

```go
// ResolveChannelDisplay resolves a channel name for display. If name looks
// like a Slack user ID (starts with 'U'), it is resolved via the users map
// and prefixed with '@'. Otherwise the channel ID is resolved normally.
func (r *Resolver) ResolveChannelDisplay(id, name string) string {
    if strings.HasPrefix(name, "U") && userIDPattern.MatchString(name) {
        return "@" + r.UserDisplayName(name)
    }
    if name != "" {
        return name
    }
    return r.ChannelName(id)
}
```

Update `resolveMessageFields` and `toMessageJSON` in
`internal/output/format.go` to call `ResolveChannelDisplay(m.ChannelID, m.ChannelName)`
instead of the current conditional.

**Affected files:**
- `internal/slack/resolver.go` — add `ResolveChannelDisplay`; add
  `userIDPattern` regex (`^U[A-Z0-9]+$`).
- `internal/slack/resolver_test.go` — add tests for DM name resolution.
- `internal/output/format.go` — update `resolveMessageFields` and
  `toMessageJSON` to use `ResolveChannelDisplay`.

## 6. Proactive rate limiting to avoid API 429 stalls

**Symptom:** Commands that paginate multiple API endpoints (channel list, history,
search) stall silently. The `Retry-After` callback only logs waits longer than 30 s,
so short waits appear as unexplained pauses.

**Root cause:** The retry mechanism is purely reactive — it waits on a 429.
For multi-page operations on large workspaces, rapid page fetches exceed Slack's
tier limits before the first 429 is even received:

| Method | Tier | Limit |
|--------|------|-------|
| `conversations.list` | 2 | 20+/min |
| `users.list` | 2 | 20+/min |
| `search.messages` | 2 | 20+/min |
| `conversations.history` | 3 | 50+/min |
| `conversations.replies` | 3 | 50+/min |

**Fix:** Add a `rateLimiter` type to `internal/slack/client.go` that enforces a
minimum interval between sequential calls. Two instances are added to `Client` —
one per tier. The first call is always immediate; subsequent calls wait until the
interval has elapsed. No goroutines, no channels — just a last-call timestamp.

Conservative safe intervals (10% margin below the documented minimum):
- Tier 2 → 18 req/min → ~3.3 s between pages
- Tier 3 → 48 req/min → ~1.25 s between pages

**Integration points:**
- `ListChannels` — `tier2.Wait(ctx)` before each page in the pagination loop
- `historyPageFetch` — `tier3.Wait(ctx)` before `callWithRetry`
- `repliesPageFetch` — `tier3.Wait(ctx)` before `callWithRetry`
- `SearchMessages` — `tier2.Wait(ctx)` before each page in the pagination loop
- `ListUsers` — `tier2.Wait(ctx)` before `GetUsersContext`

**Affected files:**
- `internal/slack/client.go` — `rateLimiter` type; `tier2`/`tier3` fields on `Client`;
  initialize in `NewClient`
- `internal/slack/client_test.go` — unit tests for `rateLimiter`
- `internal/slack/channels.go` — `Wait` calls in `ListChannels`, `historyPageFetch`,
  `repliesPageFetch`
- `internal/slack/search.go` — `Wait` call in `SearchMessages`
- `internal/slack/users.go` — `Wait` call before `GetUsersContext`

**Non-goals:** per-method limiters; shared state across concurrent invocations
(CLI is single-command); burst credit — conservative sustained rate is sufficient.

## 5. Multi-line messages misalign in table view

**Symptom:** A message containing newlines renders with continuation lines
starting at column 0 rather than staying within the cell boundary:

```
| 2024-01-15T10:00Z | Alice | #incidents | The server is down.
We need to act now. | 0 |
```

**Root cause:** The existing `truncate` helper cuts text at 80 runes but
does not strip embedded `\n` characters. When a cell value contains a newline,
tablewriter emits the continuation at the start of a new terminal line with
no column padding or border, breaking the visual grid.

**Fix:** Add a `tableSafe` helper in `internal/output/format.go` that
collapses all newlines (and any surrounding whitespace) to a single space
before truncating:

```go
func tableSafe(s string, n int) string {
    s = strings.Join(strings.Fields(s), " ")
    return truncate(s, n)
}
```

Replace every `truncate(text, N)` call in `FormatTable` switch cases with
`tableSafe(text, N)`. Non-table paths (`FormatText`, `FormatJSON`,
`FormatMarkdown`) are unaffected — they should preserve newlines.

**Affected files:**
- `internal/output/format.go` — add `tableSafe`; apply it in all
  `FormatTable` cases in `PrintMessages` and `PrintSearchResults`.
- `internal/output/format_test.go` — add a test asserting that a message
  with embedded newlines produces a single-line cell value.
