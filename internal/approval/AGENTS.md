# approval — Agent Guidelines

> **Living document.** Update this file when you learn something specific to this package.
> Same rules as the root `AGENTS.md` — see its Self-Improvement Protocol.

See `doc.go` for package overview and API usage.

## Architecture

The `Gate` type follows the same **decorator pattern** as `EventQueue` wraps `TaskQueue`. The layering is:

```
Gate -> EventQueue -> TaskQueue
```

- `Gate` intercepts `MarkRunning` for tasks requiring approval
- All other operations pass through to `EventQueue`
- The gate tracks pending approvals in an internal map, separate from the queue's own state

## Pitfalls

- **Event publishing outside the lock** — `MarkRunning` and `publishDepth` publish events *outside* the gate's mutex to avoid deadlock with event bus handlers. The pattern is: collect data under the lock, unlock, then publish. If you add new methods that publish events, follow this pattern.
- **Status count adjustment** — `Gate.Status()` adjusts the counts from the underlying `EventQueue` to move tasks from `Claimed` to `AwaitingApproval`. The gate's pending map is the source of truth for how many tasks are gated, since the underlying queue still sees them as "claimed". The `Claimed` count is clamped to zero to prevent negative values from TOCTOU races.
- **Cleanup on release/stale** — When tasks are released (via `Release` or `ClaimStaleBefore`), the pending approvals map must also be cleaned up. Forgetting this would cause phantom entries.
- **GetTask status override** — `GetTask` returns a copy (following copy-on-return) and overrides the status to `TaskAwaitingApproval` for gated tasks. The underlying queue still has the task as "claimed".

## Testing

- Use `makeLookup(plan)` helper to create a `TaskLookup` from a plan spec.
- Since task claim order is non-deterministic (depending on priority ordering), tests should identify the approval/non-approval task by checking `RequiresApproval` on the returned task.
- Always run with `-race` — concurrent approve/reject/claim is a key scenario.
