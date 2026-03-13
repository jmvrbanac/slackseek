# Data Model: 009-mcp-support

## Overview

This feature introduces no new persistent storage. All data models are reused from existing packages (`internal/slack`, `internal/output`, `internal/tokens`). The new `internal/mcp` package owns only two new data structures:

---

## 1. `tokenCache` (internal/mcp/tokencache.go)

**Purpose**: Thread-safe in-memory cache for Slack workspace credentials. Prevents re-extraction on every tool call while ensuring stale credentials are refreshed proactively (TTL) or reactively (401 retry).

```go
type tokenCache struct {
    mu         sync.Mutex
    workspaces []tokens.Workspace
    fetchedAt  time.Time
    extractFn  func() (tokens.TokenExtractionResult, error)
}
```

| Field       | Type                                                | Description |
|-------------|-----------------------------------------------------|-------------|
| `mu`        | `sync.Mutex`                                        | Guards all fields; held only during read/write, not during API calls |
| `workspaces`| `[]tokens.Workspace`                               | Last-extracted workspace list; nil until first fetch |
| `fetchedAt` | `time.Time`                                         | Wall-clock time of last successful extraction |
| `extractFn` | `func() (tokens.TokenExtractionResult, error)` | Injectable extraction function; production = `tokens.DefaultExtract` |

**Constant**:
```go
const tokenTTL = 5 * time.Minute
```

**Methods**:

| Method | Signature | Behaviour |
|--------|-----------|-----------|
| `get`  | `() ([]tokens.Workspace, error)` | Lock; check TTL; re-extract if stale; return cached workspaces |
| `refresh` | `() ([]tokens.Workspace, error)` | Lock; always re-extract; update `workspaces` + `fetchedAt` |

**State transitions**:
```
[empty] → get() → [populated, fetchedAt=T]
[populated] → get() within TTL → [populated, unchanged]
[populated] → get() after TTL → refresh() → [populated, fetchedAt=T+n]
[any] → 401 from Slack → refresh() → [populated or error]
```

---

## 2. `slackClient` interface (internal/mcp/tools.go)

**Purpose**: Narrow interface over `*slack.Client` enabling mock injection in tests. Only the methods actually called by MCP tool handlers are included.

```go
type slackClient interface {
    SearchMessages(ctx context.Context, query string, limit int) ([]slack.SearchResult, error)
    FetchHistory(ctx context.Context, channelID string, dr slack.DateRange, limit int, threads bool) ([]slack.Message, error)
    GetUserMessages(ctx context.Context, userID, channelID string, dr slack.DateRange, limit int) ([]slack.Message, error)
    FetchThread(ctx context.Context, channelID, threadTS string) ([]slack.Message, error)
    ListChannels(ctx context.Context, types []string, includeArchived bool) ([]slack.Channel, error)
    ListUsers(ctx context.Context) ([]slack.User, error)
    ResolveChannel(ctx context.Context, nameOrID string) (string, error)
    ResolveUser(ctx context.Context, nameOrID string) (string, error)
    FetchUser(ctx context.Context, id string) (slack.User, error)
    FetchChannel(ctx context.Context, id string) (slack.Channel, error)
    ListUserGroups(ctx context.Context) ([]slack.UserGroup, error)
    ForceRefreshUserGroups(ctx context.Context) ([]slack.UserGroup, error)
}
```

**Note**: `*slack.Client` implicitly satisfies this interface; no code change to `internal/slack` is needed.

---

## 3. Reused Types (no changes)

These types are used by MCP tool handlers but are defined and owned by their respective packages:

| Type | Package | Used by tool |
|------|---------|--------------|
| `tokens.Workspace` | `internal/tokens` | All tools (workspace selection) |
| `slack.DateRange` | `internal/slack` | `slack_search`, `slack_history`, `slack_messages`, `slack_digest`, `slack_postmortem`, `slack_metrics`, `slack_actions` |
| `slack.SearchResult` | `internal/slack` | `slack_search` |
| `slack.Message` | `internal/slack` | `slack_history`, `slack_messages`, `slack_thread` |
| `slack.Channel` | `internal/slack` | `slack_channels` |
| `slack.User` | `internal/slack` | `slack_users` |
| `output.ChannelDigest` | `internal/output` | `slack_digest` |
| `output.ActionItem` | `internal/output` | `slack_actions` |
| `output.ChannelMetrics` | `internal/output` | `slack_metrics` |
| `output.IncidentDoc` | `internal/output` | `slack_postmortem` |

---

## 4. Tool Parameter Schema (per tool)

### `slack_search`
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `query` | string | yes | — | Search query string |
| `channels` | array[string] | no | [] | Limit to these channels |
| `user` | string | no | "" | Limit to this user (display name or ID) |
| `since` | string | no | "" | Start of range: ISO date, RFC 3339, or duration (30m, 4h, 7d) |
| `until` | string | no | "" | End of range: ISO date, RFC 3339, or duration |
| `limit` | number | no | 100 | Maximum results (0 = unlimited) |
| `workspace` | string | no | "" | Workspace name or URL; defaults to first |

### `slack_history`
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `channel` | string | yes | — | Channel name or ID |
| `since` | string | no | "" | Start of range |
| `until` | string | no | "" | End of range |
| `limit` | number | no | 100 | Maximum messages |
| `threads` | boolean | no | false | Include thread replies |
| `workspace` | string | no | "" | Workspace selector |

### `slack_messages`
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `user` | string | yes | — | User display name or ID |
| `since` | string | no | "" | Start of range |
| `until` | string | no | "" | End of range |
| `limit` | number | no | 100 | Maximum messages |
| `workspace` | string | no | "" | Workspace selector |

### `slack_thread`
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `url` | string | yes | — | Slack permalink URL to the thread |
| `workspace` | string | no | "" | Workspace selector |

### `slack_channels`
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `filter` | string | no | "" | Substring filter on channel name |
| `include_archived` | boolean | no | false | Include archived channels |
| `workspace` | string | no | "" | Workspace selector |

### `slack_users`
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `filter` | string | no | "" | Substring filter on name/email |
| `workspace` | string | no | "" | Workspace selector |

### `slack_digest`
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `user` | string | yes | — | User display name or ID |
| `since` | string | no | "" | Start of range |
| `until` | string | no | "" | End of range |
| `workspace` | string | no | "" | Workspace selector |

### `slack_postmortem`
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `channel` | string | yes | — | Channel name or ID |
| `since` | string | no | "" | Start of range |
| `until` | string | no | "" | End of range |
| `workspace` | string | no | "" | Workspace selector |

### `slack_metrics`
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `channel` | string | yes | — | Channel name or ID |
| `since` | string | no | "" | Start of range |
| `until` | string | no | "" | End of range |
| `workspace` | string | no | "" | Workspace selector |

### `slack_actions`
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `channel` | string | yes | — | Channel name or ID |
| `since` | string | no | "" | Start of range |
| `until` | string | no | "" | End of range |
| `workspace` | string | no | "" | Workspace selector |
