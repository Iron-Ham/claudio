# taskqueue — Agent Guidelines

> **Living document.** Update this file when you learn something specific to this package.
> Same rules as the root `AGENTS.md` — see its Self-Improvement Protocol.

See `doc.go` for package overview and API usage.

## Pitfalls

- **Wrapper type mutex access** — `EventQueue` wraps `TaskQueue` to publish events. Never access `TaskQueue`'s internal mutex from `EventQueue`. If `EventQueue` needs new synchronized behavior, add a public method on `TaskQueue` and call it from the wrapper.
- **Copy-on-return semantics** — `ClaimNext()` and `GetTask()` return value copies of internal structs, not pointers. This prevents callers from mutating queue state through the returned value. Maintain this pattern when adding new accessor methods.
- **Persistence locking** — State persistence uses temp file + `os.Rename` with `flock` for crash safety. The flock is process-level; multiple goroutines within the same process coordinate via the `TaskQueue` mutex, not the flock.

## EventQueue Decorator

The `EventQueue` type wraps `TaskQueue` to publish events to the event bus without coupling core queue logic to `internal/event`. Key rules:

- **Do not** add event publishing directly to `TaskQueue` — always go through `EventQueue`.
- `EventQueue` methods should delegate to the underlying `TaskQueue` method, then publish the event based on the result.
- When adding a new `TaskQueue` method that should emit events, add a corresponding `EventQueue` wrapper.

## Testing

- Use `t.TempDir()` for persistence tests — the queue writes state files to disk.
- **Event bus testing pattern** — Subscribe to specific event types and wait on a channel with a timeout. Do not use `time.Sleep`. See `queue_events_test.go` for examples.
- Always run with `-race` — the queue is designed for concurrent access from multiple goroutines.
