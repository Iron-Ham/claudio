# mailbox — Agent Guidelines

> **Living document.** Update this file when you learn something specific to this package.
> Same rules as the root `AGENTS.md` — see its Self-Improvement Protocol.

See `doc.go` for package overview and API usage.

## Pitfalls

- **O_APPEND atomicity** — File writes use `O_APPEND` which is atomic for writes smaller than `PIPE_BUF` (4096 bytes on most systems), but is not crash-safe without `fsync`. This is an accepted trade-off — messages may be lost on hard crash but won't be corrupted or interleaved.
- **Message ID uniqueness** — `time.UnixNano()` alone is not unique under concurrent access. IDs are generated using an atomic counter combined with PID and timestamp. If you modify ID generation, ensure uniqueness under parallel `Send()` calls.
- **Store mutex scope** — The `Store` holds a `sync.Mutex` for in-process thread safety. Any method that reads or writes the JSONL file must hold the lock for the entire operation, including the JSON marshal/unmarshal step — not just the file I/O.
- **WithBus event publishing is synchronous** — When a `Mailbox` is created with `WithBus(bus)`, every successful `Send()` publishes a `MailboxMessageEvent` on the event bus synchronously. Since `event.Bus.Publish` runs handlers inline, callers of `Send` should be aware that handlers may execute significant work in their goroutine. The Hub passes its bus to `NewMailbox` automatically.

## File Layout

```
.claudio/mailbox/{sessionID}/
    broadcast/index.jsonl    -- messages to all instances
    {instanceID}/index.jsonl -- messages to a specific instance
```

## Testing

- Use `t.TempDir()` for all persistence tests — avoids cross-test pollution and auto-cleans.
- The `Store` tests exercise concurrent writes via goroutines; always run with `-race`.
