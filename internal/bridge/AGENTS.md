# bridge — Agent Guidelines

> **Living document.** Update this file when you learn something specific to this package.
> Same rules as the root `AGENTS.md` — see its Self-Improvement Protocol.

See `doc.go` for package overview and API usage.

## Architecture

The bridge package connects team Hubs (Orchestration 2.0's task pipeline) to real Claude Code instances (worktrees + tmux). Each Bridge is scoped to one team and runs independently.

**Core Flow:**
```
Gate.ClaimNext() → InstanceFactory.CreateInstance() → StartInstance()
    → monitor loop (poll CompletionChecker)
    → Gate.Complete/Fail() + SessionRecorder
```

**Interfaces (Ports):**
- `InstanceFactory` — Creates and starts Claude Code instances
- `CompletionChecker` — Detects sentinel files and verifies work
- `SessionRecorder` — Records session state changes (task assignments, completions, failures)
- `Instance` — Read-only handle to a created instance (ID, WorktreePath, Branch)

These interfaces are implemented by adapters in `internal/orchestrator/bridgewire/`.

**Concurrency Model:**
- One claim-loop goroutine per Bridge
- One monitor goroutine per active task
- All goroutines tracked via `sync.WaitGroup` for clean shutdown
- Running map (`taskID → instanceID`) protected by `sync.RWMutex`
- `dynamicSemaphore` gates concurrency — claim loop acquires a slot before `ClaimNext`, monitor releases it on completion/failure. `SetMaxConcurrency(0)` = unlimited (default, backward compatible).

## Pitfalls

- **Import cycle with ultraplan** — The `bridge` package must NOT import `ultraplan` or `orchestrator`. The chain `bridge → team → coordination → ... → ultraplan → orchestrator` creates a cycle if `orchestrator` imports `bridge`. Use simple types (strings, slices) rather than concrete domain types in the bridge API. The `BuildTaskPrompt` function accepts `(title, description string, files []string)` instead of `ultraplan.PlannedTask` for this reason.
- **Event-driven wake pattern** — The claim loop subscribes to `queue.depth_changed` events and blocks on a buffered channel. Don't replace this with polling — the event-driven approach is more efficient and responsive.
- **Gate.IsComplete exit condition** — The claim loop exits when there are no tasks and `gate.IsComplete()` returns true (all tasks terminal). Without this check, the loop would block forever waiting for new tasks that will never arrive.
- **Publish events outside the lock** — `BridgeTaskStartedEvent` and `BridgeTaskCompletedEvent` are published outside the mutex to avoid deadlock with synchronous event handlers that might call back into the bridge.
- **Clean running map before callbacks** — The monitor cleans up the `running` map before calling `RecordCompletion`/`RecordFailure` or publishing events. This ensures observers see consistent state when their callbacks fire.
- **Stop() lifecycle: cancel → drain → mark stopped** — `Stop()` cancels the context, releases the lock, calls `wg.Wait()`, then re-acquires the lock to set `started=false`. Setting `started=false` before `wg.Wait()` would allow a premature `Start()` that corrupts the WaitGroup.
- **Nil interface arguments panic immediately** — The `New()` constructor panics on nil arguments rather than deferring the nil-pointer dereference to runtime. This surfaces wiring bugs at construction time.
- **Retry limit on completion check errors** — The monitor gives up after `maxCheckErrors` (10) consecutive `CheckCompletion` failures and fails the task. Without this, a bad worktree path would cause indefinite retries.
- **TaskQueue retry interacts with bridge claim loop** — `TaskQueue.Fail()` has retry logic (`defaultMaxRetries=2`). When the bridge monitor calls `gate.Fail()`, the task may return to `TaskPending` (not permanently failed), and the claim loop re-claims it. Tests that assert on `Running()` after failure must either disable retries via `SetMaxRetries(taskID, 0)` or account for the re-claim cycle.
- **Always log gate.Fail errors** — `gate.Fail()` can fail if the task has already transitioned. Always check and log the return error rather than discarding with `_ =`.

## Testing

- Use `newTestTeam` helper (in `bridge_test.go`) which creates a `team.Manager` → `AddTeam` → `Start` flow to get a real `*team.Team` with a functioning Hub.
- Always use `coordination.WithRebalanceInterval(-1)` to disable the adaptive lead's rebalance loop.
- Use `WithPollInterval(10*time.Millisecond)` for fast tests.
- Event assertions use `waitForEvent` with channel + timeout, not `time.Sleep`.
- For error-path tests where the task becomes terminal (e.g., `CreateInstanceError`), use `stopWithTimeout` — the claim loop exits via `IsComplete()` once the task fails, so `Stop()` returns without needing a separate event.
- For tests that need to observe side effects before stopping (e.g., recorder signals), use a `signalingRecorder` that sends on a channel. Don't call `Stop()` before the signal — `Stop()` cancels the context, which races with the monitor.
- Always run with `-race` — the bridge has concurrent goroutines.
