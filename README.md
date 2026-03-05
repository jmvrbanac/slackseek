# slackseek

Query your Slack workspaces from the command line — no API keys, no OAuth setup.
slackseek extracts credentials directly from your locally installed Slack desktop app.

## How it works

slackseek reads the auth tokens and cookies stored by the Slack desktop application on your machine. As long as you're logged in to Slack, slackseek can make API calls on your behalf without any manual configuration.

## Installation

### Download a release

Grab the latest binary for your platform from the [Releases](../../releases) page.

### Build from source

```sh
go install github.com/jmvrbanac/slackseek@latest
```

Or clone and build:

```sh
git clone https://github.com/jmvrbanac/slackseek
cd slackseek
go build -o slackseek .
```

**Requirements:** Go 1.24+, Slack desktop app installed and logged in.

## Quick start

```sh
# Verify credentials were found
slackseek auth show

# List channels
slackseek channels list

# Search for messages
slackseek search "incident postmortem"

# Get the last 50 messages from a channel
slackseek history general --limit 50

# Find messages from a specific user
slackseek messages alice
```

## Commands

### `auth`

Inspect discovered workspace credentials.

```sh
slackseek auth show          # Show all discovered workspaces and tokens
slackseek auth export        # Print shell export statements for tokens
```

`auth export` outputs `export SLACK_TOKEN_<WORKSPACE>=<token>` lines suitable for sourcing.

---

### `channels list`

List channels in the workspace.

```sh
slackseek channels list [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--type` | Filter by type: `public`, `private`, `mpim`, `im` | all |
| `--archived` | Include archived channels | false |

---

### `users list`

List workspace members.

```sh
slackseek users list [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--deleted` | Include deactivated users | false |
| `--bot` | Include bot accounts | false |

---

### `messages`

Retrieve messages sent by a specific user.

```sh
slackseek messages <user> [flags]
```

`<user>` can be a display name, real name, or Slack user ID.

| Flag | Description | Default |
|------|-------------|---------|
| `-c, --channel` | Limit to a specific channel | all |
| `-n, --limit` | Max messages to return (0 = unlimited) | 1000 |

---

### `history`

Retrieve message history for a channel.

```sh
slackseek history <channel> [flags]
```

`<channel>` can be a channel name or ID.

| Flag | Description | Default |
|------|-------------|---------|
| `-T, --threads` | Include thread replies inline | true |
| `-n, --limit` | Max messages to return (0 = unlimited) | 1000 |

---

### `search`

Search messages across the workspace.

```sh
slackseek search <query> [flags]
```

`--channel` is repeatable — pass it multiple times to search across several channels in parallel.

| Flag | Description | Default |
|------|-------------|---------|
| `-c, --channel` | Limit to a specific channel (repeatable) | all |
| `-u, --user` | Limit to a specific user | all |
| `-n, --limit` | Max results to return (0 = unlimited) | 100 |

```sh
# Search multiple channels at once
slackseek search "deploy" --channel eng --channel incidents --channel alerts
```

---

### `thread`

Fetch a Slack thread by its permalink URL.

```sh
slackseek thread <permalink-url> [flags]
```

`<permalink-url>` is the full Slack URL copied from "Copy link" on a message (e.g. `https://acme.slack.com/archives/C01234/p1700000000123456`).

Output includes the thread messages and a sorted list of participants.

---

### `postmortem`

Generate an incident timeline from a channel's message history.

```sh
slackseek postmortem <channel> [flags]
```

Default output format is `markdown` (a structured timeline table). Use `--format json` for structured data.

| Flag | Description | Default |
|------|-------------|---------|
| `--since` | Relative time range to fetch | — |
| `--format` | Output format: `markdown`, `json` | `markdown` |

```sh
slackseek postmortem incidents --since 24h
slackseek postmortem incidents --since 7d --format json
```

---

### `digest`

Summarise a user's messages grouped by channel.

