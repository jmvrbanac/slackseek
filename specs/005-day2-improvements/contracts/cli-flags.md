# CLI Contracts: 005 Day 2 Improvements

## Global Flag Changes (`slackseek [global flags] <command>`)

### New global flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--quiet` | `-q` | bool | false | Suppress progress and rate-limit notices from stderr |
| `--since` | — | string | "" | Start of range: ISO date, RFC 3339, or duration (`30m`, `4h`, `7d`, `2w`) |
| `--until` | — | string | "" | End of range: ISO date, RFC 3339, or duration offset |
| `--width` | — | int | 0 | Text wrap column width (0 = auto-detect tty or 120 for pipes) |
| `--emoji` | — | bool | auto | Render `:name:` tokens as Unicode (default: on for tty, off for pipe) |

### Flag interaction rules

- `--since` and `--from` are mutually exclusive (error if both provided).
- `--until` and `--to` are mutually exclusive (error if both provided).
- `--emoji` and `--no-emoji` are mutually exclusive cobra `BoolVar` with
  default tied to `isatty(os.Stdout.Fd())`.
- `--width 0` disables wrapping entirely (even on tty).

---

## Modified Commands

### `search` command

```
slackseek search <query> [flags]
```

**Changed flags:**

| Flag | Before | After |
|------|--------|-------|
| `--channel` / `-c` | `string` (single) | `[]string` (repeatable) |

**Example:**
```sh
slackseek search "incident" --channel ic-5697 --channel ic-5698
```

---

## New Commands

### `thread` command

```
slackseek thread <permalink-url> [flags]
```

**Args:** exactly one Slack permalink URL.

**Output formats:** `text` | `table` | `json` | `markdown` (via global `--format`).

**Text output example:**
```
2026-02-25T16:03:00Z  C01234  Alice   The deploy is failing.
  └─ 2026-02-25T16:04:00Z  Bob     Investigating now.
  └─ 2026-02-25T16:07:00Z  Alice   Found it.

Participants: Alice, Bob
```

**JSON output schema:**
```json
{
  "thread_ts": "1700000000.123456",
  "channel_id": "C01234567",
  "participants": ["Alice", "Bob"],
  "messages": [
    {
      "timestamp": "...",
      "time": "...",
      "user": "Alice",
      "text": "...",
      "thread_depth": 0,
      "reactions": []
    }
  ]
}
```

---

### `postmortem` command

```
slackseek postmortem <channel> [--since <duration>] [--until <duration>] [flags]
```

**Args:** exactly one channel name or ID.

**Default format:** `markdown` (unlike other commands which default to `text`).

**Markdown output example:**
```markdown
# Incident: ic-5697

**Period:** 2026-02-25 15:00 UTC – 2026-02-25 17:30 UTC
**Participants:** Alice, Bob, Carol

## Timeline

| Time (UTC)       | Who   | Event                                     |
|------------------|-------|-------------------------------------------|
| 2026-02-25 15:00 | Alice | Deploy started                            |
| 2026-02-25 15:05 | Bob   | Error reported (3 replies)                |
| 2026-02-25 15:20 | Carol | Rollback initiated                        |
```

**JSON output schema:**
```json
{
  "channel": "ic-5697",
  "period": {"from": "...", "to": "..."},
  "participants": ["Alice", "Bob", "Carol"],
  "timeline": [
    {"time": "...", "who": "Alice", "event": "...", "replies": 0}
  ]
}
```

---

### `digest` command

```
slackseek digest --user <name-or-id> [--since <duration>] [--until <duration>] [flags]
```

**Required flags:** `--user` / `-u`

**Text output example:**
```
## #general (12 messages)
2026-02-25T10:00:00Z  Alice  The staging env is down again
2026-02-25T10:15:00Z  Alice  Scratch that, it's back up

## #incidents (3 messages)
2026-02-25T14:00:00Z  Alice  Starting the hotfix deploy now
```

**JSON output schema:**
```json
[
  {
    "channel": "general",
    "count": 12,
    "messages": [{"timestamp": "...", "time": "...", "text": "...", ...}]
  }
]
```

---

### `metrics` command

```
slackseek metrics <channel> [--since <duration>] [--until <duration>] [flags]
```

**Args:** exactly one channel name or ID.

**Text output example:**
```
=== Message counts ===
Alice    42
Bob      28
Carol    15

=== Thread stats ===
Thread count: 12  Average replies: 3.2

=== Top reactions ===
👍×18  ✅×12  🔥×7  ❤️×5  😂×3

=== Messages by hour (UTC) ===
00 |
...
14 |██████████████████  23
15 |████████████  14
16 |████████  9
```

**JSON output schema:**
```json
{
  "users": [{"name": "Alice", "count": 42}],
  "threads": {"count": 12, "avg_reply_depth": 3.2},
  "top_reactions": [{"name": "thumbsup", "total": 18}],
  "hourly": [{"hour": 14, "count": 23}, {"hour": 15, "count": 14}]
}
```

---

### `actions` command

```
slackseek actions <channel> [--since <duration>] [--until <duration>] [flags]
```

**Args:** exactly one channel name or ID.

**Text output example:**
```
Action items in #incidents (last 7 days):

[ ] @alice   I'll send the postmortem draft by EOD        2026-02-25 16:10
[ ] @bob     I will investigate the V1 endpoint scope     2026-02-25 16:07
[ ] team     Action item: confirm impacted customer list  2026-02-25 16:07

3 action item(s) found.
```

**JSON output schema:**
```json
[
  {
    "user": "alice",
    "text": "I'll send the postmortem draft by EOD",
    "timestamp": "2026-02-25T16:10:00Z"
  }
]
```

---

## `internal/slack` API Contracts

### `ParsePermalink(url string) (ThreadPermalink, error)`

- Accepts: `https://<workspace>.slack.com/archives/<channelID>/p<ts>[?thread_ts=<ts>]`
- Returns: `ThreadPermalink{WorkspaceURL, ChannelID, ThreadTS}` where `ThreadTS`
  is the root thread timestamp (from `?thread_ts` if present, else from path).
- Error: if scheme is not `https`, path segments are missing, or `p`-prefix is absent.

### `(c *Client) FetchThread(ctx, channelID, threadTS string) ([]Message, error)`

- Returns root + replies in chronological order (first element = root).
- Empty slice = thread not found or deleted.
- Error wraps `conversations.replies` API errors with context.

### `ParseRelativeDateRange(since, until string) (DateRange, error)`

- Accepts: empty string, ISO date (`2006-01-02`), RFC 3339, or `\d+[mhdw]`.
- Returns a `DateRange` with `From` and `To` set appropriately.
- Error: unrecognised format or `From > To`.
