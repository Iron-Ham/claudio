# filelock — Agent Guidelines

> **Living document.** Update this file when you learn something specific to this package.
> Same rules as the root `AGENTS.md` — see its Self-Improvement Protocol.

See `doc.go` for package overview and API usage.

## Pitfalls

- **Broadcast-then-update ordering** — The registry broadcasts the claim via mailbox *before* updating the in-memory map. If the mailbox Send fails, the in-memory state is unchanged (no rollback needed). This ensures remote instances learn about the claim before the local state reflects it.
- **Event publishing outside the lock** — `bus.Publish` and WatchClaims handlers are invoked *outside* the registry's write lock to avoid deadlock. Handlers may safely call read methods like `Owner`, `IsAvailable`, and `GetInstanceFiles`.
- **RWMutex usage** — Read-only methods (`Owner`, `IsAvailable`, `GetInstanceFiles`) use `RLock`. Write methods (`Claim`, `Release`, `ReleaseAll`) use full `Lock`. Never call a write method while holding a read lock.
- **Metadata format** — Mailbox messages use `msg.Metadata` with keys `"path"` and `"scope"` for structured claim data. Always use these exact keys when constructing or parsing claim messages.

## File Layout

- `doc.go` — Package documentation
- `types.go` — FileClaim struct, ClaimScope, sentinel errors, Option functions
- `registry.go` — Registry type with all public methods
- `registry_test.go` — Comprehensive tests

## Testing

- Use table-driven tests with `t.Run()` subtests.
- Test concurrent claims with goroutines and `-race` flag.
- The mailbox dependency requires `t.TempDir()` for the session directory.
- Event bus assertions use channel-based waiting with timeouts, not `time.Sleep`.
