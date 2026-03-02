# Quickstart: slackseek CLI

**Feature**: 001-slackseek-cli
**Date**: 2026-03-02

---

## Prerequisites

- Go 1.24 or newer (`go version`)
- Slack desktop application installed and logged in on the local machine
- Linux or macOS (Windows is not supported in v1)
- Linux: D-Bus session running (standard on any desktop session)
- macOS: Login Keychain accessible (standard on any macOS session)

---

## Build

```sh
git clone https://github.com/jmvrbanac/slackseek.git
cd slackseek
go build -o slackseek ./...
```

The result is a single self-contained binary (`slackseek`). No C compiler
or CGO is required.

### Cross-compile (from Linux to macOS)

```sh
GOOS=darwin GOARCH=arm64 go build -o slackseek-darwin-arm64 ./...
```

Note: macOS Keychain access requires running the binary on a macOS machine;
a cross-compiled binary will fail keychain calls at runtime.

### Verify build targets

```sh
GOOS=linux  go build ./...
GOOS=darwin go build ./...
```

Both MUST succeed without errors.

---

## Verify local Slack session

```sh
./slackseek auth show
```

Expected output (table format):

```
WORKSPACE    URL                          TOKEN          COOKIE
Acme Corp    https://acme.slack.com       xoxs-1234…     abcd1234…
```

If no workspaces are found, ensure Slack is installed and you are logged in.

Export tokens to environment variables (for use in scripts):

```sh
eval "$(./slackseek auth export)"
echo $SLACK_TOKEN_ACME_CORP
```

---

## Search messages

```sh
# Basic keyword search
./slackseek search "deploy failed"

# Narrow by date range and channel
./slackseek search "deploy failed" --channel deployments --from 2025-01-01 --to 2025-01-31

# Output as JSON for downstream processing
./slackseek search "deploy failed" --format json | jq '.[].permalink'
```

---

## Retrieve channel history

```sh
# Last 100 messages in #general (with thread replies)
./slackseek history general --limit 100

# Full history for January 2025, JSON output
./slackseek history general --from 2025-01-01 --to 2025-02-01 --format json

# History without thread replies
./slackseek history general --threads=false --limit 500
```

---

## Get messages by user

```sh
# All messages from a user (by display name)
./slackseek messages jane.doe

# Limit to a specific channel and date range
./slackseek messages jane.doe --channel engineering --from 2025-06-01
```

---

## Browse workspace resources

```sh
# List all channels (table format)
./slackseek channels list

# List only public channels, including archived
./slackseek channels list --type public --archived

# List all users as JSON
./slackseek users list --format json

# Include bots and deactivated accounts
./slackseek users list --deleted --bot
```

---

## Select a specific workspace

```sh
# By name (case-insensitive)
./slackseek search "standup notes" --workspace "acme corp"

# By URL
./slackseek channels list --workspace https://acme.slack.com
```

---

## Running tests

```sh
# Unit tests (no OS resources required)
go test ./...

# Unit tests with race detector (MANDATORY before any merge)
go test -race ./...

# Integration tests (requires Slack installed + D-Bus/Keychain accessible)
INTEGRATION=1 go test -race ./...

# Linting (requires golangci-lint installed)
golangci-lint run

# Cross-platform build check
GOOS=linux  go build ./...
GOOS=darwin go build ./...
```

---

## Quickstart validation checklist

After building and with Slack installed and logged in:

- [ ] `./slackseek auth show` prints at least one workspace.
- [ ] `./slackseek auth export` prints one `export` line per workspace.
- [ ] `./slackseek channels list` returns a non-empty table.
- [ ] `./slackseek users list --format json` produces valid JSON (`| jq .`).
- [ ] `./slackseek search "test" --limit 5` returns results or an empty table
  (not an error) when Slack has accessible messages.
- [ ] `go test -race ./...` passes with zero failures.
- [ ] `GOOS=darwin go build ./...` succeeds from a Linux host.
