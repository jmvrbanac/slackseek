---

description: "Task list for slackseek CLI implementation"
---

# Tasks: slackseek CLI

**Input**: Design documents from `specs/001-slackseek-cli/`
**Prerequisites**: plan.md ✅ spec.md ✅ research.md ✅ data-model.md ✅ contracts/ ✅ quickstart.md ✅

> **⚠️ CONSTITUTION MANDATE — Test-First (NON-NEGOTIABLE)**
> Per Principle II of the slackseek Constitution: every test task MUST be
> written and confirmed FAILING before its corresponding implementation task
> begins. Test tasks appear before implementation tasks within every phase.
> `go test -race ./...` MUST pass at every checkpoint.

## Format: `[ID] [P?] [Story?] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: User story this task belongs to (US1–US5)
- Exact file paths are included in every task description

## Path Conventions

All paths are relative to the repository root `github.com/jmvrbanac/slackseek/`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Initialize the Go module, directory layout, entry point, and tooling.

- [ ] T001 Create `go.mod` at repository root with `module github.com/jmvrbanac/slackseek` and `go 1.24`
- [ ] T002 Create directory structure: `cmd/`, `internal/tokens/`, `internal/slack/`, `internal/output/` (empty `.gitkeep` files are fine)
- [ ] T003 [P] Create `main.go` at repository root with a minimal `main()` that calls the cobra root command
- [ ] T004 [P] Create `.golangci.yml` at repository root enabling: `errcheck`, `govet`, `staticcheck`, `revive`, `gocyclo` (max complexity 15), `funlen` (max 40 lines)
- [ ] T005 [P] Add all project dependencies: `go get github.com/spf13/cobra github.com/syndtr/goleveldb/leveldb modernc.org/sqlite github.com/godbus/dbus/v5 github.com/keybase/go-keychain github.com/slack-go/slack@latest github.com/olekukonko/tablewriter github.com/cenkalti/backoff/v4`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core shared infrastructure required by every user story.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

> **TEST-FIRST**: Write tests T006–T008 first. Confirm they FAIL. Then implement T009–T011.

- [ ] T006 [P] Write unit tests for output formatters in `internal/output/format_test.go`: verify `JSON()` output is valid JSON array, `Table()` output contains expected column headers, `Text()` output has one entry per line; use `Workspace`, `Channel`, `Message`, `User`, `SearchResult` fixture structs
- [ ] T007 [P] Write unit tests for global flag validation in `cmd/root_test.go`: `--format xyz` exits with code 1 and stderr message; `--format json/text/table` accepted; `--from 2025-02-01 --to 2025-01-01` (from > to) exits with code 1 and stderr message before any API call
- [ ] T008 [P] Write unit tests for `DateRange` parsing in `internal/slack/daterange_test.go`: `YYYY-MM-DD` parses as `00:00:00 UTC`, RFC 3339 parses correctly, `From > To` returns error, nil fields when flags omitted
- [ ] T009 [P] Create `internal/output/doc.go` with package comment: `// Package output formats slackseek results as text, aligned tables, or JSON arrays.`
- [ ] T010 Create `internal/output/format.go` with: `Format` string type and constants (`FormatText`, `FormatTable`, `FormatJSON`); `PrintWorkspaces`, `PrintChannels`, `PrintMessages`, `PrintSearchResults`, `PrintUsers` functions each accepting format and writing to `io.Writer`; use `tablewriter` for table format, `encoding/json` for JSON, plain text for text
- [ ] T011 Create `cmd/root.go` with: cobra root command; global persistent flags `--workspace/-w` (string), `--format` (string, default `text`), `--from` (string), `--to` (string); `PersistentPreRunE` that validates `--format` and parses `--from`/`--to` into `DateRange`; workspace selection helper `SelectWorkspace(workspaces []tokens.Workspace, selector string) (tokens.Workspace, error)` that emits a notice to stderr when defaulting

**Checkpoint**: `go test -race ./internal/output/... ./cmd/... ./internal/slack/daterange_test.go` passes (or skips for not-yet-implemented packages). Foundation ready.

