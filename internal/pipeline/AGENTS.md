# pipeline — Agent Guidelines

> **Living document.** Update this file when you learn something specific to this package.
> Same rules as the root `AGENTS.md` — see its Self-Improvement Protocol.

See `doc.go` for package overview and API usage.

## Architecture

The pipeline package implements Phase 3 of the Orchestrator of Orchestrators. It decomposes a `PlanSpec` into teams and orchestrates multi-phase execution.

**Core Components:**
- **Decomposer** — Groups tasks by file affinity using union-find, producing `team.Spec` instances for the execution phase plus optional planning, review, and consolidation teams.
- **Pipeline** — Runs a multi-phase session (planning → execution → review → consolidation → done). Each phase creates its own `team.Manager`, registers teams, runs them to completion, and advances to the next phase.

**Phase Flow:**
```
Pipeline.Start(ctx)
  ├─ PlanningTeam != nil → run planning phase
  ├─ ExecutionTeams → run execution phase
  ├─ ReviewTeam != nil → run review phase
  ├─ ConsolidationTeam != nil → run consolidation phase
  └─ All succeed → PhaseDone
      Any fail → PhaseFailed
```

## Pitfalls

- **One Manager per phase, not one for the whole pipeline** — Each phase creates a fresh `team.Manager`. This keeps each phase's event subscriptions, budget tracking, and completion monitoring scoped and prevents cross-phase interference. Attempting to add teams from different phases into a single Manager would cause confusion in the completion monitor.
- **Manager's context, not caller's** — `AddTeamDynamic` on `team.Manager` uses the Manager's stored context (from `Start`), not the caller-provided context. Without this, `Stop` cannot cancel dynamically-added teams' monitor goroutines, causing `wg.Wait()` to hang.
- **Pipeline.Start returns immediately** — The `run` goroutine handles phase sequencing. The caller drives the pipeline through `Stop()`, context cancellation, or by completing tasks in each phase's teams.
- **Bus.Publish is synchronous** — Event handlers run in the caller's goroutine. This means `startTeamLocked` → monitor goroutine → `team.completed` → `onTeamCompleted` all chain through the same goroutine. The `AddTeamDynamic` split (register under lock, start outside lock) prevents deadlock with this chain.
- **completeAllTeamTasks test helper must poll** — Due to async phase transitions, the test helper can't just complete tasks once and return. It must poll until all teams reach terminal phases, because some teams may not be started yet (blocked on deps) when the helper first runs.
- **Store Manager in map BEFORE publishing phase events** — `runPhase` must call `p.managers[phase] = mgr` before publishing `PipelinePhaseChangedEvent`. Event handlers may call `p.Manager(phase)` and get nil if the order is wrong.
- **Pipeline.run() goroutine must be tracked with WaitGroup** — `Stop()` calls `p.wg.Wait()` after cancelling context to guarantee the `run()` goroutine has exited. Without this, tests checking post-Stop state may race with the goroutine.
- **fail() must receive phasesRun from caller** — The `fail()` helper publishes a `PipelineCompletedEvent`. It accepts a `phasesRun int` parameter rather than computing it, because the `run()` function already tracks this counter incrementally and passing it avoids redundant (and possibly wrong) recalculation.

## Testing

- Use `coordination.WithRebalanceInterval(-1)` on the pipeline's hub options to disable the adaptive lead's rebalance loop in tests.
- Use `t.TempDir()` for the pipeline's `BaseDir`.
- Event assertions use channel-based waiting with timeouts, not `time.Sleep`.
- Always run with `-race` — the pipeline spawns goroutines via `run()` and team managers.
- Use `failAllTeamTasks` helper to test failure paths; `completeAllTeamTasks` for success paths.

## Coverage Notes

The `run` function has some uncovered branches for failure paths in optional phases (planning, review, consolidation). These paths are structurally identical to the execution failure path (which is tested) and are difficult to test deterministically because they depend on async phase transitions and context cancellation timing.
