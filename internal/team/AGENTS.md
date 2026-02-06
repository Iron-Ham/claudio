# team — Agent Guidelines

> **Living document.** Update this file when you learn something specific to this package.
> Same rules as the root `AGENTS.md` — see its Self-Improvement Protocol.

See `doc.go` for package overview and API usage.

## Architecture

The team package implements multi-team orchestration (Phase 2 of the Orchestrator of Orchestrators). It manages multiple teams running in parallel, each wrapping a `coordination.Hub`.

**Core Components:**
- **Manager** — Orchestrates team lifecycle, dependency ordering, and event routing. Teams are added before `Start`, then the manager handles cascading dependencies.
- **Team** — Wraps a `coordination.Hub` with team metadata, phase tracking, and budget monitoring.
- **Router** — Delivers inter-team messages via each team's Hub mailbox as broadcasts. Uses `team:<teamID>` as the sender prefix.
- **BudgetTracker** — Per-team resource monitoring via event bus. Publishes `TeamBudgetExhaustedEvent` when limits are exceeded.

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
- **AddTeam before Start only** — Adding teams after `Start` returns an error. Dynamic team addition is Phase 3 scope.
- **Manager holds write lock during startTeamLocked** — The `onTeamCompleted` handler acquires `m.mu` write lock. Since it's called from an event handler (which runs synchronously on the bus), avoid publishing events that would re-enter `onTeamCompleted` from within the lock.

## Testing

- Use `coordination.WithRebalanceInterval(-1)` on the manager's hub options to disable the adaptive lead's rebalance loop.
- Use `t.TempDir()` for the manager's `BaseDir` — per-team subdirectories are created automatically.
- The dependency cascade test uses `EventQueue` (not `TaskQueue`) to simulate task completion, ensuring events propagate correctly.
- Event assertions use channel-based waiting with timeouts, not `time.Sleep`.
- Always run with `-race` — the manager uses goroutines for monitoring.
