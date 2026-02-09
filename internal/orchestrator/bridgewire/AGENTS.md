# bridgewire — Agent Guidelines

> **Living document.** Update this file when you learn something specific to this package.
> Same rules as the root `AGENTS.md` — see its Self-Improvement Protocol.

See `doc.go` for package overview and API usage.

## Architecture

The bridgewire package provides adapter types that connect the orchestrator's concrete infrastructure (worktrees, tmux, verification) to the bridge package's narrow interfaces. It exists specifically to break the import cycle: `bridge` cannot import `orchestrator` (which would create `bridge → team → ... → orchestrator → bridge`), so this sub-package inside `orchestrator/` imports both and bridges the gap.

**Key Types:**
- `PipelineExecutor` — Subscribes to `pipeline.phase_changed` events; when the execution phase starts, creates a Bridge per execution-role team. Accepts `bridge.InstanceFactory` and `bridge.CompletionChecker` via dependency injection for testability.
- `NewPipelineExecutorFromOrch` — Production convenience constructor that wraps `NewInstanceFactory`/`NewCompletionChecker` adapter creation. Tests should use `NewPipelineExecutor` directly with mock factory/checker.
- `instanceFactory` — Adapts `*orchestrator.Orchestrator` to `bridge.InstanceFactory`
- `completionChecker` — Adapts `orchestrator.Verifier` to `bridge.CompletionChecker`
- `sessionRecorder` — Callback-based `bridge.SessionRecorder` using `SessionRecorderDeps`

**Data Flow:**
```
Pipeline fires PipelinePhaseChangedEvent
  → PipelineExecutor.attachBridges() (dispatched via goroutine)
    → polls for teams to reach PhaseWorking (event fires before Manager.Start)
    → creates Bridge per execution team using injected factory/checker
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

## Testing

- Adapter tests (`adapters_test.go`) verify interface satisfaction and basic wiring with mock orchestrator types.
- `PipelineExecutor` tests (`executor_test.go`) cover Start/Stop lifecycle, config validation, and 5 E2E integration tests.
- **E2E tests use `autoCompleteFactory`** — The factory triggers `checker.MarkComplete()` inside `StartInstance`, simulating instant-completion Claude Code instances. This lets Bridge's claim loop → monitor → completion → gate.Complete flow run without real infrastructure.
- Use `bridge.WithPollInterval(10*time.Millisecond)` via `BridgeOpts` for fast E2E tests.
- Use `coordination.WithRebalanceInterval(-1)` on the pipeline's hub options to disable rebalance interference.
- Always run with `-race` — the executor has concurrent event handlers and bridge goroutines.
