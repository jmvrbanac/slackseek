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

| Flag | Description | Default |
|------|-------------|---------|
| `-c, --channel` | Limit to a specific channel | all |
| `-u, --user` | Limit to a specific user | all |
| `-n, --limit` | Max results to return (0 = unlimited) | 100 |

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

# Export all messages from #incidents as JSON
slackseek history incidents --limit 0 --format json > incidents.json

# Find DMs from a user
slackseek messages bob --channel im

# List all private channels as a table
slackseek channels list --type private --format table
```

## Platform support

| Platform | Status |
|----------|--------|
| Linux    | Supported |
| macOS    | Supported |

## License

[MIT](LICENSE)
