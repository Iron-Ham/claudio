# bridgewire — Agent Guidelines

> **Living document.** Update this file when you learn something specific to this package.
> Same rules as the root `AGENTS.md` — see its Self-Improvement Protocol.

See `doc.go` for package overview and API usage.

## Architecture

The bridgewire package provides adapter types that connect the orchestrator's concrete infrastructure (worktrees, tmux, verification) to the bridge package's narrow interfaces. It exists specifically to break the import cycle: `bridge` cannot import `orchestrator` (which would create `bridge → team → ... → orchestrator → bridge`), so this sub-package inside `orchestrator/` imports both and bridges the gap.

**Key Types:**
- `PipelineExecutor` — Subscribes to `pipeline.phase_changed` events; when the execution phase starts, creates a Bridge per execution-role team
- `instanceFactory` — Adapts `*orchestrator.Orchestrator` to `bridge.InstanceFactory`
- `completionChecker` — Adapts `orchestrator.Verifier` to `bridge.CompletionChecker`
- `sessionRecorder` — Callback-based `bridge.SessionRecorder` using `SessionRecorderDeps`

**Data Flow:**
```
Pipeline fires PipelinePhaseChangedEvent
  → PipelineExecutor.attachBridges() (dispatched via goroutine)
    → creates adapters (factory, checker)
    → creates Bridge per execution team
    → Bridge.Start(ctx)
```

## Pitfalls

- **Import direction is critical** — This package imports both `orchestrator` and `bridge`. The `bridge` package must NOT import this package or `orchestrator`. If you add new types here, ensure they don't leak orchestrator types into bridge's API.
- **Goroutine dispatch in event handler** — The `PipelineExecutor` dispatches `attachBridges` via `go` from the event handler. This is required because `event.Bus.Publish` runs handlers inline, and `attachBridges` acquires `pe.mu`. Calling it inline would deadlock if the publisher holds a conflicting lock.
- **Stop() releases lock before blocking** — `PipelineExecutor.Stop()` copies the bridge slice and releases `pe.mu` before calling `bridge.Stop()` on each bridge. Holding the lock through `Stop()` (which calls `wg.Wait()`) would deadlock goroutines that need `pe.mu`.
- **PipelineExecutor.started = false before wg.Wait** — Unlike `Bridge.Stop()` which sets `started=false` after `wg.Wait()`, `PipelineExecutor.Stop()` sets it before because the executor doesn't own the bridge goroutines — it only owns the event subscription. The bridges manage their own WaitGroups.
- **Nil-safe defaults** — `NewPipelineExecutor` defaults nil `Logger` to `NopLogger()` and nil `Recorder` to a no-op `SessionRecorder`. This matches the pattern in bridge's `New()` constructor.
- **Coverage exceptions** — `CreateInstance`, `StartInstance`, and `attachBridges` require real orchestrator infrastructure (worktrees, tmux) and are tested via integration tests. Each has a `// Coverage:` comment explaining this.

## Testing

- Adapter tests (`adapters_test.go`) verify interface satisfaction and basic wiring with mock orchestrator types.
- `PipelineExecutor` tests (`executor_test.go`) cover the Start/Stop lifecycle and config validation.
- `attachBridges` is not unit-tested (requires real Pipeline with active teams); this is documented with a `// Coverage:` comment.
- Always run with `-race` — the executor has concurrent event handlers.
