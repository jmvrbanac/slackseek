# CLI Contract: slackseek

**Feature**: 001-slackseek-cli
**Date**: 2026-03-02

This document defines the stable command-line interface contract for
`slackseek`. Any change to flags, argument ordering, or exit codes is a
breaking change requiring a MAJOR version bump.

---

## Global Flags

Applied to every subcommand.

| Flag | Short | Type | Default | Description |
|---|---|---|---|---|
| `--workspace` | `-w` | string | (first found) | Workspace name or base URL to target |
| `--format` | | string | `text` | Output format: `text` \| `table` \| `json` |
| `--from` | | string | (none) | Start of date range: `YYYY-MM-DD` or RFC 3339 |
| `--to` | | string | (none) | End of date range: `YYYY-MM-DD` or RFC 3339 |

**Constraints**:
- `--format` MUST be one of the three enumerated values; any other value
  causes exit 1 with a validation error before any operation.
- When `--from` and `--to` are both provided, `--from` MUST be earlier than
  `--to`; violation causes exit 1 before any API call.
- Dates without a time component are treated as `00:00:00 UTC`.

---

## Exit Codes

| Code | Meaning |
|---|---|
| 0 | Success — output written to stdout |
| 1 | Error — description written to stderr |

All error output goes to **stderr**. All data output goes to **stdout**.

---

## Commands

### `slackseek auth show`

Displays discovered workspaces, partial tokens, and partial cookie values.
Makes **no network calls**.

```
slackseek auth show [global-flags]
```

**Output columns (table)**:

| Column | Description |
|---|---|
| Workspace | Workspace name |
| URL | Workspace base URL |
| Token | First 12 chars of token + `…` |
| Cookie | First 8 chars of cookie + `…` |

---

### `slackseek auth export`

Prints shell-compatible `export VAR=value` statements, one per workspace.
Variable name format: `SLACK_TOKEN_<UPPERCASE_NAME>`.

```
slackseek auth export [global-flags]
```

**Output** (text only — `--format` flag is ignored for this command):

```sh
export SLACK_TOKEN_ACME_CORP=xoxs-…
```

**Note**: The full token IS included in export output by design (it is
intended for `eval` in a shell session). This command should not be run
in logged shell sessions or CI pipelines.

---

### `slackseek channels list`

Lists all channels the authenticated user can see.

```
slackseek channels list [flags] [global-flags]
```

**Command flags**:

| Flag | Type | Default | Description |
|---|---|---|---|
| `--type` | string | (all) | Filter by type: `public` \| `private` \| `mpim` \| `im` |
| `--archived` | bool | false | Include archived channels |

**Output columns (table)**:

| Column | Description |
|---|---|
| ID | Slack channel ID |
| Name | Channel name |
| Type | Channel type |
| Members | Member count |
| Topic | Topic text (truncated to 60 chars) |

---

### `slackseek history <channel>`

Fetches message history for a channel, with optional inline thread replies.

```
slackseek history <channel> [flags] [global-flags]
```

**Arguments**:

| Argument | Required | Description |
|---|---|---|
| `channel` | yes | Channel name (e.g., `general`) or Slack ID (e.g., `C01234567`) |

**Command flags**:

| Flag | Short | Type | Default | Description |
|---|---|---|---|---|
| `--threads` | `-T` | bool | true | Fetch and inline all thread replies |
| `--limit` | `-n` | int | 1000 | Max messages to return (0 = unlimited) |

**Output columns (table)**:

| Column | Description |
|---|---|
| Timestamp | RFC 3339 formatted message time |
| User | User display name or ID |
| Text | Message text (truncated to 80 chars) |
| Depth | Thread depth: 0 = root, 1+ = reply |
| Reactions | Emoji reactions with counts, e.g. `+1×3` |

