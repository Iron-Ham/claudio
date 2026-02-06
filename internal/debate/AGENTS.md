# debate — Agent Guidelines

> **Living document.** Update this file when you learn something specific to this package.
> Same rules as the root `AGENTS.md` — see its Self-Improvement Protocol.

See `doc.go` for package overview and API usage.

## Pitfalls

- **Nil event bus** — `NewSession` and `Resolve` publish events to the event bus. Both nil-check the bus before publishing, so a nil bus is safe and useful in tests that don't need event verification.
- **Participant validation** — All message-sending methods (Challenge, Defend, Resolve) validate that `from` is one of the two participants. Non-participants get a clear error.
- **State machine enforcement** — Session status transitions are strictly enforced: Pending -> Active -> Resolved. Defend and Resolve require Active status. Challenge requires non-Resolved status.

## Architecture

- **Session wraps Mailbox** — Debate messages are sent through the mailbox using targeted (non-broadcast) delivery. The Session tracks its own copy of messages for transcript access without re-reading the mailbox.
- **Metadata conventions** — All debate messages include `debate_id` and `round` in their metadata map. User-provided metadata is merged with these fields (user values for these keys are overwritten).
- **Copy-on-return** — `Messages()` returns a copy of the internal slice to prevent data races.

## Testing

- Use `t.TempDir()` for the mailbox session directory.
- Event verification: subscribe to specific event types and check the received event inline (no channels needed since the bus is synchronous).
- Always run with `-race` — Session is designed for concurrent access.
