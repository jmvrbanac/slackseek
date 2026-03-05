# Research: 005 Day 2 Improvements

All design decisions are documented below. No NEEDS CLARIFICATION items remain.

---

## 1. Terminal Width Detection (line wrapping)

**Decision:** Add `golang.org/x/term` as a direct dependency.
Use `term.GetSize(int(os.Stdout.Fd()))` which works on both Linux and macOS
without OS-specific code. When the call fails (non-tty, dumb terminal) default
to 120. A `--width` flag and `SLACKSEEK_WIDTH` env var override in priority
order: flag > env var > tty detection > 120 fallback.

**Rationale:** `golang.org/x/term` is part of the Go extended standard library,
already transitively available via `golang.org/x/crypto` (both live in the
`golang.org/x` tree). Adding it as an explicit `require` is the idiomatic
approach. The `term.GetSize` API is cross-platform (Linux/macOS) and returns
`(width, height, error)`, eliminating the need for platform-specific files.

**Word-wrap algorithm:**
`internal/output/wrap.go` will expose `WordWrap(s string, width int, indent int) string`.
- Split `s` into words on whitespace.
- Greedily fill lines up to `width` characters.
- Prefix continuation lines with `indent` spaces.
- Width of 0 → return `s` unchanged.

**Tab-stop calculation:**
Text output columns are `time \t channel \t user \t message`. After splitting
on `\t`, the message column starts at `sum(len(col)+1)` characters from column
0. The wrapper receives `remainingWidth = totalWidth − prefixWidth`.

**Alternatives considered:**
- `syscall.TIOCGWINSZ` directly — requires platform-specific code and
  `//go:build` tags (violates Principle V for a pure display concern); `x/term`
  already abstracts this.
- `github.com/muesli/reflow` — external dependency for word-wrap only;
  `internal/output/wrap.go` is ~30 lines of stdlib code.

---

## 2. `--since` / `--until` Relative Dates

**Decision:** Extend `internal/slack/daterange.go`. Add `parseDateOrDuration(s string, now time.Time) (time.Time, error)`.
Supported duration units: `m` (minutes), `h` (hours), `d` (days), `w` (weeks).
Pattern: `^\d+[mhdw]$`. For `--since 7d`, result = `now.Add(-7 * 24 * time.Hour)`.
Weeks are defined as 7 days (no calendar logic needed).

`ParseDateRange` is called with `from`/`to` strings. A sibling
`ParseRelativeDateRange(since, until string) (DateRange, error)` is added that
wraps the same validation logic (From < To).

In `cmd/root.go`: add `--since` and `--until` string flags alongside existing
`--from` / `--to`. Mutual exclusion checked in `PersistentPreRunE`.

**Rationale:** Adding a sibling parse function keeps `ParseDateRange` unchanged
(no breaking change to call sites in tests). The mutual-exclusion guard in the
root command is the right layer — it avoids duplicating the guard in every
subcommand.

**Alternatives considered:**
- Replace `--from` / `--to` entirely — breaks existing scripts that use the old
  flags; additive is safer.
- Accept full Go duration strings (`time.ParseDuration`) — would allow `1h30m`
  naturally but the spec says `m`, `h`, `d`, `w` units; `time.ParseDuration`
  does not know `d` or `w`. Custom parsing is 10 lines.

---

## 3. `slackseek thread <permalink-url>` Command

**Decision:** New `cmd/thread.go` command. URL parsing in a new helper
`internal/slack/permalink.go`.

**Permalink format:**
`https://<workspace>.slack.com/archives/<channelID>/p<tsEncoded>`
where `<tsEncoded>` = Unix epoch seconds concatenated with 6-digit microseconds
(no dot). Example: `p1700000000123456` → `1700000000.123456`.

**Parse algorithm:**
1. Split path on `/` to get `[archives, channelID, pXXX]`.
2. Strip leading `p` from the last segment.
3. If length > 6: insert `.` before the last 6 digits.
4. Return `ThreadPermalink{WorkspaceURL: scheme+host, ChannelID, ThreadTS}`.

**Workspace selection:** Match `ThreadPermalink.WorkspaceURL` against
`ws.URL` (stored credential) using `strings.Contains` (Slack URLs are
`https://company.slack.com`; `ws.URL` = `https://company.slack.com`).

**Reply-vs-root detection:** If the permalink includes a `?thread_ts=<ts>`
query param, the permalink points to a reply; use the `thread_ts` value to
fetch from the root. Otherwise `ThreadTS = tsEncoded` and that is the root.

**Thread fetch:** Reuse `c.FetchHistory` with `threads=true` and filter to
`ThreadTS == rootTS` or call `GetConversationRepliesContext` directly. The
latter is cleaner (dedicated API). Add `FetchThread(ctx, channelID, threadTS)
([]Message, error)` to `internal/slack/client.go`.

**Participant list:** Post-process the returned messages. Collect unique
display names (via resolver) sorted alphabetically. Append to output under
a `## Participants` section (text/markdown) or a `"participants"` field (JSON).

**Alternatives considered:**
- Reuse `FetchHistory` with thread filter — would fetch entire channel; the
  `conversations.replies` API is the correct, paginated endpoint.
- Parse workspace from stored `ws.Name` — less reliable than URL matching.

---

## 4. `--quiet` Flag

**Decision:** Add `--quiet` / `-q` to the root persistent flags as `flagQuiet bool`.
In `defaultRunHistory`, wrap the `SetPageFetchedCallback` and `SetRateLimitCallback`
calls behind `if !flagQuiet`. No other changes required; `Warning:` prints use
`fmt.Fprintln(os.Stderr, "Warning:", w)` which are in `runHistoryE` / `runSearchE`
and are intentionally not gated.