**Behaviour**:
1. Resolve channel name → ID if needed.
2. Paginate `conversations.history` until date range or limit satisfied.
3. For each message with replies (when `--threads` is set), fetch
   `conversations.replies` and interleave replies in order.

---

### `slackseek messages <user>`

Aggregates all messages sent by a specific user across accessible channels.

```
slackseek messages <user> [flags] [global-flags]
```

**Arguments**:

| Argument | Required | Description |
|---|---|---|
| `user` | yes | Display name, real name, or Slack user ID (e.g., `U01234567`) |

**Command flags**:

| Flag | Short | Type | Default | Description |
|---|---|---|---|---|
| `--channel` | `-c` | string | (all) | Restrict to a specific channel (name or ID) |
| `--limit` | `-n` | int | 1000 | Max messages to return (0 = unlimited) |
| `--threads` | `-T` | bool | true | Include thread replies the user posted |

**Output columns (table)**:

| Column | Description |
|---|---|
| Timestamp | RFC 3339 formatted message time |
| Channel | Channel name |
| Text | Message text (truncated to 80 chars) |
| Thread | `reply` if a thread reply, `root` otherwise |

**Note**: Uses `search.messages` API with `from:<userID>` query; requires a
user token (xoxs-* or xoxc-*).

---

### `slackseek search <query>`

Full-text search across all accessible messages.

```
slackseek search <query> [flags] [global-flags]
```

**Arguments**:

| Argument | Required | Description |
|---|---|---|
| `query` | yes | Search query string (Slack search syntax supported) |

**Command flags**:

| Flag | Short | Type | Default | Description |
|---|---|---|---|---|
| `--channel` | `-c` | string | (all) | Restrict to a specific channel (name or ID) |
| `--user` | `-u` | string | (all) | Restrict to messages from a user (name or ID) |
| `--limit` | `-n` | int | 100 | Max results (0 = unlimited) |

**Output columns (table)**:

| Column | Description |
|---|---|
| Timestamp | RFC 3339 formatted message time |
| Channel | Channel name |
| User | User display name or ID |
| Text | Message text (truncated to 80 chars) |
| Permalink | Full URL to the message in Slack web |

---

### `slackseek users list`

Lists all workspace users.

```
slackseek users list [flags] [global-flags]
```

**Command flags**:

| Flag | Type | Default | Description |
|---|---|---|---|
| `--deleted` | bool | false | Include deactivated users |
| `--bot` | bool | false | Include bot accounts |

**Output columns (table)**:

| Column | Description |
|---|---|
| ID | Slack user ID |
| Display Name | @-mentionable name |
| Real Name | Full name |
| Email | Email address (may be empty) |
| Bot | `true` / `false` |
| Deleted | `true` / `false` |

---

## Argument Resolution Rules

**Channel resolution** (applies to `history`, `messages --channel`,
`search --channel`):
1. If argument matches `C[A-Z0-9]+`, use as-is (Slack channel ID).
2. Otherwise call `conversations.list` and match on `name` (case-insensitive).
3. If multiple matches: exit 1 with message listing all ambiguous IDs.
4. If no match: exit 1 with message listing available channels.

**User resolution** (applies to `messages`, `search --user`):
1. If argument matches `U[A-Z0-9]+`, use as-is (Slack user ID).
2. Otherwise call `users.list` and match on `display_name` or `real_name`
   (case-insensitive substring match).
3. If multiple matches: exit 1 with message listing all ambiguous users.
4. If no match: exit 1 with message listing closest matches.

---

## Workspace Selection

When `--workspace` is omitted:
1. Run token extraction → `TokenExtractionResult`.
2. Select `Workspaces[0]`.
3. Print to stderr: `Using workspace: <Name> (<URL>)`.
4. If `len(Workspaces) > 1`, also print: `Other workspaces available: <names>`.

When `--workspace` is provided:
1. Run token extraction.
2. Match on `Name` (case-insensitive) or exact `URL`.
3. If no match: exit 1 with available workspace names listed.
