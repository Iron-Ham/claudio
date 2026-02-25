# bridgewire — Agent Guidelines

> **Living document.** Update this file when you learn something specific to this package.
> Same rules as the root `AGENTS.md` — see its Self-Improvement Protocol.

See `doc.go` for package overview and API usage.

## Architecture

The bridgewire package provides adapter types that connect the orchestrator's concrete infrastructure (worktrees, tmux, verification) to the bridge package's narrow interfaces. It exists specifically to break the import cycle: `bridge` cannot import `orchestrator` (which would create `bridge → team → ... → orchestrator → bridge`), so this sub-package inside `orchestrator/` imports both and bridges the gap.

**Key Types:**
- `PipelineExecutor` — Subscribes to `pipeline.phase_changed` events; when the execution phase starts, creates a Bridge per execution-role team. Accepts `bridge.InstanceFactory` and `bridge.CompletionChecker` via dependency injection for testability. Supports per-role CLI flag overrides via `RoleOverrides` and `FactoryWithOverrides`.
- `NewPipelineExecutorFromOrch` — Production convenience constructor that wraps `NewInstanceFactory`/`NewCompletionChecker` adapter creation, and wires `FactoryWithOverrides` for per-role overrides. Tests should use `NewPipelineExecutor` directly with mock factory/checker.
- `instanceFactory` — Adapts `*orchestrator.Orchestrator` to `bridge.InstanceFactory`. Optionally carries `startOverrides ai.StartOptions` for role-specific CLI flags (use `NewInstanceFactoryWithOverrides` constructor).
- `subprocessFactory` — Alternative `bridge.InstanceFactory` that uses `streamjson.RunSubprocess` instead of tmux. `CreateInstance` delegates to the orchestrator (same worktree/branch creation). `StartInstance` writes a prompt file and launches `claude --print --output-format stream-json` in a goroutine. Enabled via `PipelineRunnerConfig.SubprocessMode`. Has its own `Stop()` method (called via type-assert from `PipelineExecutor.Stop()`) with `sync.WaitGroup`-based goroutine draining.
- `completionChecker` — Adapts `orchestrator.Verifier` to `bridge.CompletionChecker`
- `sessionRecorder` — Callback-based `bridge.SessionRecorder` using `SessionRecorderDeps`

**Data Flow:**
```
Pipeline fires PipelinePhaseChangedEvent
  → PipelineExecutor.attachBridges() (dispatched via goroutine)
    → polls for teams to reach PhaseWorking (event fires before Manager.Start)
    → for each execution team:
      → if RoleOverrides has entry for team's role AND FactoryWithOverrides is set:
        → creates per-team factory with role-specific StartOptions overrides
      → else: uses default shared factory
    → creates Bridge per execution team using factory + checker
    → Bridge.Start(ctx)
```

## Pitfalls