---

## Phase 3: User Story 1 — Verify Local Slack Session (Priority: P1) 🎯 MVP

**Goal**: `slackseek auth show` and `slackseek auth export` display discovered workspace credentials without network calls.

**Independent Test**: On a machine with Slack installed and logged in, `slackseek auth show` prints at least one workspace row with a truncated token and cookie. `go test -race ./internal/tokens/...` passes with mocks (no real Slack required).

> **TEST-FIRST**: Write all test tasks T012–T015 first. Confirm they FAIL. Then implement T016–T026.

### Tests for User Story 1 — Write FIRST, ensure FAIL before implementing

- [ ] T012 [P] [US1] Write unit tests for LevelDB extraction in `internal/tokens/leveldb_test.go`: create a synthetic LevelDB fixture using `goleveldb` in a temp dir with a `localConfig_v2` key containing a JSON blob with `teams` array; assert `ExtractWorkspaceTokens(dir string) ([]WorkspaceToken, error)` returns correct Name, Token, URL for each team entry; assert non-localConfig_v2 keys are ignored
- [ ] T013 [P] [US1] Write unit tests for cookie decryption in `internal/tokens/cookie_test.go`: create a mock SQLite cookie DB in a temp dir with a pre-encrypted `d` cookie value; provide a mock `KeyringReader` returning a known password; assert `DecryptCookie(dbPath string, kr KeyringReader, iterations int) (string, error)` returns the expected plaintext after PBKDF2-HMAC-SHA1 + AES-128-CBC + PKCS7 unpad + v10 prefix strip
- [ ] T014 [P] [US1] Write unit tests for the `Extract()` orchestrator in `internal/tokens/extractor_test.go`: use a mock `KeyringReader` and mock `CookiePathProvider`; assert single-workspace happy path returns `TokenExtractionResult` with one `Workspace`; assert missing cookie is non-fatal (Warnings field populated, Workspaces still returned); assert zero workspaces found returns an error
- [ ] T015 [P] [US1] Write unit tests for `cmd/auth.go` in `cmd/auth_test.go`: mock `Extract()` via injectable function; assert `auth show` table output contains columns Name/URL/Token/Cookie; assert `auth show --format json` output is valid JSON with fields `name`, `url`, `token`, `cookie`; assert `auth export` output lines match `export SLACK_TOKEN_<NAME>=<full-token>`; assert extraction failure exits with code 1 and actionable stderr message

### Implementation for User Story 1

