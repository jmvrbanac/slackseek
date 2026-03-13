# Tasks: 009-mcp-support (MCP Server)

**Input**: Design documents from `/specs/009-mcp-support/`
**Prerequisites**: plan.md ✅, research.md ✅, data-model.md ✅, contracts/mcp-tools.md ✅, quickstart.md ✅

**Tests**: MANDATORY per Constitution Principle II (Test-First, NON-NEGOTIABLE). Test tasks appear before their implementation targets in each phase.

**Organization**: Tasks grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story?] Description`

- **[P]**: Can run in parallel (different files, no shared dependencies)
- **[Story]**: User story this task belongs to (US1–US4)

## Path Conventions

```text
cmd/mcp.go                       # new Cobra subcommand
internal/mcp/doc.go              # package comment
internal/mcp/tokencache.go       # token refresh cache
internal/mcp/tokencache_test.go  # tokencache unit tests
internal/mcp/tools.go            # slackClient interface + helpers + handlers
internal/mcp/tools_test.go       # handler unit tests
internal/mcp/server.go           # MCP server init + tool registration
internal/mcp/server_test.go      # server unit tests
cmd/mcp_test.go                  # CLI subcommand test
go.mod / go.sum                  # updated for mcp-go dependency
```

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add the new dependency and create the package skeleton.

- [ ] T001 Add `github.com/mark3labs/mcp-go` to `go.mod` via `go get github.com/mark3labs/mcp-go` then run `go mod tidy` to update `go.sum`
- [ ] T002 Create `internal/mcp/doc.go` with package-level comment stating the package's single purpose: exposing Slack operations as MCP tools over stdio transport (Constitution Principle III)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Token cache and shared helpers that MUST be complete before any user story handler can be implemented.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

> **NOTE: Write test files FIRST (T003, T005) and ensure they FAIL before writing implementations (T004, T006).**

- [ ] T003 Write `internal/mcp/tokencache_test.go` with failing tests covering: cache hit within TTL (no re-extract called), cache miss after TTL expires (re-extract called), `refresh()` always calls extractFn regardless of TTL, concurrent `get()` calls do not race (use `go test -race`), extractFn error propagated on empty cache
- [ ] T004 Implement `internal/mcp/tokencache.go` — `tokenCache` struct with `sync.Mutex`, `[]tokens.Workspace`, `time.Time fetchedAt`, injectable `extractFn`; implement `get()` (TTL check → refresh if stale) and `refresh()` (always re-extract, update fields); `tokenTTL = 5 * time.Minute` constant; all functions ≤ 40 lines
- [ ] T005 [P] Define `slackClient` interface (12 methods) at the top of `internal/mcp/tools.go`: `SearchMessages`, `FetchHistory`, `GetUserMessages`, `FetchThread`, `ListChannels`, `ListUsers`, `ResolveChannel`, `ResolveUser`, `FetchUser`, `FetchChannel`, `ListUserGroups`, `ForceRefreshUserGroups` — signatures must match `*slack.Client` exactly so no changes to `internal/slack` are needed
- [ ] T006 Implement four private helper functions in `internal/mcp/tools.go`: `parseDateRange(since, until string) (slack.DateRange, error)` (delegates to `slack.ParseRelativeDateRange` or `slack.ParseDateRange`); `selectWorkspace(workspaces []tokens.Workspace, selector string) (tokens.Workspace, error)` (silent — no stderr); `buildMCPClient(ws tokens.Workspace) slackClient` (creates `*slack.Client` with 24h cache TTL); `buildMCPResolver(ctx context.Context, ws tokens.Workspace, c slackClient) *slack.Resolver` (mirrors `cmd.buildResolver` without global flags or stderr)

**Checkpoint**: Token cache tests pass with `go test -race ./internal/mcp/...`. Foundation ready.

---

## Phase 3: User Story 1 — MCP Server + CLI Command (Priority: P1) 🎯 MVP

**Goal**: `slackseek mcp serve` starts an MCP server over stdio that an MCP client can connect to. No tools exposed yet — just the server running without crashing.

**Independent Test**: `echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}' | slackseek mcp serve` returns a valid JSON-RPC initialize response and exits cleanly (or waits for next message).

> **Write T007 and T008 first. Ensure they fail before writing T009 and T010.**

