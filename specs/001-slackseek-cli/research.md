# Research: slackseek CLI

**Feature**: 001-slackseek-cli
**Date**: 2026-03-02
**Phase**: 0 — Outline & Research

---

## Decision: LevelDB library

- **Decision**: Use `github.com/syndtr/goleveldb` for reading the Slack Local
  Storage LevelDB database.
- **Rationale**: Last commit July 2022; v1.0.0 released 2019. Despite being
  effectively unmaintained it is the only production-grade pure-Go LevelDB
  reader. For a read-only operation against a format-stable database (LevelDB
  has not changed its on-disk format), the maintenance risk is low. No write
  operations are performed so corruption risk is zero.
- **Alternatives considered**:
  - `github.com/golang/leveldb` — Google's official Go port, explicitly
    marked "incomplete"; not suitable for production reads.
  - `github.com/kezhuw/leveldb` — pure Go, described as "early stage", not
    stable.
  - `github.com/jmhodges/levigo` — CGO bindings to C++ LevelDB; reintroduces
    the CGO complexity we want to avoid.
  - BadgerDB — actively maintained but uses its own format; cannot read
    existing LevelDB files.
- **Risk mitigation**: The LevelDB copy is made to a temp dir before opening
  (avoiding file-lock contention), so even a bug in goleveldb will not
  corrupt Slack's data.

---

## Decision: SQLite driver — modernc.org/sqlite (no CGO)

- **Decision**: Use `modernc.org/sqlite` (a transpiled-to-Go port of the
  canonical SQLite amalgamation) instead of `mattn/go-sqlite3`.
- **Rationale**: `mattn/go-sqlite3` requires `CGO_ENABLED=1`, which:
  - Requires a C compiler at build time (breaks `go build` on stock CI images).
  - Prevents cross-compilation (e.g., building a macOS binary from a Linux
    runner without a cross-compiler toolchain).
  - Prevents fully static single-binary output without extra linker flags.
  `modernc.org/sqlite` wraps SQLite 3.51.2 (as of v1.46, Feb 2026), requires
  no C toolchain, and produces a single statically linked binary on any host
  for any target. Performance difference (reads ~10–50 % slower) is irrelevant
  for a one-time cookie-file read at CLI startup. Grafana and gogs have both
  migrated to modernc specifically to remove CGO. The `modernc.org/sqlite`
  module requires `go 1.24` — this becomes the project's Go floor.
- **Alternatives considered**:
  - `mattn/go-sqlite3` — proven, battle-tested, but CGO cost is unacceptable
    for a binary-distribution CLI.
  - `glebarez/go-sqlite` — thin wrapper around modernc with mattn-compatible
    API; valid alternative but adds an unnecessary indirection layer.
  - `zombiezen/go-sqlite` — lower-level bindings over modernc; more boilerplate
    with no benefit for this use case.

---

## Decision: Go module version — go 1.24

- **Decision**: Declare `go 1.24` in `go.mod`.
- **Rationale**: The binding constraint is `modernc.org/sqlite`, which declares
  `go 1.24` in its own `go.mod`. Since Go 1.21 the `go` directive is enforced
  (older toolchains refuse to build). Other dependencies are satisfied by lower
  versions:

  | Dependency | Min Go |
  |---|---|
  | `github.com/spf13/cobra` | 1.15 |
  | `github.com/syndtr/goleveldb` | 1.14 |
  | `github.com/godbus/dbus/v5` | 1.20 |
  | `github.com/keybase/go-keychain` | 1.17 |
  | `modernc.org/sqlite` | **1.24** |
  | `github.com/slack-go/slack` | pin stable tag (see note) |

  **Note on `slack-go/slack`**: the master branch declares `go 1.25` (pre-release
  as of March 2026). Pin a stable tagged release (e.g., `v0.14.0`) which requires
  only Go 1.21, keeping the effective floor at 1.24.
- **Alternatives considered**:
  - Stay on Go 1.21/1.22 by using `mattn/go-sqlite3` — rejected (CGO decision
    above).
  - Vendor an older modernc tag — possible but loses SQLite security patches.