**Rationale:** The quiet flag only needs to be threaded through the two callbacks
in `defaultRunHistory`. There are no other progress-emitting paths (channels and
users fetch is silent). Adding a global flag avoids touching every subcommand.

**Alternatives considered:**
- `--no-progress` — less standard; `-q`/`--quiet` is the Unix convention.
- Per-command quiet flags — unnecessary since only `history` emits progress.

---

## 5. Multi-Channel Search

**Decision:** Change `--channel` flag in `cmd/search.go` from `StringVarP` to
`StringArrayVarP`. Update `searchRunFunc` signature to accept `[]string channels`
instead of `string channel`. In `defaultRunSearch`: if `len(channels) == 1`,
existing single-call path is used. If `len(channels) > 1`, use
`golang.org/x/sync/errgroup` with a `semaphore` limiting concurrency to 3.
Merge results, deduplicate by `Timestamp` (use `map[string]struct{}`), sort by
`Message.Time` ascending.

**errgroup source:** `golang.org/x/sync` — needs to be added to `go.mod`.
It is a canonical Go extended library. Alternatively, a manual
`sync.WaitGroup`+channel approach can avoid the dependency.

**Decision:** Use `sync.WaitGroup` + `chan []SearchResult` + `chan error` to
avoid adding a new dependency. The pattern is ~20 lines and straightforward.
A semaphore channel (`make(chan struct{}, 3)`) limits parallelism.

**Rationale:** Avoids a new `go.mod` entry. The stdlib approach is fully
readable within 40 lines.

**Alternatives considered:**
- `errgroup` — cleaner API but adds `golang.org/x/sync` to go.mod.
- Sequential calls with `for range channels` — simpler but slower for large
  channel lists.

---

## 6. Emoji Rendering

**Decision:** New `internal/emoji` package with:
- `emoji-data.json` embedded via `//go:embed` — a minimal `name → unified`
  map generated from the MIT-licensed `iamcal/emoji-data` dataset.
- `Render(text string) string` — regex-replaces `:name:` tokens.
- `RenderReaction(name string) string` — looks up a single name.

**Dataset:** The `iamcal/emoji-data` JSON is ~3 MB uncompressed. We need only
the `short_name → unified` (hex codepoint) mapping. A pre-generated
`emoji-map.json` (name → Unicode string) of ~150 KB will be committed to
`internal/emoji/`.

**Initialization:** `var defaultTable = mustLoad()` at package level — parsed
once using `sync.Once` semantics (via package init). Parse errors cause a
`log.Fatal` (acceptable: the file is bundled, parse failure = build problem).

**Emoji rendering toggle:** `IsTerminal(os.Stdout.Fd())` check using
`golang.org/x/term` (already added for line wrap). Default on for tty.
Global flags `--emoji` / `--no-emoji` override.

**Alternatives considered:**
- `kyokomi/emoji` package — external dependency that bundles the full dataset;
  our embedded map approach is equivalent in functionality at lower
  dependency cost.
- Runtime download — rejected; offline-first design principle.

---

## 7. Dev Manager Commands (`postmortem`, `digest`, `metrics`, `actions`)

All four commands are pure post-processing over existing `FetchHistory` /
`GetUserMessages` output. No new API calls.

**`postmortem`:**
- Fetches full history with threads (`FetchHistory`, `threads=true`).
- Derives period from `min(Message.Time)` → `max(Message.Time)`.
- Participants = unique resolved display names sorted alphabetically.
- Timeline table: `| Time | Who | Event |` with thread replies collapsed
  to `(N replies)` footnote.
- Output: Markdown only (default); `--format json` → structured JSON.

**`digest`:**
- Calls `c.GetUserMessages(ctx, userID, "", dr, 0)`.
- Groups by `ChannelName` (from search results), sorts by count descending.
- Text: channel heading + one line per message (first 80 chars).
- JSON: full message list keyed by channel.

**`metrics`:**
- Calls `FetchHistory` with threads.
- Aggregates: `map[displayName]int` counts; thread reply depths; reaction
  counts across all messages; `[24]int` hourly histogram (UTC).
- Text: table + ASCII bar chart for hourly histogram.
- JSON: structured JSON with all four aggregations.

**`actions`:**
- Calls `FetchHistory` without threads (root messages only).
- Applies regex patterns (case-insensitive):
  - `(?i)\bI'?ll\b`, `(?i)\bI will\b`, `(?i)\bwill do\b`, `(?i)\bon it\b`
  - `(?i)action item`, `(?i)\bTODO\b`, `(?i)\bfollow[ -]?up\b`
  - `(?i)<@[A-Z0-9]+>.*(?:can you|please)`
- Emits checklist: `[ ] @user  <message preview>  <timestamp>`.
- JSON: array of `{user, text, timestamp}` objects.

**Structure decision:** Each command gets its own `cmd/*.go` file. Output
helpers go into `internal/output/postmortem.go`, `digest.go`, `metrics.go`,
`actions.go`. The `internal/output` package is the right layer for all
formatting logic (Single-Responsibility principle).

---

## Dependency Summary

| Dependency | Already in go.mod? | Action |
|---|---|---|
| `golang.org/x/term` | No | Add (`go get golang.org/x/term`) |
| `golang.org/x/sync` | No | Not needed — stdlib WaitGroup pattern |
| `internal/emoji/emoji-map.json` | N/A | Generate + commit |