- [ ] T007 [P] [US1] Write `internal/mcp/server_test.go` with a test verifying that `Serve()` propagates an error when `extractFn` returns an error immediately (use a mock tokenCache / extractFn returning `errors.New("no creds")`); verify `Serve()` does not panic
- [ ] T008 [P] [US1] Write `cmd/mcp_test.go` with a test verifying the `mcp` subcommand and its `serve` child are registered on the root command (use `NewRootCmd().Find([]string{"mcp","serve"})`)
- [ ] T009 [US1] Implement `internal/mcp/server.go` — export `Serve(extractFn func() (tokens.TokenExtractionResult, error)) error`: create `tokenCache`, create `server.NewMCPServer("slackseek", version, server.WithToolCapabilities(true))`, register zero tools initially (will be filled in later phases), call `server.ServeStdio(s)` and return any error; keep function ≤ 40 lines
- [ ] T010 [US1] Implement `cmd/mcp.go` — Cobra `mcp` parent command and `serve` child command; `serve.RunE` calls `mcp.Serve(tokens.DefaultExtract)` and returns the error; register via `init()` on `rootCmd`; add a descriptive `Short` and `Long` for both commands

**Checkpoint**: `go build ./...` succeeds. `go test -race ./...` passes. US1 complete — server starts over stdio.

---

## Phase 4: User Story 2 — Core Retrieval Tools (Priority: P2)

**Goal**: An MCP client connected to `slackseek mcp serve` can call `slack_search`, `slack_history`, `slack_messages`, and `slack_thread` and receive JSON results.

**Independent Test**: Use `slack_search` with `query="hello"` in an MCP client (or via raw JSON-RPC) and receive a JSON array of search results. Each of the four tools returns a non-empty JSON array or an actionable MCP error.

> **Write T011 first. Ensure it fails before writing T012–T015.**

- [ ] T011 [US2] Write handler tests in `internal/mcp/tools_test.go` for: `TestHandleSlackSearch` (mock returns results → verify JSON marshaled in TextContent); `TestHandleSlackHistory` (mock returns messages → verify JSON); `TestHandleSlackMessages` (mock returns messages → verify JSON); `TestHandleSlackThread` (mock returns messages → verify JSON); also test error paths: user not found, channel not found, auth error (IsError=true, actionable message)
- [ ] T012 [P] [US2] Implement `handleSlackSearch()` in `internal/mcp/tools.go` — parse `query` (required), `channels` (array), `user`, `since`, `until`, `limit`, `workspace` params; call `selectWorkspace`, `buildMCPClient`, resolve userID if `user` non-empty, call `SearchMessages`, marshal to JSON, return `TextContent`; wrap errors per contracts/mcp-tools.md
- [ ] T013 [P] [US2] Implement `handleSlackHistory()` in `internal/mcp/tools.go` — parse `channel` (required), `since`, `until`, `limit`, `threads`, `workspace`; call `ResolveChannel`, `FetchHistory`; marshal messages to JSON; wrap "channel not found" with hint to use `slack_channels`
- [ ] T014 [P] [US2] Implement `handleSlackMessages()` in `internal/mcp/tools.go` — parse `user` (required), `since`, `until`, `limit`, `workspace`; call `ResolveUser`, `GetUserMessages`; marshal to JSON; wrap "user not found" with hint to use `slack_users`
- [ ] T015 [P] [US2] Implement `handleSlackThread()` in `internal/mcp/tools.go` — parse `url` (required), `workspace`; call `slack.ParsePermalink(url)` to extract channelID + threadTS; call `FetchThread`; marshal to JSON; wrap invalid permalink with actionable error
- [ ] T016 [US2] Register `slack_search`, `slack_history`, `slack_messages`, `slack_thread` in `internal/mcp/server.go` using `s.AddTool(mcp.NewTool(...), handler)` — define parameter schemas matching `contracts/mcp-tools.md` (required fields, optional fields, descriptions)

**Checkpoint**: `go test -race ./...` passes. Four retrieval tools are functional end-to-end. US2 complete.

---

## Phase 5: User Story 3 — Entity Listing Tools (Priority: P3)

**Goal**: An MCP client can call `slack_channels` and `slack_users` to explore workspace structure and discover channel/user names for use in other tool calls.