- [ ] T016 [P] [US1] Create `internal/tokens/doc.go` with package comment: `// Package tokens extracts Slack authentication credentials from the locally installed Slack desktop application.`
- [ ] T017 [P] [US1] Define `KeyringReader` interface in `internal/tokens/keyring.go`: `type KeyringReader interface { ReadPassword(service, account string) ([]byte, error) }`; also define `MockKeyringReader` struct for use in tests
- [ ] T018 [P] [US1] Define `CookiePathProvider` interface in `internal/tokens/paths.go`: `type CookiePathProvider interface { LevelDBPath() string; CookiePath() string }`
- [ ] T019 [US1] Implement LevelDB workspace token extraction in `internal/tokens/leveldb.go`: `WorkspaceToken` struct with Name/Token/URL fields; `ExtractWorkspaceTokens(leveldbDir string) ([]WorkspaceToken, error)` copies dir to `os.MkdirTemp`, opens with `goleveldb`, iterates keys looking for one containing `localConfig_v2`, unmarshals JSON into struct with `teams` map, extracts `token`/`name`/`url` fields, removes temp dir with `defer os.RemoveAll`
- [ ] T020 [US1] Implement AES-128-CBC cookie decryption in `internal/tokens/cookie.go`: `DecryptCookie(dbPath string, kr KeyringReader, iterations int) (string, error)` copies SQLite file to temp dir, opens with `modernc.org/sqlite`, reads `version` from `meta` table, queries `cookies` for `host_key LIKE '%slack.com%' AND name='d'`, fetches `encrypted_value`, derives 16-byte AES key via `pbkdf2.Key(password, []byte("saltysalt"), iterations, 16, sha1.New)`, decrypts with `aes.NewCipher` + CBC mode + IV of 16 `0x20` bytes, strips `v10`/`v11` prefix, strips additional 32 bytes if DB version >= 24, removes PKCS7 padding, returns UTF-8 string
- [ ] T021 [P] [US1] Implement Linux D-Bus SecretService keyring in `internal/tokens/keyring_linux.go` (file starts with `//go:build linux`): `LinuxKeyringReader` struct implementing `KeyringReader`; connect to session D-Bus via `github.com/godbus/dbus/v5`, open default secret collection, iterate items, find item with attribute `application == "Slack"`, return its secret bytes; PBKDF2 iterations constant = 1
- [ ] T022 [P] [US1] Implement macOS Keychain keyring in `internal/tokens/keyring_darwin.go` (file starts with `//go:build darwin`): `DarwinKeyringReader` struct implementing `KeyringReader`; query login keychain via `github.com/keybase/go-keychain` for service `"Slack Safe Storage"` account `"Slack"`, return password bytes; PBKDF2 iterations constant = 1003
- [ ] T023 [P] [US1] Implement Linux paths in `internal/tokens/paths_linux.go` (file starts with `//go:build linux`): `LinuxPathProvider` struct implementing `CookiePathProvider`; `LevelDBPath()` returns `filepath.Join(os.UserHomeDir(), ".config", "Slack", "Local Storage", "leveldb")`; `CookiePath()` returns `filepath.Join(os.UserHomeDir(), ".config", "Slack", "Cookies")`
- [ ] T024 [P] [US1] Implement macOS paths in `internal/tokens/paths_darwin.go` (file starts with `//go:build darwin`): `DarwinPathProvider` struct implementing `CookiePathProvider`; `LevelDBPath()` and `CookiePath()` return paths under `~/Library/Application Support/Slack/`
- [ ] T025 [US1] Implement `Extract()` orchestrator in `internal/tokens/extractor.go`: `Workspace` struct (Name, URL, Token, Cookie); `TokenExtractionResult` struct (Workspaces []Workspace, Warnings []string); `Extract(kr KeyringReader, pp CookiePathProvider) (TokenExtractionResult, error)` calls `ExtractWorkspaceTokens(pp.LevelDBPath())`, for each workspace calls `DecryptCookie(pp.CookiePath(), kr, platformIterations)` (cookie failure appends to Warnings, does not fail), assembles result; zero workspaces returns error; also provide `DefaultExtract() (TokenExtractionResult, error)` that constructs platform-appropriate `KeyringReader` and `CookiePathProvider` at runtime
- [ ] T026 [US1] Implement `cmd/auth.go` with `auth` parent command, `auth show` subcommand (calls `DefaultExtract()`, displays table/text/JSON with columns Workspace/URL/Token-prefix-12chars/Cookie-prefix-8chars to stdout; errors to stderr with actionable message), `auth export` subcommand (calls `DefaultExtract()`, prints `export SLACK_TOKEN_<UPPER_NAME>=<full-token>` to stdout ignoring `--format` flag)

**Checkpoint**: `slackseek auth show` functional and independently testable. `go test -race ./...` passes.

---

## Phase 4: User Story 2 — Search Workspace Messages (Priority: P2)

**Goal**: `slackseek search <query>` returns matching messages with optional channel, user, and date filters.

**Independent Test**: Run `slackseek search "test" --limit 5` — returns results or empty table (not an error) with timestamp, channel, user, and text columns.

> **TEST-FIRST**: Write all test tasks T027–T030 first. Confirm they FAIL. Then implement T031–T035.

### Tests for User Story 2 — Write FIRST, ensure FAIL before implementing

