# Data Model: slackseek CLI

**Feature**: 001-slackseek-cli
**Date**: 2026-03-02

All types are pure Go value types in the `internal/` packages. No database
schema is produced — the tool is stateless and reads external stores
(LevelDB, SQLite) that it does not own.

---

## Workspace

**Package**: `internal/tokens`
**Purpose**: Represents a single authenticated Slack workspace discovered from
local storage.

```go
type Workspace struct {
    Name   string // human-readable workspace name, e.g. "Acme Corp"
    URL    string // workspace base URL, e.g. "https://acme.slack.com"
    Token  string // user token (xoxs-* or xoxc-*); never written to disk
    Cookie string // decrypted value of the 'd' session cookie
}
```

**Validation rules**:
- `URL` MUST be a non-empty string beginning with `https://`.
- `Token` MUST be non-empty; a prefix check (`xoxs-`, `xoxc-`, `xoxp-`) is
  performed and logged as a warning if the prefix is unrecognised.
- `Cookie` MAY be empty if cookie extraction is not available on the platform
  (non-fatal; a warning is emitted).

**Display**: Token is truncated to its first 12 characters + `…` in all user-
facing output. Cookie is truncated to its first 8 characters + `…`.

---

## TokenExtractionResult

**Package**: `internal/tokens`
**Purpose**: Aggregates all workspaces found in local storage along with any
non-fatal warnings from the extraction process.

```go
type TokenExtractionResult struct {
    Workspaces []Workspace
    Warnings   []string // non-fatal issues encountered during extraction
}
```

**Validation rules**:
- `Workspaces` MUST contain at least one entry for the extraction to be
  considered successful; zero entries is an error condition.

---

## Channel

**Package**: `internal/slack`
**Purpose**: Represents a Slack conversation space returned by the API.

```go
type Channel struct {
    ID          string // Slack channel ID, e.g. "C01234567"
    Name        string // display name without '#', e.g. "general"
    Type        string // "public_channel" | "private_channel" | "mpim" | "im"
    MemberCount int    // number of members (0 for IMs)
    Topic       string // current topic text (may be empty)
    IsArchived  bool
}
```

**Validation rules**:
- `ID` MUST match the pattern `[CGDW][A-Z0-9]{8,}` (Slack ID format).
- `Type` MUST be one of the four enumerated values.

---

## Message

**Package**: `internal/slack`
**Purpose**: Represents a single Slack message, including thread metadata.

```go
type Message struct {
    Timestamp   string    // Slack ts format, e.g. "1700000000.123456"
    Time        time.Time // parsed from Timestamp (UTC)
    UserID      string    // Slack user ID, e.g. "U01234567"
    Text        string    // message body (may contain mrkdwn)
    ChannelID   string    // channel this message belongs to
    ThreadTS    string    // parent message ts; empty if root message
    ThreadDepth int       // 0 = root, 1 = direct reply
    Reactions   []Reaction
}

type Reaction struct {
    Name  string // emoji name without colons, e.g. "thumbsup"
    Count int
}
```

**Validation rules**:
- `Timestamp` MUST be a non-empty string parseable as a Unix decimal
  timestamp (seconds.microseconds).
- `ThreadDepth` MUST be 0 when `ThreadTS` is empty; MUST be ≥ 1 when
  `ThreadTS` is non-empty.
- `Reactions` MAY be nil or empty.

**Sort order**: Messages are sorted ascending by `Timestamp` within their
output collection. Thread replies are interleaved directly after their parent.

---

## User

**Package**: `internal/slack`
**Purpose**: Represents a Slack workspace member.

```go
type User struct {
    ID          string // Slack user ID, e.g. "U01234567"
    DisplayName string // @-mentionable name
    RealName    string // full legal or preferred name
    Email       string // may be empty if not accessible with current token
    IsBot       bool
    IsDeleted   bool   // true for deactivated accounts
}
```

**Validation rules**:
- `ID` MUST match the pattern `[UW][A-Z0-9]{8,}`.
- At least one of `DisplayName` or `RealName` MUST be non-empty.

---

## SearchResult

**Package**: `internal/slack`
**Purpose**: Extends Message with search-specific metadata returned by the
`search.messages` API.

```go
type SearchResult struct {
    Message             // embedded — all Message fields apply
    ChannelName string  // resolved channel name (API provides this in context)
    Permalink   string  // full URL to the message in Slack web UI
}
```

**Validation rules**:
- `Permalink` MUST be a non-empty HTTPS URL when returned from the API.
- `ChannelName` MAY be empty if the API does not return context; the tool
  falls back to displaying `ChannelID`.

---

## DateRange

**Package**: `internal/slack` (or `internal/util`)
**Purpose**: Parsed representation of the `--from` / `--to` global flags used
to bound all time-sensitive queries.

```go
type DateRange struct {
    From *time.Time // nil means "no lower bound"
    To   *time.Time // nil means "no upper bound"
}
```

**Validation rules**:
- If both `From` and `To` are non-nil, `From` MUST be before `To`; violation
  causes an immediate CLI validation error (exit code 1) before any API call.
- Input strings without a time component (e.g., `2025-01-15`) are parsed as
  `00:00:00 UTC` on that date.
- Both RFC 3339 and `YYYY-MM-DD` formats are accepted.

---

## Entity Relationships

```
TokenExtractionResult
  └── []Workspace (1..*)

Channel       (independent — fetched from Slack API)
User          (independent — fetched from Slack API)

Message
  ├── UserID → User.ID
  ├── ChannelID → Channel.ID
  └── ThreadTS → Message.Timestamp (self-referential, optional)

SearchResult
  └── embeds Message
      └── ChannelName (resolved from Channel.Name)
```

---

## State Transitions

This project is stateless (no persistence). The only "state" is the
in-memory flow of a single CLI invocation:

```
CLI invoked
  → Parse flags & validate DateRange
  → Extract tokens from local storage → TokenExtractionResult
  → Select Workspace (first or --workspace match)
  → Execute API command
  → Format output (text / table / JSON)
  → Exit 0
```

Error at any step → formatted error message to stderr → exit 1.