**Independent Test**: Call `slack_channels` — receive a JSON array of channel objects with `id`, `name`, `type` fields. Call `slack_users` — receive a JSON array of user objects with `id`, `displayName`, `realName` fields.

> **Write T017 first. Ensure it fails before writing T018–T019.**

- [ ] T017 [US3] Write handler tests in `internal/mcp/tools_test.go` for: `TestHandleSlackChannels` (mock returns channels → verify JSON array, verify `filter` substring match applied, verify `include_archived` forwarded); `TestHandleSlackUsers` (mock returns users → verify JSON array, verify `filter` applied to displayName/realName/email)
- [ ] T018 [P] [US3] Implement `handleSlackChannels()` in `internal/mcp/tools.go` — parse `filter`, `include_archived`, `workspace`; call `ListChannels(ctx, nil, includeArchived)`; apply case-insensitive substring filter on `name` if `filter` non-empty; marshal to JSON
- [ ] T019 [P] [US3] Implement `handleSlackUsers()` in `internal/mcp/tools.go` — parse `filter`, `workspace`; call `ListUsers`; apply case-insensitive substring filter on `DisplayName + RealName + Email` if `filter` non-empty; marshal to JSON
- [ ] T020 [US3] Register `slack_channels` and `slack_users` in `internal/mcp/server.go` with parameter schemas matching `contracts/mcp-tools.md`

**Checkpoint**: `go test -race ./...` passes. Listing tools functional. US3 complete.

---

## Phase 6: User Story 4 — Analysis Tools (Priority: P4)

**Goal**: An MCP client can call `slack_digest`, `slack_postmortem`, `slack_metrics`, and `slack_actions` to get structured analytical summaries of Slack channel activity.

**Independent Test**: Call `slack_digest` with `user` + `since="7d"` — receive a JSON array of `ChannelDigest` objects. Call `slack_postmortem`, `slack_metrics`, `slack_actions` with a channel + date range — receive structured JSON matching the schemas in `contracts/mcp-tools.md`.

> **Write T021 first. Ensure it fails before writing T022–T025.**

- [ ] T021 [US4] Write handler tests in `internal/mcp/tools_test.go` for: `TestHandleSlackDigest` (mock user messages grouped by channel → verify ChannelDigest JSON); `TestHandleSlackPostmortem` (mock history → verify IncidentDoc JSON); `TestHandleSlackMetrics` (mock history → verify ChannelMetrics JSON); `TestHandleSlackActions` (mock history → verify ActionItem array JSON); test missing required params return IsError=true
- [ ] T022 [P] [US4] Implement `handleSlackDigest()` in `internal/mcp/tools.go` — parse `user` (required), `since`, `until`, `workspace`; resolve user, call `GetUserMessages` with empty channelID to get all channels; group messages by channelID into `[]output.ChannelDigest`; marshal to JSON
- [ ] T023 [P] [US4] Implement `handleSlackPostmortem()` in `internal/mcp/tools.go` — parse `channel` (required), `since`, `until`, `workspace`; resolve channel, fetch history with threads=true; build `output.IncidentDoc` (reuse `output.BuildIncidentDoc` if it exists, else reproduce logic inline); marshal to JSON
- [ ] T024 [P] [US4] Implement `handleSlackMetrics()` in `internal/mcp/tools.go` — parse `channel` (required), `since`, `until`, `workspace`; resolve channel, fetch history; build `output.ChannelMetrics` (reuse builder if exported, else reproduce logic inline); marshal to JSON
- [ ] T025 [P] [US4] Implement `handleSlackActions()` in `internal/mcp/tools.go` — parse `channel` (required), `since`, `until`, `workspace`; resolve channel, fetch history; extract `[]output.ActionItem` (reuse `output.ExtractActions` if exported, else reproduce logic inline); marshal to JSON
- [ ] T026 [US4] Register `slack_digest`, `slack_postmortem`, `slack_metrics`, `slack_actions` in `internal/mcp/server.go` with parameter schemas matching `contracts/mcp-tools.md`

**Checkpoint**: `go test -race ./...` passes. All 10 MCP tools functional. US4 complete.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Verify quality gates, fix any remaining issues, confirm cross-platform builds.

