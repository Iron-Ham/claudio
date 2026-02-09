# team — Agent Guidelines

> **Living document.** Update this file when you learn something specific to this package.
> Same rules as the root `AGENTS.md` — see its Self-Improvement Protocol.

See `doc.go` for package overview and API usage.

## Architecture

The team package implements multi-team orchestration (Phase 2 of the Orchestrator of Orchestrators). It manages multiple teams running in parallel, each wrapping a `coordination.Hub`.

**Core Components:**
- **Manager** — Orchestrates team lifecycle, dependency ordering, and event routing. Teams are added with `AddTeam` before `Start` or with `AddTeamDynamic` after. The manager handles cascading dependencies via `onTeamCompleted`.
- **Team** — Wraps a `coordination.Hub` with team metadata, phase tracking, and budget monitoring.
- **Router** — Delivers inter-team messages via each team's Hub mailbox as broadcasts. Uses `team:<teamID>` as the sender prefix. Delivery is best-effort; send errors are silently discarded so one failed delivery doesn't block a broadcast to others.
- **BudgetTracker** — Per-team resource monitoring. The manager calls `Record()` after mapping instance metrics to teams. Does NOT subscribe to the event bus directly — the manager handles routing externally.

**Dependency Flow:**
```
Manager.Start(ctx)
  ├─ Teams with no deps → phase=Working, hub.Start()
  └─ Teams with deps → phase=Blocked
      └─ When dep completes → check blocked teams → start satisfied ones
```

## Pitfalls

- **Use EventQueue, not TaskQueue, for task operations** — The `monitorTeamCompletion` goroutine listens for `queue.depth_changed` events, which are only published by `EventQueue`. Operating directly on `TaskQueue` bypasses event publishing and the monitor won't detect completion. Use `hub.EventQueue()` or `hub.Gate()` for task lifecycle operations.
- **Shared event bus, team-specific filtering** — All teams share one `event.Bus`. Event handlers must filter by team ID or instance membership. The `BudgetTracker` exposes `Record()` for the manager to call after mapping instances to teams.
- **Insertion order for determinism** — `Manager.AllStatuses()` returns teams in insertion order using the `order` slice, not map iteration order. Always use the `order` slice for any deterministic iteration.
- **AddTeam vs AddTeamDynamic** — `AddTeam` is for pre-Start registration. `AddTeamDynamic` can add teams after `Start` but uses a two-phase approach: register under lock, then start outside lock to prevent deadlock with the monitor goroutine's event chain.
- **AddTeamDynamic uses Manager's context** — The `ctx` parameter on `AddTeamDynamic` is ignored; it uses the Manager's stored context (from `Start`) so `Stop` can cancel the new team's monitor goroutine. Passing a different context would cause `wg.Wait()` to hang on Stop.
- **Manager holds write lock during startTeamLocked** — The `onTeamCompleted` handler acquires `m.mu` write lock. Since it's called from an event handler (which runs synchronously on the bus), avoid publishing events that would re-enter `onTeamCompleted` from within the lock.
- **Failed dependencies cascade to blocked dependents** — `allDepsSatisfiedLocked` requires `PhaseDone`, not just any terminal phase. When a dependency fails, `hasFailedDepLocked` detects it and `onTeamCompleted` transitions the blocked team to `PhaseFailed`. This cascades through multi-hop chains (A fails → B fails → C fails) via a loop in `onTeamCompleted`. The two-phase pattern (collect state under lock, publish events outside lock) prevents re-entrancy deadlock with the synchronous event bus.
- **onTeamCompleted two-phase cascade** — `onTeamCompleted` uses `checkBlockedTeamsLocked` to scan blocked teams under the lock. Failed teams' phase is set under the lock, but `TeamPhaseChangedEvent` and `TeamCompletedEvent` are published *outside* the lock. The outer loop repeats until no new transitions occur, handling multi-hop dependency chains in a single handler invocation without re-entrancy.
- **Budget cleanup on Hub start failure** — `startTeamLocked` calls `t.budget.Stop()` if `t.hub.Start(ctx)` fails. Without this, the budget tracker leaks its "active" sentinel and appears started despite the team being in `PhaseFailed`.
- **Stop() releases lock before wg.Wait()** — `Stop()` sets `m.started = false` and releases `m.mu` before calling `m.wg.Wait()`. This prevents deadlock with `monitorTeamCompletion` publishing `TeamCompletedEvent` (which triggers `onTeamCompleted` inline, acquiring `m.mu`). The `started = false` guard ensures any racing handler bails out immediately. Same principle as `Pipeline.Stop()` and `PipelineExecutor.Stop()`.

## Testing

- Use `coordination.WithRebalanceInterval(-1)` on the manager's hub options to disable the adaptive lead's rebalance loop.
- Use `t.TempDir()` for the manager's `BaseDir` — per-team subdirectories are created automatically.
- The dependency cascade test uses `EventQueue` (not `TaskQueue`) to simulate task completion, ensuring events propagate correctly. Use `tq.SetMaxRetries(taskID, 0)` before `eq.Fail()` to bypass retry logic when testing failure paths.
- The failed-dependency cascade test (`TestManager_FailedDependencyCascade`) uses `team.completed` events (not `team.phase_changed`) to verify cascading because a single `phaseChanges` channel can consume events out of order when multiple teams transition simultaneously.
- Event assertions use channel-based waiting with timeouts, not `time.Sleep`.
- Always run with `-race` — the manager uses goroutines for monitoring.