---

## Decision: Rate-limit handling — Retry-After header + backoff for 5xx

- **Decision**: On HTTP 429, read the `Retry-After` response header and sleep
  exactly that duration before retrying (up to 3 attempts). On transient 5xx
  errors, use exponential backoff with jitter. Use the `cenkalti/backoff/v4`
  library for the retry loop.
- **Rationale**: Slack's API returns a `Retry-After` header (in seconds) with
  every 429. Using generic exponential backoff for 429s would be incorrect —
  Slack tells callers exactly how long to wait. Relevant per-method tiers:

  | Method | Tier | Typical limit |
  |---|---|---|
  | `search.messages` | Tier 2 | 20+ req/min |
  | `conversations.history` | Tier 3 (internal/marketplace) | 50+ req/min |
  | `conversations.history` | Non-marketplace (post May 2025) | **1 req/min, 15 results/call** |
  | `conversations.list` | Tier 2 | 20+ req/min |
  | `users.list` | Tier 2 | 20+ req/min |

  **Critical finding**: For non-marketplace Slack apps, `conversations.history`
  is limited to 1 request/minute with a maximum of 15 messages per call (a
  restriction introduced in May 2025 for newer app installs). For a 30-day
  history fetch of 1,000 messages this would take over an hour. The tool should
  warn the user when paginating slowly due to rate limits and display progress.
  Locally extracted tokens (xoxs-*/xoxc-*) are user tokens, not bot tokens;
  they should be subject to user token rate-limit tiers which are typically more
  generous.
- **Alternatives considered**:
  - Pure exponential backoff for all errors — incorrect for 429 (ignores the
    Retry-After signal from Slack).
  - Manual sleep loops — harder to test and maintain than a backoff library.
  - `golang.org/x/time/rate` (token bucket) for proactive throttling — useful
    as a complement for batch operations but not sufficient on its own.

---

## Decision: Platform-specific testing strategy — interfaces + t.Skip

- **Decision**: Use interface injection for unit tests (mock `KeyringReader`,
  `PathResolver`); use `t.Skip()` with an `INTEGRATION=1` env guard for tests
  requiring real OS resources; use `//go:build` file-level constraints only
  for code that won't compile cross-platform.
- **Rationale**:
  - **Interface mocking** (idiomatic Go): Define `KeyringReader` and
    `CookiePathProvider` interfaces. Production code uses platform
    implementations; tests inject mocks. This gives fast, hermetic coverage
    of all business logic without any OS dependency.
  - **`t.Skip()`**: Integration tests that need a real D-Bus session or
    macOS Keychain are guarded with
    `if os.Getenv("INTEGRATION") == "" { t.Skip(...) }`.
    They run in CI only on platform-native runners (Linux runner for D-Bus,
    macOS runner for Keychain).
  - **`//go:build linux` / `//go:build darwin`** on `_test.go` files: Used
    only when the test source itself uses OS-specific types (e.g., D-Bus
    connection types) that won't compile on other platforms. This is a
    compilation guard, not a test-skip mechanism.
  - Old `// +build` syntax is deprecated since Go 1.17; use `//go:build`
    exclusively.
- **Alternatives considered**:
  - Build-tag-only approach: Silently skips tests on wrong platform without
    explanation — hard to diagnose CI failures.
  - Separate test binaries per platform — excessive complexity for what
    interface mocking handles cleanly.

---

## Decision: Slack API output — user token (xoxs-*/xoxc-*) required

- **Decision**: Acknowledge that `search.messages` requires a user token;
  document this in the error message when an API call fails authentication.
- **Rationale**: The `search.messages` and `conversations.history` endpoints
  accept user tokens (xoxs-*, xoxc-*) or workspace tokens. Bot tokens (xoxb-*)
  cannot call `search.messages`. The locally extracted token satisfies this
  requirement because it is a user session token. If extraction returns only
  a bot token (unexpected but possible in enterprise setups), the tool MUST
  surface an actionable error rather than a raw Slack API error string.
- **Alternatives considered**: None — the token type is determined by what
  Slack stores locally; no user choice is possible.
