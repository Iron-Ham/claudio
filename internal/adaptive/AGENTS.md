# adaptive — Agent Guidelines

> **Living document.** Update this file when you learn something specific to this package.
> Same rules as the root `AGENTS.md` — see its Self-Improvement Protocol.

See `doc.go` for package overview and API usage.

## Pitfalls

- **Event handler thread safety** — Event handlers are called synchronously by the event bus under its read lock. The Lead's handlers acquire the Lead's own mutex. Never call `bus.Subscribe` or `bus.Publish` while holding the Lead's mutex, as this can deadlock with the bus's lock.
- **TaskQueue interface** — The Lead depends on a `TaskQueue` interface, not the concrete `*taskqueue.EventQueue`. Tests should use a mock implementation. Never import `*taskqueue.TaskQueue` directly.
- **Subscription cleanup** — `Stop()` must unsubscribe all event handlers. Failing to do so leaks subscriptions and causes stale handler calls after the Lead is stopped.
- **Scaling signal debouncing** — The Lead tracks `lastScalingSignal` to avoid flooding the bus with scaling events. Respect the `rebalanceInterval` minimum between signals.
- **Zero rebalance interval** — `time.NewTicker` panics with a non-positive duration. The `rebalanceLoop` handles this by skipping the ticker when `rebalanceInterval <= 0`, which is useful in tests that only need event-driven behavior without the periodic loop.
- **Stop() without Start()** — `Stop()` is safe to call even if `Start()` was never called. It only waits on the `stopped` channel if `stopFunc` is non-nil (i.e., `Start` was called).

## Design Decisions

- The Lead does not own or create instances. It only observes events and publishes recommendations. The orchestrator acts on these recommendations.
- `Reassign` is a two-step operation: release from source, claim for target. If the claim fails, the task returns to pending (not lost).
- Workload distribution only counts non-terminal tasks (claimed + running).
- `Reassign` always publishes the original `taskID` in the event, even though `ClaimNext` may claim a different task. This makes the event truthful about the *intent* of the reassignment.

## Testing

- Use a `mockQueue` that implements the `TaskQueue` interface for unit tests.
- Event bus assertions use channel-based waiting with timeouts, not `time.Sleep`.
- Always run with `-race` — the Lead processes events concurrently.
- The `Start`/`Stop` lifecycle should be tested to ensure clean shutdown.
