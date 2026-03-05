# Research: 004 Bug Fixes

All five fixes are self-contained changes to existing packages with no new
external dependencies. No NEEDS CLARIFICATION items remain. This document
records the design decisions that were evaluated before settling on the
approach in the plan.

---

## Fix 1 — `<@USERID|label>` mention regex

**Decision:** Extend `mentionPattern` to `<@([A-Z0-9]+)(?:\|([^>]+))?>` and
use `ReplaceAllStringFunc` with `FindStringSubmatch` (the same pattern already
used by `subteamPattern`).

**Rationale:** The embedded label is a reliable fallback when the user ID is
not in the resolver map (e.g. guest users not returned by `users.list`). This
mirrors how Slack's own desktop client renders mentions.

**Alternatives considered:**
- Strip the label and rely solely on the resolver — rejected because guests
  and deactivated users are often absent from the user list; the embedded
  label is better than a raw ID.
- Replace the whole mention token during ingestion in `search.go` — rejected
  because resolver state isn't available there; transformation belongs in the
  output layer.

---

## Fix 2 — Thread grouping algorithm

**Decision:** Single-pass O(n) algorithm using a `[]string` slice of root
timestamps (preserves arrival order) and a `map[string][]Message` for replies
keyed by `ThreadTS`. A message is a root if `ThreadTS == ""` or
`ThreadTS == Timestamp`.

**Rationale:** Messages from the history API arrive in chronological order.
Using a slice for root keys guarantees output order matches input order without
a sort step.

**Alternatives considered:**
- Sort all messages by timestamp then group — adds an O(n log n) sort that
  is unnecessary since the API already returns them in order.
- Use `ThreadDepth == 0` to detect roots — `ThreadDepth` is set to 0 for all
  messages that are not thread replies; relying on `ThreadTS` is more robust
  because `ThreadDepth` is a derived field that could be ambiguous for
  root-level broadcast messages.

**Grouping shared between formats:** `groupByThread` returns
`(roots []Message, replies map[string][]Message)`. Both the text/table path
and the markdown path call this same helper.

---

## Fix 3 — Markdown format channel heading

**Decision:** Derive the document heading channel name from
`resolver.ChannelName(messages[0].ChannelID)` when `messages[0].ChannelName`
is empty. If the slice is empty, omit the heading. If messages span multiple
dates, use `# #{channel}` without a date suffix.

**Rationale:** `PrintMessages` receives only `[]slack.Message` and a resolver;
no channel name parameter exists. The first message's `ChannelID` is always
present and the resolver already has the channel name map loaded.

**Alternatives considered:**
- Add a `channelName string` parameter to `PrintMessages` — rejected; would
  break the existing call sites and the function signature is already stable.
- Use `ChannelName` from the first message directly — too fragile; `ChannelName`
  is empty in history context (set only for search results).

**Markdown format scope:** Limited to `PrintMessages` and `PrintSearchResults`.
`PrintChannels`, `PrintUsers`, and `PrintWorkspaces` are excluded (tabular
data offers no benefit from Markdown over `--format table`; workspace output
contains credential fragments).

---

## Fix 4 — DM channel name resolution

**Decision:** Add `ResolveChannelDisplay(id, name string) string` to
`slack.Resolver`. The method checks if `name` matches `^U[A-Z0-9]+$`
(Slack user ID pattern) and, if so, looks up the user display name.

**Rationale:** Slack search API sets `Channel.Name` to the other user's ID for
DM channels (IDs starting with `D`). This is not documented but is consistent
across all tested workspaces. The pattern check is cheap and safe — regular
channel names never start with `U` followed by uppercase alphanumerics.

**Alternatives considered:**
- Check `ChannelID` prefix `D` to detect DMs — less reliable; `ChannelID`
  is the DM channel's own ID, not the other user's ID.  Knowing the DM channel
  ID does not directly yield the other user's display name without a separate
  API call.
- Fetch DM participants from the API — rejected; requires a new API call per
  DM, breaks the offline/cache-first design, and is disproportionate for a
  display fix.

---

## Fix 5 — Table newline alignment

**Decision:** Add `tableSafe(s string, n int) string` that calls
`strings.Fields(s)` + `strings.Join(..., " ")` before truncating. Apply in
all `FormatTable` cases that pass user-controlled text to table cells.

**Rationale:** `strings.Fields` handles `\n`, `\r\n`, `\t`, and runs of
spaces in a single pass. It is idiomatic Go and requires no regex.

**Alternatives considered:**
- Configure tablewriter `AutoWrap` — tablewriter v1.1.3 supports `WrapNormal`
  with `ColMaxWidths`, but this wraps within column boundaries rather than
  collapsing multi-line input. A long Slack message would still produce
  multiple visual rows; the user experience is better with a single collapsed
  line that is truncated.
- `strings.ReplaceAll(s, "\n", " ")` — handles only `\n`, misses `\r\n` and
  tabs. `strings.Fields` is strictly better.
