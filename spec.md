# slackseek — Specification

## Overview

`slackseek` is a Go CLI tool that extracts authentication tokens from a locally installed Slack desktop app and uses them to query the Slack API. It supports Linux and macOS. It enables offline-friendly, scriptable access to workspace history, user messages, channels, and search — all filterable by date range.

---

## Token Extraction

The token extraction logic mirrors `get_local_tokens_poc.py` and must run without Slack's cooperation.

### Platform Paths

Token storage paths differ by OS:

| Resource  | Linux                                              | macOS                                                        |
|-----------|----------------------------------------------------|--------------------------------------------------------------|
| LevelDB   | `~/.config/Slack/Local Storage/leveldb`            | `~/Library/Application Support/Slack/Local Storage/leveldb`  |
| Cookies   | `~/.config/Slack/Cookies`                          | `~/Library/Application Support/Slack/Cookies`                |

Paths are resolved via build-tagged files (`paths_linux.go`, `paths_darwin.go`) so the rest of the codebase is platform-agnostic.

### Workspace Tokens (LevelDB)

- **Source:** platform LevelDB path (see above)
- **Method:**
  1. Copy the LevelDB directory to a temp dir (Slack may hold a file lock)
  2. Open the copy with a LevelDB reader (`syndtr/goleveldb`)
  3. Scan all keys for one containing the byte sequence `localConfig_v2`
  4. Parse the value as JSON; extract `teams[*].token`, `teams[*].name`, and `teams[*].url`
- **Output:** A map of workspace URL → `{name, token}`

### Session Cookie (SQLite + AES)

- **Source:** platform Cookies path (see above) — Chromium SQLite cookie store
- **Method:**
  1. Copy the file to a temp dir
  2. Open the copy with `database/sql` + `mattn/go-sqlite3`
  3. Read the `version` field from the `meta` table
  4. Query `cookies` for `host_key LIKE '%slack.com%' AND name='d'`; get `encrypted_value`
  5. Fetch the raw keyring password via the platform keyring (see below)
  6. Derive a 16-byte AES key: PBKDF2-HMAC-SHA1, salt=`saltysalt`, platform-specific iterations (see below)
  7. Decrypt with AES-128-CBC, IV = 16 space bytes (`0x20`)
  8. Strip the 3-byte version prefix (`v10`/`v11`)
  9. If DB `version >= 24`, strip an additional leading 32 bytes (SHA-256 domain hash)
  10. Remove PKCS7 padding; decode as UTF-8
- **Output:** Cookie value string for the `d` cookie

### Keyring Password

The keyring backend and PBKDF2 iteration count differ by platform and are implemented in build-tagged files (`keyring_linux.go`, `keyring_darwin.go`):

**Linux — D-Bus SecretService**
- **Library:** `github.com/godbus/dbus/v5`
- **Method:** Connect to the session D-Bus, open the default secret collection, iterate items, find the one with attribute `application == "Slack"`, return its secret bytes.
- **PBKDF2 iterations:** 1

**macOS — Keychain**
- **Library:** `github.com/keybase/go-keychain`
- **Method:** Query the login keychain for service `"Slack Safe Storage"`, account `"Slack"`, return the password bytes.
- **PBKDF2 iterations:** 1003

---

## CLI Architecture

### Framework & Libraries

| Purpose                        | Library                               |
|--------------------------------|---------------------------------------|
| CLI framework                  | `github.com/spf13/cobra`              |
| LevelDB                        | `github.com/syndtr/goleveldb`         |
| SQLite                         | `github.com/mattn/go-sqlite3`         |
| D-Bus / SecretService (Linux)  | `github.com/godbus/dbus/v5`           |
| Keychain (macOS)               | `github.com/keybase/go-keychain`      |
| Slack API                      | `github.com/slack-go/slack`           |
| Table output                   | `github.com/olekukonko/tablewriter`   |

### Module Name

```
github.com/jmvrbanac/slackseek
```

### Directory Layout

```
slackseek/
├── cmd/
│   ├── root.go          # Root cobra command, global flags
│   ├── auth.go          # auth subcommand
│   ├── channels.go      # channels subcommand
│   ├── history.go       # history subcommand
│   ├── messages.go      # messages subcommand (by user)
│   ├── search.go        # search subcommand
│   └── users.go         # users subcommand
├── internal/
│   ├── tokens/
│   │   ├── leveldb.go        # LevelDB token extraction (cross-platform)
│   │   ├── cookie.go         # Cookie decryption (cross-platform)
│   │   ├── keyring_linux.go  # D-Bus SecretService, iterations=1
│   │   ├── keyring_darwin.go # macOS Keychain, iterations=1003
│   │   ├── paths_linux.go    # ~/.config/Slack/...
│   │   └── paths_darwin.go   # ~/Library/Application Support/Slack/...
│   ├── slack/
│   │   ├── client.go    # Authenticated Slack client wrapper
│   │   ├── channels.go  # Channel listing and history fetching
│   │   ├── messages.go  # Per-user message aggregation
│   │   └── search.go    # Search API calls
│   └── output/
│       └── format.go    # JSON / table / text formatters
├── main.go
├── go.mod
├── go.sum
└── spec.md
```

---

## Global Flags

These flags apply to every command:

| Flag                  | Short | Default    | Description                                      |
|-----------------------|-------|------------|--------------------------------------------------|
| `--workspace`         | `-w`  | (first found) | Workspace name or URL to target               |
| `--format`            |       | `text`     | Output format: `text`, `table`, `json`           |
| `--from`              |       | (none)     | Start of date range (RFC3339 or `YYYY-MM-DD`)    |
| `--to`                |       | (none)     | End of date range (RFC3339 or `YYYY-MM-DD`)      |

