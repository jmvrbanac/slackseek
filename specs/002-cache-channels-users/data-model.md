# Data Model: Channel and User List Caching

**Feature**: `002-cache-channels-users`
**Date**: 2026-03-03

---

## Entities

### CacheStore

Represents an on-disk cache directory for a single tool installation. Constructed
once per command invocation at the `cmd/` layer and injected into `internal/slack`.

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `dir` | `string` | Absolute path to the base cache directory (e.g., `~/.cache/slackseek`) |
| `ttl` | `time.Duration` | Maximum age of a cache entry before it is considered stale. Zero disables caching. |

**Behaviour**:

- `Load(key, kind string) ([]byte, bool, error)`:
  Reads `{dir}/{key}/{kind}.json`. Returns `(data, true, nil)` if the file exists and
  its `ModTime` is within TTL. Returns `(nil, false, nil)` on a cache miss. Returns
  `(nil, false, err)` only on unexpected I/O errors that are not `os.IsNotExist`.

- `Save(key, kind string, data []byte) error`:
  Writes `data` atomically to `{dir}/{key}/{kind}.json` using a temp-file + rename
  pattern. Creates the `{dir}/{key}/` subdirectory as needed.

- `Clear(key string) error`:
  Removes the `{dir}/{key}/` subdirectory and all its contents.

- `ClearAll() error`:
  Removes the entire `{dir}/` directory and all its contents.

---

### Cache Entry (on-disk file)

Each cache entry is a plain JSON file. The file's modification time is used as the
cached-at timestamp; no envelope or metadata is embedded in the JSON payload itself.

**File paths**:

```
$XDG_CACHE_HOME/slackseek/          ← base dir (os.UserCacheDir()+"/slackseek")
└── {workspaceKey}/                 ← 16-char hex prefix of SHA-256(workspaceURL)
    ├── channels.json               ← JSON array of Channel objects
    └── users.json                  ← JSON array of User objects
```

**channels.json payload** (JSON array of `slack.Channel`):

```json
[
  {
    "id":          "C01234567",
    "name":        "general",
    "type":        "public_channel",
    "memberCount": 42,
    "topic":       "Company-wide announcements",
    "isArchived":  false
  }
]
```

**users.json payload** (JSON array of `slack.User`):

```json
[
  {
    "id":          "U01234567",
    "displayName": "alice",
    "realName":    "Alice Smith",
    "email":       "alice@example.com",
    "isBot":       false,
    "isDeleted":   false
  }
]
```

---

## State Transitions

```
               invocation
                   │
         ┌─────────▼──────────┐
         │  Check cache file   │
         └─────────┬──────────┘
                   │
        ┌──────────┴──────────┐
        │                     │
  File missing           File exists
  or stale               and fresh
        │                     │
        ▼                     ▼
  Fetch from API         Return cached data
        │                     │
        ▼                     └──────────────┐
  Save to cache                              │
        │                                    │
        └────────────────────┬───────────────┘
                             ▼
                      Return results to caller
```

**Flag overrides**:
- `--refresh-cache`: always go through "Fetch from API → Save to cache" path.
- `--no-cache` / `--cache-ttl 0`: skip both Load and Save entirely.

---

## Validation Rules

| Rule | Description |
|------|-------------|
| TTL ≥ 0 | Negative TTL is rejected at flag-parse time with an actionable error. |
| `--refresh-cache` + `--no-cache` | Mutually exclusive; tool exits with a validation error. |
| Corrupt cache file | Unmarshalling error on Load → treated as cache miss; log at debug level. |
| Unwritable cache dir | Save error → warn to stderr, return nil (continue without caching). |
| Missing workspace URL | An empty workspace URL produces an empty-string hash key; caching is skipped. |

---

## Package Layout (new and modified files)

```
internal/
└── cache/
    ├── doc.go          # Package-level documentation
    ├── store.go        # CacheStore type, Load, Save, Clear, ClearAll
    └── store_test.go   # Unit tests (temp dir, mock os.Stat times)

internal/slack/
├── client.go           # Add optional *cache.Store field + constructor variant
├── channels.go         # ListChannels: check/write cache when store is non-nil
└── users.go            # ListUsers: check/write cache when store is non-nil

cmd/
├── root.go             # Add --cache-ttl, --refresh-cache, --no-cache flags
├── cache.go            # New: `cache clear` command
├── cache_test.go       # New: unit tests for cache clear command
├── channels.go         # Pass cache store through defaultRunChannels
└── users.go            # Pass cache store through defaultRunUsers
```