- [ ] T027 Run `go test -race ./...` from repo root — fix any test failures or race conditions before proceeding
- [ ] T028 Run `golangci-lint run` — fix all lint issues (unused vars, missing error wrapping, function length violations, etc.)
- [ ] T029 [P] `GOOS=linux go build ./...` — confirm Linux cross-compile succeeds
- [ ] T030 [P] `GOOS=darwin go build ./...` — confirm macOS cross-compile succeeds
- [ ] T031 Validate quickstart.md scenarios manually: start `slackseek mcp serve`, connect MCP client, verify initialize handshake, call `slack_channels` + `slack_users`, confirm JSON output shape matches `contracts/mcp-tools.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (needs `mcp-go` module available)
- **US1 (Phase 3)**: Depends on Phase 2 (needs tokenCache, helpers)
- **US2 (Phase 4)**: Depends on Phase 3 (server.go must exist to add tool registrations)
- **US3 (Phase 5)**: Depends on Phase 3; can run in parallel with US2 if desired
- **US4 (Phase 6)**: Depends on Phase 3; can run in parallel with US2 and US3 if desired
- **Polish (Phase 7)**: Depends on all desired user stories complete

### User Story Dependencies

- **US1 (Phase 3)**: Requires Foundational complete. No dependency on US2/US3/US4.
- **US2 (Phase 4)**: Requires US1 complete (server.go exists). No dependency on US3/US4.
- **US3 (Phase 5)**: Requires US1 complete. Independent of US2/US4.
- **US4 (Phase 6)**: Requires US1 complete. Independent of US2/US3.

### Within Each User Story

1. Write failing tests FIRST (Constitution Principle II — NON-NEGOTIABLE)
2. Implement to make tests pass
3. Register tools in server.go last (depends on handler implementations)
4. Commit after each logical group

### Parallel Opportunities

- T003 (write tokencache tests) and T005 (define interface) are independent — run in parallel
- T007 (server test) and T008 (cmd test) — write in parallel (different files)
- Within US2: T012, T013, T014, T015 — handlers are independent files/functions, write in parallel
- Within US3: T018, T019 — parallel
- Within US4: T022, T023, T024, T025 — parallel
- T029, T030 (cross-platform builds) — parallel

---

## Parallel Example: Phase 4 (US2 Handlers)

```bash
# After T011 (tests written), launch all four handlers in parallel:
Task: "Implement handleSlackSearch() in internal/mcp/tools.go"     # T012
Task: "Implement handleSlackHistory() in internal/mcp/tools.go"    # T013
Task: "Implement handleSlackMessages() in internal/mcp/tools.go"   # T014
Task: "Implement handleSlackThread() in internal/mcp/tools.go"     # T015
# Then register all four: T016
```

---

## Implementation Strategy

### MVP First (US1 + US2 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: US1 — server starts, CLI command works
4. Complete Phase 4: US2 — search and history tools work
5. **STOP and VALIDATE**: Four tools functional, connect Claude Code, test live
6. Continue with US3 and US4 if validation passes

### Incremental Delivery

1. Phase 1 + 2 → Foundation ready
2. Phase 3 (US1) → `slackseek mcp serve` starts
3. Phase 4 (US2) → Core retrieval tools → **MVP deliverable**
4. Phase 5 (US3) → Listing tools → Workspace exploration
5. Phase 6 (US4) → Analysis tools → Full feature complete
6. Phase 7 → Quality gates pass → Ready to merge

### Parallel Team Strategy

After Phase 2 (Foundational) completes and Phase 3 (US1) completes:
- Developer A: US2 (retrieval tools)
- Developer B: US3 (listing tools)
- Developer C: US4 (analysis tools)

---

## Notes

- [P] tasks = different files, no incomplete dependencies — safe to parallelize
- [Story] label maps each task to a specific user story for traceability
- Constitution Principle II is NON-NEGOTIABLE: write tests before implementation, confirm they fail first
- `internal/mcp/tools.go` grows incrementally across phases — each phase adds handlers; avoid conflicts by working sequentially per phase unless using parallel branches
- The `slackClient` interface (T005) must not require any changes to `internal/slack` — verify by compiling
- Handlers must NOT write to stderr (would corrupt the JSON-RPC stdio stream)
- All functions must remain ≤ 40 lines (Constitution Principle I) — split helpers if needed
- For analysis tools (T022–T025): check if `output.Build*` / `output.Extract*` functions are exported before reproducing logic; prefer reuse