- [ ] T027 [P] [US2] Write unit tests for rate-limit retry in `internal/slack/client_test.go`: mock `http.RoundTripper` returning 429 with `Retry-After: 1` then 200; assert retry occurs exactly once with ~1s delay; mock returning three consecutive 500s; assert exponential backoff fires and final error is returned after 3 attempts; mock 200 on first attempt; assert no retry
- [ ] T028 [P] [US2] Write unit tests for user resolution in `internal/slack/users_test.go`: assert exact Slack user ID passthrough (no API call made); assert display name match returns correct user ID; assert real name substring match returns correct user ID; assert ambiguous match (two users share substring) returns descriptive error; assert no match returns error listing closest candidates
- [ ] T029 [P] [US2] Write unit tests for search query composition in `internal/slack/search_test.go`: assert `BuildSearchQuery(query, channel, userID string, dr DateRange)` returns `"test in:#general from:U123 after:2025-01-01 before:2025-02-01"` for given inputs; assert empty optional fields are omitted from query; assert `SearchMessages(ctx, query string, limit int)` maps API response fields to `[]SearchResult` with Permalink, ChannelName, and embedded Message fields populated
- [ ] T030 [P] [US2] Write unit tests for `cmd/search.go` in `cmd/search_test.go`: mock the slack client via injectable function; assert `--limit 2` returns at most 2 results; assert `--format json` output is a valid JSON array matching json-schema.md SearchResult schema; assert missing `<query>` argument exits with code 1; assert `--user` and `--channel` flags are passed through to query builder

### Implementation for User Story 2

- [ ] T031 [P] [US2] Create `internal/slack/doc.go` with package comment: `// Package slack provides an authenticated Slack API client with rate-limit retry and entity resolution helpers.`
- [ ] T032 [US2] Implement `internal/slack/client.go`: `Client` struct wrapping `slack.Client` (slack-go); `NewClient(token string) *Client`; `callWithRetry(ctx context.Context, fn func() error) error` that on HTTP 429 reads `Retry-After` header and sleeps that duration (max 3 attempts), on 5xx uses `cenkalti/backoff/v4` exponential backoff with jitter (max 3 attempts), on other errors returns immediately; `DateRange` struct (From, To *time.Time) with `ParseDateRange(from, to string) (DateRange, error)` that accepts YYYY-MM-DD or RFC3339, returns error if From > To
- [ ] T033 [P] [US2] Implement user listing and name→ID resolution in `internal/slack/users.go`: `ListUsers(ctx context.Context) ([]User, error)` paginates `users.list` collecting all pages; `ResolveUser(ctx context.Context, nameOrID string) (string, error)` passes through if matches `[UW][A-Z0-9]+` pattern, otherwise calls `ListUsers` and searches `DisplayName` and `RealName` case-insensitively; returns descriptive error listing candidates on ambiguity or no match
- [ ] T034 [US2] Implement full-text search in `internal/slack/search.go`: `BuildSearchQuery(query, channel, userID string, dr DateRange) string` composes Slack search modifiers; `SearchMessages(ctx context.Context, query string, limit int) ([]SearchResult, error)` paginates `search.messages` via `callWithRetry`, maps each match to `SearchResult` (embedded Message + ChannelName + Permalink), stops at limit (0 = unlimited)
- [ ] T035 [US2] Implement `cmd/search.go`: `search` command with positional `<query>` arg (required); flags `--channel/-c` (string), `--user/-u` (string), `--limit/-n` (int, default 100); `RunE` resolves workspace via `DefaultExtract()`, creates `Client`, optionally resolves `--user` to user ID via `ResolveUser`, calls `BuildSearchQuery` + `SearchMessages`, writes results via `output.PrintSearchResults`

**Checkpoint**: `slackseek search "test" --limit 5 --format json` functional. `go test -race ./...` passes.

---

## Phase 5: User Story 3 — Retrieve Channel Message History (Priority: P3)

**Goal**: `slackseek history <channel>` fetches ordered messages with inline thread replies and date/limit filtering.

**Independent Test**: `slackseek history general --from 2025-01-01 --to 2025-02-01 --limit 50` returns messages in chronological order; replies appear immediately after their parent with thread_depth > 0.

