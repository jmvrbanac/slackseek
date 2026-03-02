# JSON Output Contract: slackseek

**Feature**: 001-slackseek-cli
**Date**: 2026-03-02

When `--format json` is passed, every command writes a JSON array to stdout.
The schema for each command's array element is defined below. The array is
always present even when empty (`[]`). All timestamps are RFC 3339 strings
in UTC.

---

## `slackseek auth show --format json`

```json
[
  {
    "name":    "Acme Corp",
    "url":     "https://acme.slack.com",
    "token":   "xoxs-1234…",
    "cookie":  "abcd1234…"
  }
]
```

| Field | Type | Notes |
|---|---|---|
| `name` | string | Workspace display name |
| `url` | string | Workspace base URL (https) |
| `token` | string | Truncated token (12 chars + `…`) |
| `cookie` | string | Truncated cookie (8 chars + `…`) |

---

## `slackseek channels list --format json`

```json
[
  {
    "id":           "C01234567",
    "name":         "general",
    "type":         "public_channel",
    "member_count": 42,
    "topic":        "Company-wide announcements",
    "is_archived":  false
  }
]
```

| Field | Type | Notes |
|---|---|---|
| `id` | string | Slack channel ID |
| `name` | string | Channel name without `#` |
| `type` | string | `public_channel` \| `private_channel` \| `mpim` \| `im` |
| `member_count` | int | Number of members (0 for IMs) |
| `topic` | string | Current topic; empty string if none |
| `is_archived` | bool | |

---

## `slackseek history <channel> --format json`

```json
[
  {
    "timestamp":    "2025-01-15T09:30:00Z",
    "slack_ts":     "1736936400.000001",
    "user_id":      "U01234567",
    "text":         "Good morning everyone!",
    "channel_id":   "C01234567",
    "thread_ts":    "",
    "thread_depth": 0,
    "reactions": [
      { "name": "wave", "count": 3 }
    ]
  }
]
```

| Field | Type | Notes |
|---|---|---|
| `timestamp` | string | RFC 3339 UTC |
| `slack_ts` | string | Raw Slack timestamp (seconds.microseconds) |
| `user_id` | string | Slack user ID |
| `text` | string | Full message text (not truncated in JSON) |
| `channel_id` | string | Slack channel ID |
| `thread_ts` | string | Parent ts; empty string for root messages |
| `thread_depth` | int | 0 = root, 1 = reply |
| `reactions` | array | May be empty array |
| `reactions[].name` | string | Emoji name without colons |
| `reactions[].count` | int | Number of users who reacted |

---

## `slackseek messages <user> --format json`

```json
[
  {
    "timestamp":    "2025-01-15T09:30:00Z",
    "slack_ts":     "1736936400.000001",
    "user_id":      "U01234567",
    "text":         "Shipping the fix now.",
    "channel_id":   "C01234567",
    "channel_name": "engineering",
    "thread_ts":    "",
    "thread_depth": 0,
    "reactions":    []
  }
]
```

Same schema as history output with the addition of `channel_name`:

| Field | Type | Notes |
|---|---|---|
| `channel_name` | string | Resolved channel display name; may be empty |
| (all other fields) | | Same as history schema |

---

## `slackseek search <query> --format json`

```json
[
  {
    "timestamp":    "2025-01-15T09:30:00Z",
    "slack_ts":     "1736936400.000001",
    "user_id":      "U01234567",
    "text":         "The deploy finished successfully.",
    "channel_id":   "C01234567",
    "channel_name": "deployments",
    "thread_ts":    "",
    "thread_depth": 0,
    "reactions":    [],
    "permalink":    "https://acme.slack.com/archives/C01234567/p1736936400000001"
  }
]
```

| Field | Type | Notes |
|---|---|---|
| `permalink` | string | Full HTTPS URL to message in Slack web |
| (all other fields) | | Same as messages schema |

---

## `slackseek users list --format json`

```json
[
  {
    "id":           "U01234567",
    "display_name": "jane.doe",
    "real_name":    "Jane Doe",
    "email":        "jane@acme.com",
    "is_bot":       false,
    "is_deleted":   false
  }
]
```

| Field | Type | Notes |
|---|---|---|
| `id` | string | Slack user ID |
| `display_name` | string | @-mentionable name |
| `real_name` | string | Full name |
| `email` | string | May be empty string if not accessible |
| `is_bot` | bool | |
| `is_deleted` | bool | True for deactivated accounts |

---

## Error Output (all commands)

Errors are **never** written to stdout. When a command fails:
- stderr receives a human-readable error message.
- stdout receives nothing (or an empty partial result if the error occurred
  mid-stream — the JSON array will be closed and valid).
- Exit code is 1.

```
Error: failed to open LevelDB copy at /tmp/slackseek-*:
  Slack may be holding a file lock on ~/.config/Slack/Local Storage/leveldb.
  Try: close Slack and retry, or use --workspace to select a different source.
```

No JSON error envelope is emitted; errors are human-readable text on stderr
only. Tools that consume the JSON output MUST check the exit code to detect
failures.
