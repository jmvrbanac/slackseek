# Quickstart: 004 Bug Fixes

## Build & test

```sh
# Build
go build -o slackseek ./...

# Full test suite (mandatory)
go test -race ./...

# Lint
golangci-lint run
```

## Verify each fix manually

### Fix 1 — Mentions with embedded labels

Run a search that you know returns messages with `<@USERID|username>` tokens:

```sh
./slackseek search "please tag" --format text
```

**Before:** `<@U22JKTL6N|nmollenkopf> please tag this…`
**After:** `@Nick Mollenkopf please tag this…`

### Fix 2 — Thread grouping

```sh
./slackseek history incidents --threads --format text
```

**Before:** flat chronological list with replies interleaved.
**After:** replies indented under their parent with `  └─ ` prefix; blank
line between thread groups.

JSON nesting:

```sh
./slackseek history incidents --threads --format json | jq '.[0].replies'
```

### Fix 3 — Markdown export

```sh
./slackseek history incidents --format markdown > incidents.md
```

Open `incidents.md` in any Markdown viewer. Expected structure:

```markdown
# #incidents — 2024-01-15

## 10:00 · Alice
…

> **10:02 · Bob**
> …
```

Search export:

```sh
./slackseek search "deploy failed" --format markdown > results.md
```

### Fix 4 — DM channel names

```sh
./slackseek messages alice --format text
```

**Before:** DM results show `U01ABCDEF` in the channel column.
**After:** DM results show `@Bob Smith` (or `@username` as fallback).

### Fix 5 — Table newline alignment

```sh
./slackseek history incidents --format table
```

**Before:** Multi-line messages break out of their cell column.
**After:** All text stays in one line per cell (truncated at 80 chars).

## Key files changed

| File | Fixes |
|------|-------|
| `internal/slack/resolver.go` | 1, 4 |
| `internal/slack/resolver_test.go` | 1, 4 |
| `internal/output/format.go` | 2, 3, 4, 5 |
| `internal/output/format_test.go` | 2, 3, 5 |
| `cmd/root.go` | 3 (flag description only) |