---

## Commands

### `slackseek auth`

Extracts and displays local tokens without making any API calls. Useful for verifying the extraction mechanism works.

```
slackseek auth [flags]
```

**Subcommands:**

- `slackseek auth show` — Print discovered workspaces, token prefixes, and cookie snippet
- `slackseek auth export` — Print `export SLACK_TOKEN_<NAME>=...` shell statements (eval-friendly)

---

### `slackseek channels`

Interact with channel listings.

```
slackseek channels [subcommand] [flags]
```

**Subcommands:**

- `slackseek channels list` — List all channels the authenticated user can see

**Flags:**

| Flag         | Short | Description                         |
|--------------|-------|-------------------------------------|
| `--type`     |       | Filter by type: `public`, `private`, `mpim`, `im` (default: all) |
| `--archived` |       | Include archived channels (default: false) |

**Output columns (table mode):** ID, Name, Type, Member Count, Topic

---

### `slackseek history`

Fetch message history for a channel, including thread replies.

```
slackseek history <channel> [flags]
```

`<channel>` may be a channel name (e.g., `general`) or ID (e.g., `C01234567`).

**Flags:**

| Flag        | Short | Description                                     |
|-------------|-------|-------------------------------------------------|
| `--threads` | `-T`  | Also fetch and inline all thread replies (default: true) |
| `--limit`   | `-n`  | Max messages to return (default: 1000, 0 = all) |

**Behavior:**

1. Resolve channel name → ID if needed (via `conversations.list`)
2. Call `conversations.history` with `oldest`/`latest` params derived from `--from`/`--to`
3. For each message with `reply_count > 0` (and `--threads` is set), call `conversations.replies`
4. Merge and sort replies inline beneath their parent message
5. Paginate automatically until date range is exhausted or limit reached

**Output columns (table mode):** Timestamp, User, Text (truncated), Thread Depth, Reactions

---

### `slackseek messages`

Aggregate all messages sent by a specific user across all accessible channels.

```
slackseek messages <user> [flags]
```

`<user>` may be a display name, real name, or user ID (e.g., `U01234567`).

**Flags:**

| Flag        | Short | Description                                          |
|-------------|-------|------------------------------------------------------|
| `--channel` | `-c`  | Restrict to a specific channel (name or ID)          |
| `--limit`   | `-n`  | Max messages to return (default: 1000, 0 = all)      |
| `--threads` | `-T`  | Include replies the user posted in threads (default: true) |

**Behavior:**

1. Resolve user name → ID if needed (via `users.list`)
2. Use `search.messages` API with `from:<userID>` query and optional channel qualifier
3. Apply `--from`/`--to` date filters as API `after:`/`before:` modifiers in the search query
4. Paginate through results

**Note:** `search.messages` requires a user token (xoxs-* or xoxc-*); bot tokens are insufficient. The locally extracted token satisfies this requirement.

**Output columns (table mode):** Timestamp, Channel, Text (truncated), Thread

---

### `slackseek search`

Full-text search across all messages.

```
slackseek search <query> [flags]
```

**Flags:**

| Flag        | Short | Description                                           |
|-------------|-------|-------------------------------------------------------|
| `--channel` | `-c`  | Restrict search to a channel (name or ID)             |
| `--user`    | `-u`  | Restrict to messages from a user (name or ID)         |
| `--limit`   | `-n`  | Max results (default: 100, 0 = all)                   |

**Behavior:**

1. Compose a Slack search query string from flags and `--from`/`--to` using Slack search modifiers:
   - `after:YYYY-MM-DD`, `before:YYYY-MM-DD`
   - `in:#channel`
   - `from:@user`
2. Call `search.messages`; paginate through all pages up to limit

**Output columns (table mode):** Timestamp, Channel, User, Text (truncated), Permalink

---

### `slackseek users`

List workspace users.

```
slackseek users list [flags]
```

**Flags:**

| Flag       | Description                      |
|------------|----------------------------------|
| `--deleted`| Include deactivated users        |
| `--bot`    | Include bot accounts             |

**Output columns (table mode):** ID, Display Name, Real Name, Email, Bot, Deleted

---

## Date Filtering

- `--from` and `--to` accept `YYYY-MM-DD` or RFC3339 strings
- Dates without a time component are interpreted as start-of-day UTC
- Converted to Unix timestamps for `conversations.history` (`oldest`/`latest`)
- Converted to Slack search date modifiers (`after:`, `before:`) for `search.messages`

---

## Output Formats

Controlled by the global `--format` flag.

| Format  | Description                                              |
|---------|----------------------------------------------------------|
| `text`  | Human-readable, one message per line with basic fields   |
| `table` | Aligned ASCII table via `tablewriter`                    |
| `json`  | Full JSON array of raw API response objects              |

---

## Authentication Flow

On first use (or when `--workspace` is not specified):

1. Run token extraction automatically
2. If multiple workspaces are found, select the first one and note the others
3. Print a one-line notice: `Using workspace: <name> (<url>)`

The user can override with `--workspace <name-or-url>`.

Tokens are **not cached** to disk — they are re-extracted from LevelDB/Cookies on each invocation. This keeps the tool stateless and always in sync with Slack's current session.

---

## Error Handling

- If LevelDB copy/parse fails: print actionable error and exit 1
- If Slack API returns rate limit (429): retry with exponential backoff up to 3 times
- If API returns auth error: hint that Slack must be running and logged in
- If `--from` > `--to`: exit with validation error before any API call

---

## Non-Goals (v1)

- Windows support
- Token caching / config file
- DM/IM history (can be addressed in v2)
- Posting or modifying messages
- Real-time event streaming
