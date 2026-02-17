# Claudio Development Guidelines

> **This is a living document.** Agents working on this codebase are expected to improve it.
> See [Self-Improvement Protocol](#self-improvement-protocol) for details.

## Self-Improvement Protocol

### Mandate

When you complete a task on this codebase, review this file (and any relevant directory-level `AGENTS.md`) and make meaningful updates based on what you learned. This is not optional — it is part of completing your work. Updates should be included in your commit alongside the code changes.

### What Constitutes a Meaningful Update

**DO update when you discover:**
- A codebase pattern or convention not already documented here (add to [Codebase Patterns](#codebase-patterns))
- A pitfall, gotcha, or subtle bug you hit or nearly hit (add to [Known Pitfalls](#known-pitfalls))
- A new package, significant type, or architectural component (update [Architecture Map](#architecture-map))
- A testing trick or technique specific to this codebase (add to [Testing Notes](#testing-notes))
- A correction to something already written here that turned out to be wrong (fix it directly)
- A dependency quirk or build/toolchain issue (add to [Build & Toolchain](#build--toolchain))

**DO NOT update for:**
- Generic Go knowledge that any Go developer would know
- Restating what's already documented in this file
- Trivial observations that won't help future agents
- Speculative advice not grounded in actual experience on this codebase

### How to Update

1. Add entries to the appropriate section below
2. Keep entries concise — one to two sentences, with a code reference where relevant
3. If a section grows beyond ~15 entries, reorganize or split it
4. If you discover something here is wrong, fix or remove it — don't leave stale knowledge
5. Preserve the structure and heading hierarchy of this document

### Directory-Scoped Guidelines

When your knowledge is specific to a single package or directory, put it in that directory's `AGENTS.md` instead of here. If one already exists, update it. If not, create it. These are living documents just like this root file — the self-improvement mandate applies to all of them.

**When creating a new directory-level `AGENTS.md`:**
1. Create `AGENTS.md` in the target directory with package-specific guidance
2. Create a `CLAUDE.md` symlink pointing to it: `ln -s AGENTS.md CLAUDE.md`
3. Do not duplicate root-level guidelines — directory files extend, not replace, the root
4. Follow the same quality bar and entry format as this file

**When updating an existing directory-level `AGENTS.md`:**
- Apply the same standards as updating this root file — fix stale info, add new pitfalls, update patterns
- If you worked inside a package and learned something, check its `AGENTS.md` before you commit

**When to use a directory file vs. adding here:**
- Knowledge that only matters when working *inside* that package → directory-level `AGENTS.md`
- Knowledge that affects how other packages *interact with* that package → root `AGENTS.md`
- Architectural patterns that span multiple packages → root `AGENTS.md`

### Quality Bar

Every entry should pass this test: *"Would this save a future agent at least 5 minutes of confusion or debugging?"* If not, it's not worth adding.

---

## Go Best Practices

### Code Formatting

All Go code must be properly formatted before committing:

```bash
# Format all Go files
gofmt -w .

# Or use goimports to also organize imports
goimports -w .
```

- Run `gofmt -d .` to check for formatting issues without modifying files
- Imports should be organized in groups: standard library, external packages, internal packages

### Linting

Run static analysis before committing:

```bash
# Basic linting (always run)
go vet ./...

# If golangci-lint is available (recommended)
golangci-lint run
```

Address all linting warnings before committing code.

## Architecture & Design Principles

### Single Responsibility

Each package, type, and function should have one clear purpose:

- **Packages** - A package should represent a single concept (e.g., `config`, `worktree`, `tui`)
- **Types** - A struct should model one thing; avoid "god objects" that do everything
- **Functions** - A function should do one thing well; if it needs "and" in its description, consider splitting it

### Separation of Concerns

Keep different layers distinct:

- **Domain logic** should not depend on I/O or presentation
- **I/O operations** (file, network, process) should be isolated behind interfaces
- **TUI/CLI code** should be thin wrappers that delegate to business logic

### Modular Design

Prefer small, focused packages over large monolithic ones:

- Extract reusable logic into dedicated packages under `internal/`
- Use interfaces to define boundaries between packages
- Avoid circular dependencies—if package A imports B, B should not import A

### Dependency Injection

Design for testability by accepting dependencies rather than creating them:

```go
// Prefer: accepts dependencies
func NewManager(logger Logger, store Store) *Manager

// Avoid: creates its own dependencies
func NewManager() *Manager {
    logger := log.New(...)
    store := NewFileStore(...)
}
```

This makes code easier to test with mocks and more flexible to configure.

### Interface Design

Follow Go idioms for interfaces:

- Define interfaces where they're used, not where they're implemented
- Keep interfaces small—one or two methods is often ideal
- Accept interfaces, return concrete types

### Building

```bash
# Build the project
go build ./...

# Ensure the build succeeds before committing
```

## Testing Requirements

### Coverage Expectations

- **All new code must have corresponding tests**
- **Target: 100% test coverage on new code**
- Tests should live alongside the code they test (e.g., `foo.go` and `foo_test.go` in the same package)

If 100% coverage isn't achievable, document why in a code comment. Acceptable exceptions:
- `main()` functions and CLI entrypoints
- Defensive error handling that's unreachable in practice
- Platform-specific code paths that can't run in the test environment
- Code that requires external services that can't be reasonably mocked

For these cases, add a comment like:
```go
// Coverage: This branch handles [scenario] which requires [external dependency/condition]
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage report
go test -cover ./...

# Run tests with detailed coverage output
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out  # View in browser

# Run tests for a specific package
go test ./internal/config/...

# Run tests with verbose output
go test -v ./...
```

### Test Patterns

This project uses standard Go testing conventions:

1. **Table-driven tests** - Preferred for testing multiple cases:
   ```go
   func TestFoo(t *testing.T) {
       tests := []struct {
           name     string
           input    string
           expected string
       }{
           {"empty input", "", ""},
           {"normal input", "hello", "HELLO"},
       }
       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               got := Foo(tt.input)
               if got != tt.expected {
                   t.Errorf("Foo(%q) = %q, want %q", tt.input, got, tt.expected)
               }
           })
       }
   }
   ```

2. **Subtests** - Use `t.Run()` for grouping related test cases

3. **Error messages** - Use descriptive error messages that show got vs want:
   ```go
   t.Errorf("FunctionName() = %v, want %v", got, want)
   ```

4. **Test helpers** - Mark helper functions with `t.Helper()` for better error reporting

### What to Test

- Public functions and methods
- Edge cases and error conditions
- Concurrent behavior where applicable
- Integration points between packages

## Changelog

This project maintains a [CHANGELOG.md](CHANGELOG.md) following the [Keep a Changelog](https://keepachangelog.com/) format. The changelog has an **Unreleased** section at the top where changes accumulate until the next release.

### MANDATORY: Every Pull Request MUST Include a Changelog Entry

**NO EXCEPTIONS.** Every pull request must add an entry to the `## [Unreleased]` section of CHANGELOG.md. This requirement is absolute and applies to all changes, regardless of size or type.

Use the appropriate category for your change:
- **New features** → `### Added`
- **Bug fixes** → `### Fixed`
- **Performance improvements** → `### Performance`
- **Breaking changes** → `### Changed` or `### Removed`
- **Deprecations** → `### Deprecated`
- **Internal refactors** → `### Changed`
- **Test improvements** → `### Changed`
- **Documentation updates** → `### Changed`
- **Dependency updates** → `### Changed`

If you're unsure which category to use, use `### Changed`.

### Entry Format

Each entry should be a single bullet point with:
1. **Bold feature name** - Brief description
2. PR number in parentheses if available

Example:
```markdown
### Added
- **Task Chaining** - Chain tasks together in normal Claudio mode (#228)

### Fixed
- **Git Subdirectory Detection** - Correctly detect git repository from subdirectories (#142)
```

### At Release Time

When cutting a release:
1. Rename `## [Unreleased]` to `## [X.Y.Z] - YYYY-MM-DD`
2. Add a new empty `## [Unreleased]` section at the top
3. Add the version link at the bottom of the file

## Pre-Commit Checklist

Before committing, ensure:

1. Code is formatted: `gofmt -d .` shows no output
2. Linting passes: `go vet ./...` has no errors
3. Build succeeds: `go build ./...`
4. All tests pass: `go test ./...`
5. New code has tests with reasonable coverage
6. **CHANGELOG.md has been updated (MANDATORY - NO EXCEPTIONS)**

---

## Architecture Map

### Package Overview

This is not exhaustive — update it when you add or discover undocumented packages. Packages with their own `AGENTS.md` are marked; check for one before working in any package.

- `cmd/claudio/` — Main entry point
- `internal/adaptive/` — Event-driven adaptive lead for dynamic task coordination *(has `AGENTS.md`)*
- `internal/approval/` — Per-task approval gates using decorator pattern *(has `AGENTS.md`)*
- `internal/config/` — Configuration loading and validation
- `internal/contextprop/` — Context propagation between instances *(has `AGENTS.md`)*
- `internal/debate/` — Structured peer debate protocol *(has `AGENTS.md`)*
- `internal/event/` — Event bus and all event type definitions
- `internal/coordination/` — Hub that wires all Orchestration 2.0 components for a session *(has `AGENTS.md`)*
- `internal/filelock/` — Advisory file lock registry for conflict prevention *(has `AGENTS.md`)*
- `internal/instance/` — Claude Code instance lifecycle management
- `internal/mailbox/` — JSONL file-based inter-instance messaging *(has `AGENTS.md`)*
- `internal/orchestrator/` — Session coordination, instance orchestration
- `internal/scaling/` — Queue-depth-based elastic scaling policies *(has `AGENTS.md`)*
- `internal/taskqueue/` — Dependency-aware task queue with persistence *(has `AGENTS.md`)*
- `internal/team/` — Multi-team orchestration with dependency ordering, budget tracking, and inter-team routing *(has `AGENTS.md`)*
- `internal/bridge/` — Connects team Hubs to real Claude Code instances (worktree + tmux) *(has `AGENTS.md`)*
- `internal/orchestrator/bridgewire/` — Adapter types that wire orchestrator infrastructure to bridge interfaces *(has `AGENTS.md`)*
- `internal/orchestrator/workflows/tripleshot/` — Triple-shot workflow: 3 parallel attempts + judge evaluation. Defines sentinel file types (`CompletionFile`, `Evaluation`, `AdversarialReviewFile`) with flexible JSON unmarshaling *(has `AGENTS.md`)*
- `internal/orchestrator/workflows/tripleshot/teamwire/` — Adapts TripleShot to Orchestration 2.0 teams via `TeamCoordinator` + bridge adapters *(has `AGENTS.md`)*
- `internal/pipeline/` — Plan decomposer and multi-phase team pipeline *(has `AGENTS.md`)*
- `internal/tui/` — Bubble Tea terminal UI components *(has `AGENTS.md`)*
- `internal/worktree/` — Git worktree creation and management

### Key Architectural Patterns

- **Event bus** (`internal/event/`) — Decoupled communication between components. All event types live in `types.go` and embed `baseEvent`. If you add a new event type, put it there.
- **EventQueue decorator** — `internal/taskqueue/` wraps `TaskQueue` with `EventQueue` to publish events without coupling core logic to the event bus. See `internal/taskqueue/AGENTS.md` for implementation details.
- **Approval Gate decorator** — `internal/approval/` wraps `EventQueue` to add approval checkpoints. This creates a decorator chain: `TaskQueue → EventQueue → Gate`. Each layer adds behavior without modifying the layer below.
- **Copy-on-return** — Accessor methods on shared types (e.g., `ClaimNext()`, `GetTask()`) return value copies, not pointers, to prevent data races. Maintain this pattern across packages.
- **Atomic persistence** — File-backed state uses crash-safe write patterns. See `internal/taskqueue/AGENTS.md` and `internal/mailbox/AGENTS.md` for package-specific details.
- **Functional options** — New coordination packages (`internal/adaptive/`, `internal/scaling/`, `internal/filelock/`) use the `WithXxx()` functional options pattern for configurable constructors. Follow this when adding new packages.
- **Bridge pattern** — `internal/bridge/` connects abstract team queues to concrete instance infrastructure via narrow interfaces (`InstanceFactory`, `CompletionChecker`, `SessionRecorder`). Adapters in `internal/orchestrator/bridgewire/` implement these. The bridge must not import `orchestrator` (cycle); keep its API using simple types.

---

## Known Pitfalls

These are real issues agents have encountered in this codebase. Package-specific pitfalls live in directory-level `AGENTS.md` files (see `internal/mailbox/`, `internal/taskqueue/`, `internal/tui/`).

- **Map iteration ordering** — Go map iteration is non-deterministic. When output must be stable (tests, serialization, UI), sort the keys first.
- **Mutex scope during marshaling** — `json.MarshalIndent` on a map must hold the mutex through the entire marshal, not just while copying the map. The marshal reads the map's values lazily.
- **Synchronous event bus and lock re-entrancy** — `event.Bus.Publish` runs handlers inline in the caller's goroutine. If a handler acquires a lock that the publisher already holds, deadlock results. The `team.Manager` uses a two-phase pattern (register under lock, start outside lock) specifically to avoid this. See `internal/team/AGENTS.md` for details.
- **Release locks before blocking on wg.Wait()** — When a `Stop()` method holds a mutex through `wg.Wait()`, goroutines tracked by the WaitGroup that publish events (triggering inline handlers that acquire the same mutex) will deadlock. The fix: do all cleanup under the lock, set a `started = false` guard, release the lock, then call `wg.Wait()`. See `Manager.Stop()` and `Pipeline.Stop()` for this pattern; `Bridge.Stop()` uses a variant where `started` is set after `wg.Wait()` (safe because it has no event-handler race).
- **Store state before publishing events** — When an event handler may look up the state you're about to set, store it first. Example: `pipeline.runPhase` stores the Manager in `p.managers[phase]` before publishing `PipelinePhaseChangedEvent`, so handlers calling `p.Manager(phase)` don't get nil.
- **Dispatch from event handlers to goroutines** — When subscribing to events, if the handler acquires a lock or does blocking work, dispatch to a goroutine (`go pe.attachBridges()`) instead of calling inline. `event.Bus.Publish` runs handlers synchronously, so inline handlers that block or acquire locks risk deadlock with the publisher. See `internal/orchestrator/bridgewire/executor.go` for an example.
- **Release locks before blocking on Stop()** — When stopping a component that holds a mutex, copy shared state (e.g., a slice of bridges) under the lock, release the lock, then perform blocking cleanup. Holding a lock while calling `bridge.Stop()` (which calls `wg.Wait()`) blocks goroutines that need the same lock. See `PipelineExecutor.Stop()` in `bridgewire/executor.go`.
- **Two-phase event publishing for cascading state changes** — When an event handler (`onTeamCompleted`) modifies state that triggers further events of the same type, use a two-phase approach: (1) collect state changes under the lock, (2) publish events outside the lock. Repeat until no new transitions occur. Publishing `TeamCompletedEvent` from within the `onTeamCompleted` handler would re-enter the handler via the synchronous bus, deadlocking on `m.mu`. See `team.Manager.checkBlockedTeamsLocked`.
- **Semaphore slot lifecycle in bridge** — When the bridge acquires a semaphore slot before `ClaimNext`, it must release on every non-monitor path (claim error, nil task, create/start failure). The monitor goroutine takes ownership of the slot via `defer b.sem.Release()`. Missing a release on any early-return path causes a permanent slot leak that eventually deadlocks the claim loop.
- **Release vs Fail for scheduling conflicts** — When a task fails due to a scheduling conflict (file lock contention), use `gate.Release()` to return it to pending instead of `gate.Fail()`. `Fail` decrements the retry counter; with scaling enabled, multiple tasks competing for the same resource can exhaust all retries and permanently fail. `Release` puts the task back without consuming retries. Always pair Release with `waitForWake` to prevent hot retry loops.

---

## Codebase Patterns

Patterns and conventions observed in this codebase that aren't covered by the general guidelines above:

- **Error wrapping** — Use `fmt.Errorf("context: %w", err)` consistently. The context should describe what operation failed, not repeat the inner error.
- **Constructor naming** — `NewXxx` functions return `(*Xxx, error)` when initialization can fail, or `*Xxx` when it cannot. Don't return an interface from a constructor.
- **File organization** — Each package keeps types, logic, and tests in separate files when the package is non-trivial (e.g., `types.go`, `queue.go`, `queue_test.go`).
- **Decorator chain** — Orchestration 2.0 builds behavior via stacked decorators: `TaskQueue → EventQueue → Gate`. Higher-level packages (`team`, `pipeline`) compose these decorators through `coordination.Hub`. When adding new orchestration behavior, consider whether it fits as a new decorator layer rather than modifying existing ones.
- **One-Manager-per-phase** — The `pipeline` package creates a fresh `team.Manager` per pipeline phase (planning, execution, review, consolidation). This keeps event subscriptions, completion monitors, and budget tracking scoped to each phase and avoids cross-phase interference.
- **TUI-local state from events** — `view/pipeline_status.go` defines `PipelineState` and `TeamSnapshot` types built entirely from event data (no backend imports). This avoids import cycles and keeps TUI types decoupled from the orchestration stack. The `m.pipeline` field is lazy-initialized on first pipeline event, so non-pipeline sessions are unaffected.
- **Per-team hub options** — `team.Manager.buildAndRegisterTeamLocked` copies the shared `m.hubOpts`, then appends per-team overrides (`WithInitialInstances`, `WithMinInstances`, `WithMaxInstances`) from the team `Spec`. This ensures each team gets its own scaling policy tuned to its bounds rather than sharing one global configuration.
- **Dynamic semaphore for resizable concurrency** — The bridge uses a `sync.Cond`-based semaphore instead of a channel because channels cannot be resized after creation. `SetLimit` calls `Broadcast` to wake all blocked goroutines so they can re-evaluate against the new limit. The `0 = unlimited` convention preserves backward compatibility.

---

## Testing Notes

Testing patterns specific to this codebase, beyond the general testing guidelines above:

- **Race detector** — Always run `go test -race ./...` before committing concurrent code. The CI enforces this.
- **Temp directories for persistence tests** — Use `t.TempDir()` for tests that exercise file-based persistence (taskqueue, mailbox). This auto-cleans on test completion.
- **Channel-based event assertions** — For async event-driven tests, subscribe to the bus with a buffered channel and use `select` with a timeout. Never use `time.Sleep` for event synchronization. See `internal/team/manager_test.go` and `internal/pipeline/pipeline_test.go` for examples.
- **Disable rebalance loop in team/pipeline tests** — Use `coordination.WithRebalanceInterval(-1)` when constructing Managers in tests to prevent the adaptive lead's periodic rebalance from interfering with deterministic test behavior.

---

## Build & Toolchain

- **Go version** — Check `go.mod` for the required Go version. Don't assume latest.
- **golangci-lint** — Must pass with zero issues. If a linter rule seems wrong for a specific case, use a `//nolint:rulename` directive with a comment explaining why, not a blanket suppression.
