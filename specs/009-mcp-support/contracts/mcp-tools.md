# MCP Tool Contracts: 009-mcp-support

All tools are exposed via the `slackseek mcp serve` stdio MCP server.
Transport: JSON-RPC 2.0 over stdin/stdout.
All successful results return a single `TextContent` item whose `text` is a JSON string.

---

## `slack_search`

**Description**: Search Slack messages across the workspace using a query string, optionally filtered by channel(s), user, and date range.

**Parameters**:
```json
{
  "query":     { "type": "string", "required": true,  "description": "Search query" },
  "channels":  { "type": "array",  "items": "string", "description": "Limit to channels (names or IDs)" },
  "user":      { "type": "string", "description": "Limit to user (display name or ID)" },
  "since":     { "type": "string", "description": "Start: ISO date, RFC 3339, or duration (7d)" },
  "until":     { "type": "string", "description": "End: ISO date, RFC 3339, or duration" },
  "limit":     { "type": "number", "default": 100, "description": "Max results (0 = unlimited)" },
  "workspace": { "type": "string", "description": "Workspace name or URL" }
}
```

**Success result** (`text` field = JSON):
```json
[
  {
    "timestamp": "1700000000.123456",
    "time": "2023-11-14T12:34:56Z",
    "userID": "U01234567",
    "text": "message body",
    "channelID": "C01234567",
    "channelName": "general",
    "threadTS": "",
    "permalink": "https://acme.slack.com/archives/C01234567/p1700000000123456"
  }
]
```

**Error cases**:
- `"search <query> failed: <upstream error>"` — Slack API error
- Auth failure (see common errors below)

---

## `slack_history`

**Description**: Retrieve the message history of a channel within a date range.

**Parameters**:
```json
{
  "channel":   { "type": "string",  "required": true,  "description": "Channel name or ID" },
  "since":     { "type": "string",  "description": "Start of range" },
  "until":     { "type": "string",  "description": "End of range" },
  "limit":     { "type": "number",  "default": 100 },
  "threads":   { "type": "boolean", "default": false, "description": "Include thread replies" },
  "workspace": { "type": "string" }
}
```

**Success result** (`text` = JSON):
```json
[
  {
    "timestamp": "1700000000.123456",
    "time": "2023-11-14T12:34:56Z",
    "userID": "U01234567",
    "text": "hello",
    "channelID": "C01234567",
    "channelName": "",
    "threadTS": "",
    "threadDepth": 0,
    "reactions": []
  }
]
```

**Error cases**:
- `"channel <name> not found — use slack_channels to list available channels"`
- Auth failure

---

## `slack_messages`

**Description**: Retrieve messages sent by a specific user across all channels.

**Parameters**:
```json
{
  "user":      { "type": "string", "required": true },
  "since":     { "type": "string" },
  "until":     { "type": "string" },
  "limit":     { "type": "number", "default": 100 },
  "workspace": { "type": "string" }
}
```

**Success result**: same schema as `slack_history`.

**Error cases**:
- `"user <name> not found — use slack_users to list available users"`
- Auth failure

---

## `slack_thread`

**Description**: Retrieve all messages in a thread by permalink URL.

**Parameters**:
```json
{
  "url":       { "type": "string", "required": true, "description": "Slack permalink URL" },
  "workspace": { "type": "string" }
}
```

**Success result**: same schema as `slack_history` (array of messages).

**Error cases**:
- `"thread <url> failed: invalid permalink format"`
- Auth failure

---

## `slack_channels`

**Description**: List channels in the workspace, optionally filtered by name.

**Parameters**:
```json
{
  "filter":           { "type": "string",  "description": "Substring filter on channel name" },
  "include_archived": { "type": "boolean", "default": false },
  "workspace":        { "type": "string" }
}
```

**Success result** (`text` = JSON):
```json
[
  {
    "id":          "C01234567",
    "name":        "general",
    "type":        "public_channel",
    "memberCount": 42,
    "topic":       "General discussion",
    "isArchived":  false
  }
]
```

---

## `slack_users`

**Description**: List workspace members, optionally filtered by name or email.

**Parameters**:
```json
{
  "filter":    { "type": "string", "description": "Substring filter on display name, real name, or email" },
  "workspace": { "type": "string" }
}
```

**Success result** (`text` = JSON):
```json
[
  {
    "id":          "U01234567",
    "displayName": "john",
    "realName":    "John Smith",
    "email":       "john@example.com",
    "isBot":       false,
    "isDeleted":   false
  }
]
```

---

## `slack_digest`

**Description**: Show a per-channel message digest for a user over a date range.

**Parameters**:
```json
{
  "user":      { "type": "string", "required": true },
  "since":     { "type": "string" },
  "until":     { "type": "string" },
  "workspace": { "type": "string" }
}
```

**Success result** (`text` = JSON array of `ChannelDigest`):
```json
[
  {
    "channelID":    "C01234567",
    "channelName":  "engineering",
    "messageCount": 12,
    "messages": [ /* slack.Message objects */ ]
  }
]
```

---

## `slack_postmortem`

**Description**: Generate an incident postmortem document from a channel's message history.

**Parameters**:
```json
{
  "channel":   { "type": "string", "required": true },
  "since":     { "type": "string" },
  "until":     { "type": "string" },
  "workspace": { "type": "string" }
}
```

**Success result** (`text` = JSON `IncidentDoc`):
```json
{
  "channel":   "incidents",
  "dateRange": "2026-03-01 — 2026-03-07",
  "timeline":  [ /* events */ ],
  "actions":   [ /* action items */ ]
}
```

---

## `slack_metrics`

**Description**: Compute message activity metrics for a channel over a date range.

**Parameters**:
```json
{
  "channel":   { "type": "string", "required": true },
  "since":     { "type": "string" },
  "until":     { "type": "string" },
  "workspace": { "type": "string" }
}
```

**Success result** (`text` = JSON `ChannelMetrics`):
```json
{
  "channel":      "engineering",
  "messageCount": 150,
  "activeUsers":  12,
  "topPosters":   [ { "user": "alice", "count": 30 } ],
  "peakHour":     14
}
```

---

## `slack_actions`

**Description**: Extract action items (TODO/follow-up) from a channel's message history.

**Parameters**:
```json
{
  "channel":   { "type": "string", "required": true },
  "since":     { "type": "string" },
  "until":     { "type": "string" },
  "workspace": { "type": "string" }
}
```

**Success result** (`text` = JSON array of `ActionItem`):
```json
[
  {
    "text":      "Follow up with Alice on the deploy",
    "userID":    "U01234567",
    "timestamp": "1700000000.123456"
  }
]
```

---

## Common Error Messages

| Condition | Error text |
|-----------|-----------|
| Auth failure (after retry) | `"slack credentials unavailable: <reason> — ensure the Slack desktop app is running and you are logged in"` |
| Channel not found | `"channel <name> not found — use slack_channels to list available channels"` |
| User not found | `"user <name> not found — use slack_users to list available users"` |
| Rate limited | `"slack rate limit exceeded — retry after <N>s"` |
| Invalid date range | `"invalid date range: <parse error>"` |
| Other errors | `"<tool name> failed: <upstream error>"` |

All errors are returned as `CallToolResult` with `IsError: true`.
