# Changelog

All notable changes to slackseek are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.3.3] - 2026-03-20

### Fixed
- Per-workspace cookie isolation, AES fallback key derivation strategies, and
  a new hidden `auth debug-cookie` diagnostic command for troubleshooting
  credential extraction.

### Changed
- Internal refactor to resolve all `golangci-lint` violations (cyclomatic
  complexity and function length).

## [0.3.2] - 2026-03-19

### Added
- MCP tool output now includes a `userName` field (resolved display name) on all
  message and search result objects.
- Inline Slack mention tokens in MCP `text` fields are resolved to human-readable
  form (`<@U123>` → `@Alice Smith`, `<!here>` → `@here`, `<url|label>` → `label`).

## [0.3.1] - 2026-03-14

### Fixed
- `slack_search` MCP tool was returning empty results due to a search query
  construction bug.

## [0.3.0] - 2026-03-13

### Added
- **MCP server** (`slackseek mcp`): exposes ten tools for LLM clients over the
  Model Context Protocol stdio transport.
  - `slack_search` — full-text search with channel, user, and date filters
  - `slack_history` — channel message history with optional thread expansion
  - `slack_messages` — messages by a specific user across all channels
  - `slack_thread` — full thread by Slack permalink
  - `slack_channels` — list and filter workspace channels
  - `slack_users` — list and filter workspace members
  - `slack_digest` — per-channel activity digest for a user
  - `slack_postmortem` — incident timeline document from a channel
  - `slack_metrics` — message count, active users, top posters, and peak hour
  - `slack_actions` — action item extraction from channel history
- Token and credential lookup reuses the same extraction path as the CLI
  (LevelDB + SQLite + keyring), so no separate authentication step is needed.
- 24-hour file cache is shared between the CLI and MCP server.

## [0.2.4] - 2026-03-13

### Added
- **Lazy entity cache**: user and channel IDs that are absent from the in-memory
  lookup maps are now fetched on first miss and cached for the lifetime of the
  command. Eliminates stale-cache gaps without requiring a full refresh.
- Resolver fetch-on-miss callbacks for users, channels, and user groups.
- Tier 4 proactive rate limiter (conservative request pacing for
  `conversations.history` and similar endpoints).

## [0.2.3] - 2026-03-09

### Added
- **Multi-day history cache** (`--since` / `--until` spanning multiple days):
  fetched day-slices are cached individually; only missing days are re-fetched
  on subsequent runs, using gap-based cache filling.

### Fixed
- Open-ended `--since` ranges (no `--until`) no longer panic when `To` is nil.

## [0.2.2] - 2026-03-06

### Added
- **Day-history cache**: `history` results for a complete calendar day are
  persisted to the file cache and served on subsequent calls without hitting
  the Slack API.
- `--no-cache` flag to bypass all caches for a single invocation.

### Fixed
- Keyring credential lookup now falls back to the `"Slack Key"` account name
  when the primary `"Slack"` account is not found (fixes macOS setups where
  the keyring entry uses the alternate name).

## [0.2.1] - 2026-03-06

### Fixed
- `--to` / `--until` values given as `YYYY-MM-DD` are now interpreted as the
  end of that day (23:59:59 UTC) rather than midnight, so the named date is
  fully included in results.

## [0.2.0] - 2026-03-05

### Added
- `history --threads` flag to expand thread replies inline.
- `postmortem` command: generates a structured incident timeline from a
  channel's history (significant messages, participants, period summary).
- `digest` command: groups a user's recent messages by channel.
- `metrics` command: message count, active users, top posters, and hourly
  distribution for a channel.
- `actions` command: extracts action-item commitments from channel history.
- Emoji stripping from message text before display.
- Terminal-width-aware text wrapping in table output.
- Parallel Slack API requests for multi-channel search aggregation.

### Fixed
- Postmortem output: significance filter removes noise, block-quote format,
  HTML entity decoding (`&amp;` → `&`, etc.).

## [0.1.2] - 2026-03-04

### Added
- Proactive rate limiting to stay within Slack API tier limits across all
  paginated commands.
- Channel list fetch now shows progress and uses a page size of 1000 for
  faster initial population.

### Fixed
- Inline `<@USERID|label>` mention tokens are now resolved using the embedded
  label when the user ID is absent from the cache.
- DM channels are displayed as `@DisplayName` instead of raw user IDs.
- Table output no longer truncates rows that contain embedded newlines.
- Thread replies are correctly grouped under their parent message.
- Bare `<S...>` subteam ID tokens (without the `<!subteam^>` prefix) are now
  resolved to their `@handle`.

## [0.1.1] - 2026-03-04

### Added
- `auth export` now outputs `SLACK_COOKIE_<TEAM>` alongside `SLACK_TOKEN_<TEAM>`
  so both credentials are available in scripting contexts.

## [0.1.0] - 2026-03-02

### Added
- Initial release.
- **CLI commands**: `search`, `history`, `messages`, `channels`, `users`,
  `auth show`, `auth export`, `cache clear`.
- Credential extraction from the running Slack desktop app (LevelDB token
  store, SQLite cookie database, system keyring) — no manual token entry
  required.
- `xoxc` cookie transport for Slack API calls that require it.
- Output formats: plain text, Markdown table, JSON (`--format` flag).
- Date range filtering via `--since` / `--until` on all history commands
  (ISO dates, RFC 3339, and relative durations such as `7d`).
- Channel and user list caching (24-hour file-backed JSON cache under
  `$XDG_CACHE_HOME/slackseek/`).
- User ID, channel ID, and user group ID resolution to human-readable names
  in all output (including inline `<@U...>`, `<!subteam^...>`, `<!here>`,
  `<!channel>`, and URL tokens).
- `--filter` flag on `channels` and `users` for substring search.
- Cross-platform builds: Linux and macOS (darwin).
- GitHub Actions CI (lint, test, cross-compile) and automated release workflow
  with version injection via `-ldflags`.

[Unreleased]: https://github.com/jmvrbanac/slackseek/compare/v0.3.2...HEAD
[0.3.2]: https://github.com/jmvrbanac/slackseek/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/jmvrbanac/slackseek/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/jmvrbanac/slackseek/compare/v0.2.4...v0.3.0
[0.2.4]: https://github.com/jmvrbanac/slackseek/compare/v0.2.3...v0.2.4
[0.2.3]: https://github.com/jmvrbanac/slackseek/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/jmvrbanac/slackseek/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/jmvrbanac/slackseek/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/jmvrbanac/slackseek/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/jmvrbanac/slackseek/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/jmvrbanac/slackseek/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/jmvrbanac/slackseek/releases/tag/v0.1.0
