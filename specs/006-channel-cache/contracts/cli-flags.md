# CLI Contract: 006 — Long-Term Day-History Cache

## New flags

### `--no-cache` (bool, default: false)

Available on: `history`, `postmortem`, `metrics`

| Property | Value |
|---|---|
| Long flag | `--no-cache` |
| Short flag | none |
| Type | `bool` |
| Default | `false` |
| Persistent | no (per-command) |

**Behaviour**:
- When `false` (default): attempt cache load first; on hit return cached messages; on miss
  fetch from API and save to cache.
- When `true`: skip cache load; fetch from API unconditionally; save result to cache
  (refreshes or creates the entry).

**Error handling**: `--no-cache` never returns an error on its own. Cache write failures
are non-fatal (warning to stderr per existing policy).

---

## Unchanged flags

All existing flags on `history`, `postmortem`, and `metrics` are unchanged. The `--no-cache`
flag is additive.

### `history` flag summary (post-006)

| Flag | Short | Default | Description |
|---|---|---|---|
| `--from` | | `""` | Start of date range (YYYY-MM-DD or RFC 3339) |
| `--to` | | `""` | End of date range (YYYY-MM-DD or RFC 3339) |
| `--since` | | `""` | Relative start (e.g. `7d`, `4h`) |
| `--until` | | `""` | Relative end |
| `--threads` | `-T` | `true` | Include inline thread replies |
| `--limit` | `-n` | `1000` | Max messages (0 = unlimited) |
| `--format` | | `text` | Output format |
| `--no-cache` | | `false` | **NEW** Skip cache load; refresh entry |
| `--quiet` | `-q` | `false` | Suppress progress output |

### `postmortem` flag summary (post-006)

| Flag | Short | Default | Description |
|---|---|---|---|
| `--from` | | `""` | Start of date range |
| `--to` | | `""` | End of date range |
| `--since` | | `""` | Relative start |
| `--until` | | `""` | Relative end |
| `--format` | | `markdown` | Output format |
| `--no-cache` | | `false` | **NEW** Skip cache load; refresh entry |

### `metrics` flag summary (post-006)

| Flag | Short | Default | Description |
|---|---|---|---|
| `--from` | | `""` | Start of date range |
| `--to` | | `""` | End of date range |
| `--since` | | `""` | Relative start |
| `--until` | | `""` | Relative end |
| `--format` | | `text` | Output format |
| `--no-cache` | | `false` | **NEW** Skip cache load; refresh entry |

---

## Cache interaction model

```
slackseek history #channel --from 2026-03-01 --to 2026-03-01
  → cache miss  → API fetch → save  → output
  → [next run]
  → cache HIT   →                    output   (no API call)

slackseek history #channel --from 2026-03-01 --to 2026-03-01 --no-cache
  → skip load   → API fetch → save  → output  (always fresh)
```

---

## Out of scope (v1)

- `digest`: uses search API; `--no-cache` not added in v1.
- `actions`: uses `FetchHistory` with `threads=false`; excluded from caching per design
  decision; `--no-cache` not added in v1.