> **TEST-FIRST**: Write test tasks T036–T037 first. Confirm they FAIL. Then implement T038–T039.

### Tests for User Story 3 — Write FIRST, ensure FAIL before implementing

- [ ] T036 [P] [US3] Write unit tests for channel resolution and history fetching in `internal/slack/channels_test.go`: assert exact channel ID passthrough (no API call); assert name match returns correct ID; assert ambiguous match returns error; assert `FetchHistory(ctx, channelID string, dr DateRange, limit int, threads bool) ([]Message, error)` converts DateRange to `oldest`/`latest` unix strings; assert thread replies are interleaved directly after parent sorted by Timestamp; assert limit is respected across root messages + replies; assert messages sorted ascending by Timestamp
- [ ] T037 [P] [US3] Write unit tests for `cmd/history.go` in `cmd/history_test.go`: assert `<channel>` argument is required (exit 1 when missing); assert `--threads=false` skips reply fetching; assert `--limit 10` passed through to `FetchHistory`; assert invalid channel name exits with code 1 and actionable error naming the unknown channel; assert table output contains columns Timestamp/User/Text/Depth/Reactions

### Implementation for User Story 3

- [ ] T038 [US3] Implement channel listing, name→ID resolution, history pagination, and thread merging in `internal/slack/channels.go`: `ListChannels(ctx context.Context, types []string, includeArchived bool) ([]Channel, error)` paginates `conversations.list`; `ResolveChannel(ctx context.Context, nameOrID string) (string, error)` passes through Slack ID pattern, otherwise searches ListChannels case-insensitively, returns descriptive error on ambiguity/not-found; `FetchHistory(ctx context.Context, channelID string, dr DateRange, limit int, threads bool) ([]Message, error)` paginates `conversations.history` with `oldest`/`latest` from DateRange, for each message with `reply_count > 0` (when threads=true) calls `conversations.replies` via `callWithRetry`, merges replies inline sorted by Timestamp, enforces limit across all messages, returns final sorted slice
- [ ] T039 [US3] Implement `cmd/history.go`: `history` command with positional `<channel>` arg (required); flags `--threads/-T` (bool, default true), `--limit/-n` (int, default 1000); `RunE` extracts workspace, creates `Client`, resolves channel via `ResolveChannel`, calls `FetchHistory`, writes via `output.PrintMessages`; prints rate-limit progress warning to stderr when `Retry-After > 30s`: `"rate limited — waiting Xs (fetched N messages so far)"`

**Checkpoint**: `slackseek history <channel> --limit 20` functional and independently testable. `go test -race ./...` passes.

---

## Phase 6: User Story 4 — Aggregate Messages by User (Priority: P3)

**Goal**: `slackseek messages <user>` aggregates all messages from a specific user, optionally scoped to a channel and date range.

**Independent Test**: `slackseek messages <display-name> --limit 10` returns messages from that user across channels; `slackseek messages <display-name> --channel general` scopes results to that channel only.

> **TEST-FIRST**: Write test tasks T040–T041 first. Confirm they FAIL. Then implement T042–T043.

### Tests for User Story 4 — Write FIRST, ensure FAIL before implementing

- [ ] T040 [P] [US4] Write unit tests for per-user aggregation in `internal/slack/messages_test.go`: assert `GetUserMessages(ctx, userID, channelID string, dr DateRange, limit int) ([]Message, error)` composes `from:<userID>` query; assert optional `channelID` adds `in:<channelID>` modifier; assert DateRange adds `after:`/`before:` modifiers; assert results are mapped to `[]Message` with ChannelID and ChannelName populated; assert limit is respected
- [ ] T041 [P] [US4] Write unit tests for `cmd/messages.go` in `cmd/messages_test.go`: assert `<user>` argument required (exit 1 when missing); assert `--channel` flag passes channel ID to `GetUserMessages`; assert unknown user exits with code 1 and actionable error; assert `--format json` output matches json-schema.md messages schema

### Implementation for User Story 4

