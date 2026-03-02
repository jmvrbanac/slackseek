<!--
SYNC IMPACT REPORT
==================
Version change: (none) → 1.0.0  (initial ratification — no prior version exists)

Added sections:
  - Core Principles (I–V)
  - Code Quality Standards
  - Development Workflow
  - Governance

Modified principles: N/A (initial creation)
Removed sections: N/A (initial creation)

Templates reviewed and alignment status:
  ✅ .specify/templates/plan-template.md
     — "Constitution Check" gate is present and generic; no updates required.
     — Recommend populating the gate with the 5 principles below when running
       /speckit.plan for this project.
  ✅ .specify/templates/spec-template.md
     — Mandatory "User Scenarios & Testing" and "Requirements" sections align
       with Principle II (Test-First) and Principle III (Single-Responsibility).
     — No structural changes required.
  ✅ .specify/templates/tasks-template.md
     — Test tasks are marked OPTIONAL in the template; per Principle II they
       are MANDATORY for this project. Annotate in plan/tasks output accordingly.
     — No template edits required; enforcement is at generation time.
  ✅ .specify/templates/agent-file-template.md
     — No constitution-specific references; no changes required.

Deferred TODOs: None — all fields resolved.
-->

# slackseek Constitution

## Core Principles

### I. Clarity Over Cleverness

All Go code in this repository MUST be written for the reader, not the writer.
Idiomatic Go patterns are preferred; non-obvious constructs MUST include an
explanatory comment. Functions MUST remain short enough to be understood without
scrolling (target ≤ 40 lines of logic; exceed only with documented justification).
Variable and function names MUST be descriptive — single-letter names are
permitted only for loop indices and well-established Go conventions (e.g., `err`,
`ok`, `n`).

**Rationale**: A CLI utility will outlive its initial author. Future contributors
MUST be able to understand, modify, and safely extend any function without
archaeology.

### II. Test-First (NON-NEGOTIABLE)

Every exported function, every CLI command, and every error path MUST have a
corresponding test before that code is merged. The Red-Green-Refactor cycle is
the default workflow:

1. Write a failing test that captures the requirement.
2. Implement the minimum code to make it pass.
3. Refactor without breaking the test.

Unit tests belong in `*_test.go` files adjacent to the code under test.
Integration tests that require OS-level resources (LevelDB, SQLite, keyring)
MUST use build tags or `t.Skip` guards so the suite can run in CI without
those dependencies present. The race detector (`go test -race`) MUST pass on
all test runs.

**Rationale**: Tests are the primary documentation for intent. Untested code is
a maintenance liability that compounds over time.

### III. Single-Responsibility Packages

Each sub-package under `internal/` MUST have exactly one clearly stated purpose,
documented in a `doc.go` or package comment. Cross-package dependencies flow
downward only (e.g., `cmd/` imports `internal/`; `internal/` packages MUST NOT
import `cmd/`). Shared helpers that do not belong to a single domain belong in
a dedicated `internal/util` or `internal/testutil` package, not scattered across
feature packages.

**Rationale**: Cohesive packages are easier to test in isolation, easier to
reason about, and easier to replace or extend independently.

### IV. Actionable Error Handling

Every error surfaced to the end user MUST include:

- What failed (e.g., "failed to open LevelDB copy").
- Why it likely failed (e.g., "Slack may be holding a lock on the database").
- What the user can do (e.g., "Close Slack and retry, or pass --workspace to
  select a different workspace").

Internal errors MUST be wrapped with `fmt.Errorf("…: %w", err)` at each layer
boundary so callers can inspect the error chain. Panics are not permitted in
production code paths; use `errors` and explicit returns.

**Rationale**: A CLI tool is only as good as its error messages. Cryptic errors
block users and generate avoidable support burden.

### V. Platform Isolation via Build Tags

All platform-specific code (keyring access, file paths, OS-level system calls)
MUST reside in files with a `//go:build` constraint (e.g., `_linux.go`,
`_darwin.go`). The platform-agnostic core MUST compile and its logic MUST be
testable without requiring a specific OS. New platforms MUST add build-tagged
files without touching cross-platform code.

**Rationale**: The project targets Linux and macOS. Keeping platform divergence
explicit and contained prevents accidental breakage when adding a new platform
or updating shared logic.

## Code Quality Standards

The following tooling checks are MANDATORY before any merge:

- `go vet ./...` — MUST report zero issues.
- `golangci-lint run` — MUST pass with the project-level `.golangci.yml`
  configuration (or equivalent linter config at repository root).
- `go test -race ./...` — MUST pass with zero failures and zero race conditions.
- `go build ./...` — MUST succeed on both `GOOS=linux` and `GOOS=darwin` targets.

Dependencies MUST be managed via `go.mod`/`go.sum`. Indirect dependencies MUST
NOT be pinned manually unless there is a documented security or compatibility
reason.

## Development Workflow

1. **Branch per feature/fix** — All work happens on a dedicated branch; direct
   commits to `main` are not permitted.
2. **Tests before implementation** — See Principle II. PRs that add production
   code without accompanying tests will not be merged.
3. **PR checklist** (enforced at review):
   - [ ] `go vet` and `golangci-lint` pass.
   - [ ] `go test -race ./...` passes.
   - [ ] New exported symbols have package-level or function-level documentation.
   - [ ] Error messages follow Principle IV (actionable, contextual).
   - [ ] Platform-specific code is build-tagged (Principle V).
4. **Review requirement** — At least one other contributor MUST approve before
   merge. Self-approval is not permitted for code changes.
5. **Constitution compliance** — Every PR reviewer MUST verify that the changeset
   does not violate any principle above. Complexity violations MUST be documented
   in the plan's "Complexity Tracking" table before implementation.

## Governance

This constitution supersedes all other development practices documented in this
repository. When a conflict exists between this document and any other guideline,
the constitution takes precedence unless a formal amendment is made.

**Amendment procedure**:

1. Open a PR that edits `.specify/memory/constitution.md`.
2. State the motivation and the affected principles in the PR description.
3. Increment the version according to the semantic versioning policy below.
4. Obtain approval from at least one other contributor.
5. Merge and propagate changes to any affected templates or guidance files.

**Versioning policy**:

- **MAJOR**: Removal or fundamental redefinition of a principle.
- **MINOR**: New principle or section added; material expansion of existing guidance.
- **PATCH**: Clarifications, wording improvements, typo corrections.

**Compliance review**: Principle compliance is checked at PR review time (see
Development Workflow above). There is no separate scheduled audit; continuous
review is the mechanism.

**Version**: 1.0.0 | **Ratified**: 2026-03-02 | **Last Amended**: 2026-03-02