- **Import direction is critical** — This package imports both `orchestrator` and `bridge`. The `bridge` package must NOT import this package or `orchestrator`. If you add new types here, ensure they don't leak orchestrator types into bridge's API.
- **Goroutine dispatch in event handler** — The `PipelineExecutor` dispatches `attachBridges` via `go` from the event handler. This is required because `event.Bus.Publish` runs handlers inline, and `attachBridges` acquires `pe.mu`. Calling it inline would deadlock if the publisher holds a conflicting lock.
- **attachBridges must wait for PhaseWorking** — The pipeline publishes `pipeline.phase_changed` *before* calling `AddTeam` and `Manager.Start`. Without polling for teams to reach `PhaseWorking`, `attachBridges` may find an empty or unstarted Manager. This race is invisible with single-team tests but reliably surfaces with multiple teams. If the 5-second timeout expires without finding working teams, a WARN is logged — check for "timed out waiting for execution teams" in logs when debugging missing bridges.
- **Stop() releases lock before blocking** — `PipelineExecutor.Stop()` copies the bridge slice and releases `pe.mu` before calling `bridge.Stop()` on each bridge. Holding the lock through `Stop()` (which calls `wg.Wait()`) would deadlock goroutines that need `pe.mu`.
- **PipelineExecutor.started = false before wg.Wait** — Unlike `Bridge.Stop()` which sets `started=false` after `wg.Wait()`, `PipelineExecutor.Stop()` sets it before because the executor doesn't own the bridge goroutines — it only owns the event subscription. The bridges manage their own WaitGroups.
- **Nil-safe defaults** — `NewPipelineExecutor` defaults nil `Logger` to `NopLogger()` and nil `Recorder` to a no-op `SessionRecorder`. This matches the pattern in bridge's `New()` constructor.
- **Coverage exceptions** — `CreateInstance` and `StartInstance` in the adapter types require real orchestrator infrastructure (worktrees, tmux) and are tested via integration tests. Each has a `// Coverage:` comment explaining this.
- **subprocessFactory does NOT mutate orchestrator instance status** — Unlike the tmux path (which delegates to `orch.StartInstanceWithOverrides` for synchronized status updates), the subprocess path intentionally avoids direct `orchInst.Status` mutation. `GetInstance()` returns a shared pointer, and writing to it from the subprocess goroutine would be a data race. The bridge's `CompletionChecker` handles task lifecycle tracking independently.
- **PipelineExecutor.Stop() type-asserts factory for optional cleanup** — `bridge.InstanceFactory` has no `Stop()` method (the tmux factory doesn't need one). `PipelineExecutor.Stop()` uses `interface{ Stop() }` type-assert to call cleanup on factories that need it (i.e., `subprocessFactory`). If you add a new factory type with cleanup needs, implement a `Stop()` method and it will be called automatically.
- **subprocessFactory.Stop() follows the wg.Wait() lock pattern** — The `Stop()` method sets `stopped=true` and copies the cancel map under the lock, releases the lock, cancels all contexts, then calls `wg.Wait()`. This follows the project's documented "Release locks before blocking on wg.Wait()" pattern. The goroutine defers acquire `f.mu` to delete their map entries, so holding `f.mu` through `wg.Wait()` would deadlock.
- **Subprocess path needs explicit BackendDefaults** — The tmux path gets permission mode, model, max-turns, and tool restrictions from `ClaudeBackend` (constructed from config at init). The subprocess path bypasses `ClaudeBackend`, so these must be passed via `PipelineRunnerConfig.BackendDefaults`. The default factory uses `BackendDefaults` directly; `FactoryWithOverrides` merges per-role overrides on top via `mergeStartOptions`. Without this, subprocess instances launch with no `--dangerously-skip-permissions` flag, triggering Claude Code's interactive trust prompt.

## Testing

- Adapter tests (`adapters_test.go`) verify interface satisfaction and basic wiring with mock orchestrator types.
- `PipelineExecutor` tests (`executor_test.go`) cover Start/Stop lifecycle, config validation, and 10 E2E integration tests.
- **E2E tests use `autoCompleteFactory`** — The factory triggers `checker.MarkComplete()` inside `StartInstance`, simulating instant-completion Claude Code instances. This lets Bridge's claim loop → monitor → completion → gate.Complete flow run without real infrastructure.
- **`selectiveFactory` for per-task failure** — Delegates to auto-complete for most tasks but returns an error for prompts matching any key in `failFor`. Used in the partial failure test. The match is by `strings.Contains` on the prompt (which includes the task title via `BuildTaskPrompt`).
- **`newE2EPipeline` returns `*DecomposeResult`** — The 4th return value lets tests modify team specs (e.g., `DependsOn`, `Budget`) before `Start()`. Existing callers use `_` to ignore it.
- **Budget test uses `slowCompleteFactory`** — `autoCompleteFactory` completes tasks too fast for budget assertions. `slowCompleteFactory` keeps the task in-flight so `BudgetTracker().Record()` can be called while the team is still `PhaseWorking`.
- **Bridge does NOT call `recorder.RecordFailure` on `CreateInstance` errors** — The recorder is only called after instance creation succeeds and the monitor detects failure. When `CreateInstance` fails, the bridge calls `gate.Fail()` directly. Tests asserting on failure should check pipeline/team status, not recorder.
- Use `bridge.WithPollInterval(10*time.Millisecond)` via `BridgeOpts` for fast E2E tests.
- Use `coordination.WithRebalanceInterval(-1)` on the pipeline's hub options to disable rebalance interference.
- **Role override tests use recording factory builder** — The `FactoryWithOverrides` function in tests captures the `ai.StartOptions` passed to it, then delegates to `autoCompleteFactory`. This verifies the executor selects the right factory for each team's role without requiring real orchestrator infrastructure.
- Always run with `-race` — the executor has concurrent event handlers and bridge goroutines.