- [ ] T042 [US4] Implement per-user message aggregation in `internal/slack/messages.go`: `GetUserMessages(ctx context.Context, userID, channelID string, dr DateRange, limit int) ([]Message, error)` composes search query via `BuildSearchQuery` with the user ID as `from:` filter and optional channel as `in:` filter, delegates to `SearchMessages`, maps `SearchResult` slice to `[]Message` slice
- [ ] T043 [US4] Implement `cmd/messages.go`: `messages` command with positional `<user>` arg (required); flags `--channel/-c` (string), `--limit/-n` (int, default 1000), `--threads/-T` (bool, default true, included in output column but does not change API call); `RunE` extracts workspace, creates `Client`, resolves user via `ResolveUser`, optionally resolves channel via `ResolveChannel`, calls `GetUserMessages`, writes via `output.PrintMessages`

**Checkpoint**: `slackseek messages <user>` functional. `go test -race ./...` passes.

---

## Phase 7: User Story 5 — Browse Workspace Resources (Priority: P4)

**Goal**: `slackseek channels list` and `slackseek users list` display workspace metadata with filter flags.

**Independent Test**: `slackseek channels list --format json | jq length` returns a positive number; `slackseek users list --bot` returns rows including bot accounts.

Note: `internal/slack/channels.go` (US3) and `internal/slack/users.go` (US2) are already complete. This phase adds only the `cmd/` layer.

> **TEST-FIRST**: Write test tasks T044–T045 first. Confirm they FAIL. Then implement T046–T047.

### Tests for User Story 5 — Write FIRST, ensure FAIL before implementing

- [ ] T044 [P] [US5] Write unit tests for `cmd/channels.go` in `cmd/channels_test.go`: assert `channels list` with `--type public` passes `["public_channel"]` to `ListChannels`; assert `--archived` passes `includeArchived=true`; assert table output contains columns ID/Name/Type/Members/Topic; assert `--format json` output matches json-schema.md channels schema; assert `--type invalid` exits with code 1
- [ ] T045 [P] [US5] Write unit tests for `cmd/users.go` in `cmd/users_test.go`: assert `users list` without flags filters out deleted and bot accounts; assert `--deleted` includes deleted users; assert `--bot` includes bot accounts; assert `--format json` output matches json-schema.md users schema

### Implementation for User Story 5

- [ ] T046 [US5] Implement `cmd/channels.go`: `channels` parent command; `channels list` subcommand with `--type` (string, validated against `public|private|mpim|im`, maps to Slack API `types` param) and `--archived` (bool, default false) flags; `RunE` extracts workspace, creates `Client`, calls `ListChannels`, writes via `output.PrintChannels` with columns ID/Name/Type/Members/Topic
- [ ] T047 [US5] Implement `cmd/users.go`: `users` parent command; `users list` subcommand with `--deleted` (bool, default false) and `--bot` (bool, default false) flags; `RunE` extracts workspace, creates `Client`, calls `ListUsers`, filters out deleted users unless `--deleted` and bot users unless `--bot`, writes via `output.PrintUsers` with columns ID/DisplayName/RealName/Email/Bot/Deleted

**Checkpoint**: All five user stories independently functional. `go test -race ./...` passes.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Integration test scaffolding, linting gate, cross-platform build verification.

