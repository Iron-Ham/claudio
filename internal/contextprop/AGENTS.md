# contextprop — Agent Guidelines

> **Living document.** Update this file when you learn something specific to this package.
> Same rules as the root `AGENTS.md` — see its Self-Improvement Protocol.

See `doc.go` for package overview and API usage.

## Pitfalls

- **Nil event bus** — `ShareDiscovery` and `ShareWarning` nil-check the bus before publishing. A nil bus is safe and useful in tests.
- **InstanceCount in events** — `ContextPropagatedEvent.InstanceCount` is currently set to 0 because broadcast messages don't know how many instances will read them. The TUI/orchestrator can populate this when they have instance awareness.
- **Empty instanceID** — `GetContextForInstance("")` will return an error from the underlying mailbox. Always validate instance IDs at the call site.

## Architecture

- **Propagator wraps Mailbox** — All message delivery goes through the mailbox. The Propagator adds high-level semantics (discovery, warning) and event publishing.
- **No mutable state** — Propagator holds no mutable state of its own; it delegates entirely to the Mailbox and Bus. This means it is inherently safe for concurrent use.
- **Filter delegation** — `GetContextForInstance` delegates to `mailbox.FormatFiltered` for filtering and formatting. All filter logic lives in the mailbox package.

## Testing

- Use `t.TempDir()` for the mailbox session directory.
- Watch tests use fast poll intervals (10ms) and deadline-based assertions rather than `time.Sleep`.
- Always run with `-race`.
