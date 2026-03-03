# CLI Contract: Resolve User IDs and Channel IDs in Output

**Feature**: 003-resolve-ids-in-output

---

## Affected Commands

No new commands or flags are introduced. The change is purely in output rendering.

---

## Output Schema Changes

### `slackseek history <channel>`

**Text / Table format** (unchanged invocation, changed content):

| Before | After |
|--------|-------|
| `2026-01-01T10:00:00Z  U01234567  Hello world` | `2026-01-01T10:00:00Z  alice  Hello world` |
| Table "User" column: `U01234567` | Table "User" column: `alice` |

**JSON format** (additive change — backwards compatible):

```json
{
  "timestamp": "2026-01-01T10:00:00Z",
  "slack_ts": "1735725600.000000",
  "user_id": "U01234567",
  "user_display_name": "alice",
  "text": "Hello world",
  "channel_id": "C01234567",
  "channel_name": "general",
  "thread_ts": "",
  "thread_depth": 0,
  "reactions": []
}
```

*New field*: `user_display_name` (string, empty if unresolvable)
*Extended field*: `channel_name` — now populated for history (was previously always empty)

---

### `slackseek messages <user>`

**Text / Table format**:

| Before | After |
|--------|-------|
| `2026-01-01T10:00:00Z  U01234567  C01234567  Hello world` | `2026-01-01T10:00:00Z  alice  #general  Hello world` |
| Table "User" column: `U01234567` | Table "User" column: `alice` |
| Table "Channel" column: `C01234567` | Table "Channel" column: `general` |

**JSON format**: same additions as `history`.

---

### `slackseek search <query>`

**Text / Table format**:

| Before | After |
|--------|-------|
| Table "User" column: `U01234567` | Table "User" column: `alice` |
| Table "Channel" column: already resolved by API | Table "Channel" column: unchanged |

**JSON format**:

```json
{
  "user_id": "U01234567",
  "user_display_name": "alice",
  "channel_id": "C01234567",
  "channel_name": "general",
  ...
}
```

---

## Backwards Compatibility

- All existing JSON fields are retained; `user_display_name` is additive.
- Text and table output changes the content of the User/Channel columns from raw IDs to
  names. This is intentional and is the purpose of the feature. Scripts parsing raw IDs
  from text output should migrate to `--format json` and use `user_id`.
- `--no-cache` continues to work; output falls back to raw IDs.

---

## Unchanged Commands

- `slackseek channels list` — already displays names; no change.
- `slackseek users list` — already displays names; no change.
- `slackseek auth show` — workspace credentials; no change.
- `slackseek cache clear` — cache management; no change.