- [ ] T048 [P] Add `INTEGRATION=1`-gated integration test scaffold in `internal/tokens/integration_test.go`: `func TestExtractIntegration(t *testing.T)` calls `t.Skip("set INTEGRATION=1 to run")` when env var unset; when set, calls `DefaultExtract()` and asserts at least one workspace returned with non-empty Token
- [ ] T049 [P] Add `INTEGRATION=1`-gated integration test scaffold in `internal/slack/integration_test.go`: `func TestSearchIntegration(t *testing.T)` skips when env var unset; when set, creates `Client` from env var `SLACK_TOKEN`, calls `SearchMessages` with a benign query, asserts no error returned
- [ ] T050 Run `go vet ./...` from repository root and fix every reported issue before proceeding
- [ ] T051 Run `golangci-lint run` from repository root and fix every reported issue before proceeding
- [ ] T052 [P] Verify `GOOS=linux GOARCH=amd64 go build -o /dev/null ./...` exits 0 (run from repository root)
- [ ] T053 [P] Verify `GOOS=darwin GOARCH=arm64 go build -o /dev/null ./...` exits 0 (run from repository root)
- [ ] T054 Run `go test -race ./...` from repository root; confirm zero test failures and zero race condition reports
- [ ] T055 [P] Run the quickstart validation checklist from `specs/001-slackseek-cli/quickstart.md` on a machine with Slack installed and confirm all seven checklist items pass

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion — **BLOCKS all user stories**
- **US1 (Phase 3)**: Depends on Phase 2 — no story dependencies
- **US2 (Phase 4)**: Depends on Phase 2 and Phase 3 (needs token extraction)
- **US3 (Phase 5)**: Depends on Phase 2 and Phase 3; reuses `Client` from US2
- **US4 (Phase 6)**: Depends on Phase 2, Phase 3, and Phase 4 (reuses `users.go` + `search.go`)
- **US5 (Phase 7)**: Depends on Phase 2, Phase 3, Phase 4 (`users.go`), and Phase 5 (`channels.go`)
- **Polish (Phase 8)**: Depends on all user stories complete

### User Story Dependencies

| Story | Depends On | Reuses |
|---|---|---|
| US1 (Auth) | Phase 2 foundation | — |
| US2 (Search) | US1 (token extraction) | — |
| US3 (History) | US1 (token extraction), US2 `Client` | `Client` from US2 |
| US4 (Messages) | US1, US2 `users.go` + `search.go` | `users.go`, `search.go` |
| US5 (Browse) | US1, US2 `users.go`, US3 `channels.go` | `users.go`, `channels.go` |

### Within Each User Story

1. **Write tests → confirm FAIL** (all [P] test tasks in parallel)
2. Create interfaces and doc.go (can be parallel)
3. Implement core package code (may depend on interfaces)
4. Implement platform-specific files (parallel where build-tagged)
5. Implement cmd/ layer (depends on core package)
6. Run `go test -race ./...` to confirm green before checkpoint

### Parallel Opportunities

Within Phase 3 (US1):
- T012, T013, T014, T015: all test tasks can run simultaneously
- T016, T017, T018: interfaces + doc, all parallel
- T021, T022, T023, T024: platform files, all parallel

Within Phase 4 (US2):
- T027, T028, T029, T030: all test tasks simultaneously
- T033: `users.go` can be done in parallel with `client.go` (T032)

Within each [P]-marked pair across phases, different developers can work simultaneously.

---

## Implementation Strategy

### MVP: User Story 1 Only (auth show/export)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: US1 (auth commands)
4. **STOP AND VALIDATE**: `slackseek auth show` works end-to-end
5. `go test -race ./...` passes

### Incremental Delivery

1. Phase 1 + 2 → Foundation ready
2. Phase 3 (US1) → MVP: credential verification without network ✅
3. Phase 4 (US2) → Add: message search ✅
4. Phase 5 (US3) → Add: channel history ✅
5. Phase 6 (US4) → Add: per-user messages ✅
6. Phase 7 (US5) → Add: workspace browsing ✅
7. Phase 8 → Polish and release gate ✅

### Parallel Team Strategy

After Phase 2 completes and US1 completes (required by all):
- Developer A: US2 (Search)
- Developer B: US3 (History)
- US4 and US5 follow once their prerequisite stories are done

---

## Notes

- **[P]** tasks operate on different files with no incomplete dependencies
- **[Story]** label maps each task to its user story for traceability
- Test tasks are MANDATORY per the slackseek Constitution Principle II — not optional
- Verify each test FAILS before implementing its corresponding code
- Run `go test -race ./...` at every phase checkpoint — any race condition is a blocker
- Stop at each **Checkpoint** line to validate the story is independently functional
- All errors must include what/why/what-to-do per Constitution Principle IV
- Platform-specific files use `//go:build linux` or `//go:build darwin` as the first non-comment line per Constitution Principle V
