# Using slackseek with Claude / AI agents

This guide documents recommended patterns for programmatic use of slackseek
in Claude conversations, CI pipelines, and other automated contexts.

## Key principles

- Always use `--format json` for machine-readable output.
- Always use `--quiet` / `-q` to suppress progress noise from stderr.
- Resolve channel names first with `channels list` before fetching history.

---

## Channel resolution pattern

```sh
# Step 1: list channels and find the ID you need
slackseek channels list --format json --quiet | jq '.[] | select(.name=="general")'

# Step 2: use the resolved channel ID (or name) in subsequent commands
slackseek history C01234567 --format json --quiet
```

---

## Commands with recommended flags

### history — fetch channel messages

```sh
slackseek history <channel> --format json --quiet [--since 7d] [--limit 500]
```

| Flag | Purpose |
|------|---------|
| `--format json` | Machine-readable output |
| `--quiet` / `-q` | Suppress progress from stderr |
| `--since 7d` | Relative time range (30m, 4h, 7d, 2w) |
| `--limit N` | Cap message count |
| `--threads=false` | Exclude thread replies (faster) |

### search — find messages

```sh
slackseek search "incident" --format json --quiet [--channel general] [--since 24h]
```

The `--channel` flag is repeatable for multi-channel search:

```sh
slackseek search "deploy" --channel ic-001 --channel ic-002 --format json --quiet
```

### thread — fetch a thread by permalink

```sh
slackseek thread "https://acme.slack.com/archives/C01234/p1700000000123456" \
  --format json --quiet
```

Returns `{"thread_ts":"...","channel_id":"...","participants":[...],"messages":[...]}`.

### postmortem — generate incident timeline

```sh
slackseek postmortem ic-5697 --since 24h --format json --quiet
```

Default format is `markdown`; use `--format json` for structured data.

### digest — per-channel summary for a user

```sh
slackseek digest --user alice --since 7d --format json --quiet
```

### metrics — channel activity statistics

```sh
slackseek metrics general --since 7d --format json --quiet
```

Returns user counts, thread stats, top reactions, and hourly distribution.

### actions — commitment pattern extraction

```sh
slackseek actions general --since 7d --format json --quiet
```

Returns messages matching commitment phrases (`I'll`, `TODO`, `action item`, etc.).

---

## Example CLAUDE.md snippet

Add this to your project's CLAUDE.md to give Claude context when using slackseek:

```markdown
## Slack access

Use `slackseek` to query Slack. Always use `--format json --quiet` for
programmatic output. Resolve channel names first:

    slackseek channels list --format json --quiet | jq '.[] | .name, .id'

Then fetch history or search:

    slackseek history <channel-id> --format json --quiet --since 24h
    slackseek search "query" --channel <channel> --format json --quiet
```

---

## Width and emoji flags

| Flag | Effect |
|------|--------|
| `--width 0` | Disable line wrapping (recommended for JSON/piped output) |
| `--no-emoji` | Disable Unicode emoji rendering (always off for pipes) |
| `--emoji` | Force emoji rendering (useful in tty contexts) |
