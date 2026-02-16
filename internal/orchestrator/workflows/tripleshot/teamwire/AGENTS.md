# teamwire — Agent Guidelines

> **Living document.** Update this file when you learn something specific to this package.
> Same rules as the root `AGENTS.md` — see its Self-Improvement Protocol.

## Purpose

This package connects the TripleShot workflow to the Orchestration 2.0 team infrastructure. It exists as a separate subpackage to break an import cycle: `tripleshot → bridge → team → coordination → ... → ultraplan → orchestrator → tripleshot`.

## Architecture

```
TeamCoordinator
  ├── team.Manager
  │   ├── Team "attempt-0" (execution, 1 task)  ──→ Bridge ──→ Instance
  │   ├── Team "attempt-1" (execution, 1 task)  ──→ Bridge ──→ Instance
  │   ├── Team "attempt-2" (execution, 1 task)  ──→ Bridge ──→ Instance
  │   └── Team "judge" (review, 1 task, depends_on=[attempt-*])  ──→ Bridge ──→ Instance
  └── Adapters
      ├── attemptFactory   → bridge.InstanceFactory
      ├── attemptInstance   → bridge.Instance
      ├── attemptCompletionChecker → bridge.CompletionChecker
      ├── judgeCompletionChecker   → bridge.CompletionChecker
      └── sessionRecorder  → bridge.SessionRecorder
```

**Uses Manager directly, not Pipeline.** The Pipeline's rigid phase model (planning → execution → review → consolidation) doesn't fit — the judge team needs completion data from all 3 attempts to construct its prompt, so it's added dynamically via `AddTeamDynamic`.

## Pitfalls

- **Import cycle** — The `tripleshot` package cannot import `bridge`, `team`, or `coordination` due to the dependency chain. All team/bridge wiring lives here in `teamwire`. If you need to reference a tripleshot type from bridge code, use the `ts` import alias for the parent package.
- **Two-phase Start** — `Start()` must not hold `tc.mu` when calling `Bridge.Start()`. The bridge's claim loop publishes `BridgeTaskStartedEvent` synchronously, and the handler `onBridgeTaskStarted` acquires `tc.mu`. Holding the lock through `Start()` → bridge claim → event publish → handler → lock = deadlock. The fix: `registerStart()` holds/releases the lock, then `Start()` creates bridges outside it.
- **Event subscription timing** — Subscriptions must happen before `Bridge.Start()` launches the claim loop. Currently done in `registerStart()` (Phase 1, under lock, before Phase 2 bridge creation) — this is the safe window. Don't move subscriptions after Phase 2 begins. For test assertions where you need events, subscribe before calling `Start()`. For production callbacks, use `SetCallbacks` before `Start`.
- **`onTeamCompleted` dispatches to goroutine** — The handler for `team.completed` dispatches `startJudge()` via `go` to avoid deadlock. The synchronous event bus would block if `startJudge` tried to publish events while the bus's `Publish` goroutine holds a lock.
- **`onTeamCompleted` must set attempt status eagerly** — The bridge calls `gate.Complete(taskID)` before publishing `BridgeTaskCompletedEvent`. `gate.Complete` triggers `TeamCompletedEvent` synchronously (via the team detecting all tasks done), which dispatches `go startJudge()`. `BridgeTaskCompletedEvent` fires next and runs `onBridgeTaskCompleted` (which sets `Attempts[i].Status`). Since `startJudge` runs in a separate goroutine, it races with `onBridgeTaskCompleted` for the last-completing attempt. To prevent `startJudge` from snapshotting a stale "working" status, `onTeamCompleted` sets `Attempts[i].Status` itself under the lock, before dispatching the goroutine.
- **Retries disabled for tripleshot tasks** — `registerStart()` and `startJudge()` call `SetMaxRetries(taskID, 0)` to disable TaskQueue's default retry logic (`defaultMaxRetries=2`). Without this, failed attempt/judge tasks would return to Pending and spawn duplicate instances, appearing as a spurious "second pass." The triple-shot workflow has its own redundancy (3 independent attempts), so retrying individual tasks is counterproductive.
- **Every `onJudgeCompleted` failure path must publish `TripleShotJudgeCompletedEvent`** — Use the `failJudge()` helper, which sets session error, transitions to `PhaseFailed`, fires callbacks, and publishes the event. Forgetting the event on one path breaks downstream listeners.
- **Session mutation lock discipline** — `tsManager.Session()` returns a raw `*Session` pointer; the `tsManager.mu` RLock only protects the pointer swap, not field access. All session field reads *and* mutations (`JudgeID`, `CompletedAt`, `Error`, `Attempts[i].*`) must hold `tc.mu`. `GetWinningBranch()` also holds `tc.mu` for reads. The lock order `tc.mu → tsManager.mu` is safe (no reverse path exists). Functions like `failJudge` and `startJudge` error paths acquire `tc.mu` for mutations, then release before `notifyCallbacks`/`bus.Publish` to avoid deadlock.
- **TUI group restructuring in `startJudge`** — The teamwire coordinator must reorganize the TUI's `InstanceGroup` hierarchy when the judge starts. `reorganizeGroupForJudge()` calls `tc.session.GetGroup(session.GroupID)`, type-asserts to `GroupWithSubGroupsInterface`, creates an "Implementers" sub-group, moves attempt instances into it, and sets `session.ImplementersGroupID`. Without this, the TUI cannot auto-collapse implementers or nest the judge under the tripleshot group. The TUI handler (`handleTeamwireJudgeStarted`) also adds the judge instance to the parent group — matching the pattern from `handleTeamwireAttemptStarted`.
- **`startJudge` snapshot-then-I/O pattern** — `startJudge()` must snapshot attempt data (Status, InstanceID) under `tc.mu` before releasing the lock for I/O (GetInstance, ParseCompletionFile). After I/O completes, it re-acquires `tc.mu` to write results back (WorktreePath, Branch) and build the judge prompt. Without the snapshot, `onBridgeTaskCompleted` can write `Attempts[i].Status` concurrently, causing a data race.

## Testing

- Handler-level tests (`onBridgeTaskCompleted`, `onJudgeCompleted`, `startJudge`) set `tc.started = true` and `tc.attemptTeamIDs` manually to bypass the full `Start()` lifecycle. This makes failure-path tests deterministic.
- Full lifecycle tests (`TestTeamCoordinator_FullLifecycle`) use callbacks, not bus event subscriptions, to avoid the timing window described above.
- Always use `coordination.WithRebalanceInterval(-1)` and `bridge.WithPollInterval(10 * time.Millisecond)` for fast, deterministic tests.
