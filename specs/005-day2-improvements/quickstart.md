# Quickstart: 005 Day 2 Improvements

## Prerequisites

- `slackseek` binary built from this branch: `go build -o slackseek ./...`
- Valid Slack credentials (run `slackseek auth show` to confirm)
- A channel you have access to (e.g. `general`)

---

## 1. Quiet Flag

Verify progress noise is suppressed:
```sh
# Without --quiet (progress should appear on stderr)
slackseek history general 2>&1 | head -3

# With --quiet (no progress on stderr, only results on stdout)
slackseek history general --quiet 2>/dev/null | head -5

# Warnings still appear even with --quiet
slackseek history general --quiet 2>&1 | grep -i warning || echo "(no warnings)"
```

---

## 2. Relative Date Flags

```sh
# Last 24 hours
slackseek history general --since 24h

# Last week up to yesterday
slackseek search "deploy" --since 7d --until 1d

# Combined with --format json
slackseek history general --since 4h --format json | jq '.[0].time'

# Error case: --since and --from together
slackseek history general --since 24h --from 2026-01-01
# Expected: error about mutual exclusion
```

---

## 3. Thread Command

```sh
# Obtain a permalink from Slack (right-click a message → Copy link)
LINK="https://yourworkspace.slack.com/archives/C01234567/p1700000000123456"

# Fetch the thread
slackseek thread "$LINK"

# JSON output (useful for AI agents)
slackseek thread "$LINK" --format json | jq '{participants: .participants, count: (.messages | length)}'

# From a reply permalink (auto-fetches from root)
REPLY_LINK="https://yourworkspace.slack.com/archives/C01234567/p1700000000654321?thread_ts=1700000000.123456"
slackseek thread "$REPLY_LINK"
```

---

## 4. Line Wrapping

```sh
# Default: auto-detects terminal width
slackseek history general | head -20

# Fixed width (60 chars)
slackseek history general --width 60 | head -20

# Disable wrapping
slackseek history general --width 0 | head -5

# Piped (default 120-char wrap)
slackseek history general | cat | head -20

# Override via env var
SLACKSEEK_WIDTH=80 slackseek history general | head -20
```

---

## 5. Multi-Channel Search

```sh
# Single channel (unchanged behaviour)
slackseek search "incident" --channel general

# Multiple channels
slackseek search "incident" --channel general --channel incidents

# JSON for programmatic use
slackseek search "deploy" --channel deploys --channel general \
  --format json | jq 'length'
```

---

## 6. Emoji Rendering

```sh
# Default: on for tty
slackseek history general | grep -m1 ":" | head -1

# Force off
slackseek history general --no-emoji | head -5

# Force on (even when piped)
slackseek history general --emoji | cat | head -5
```

---

## 7. Postmortem

```sh
# Full postmortem document (markdown default)
slackseek postmortem ic-5697 --since 7d

# Scoped to incident window
slackseek postmortem ic-5697 --since 2026-02-25 --until 2026-02-26

# JSON for ticket creation
slackseek postmortem ic-5697 --since 7d --format json | jq '.participants'
```

---

## 8. Digest

```sh
# What has alice been saying in the last week?
slackseek digest --user alice --since 7d

# Full JSON for further processing
slackseek digest --user alice --since 7d --format json | jq '.[].channel'
```

---

## 9. Metrics

```sh
# Channel activity metrics
slackseek metrics general --since 7d

# JSON for dashboard ingestion
slackseek metrics general --since 7d --format json | jq '.users[:3]'
```

---

## 10. Action Item Extraction

```sh
# Extract action items from last week
slackseek actions incidents --since 7d

# JSON output
slackseek actions incidents --since 7d --format json | jq 'length'
```

---

## Running Tests

```sh
# Full test suite (mandatory)
go test -race ./...

# Specific packages
go test -race ./internal/slack/... ./internal/output/... ./cmd/...
go test -race ./internal/emoji/...

# Linter
golangci-lint run

# Cross-platform build check
GOOS=linux  go build ./...
GOOS=darwin go build ./...
```