```sh
slackseek digest [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `-u, --user` | **Required.** User to summarise | — |
| `--since` | Relative time range | — |

```sh
slackseek digest --user alice --since 7d
```

---

### `metrics`

Show activity statistics for a channel.

```sh
slackseek metrics <channel> [flags]
```

Returns user message counts, thread statistics, top reactions, and an hourly message distribution.

```sh
slackseek metrics general --since 7d --format json
```

---

### `actions`

Extract action items and commitments from a channel.

```sh
slackseek actions <channel> [flags]
```

Scans messages for commitment phrases (`I'll`, `will do`, `action item`, `TODO`, `follow up`, etc.) and returns them as a checklist.

```sh
slackseek actions general --since 7d
slackseek actions eng --since 24h --format json
```

---

### `cache clear`

Remove cached channel and user data.

```sh
slackseek cache clear          # Clear cache for the current workspace
slackseek cache clear --all    # Clear cache for all workspaces
```

## Global flags

These flags apply to every command.

| Flag | Description | Default |
|------|-------------|---------|
| `-w, --workspace` | Workspace name or URL to target | first found |
| `--format` | Output format: `text`, `table`, `json` | `text` |
| `--from` | Start of date range (`YYYY-MM-DD` or RFC 3339) | — |
| `--to` | End of date range (`YYYY-MM-DD` or RFC 3339) | — |
| `--since` | Relative start time (`30m`, `4h`, `7d`, `2w`); alias for `--from` | — |
| `--until` | Relative end time (`30m`, `4h`, `7d`, `2w`); alias for `--to` | — |
| `-q, --quiet` | Suppress progress output on stderr | false |
| `--width` | Wrap output at this column width (0 = auto-detect) | 0 |
| `--emoji` | Force emoji rendering | false |
| `--no-emoji` | Disable emoji rendering | false |
| `--cache-ttl` | How long cached data remains valid | `24h` |
| `--refresh-cache` | Force a fresh API fetch, overwriting the cache | false |
| `--no-cache` | Bypass the cache entirely | false |

### Multiple workspaces

When `--workspace` is not set, slackseek picks the first discovered workspace and prints a notice to stderr listing any others. To target a specific one:

```sh
slackseek --workspace "Acme Corp" channels list
slackseek --workspace https://acme.slack.com channels list
```

## Output formats

All commands support `--format text` (default), `--format table`, and `--format json`.

```sh
slackseek search "deploy" --format json | jq '.[].text'
slackseek users list --format table
```

## Caching

Channel and user lists are cached under `~/.cache/slackseek/` (or `$XDG_CACHE_HOME/slackseek/` on Linux) with a 24-hour TTL. This speeds up ID resolution in message output.

```sh
slackseek history general                    # uses cache if fresh
slackseek history general --refresh-cache    # force re-fetch
slackseek history general --no-cache         # skip cache entirely
slackseek history general --cache-ttl 1h     # use a shorter TTL
```

## Examples

```sh
# Search for messages in a date range
slackseek search "outage" --from 2024-01-01 --to 2024-01-31

# Search the last 7 days using a relative time
slackseek search "deploy" --since 7d

# Search multiple channels at once
slackseek search "incident" --channel eng --channel ops --channel alerts

# Export all messages from #incidents as JSON
slackseek history incidents --limit 0 --format json > incidents.json

# Fetch a specific Slack thread by permalink
slackseek thread "https://acme.slack.com/archives/C01234/p1700000000123456"

# Generate an incident postmortem for the last 24 hours
slackseek postmortem incidents --since 24h

# Show what alice was up to this week
slackseek digest --user alice --since 7d

# Channel activity metrics
slackseek metrics general --since 7d --format json

# Extract action items from a channel
slackseek actions eng --since 24h

# Find DMs from a user
slackseek messages bob --channel im

# List all private channels as a table
slackseek channels list --type private --format table

# Pipe-friendly: suppress progress, get JSON
slackseek history general --since 24h --format json --quiet | jq '.[].text'
```

## Platform support

| Platform | Status |
|----------|--------|
| Linux    | Supported |
| macOS    | Supported |

## License

[MIT](LICENSE)
