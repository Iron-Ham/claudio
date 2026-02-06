# coordination — Agent Guidelines

> **Living document.** Update this file when you learn something specific to this package.
> Same rules as the root `AGENTS.md` — see its Self-Improvement Protocol.

See `doc.go` for package overview and API usage.

## Architecture

The Hub is the integration point for all Orchestration 2.0 components. It creates and owns:

**Task Pipeline (decorator chain):**
```
TaskQueue → EventQueue → Gate
```
Each layer adds behavior without modifying the layer below. The Gate is the primary interface for task operations — use `hub.Gate()` for claiming, completing, and approving tasks.

**Event-Driven Observers:**
- **Adaptive Lead** — Monitors queue events for workload imbalance and recommends scaling.
- **Scaling Monitor** — Evaluates a scaling policy on each queue depth change.

**Communication Infrastructure:**
- **Context Propagator** — Cross-instance knowledge sharing (discoveries, warnings).
- **File Lock Registry** — Advisory file ownership to prevent conflicts.
- **Mailbox** — Underlying JSONL message transport for propagator and file locks.

## Pitfalls

- **Monitor.Start blocks** — `scaling.Monitor.Start(ctx)` blocks until the context is cancelled. The Hub runs it in a goroutine and tracks completion via the `monitorDone` channel. Always cancel the context before waiting on the channel to avoid deadlock.
- **Lead.Start does not block** — `adaptive.Lead.Start(ctx)` spawns its own goroutine and returns immediately. This is asymmetric with the monitor. The Hub calls `lead.Stop()` directly, which waits for the internal goroutine.
- **Stop order matters** — Stop cancels the context first (unblocking the monitor), then stops the monitor (unsubscribes), waits for the monitor goroutine, and finally stops the lead. This reverse-of-start order ensures clean shutdown.
- **Double-start returns error** — `Start` is not idempotent; calling it twice returns an error. `Stop` is idempotent and safe to call multiple times or without `Start`.
- **Monitor goroutine race in tests** — The monitor subscribes to the event bus inside its goroutine. Tests that publish events immediately after `Start` may race with the subscription. Use `bus.SubscriptionCount()` polling to wait for the monitor's handler to be registered before triggering events. See the scaling decision test.
- **Accessor methods need no locking** — The component pointers are set once in `NewHub` and never change. Only the `started` flag and lifecycle fields need mutex protection.

## Testing

- Use `WithRebalanceInterval(-1)` to disable the adaptive lead's periodic rebalance loop in tests. This avoids background goroutine interference and makes tests deterministic.
- Use `scaling.WithCooldownPeriod(0)` and `scaling.WithScaleUpThreshold(0)` to make the scaling policy fire immediately in tests.
- Use `t.TempDir()` for the mailbox session directory.
- Event assertions use channel-based waiting with timeouts, not `time.Sleep`.
- Always run with `-race` — the hub manages concurrent goroutines.

## Integration Notes

The Hub is designed to be created by the Coordinator (or equivalent session manager). Typical usage:

1. Create a `Config` with the event bus, session directory, plan, and optional task lookup.
2. Apply options for scaling policy, instance counts, and timing parameters.
3. Call `Start(ctx)` to activate observers.
4. Use `hub.Gate()` for all task pipeline operations.
5. Use `hub.Propagator()` and `hub.FileLockRegistry()` for cross-instance coordination.
6. Call `Stop()` when the session ends (safe to defer).
