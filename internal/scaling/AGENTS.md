# scaling — Agent Guidelines

> **Living document.** Update this file when you learn something specific to this package.
> Same rules as the root `AGENTS.md` — see its Self-Improvement Protocol.

See `doc.go` for package overview and API usage.

## Architecture

The package has two main types:

- **Policy** — Pure evaluation logic. Given a `QueueStatus` and instance count, produces a `Decision`. No I/O, no event bus dependency. Holds a mutex for cooldown state.
- **Monitor** — Subscribes to `QueueDepthChangedEvent` on the event bus, evaluates the policy, and fires callbacks + publishes `ScalingDecisionEvent` for non-none decisions. Runs as a goroutine via `Start(ctx)`.

## Pitfalls

- **Cooldown is per-Policy** — The cooldown state lives on the `Policy`, not the `Monitor`. If you share a `Policy` across monitors (not recommended), cooldown is shared.
- **Scale down by one** — To be conservative, the policy scales down at most 1 instance per decision, even if more could be removed. This prevents rapid over-contraction.
- **Monitor blocking** — `Start(ctx)` blocks until the context is cancelled. Always run it in a goroutine.
- **SetCurrentInstances** — The monitor does not automatically track actual instance count changes. The caller must call `SetCurrentInstances` after scaling actions complete so subsequent evaluations are correct.
- **Type assertion safety** — The Monitor's event handler must use the comma-ok pattern (`de, ok := e.(Type)`) for type assertions. A bare assertion (`de := e.(Type)`) panics on an unexpected event type.
- **scaleUpThreshold** — The Policy's `scaleUpThreshold` controls the minimum pending task count before scale-up triggers. The condition is `status.Pending > p.scaleUpThreshold`, not just `> 0`. Be careful when changing the Evaluate logic to preserve this threshold check.

## Testing

- Use `WithCooldownPeriod(0)` in tests to disable cooldown (otherwise successive evaluations return `ActionNone`).
- Monitor tests use `time.Sleep` for synchronization since the event bus is synchronous — the handler runs in the publisher's goroutine, so a small sleep after `Publish` is sufficient.
- Always run with `-race` — the monitor handles events from the bus goroutine while the main goroutine may call `SetCurrentInstances` or `Stop`.
