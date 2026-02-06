// Package debate implements a structured peer debate protocol between
// Claude Code instances.
//
// When two instances disagree on an approach (e.g., architecture, file
// ownership, implementation strategy), a debate session provides a
// structured way to resolve the disagreement through challenge-defense
// rounds that converge on consensus.
//
// # Session Lifecycle
//
// A debate session progresses through three states:
//
//   - Pending: Session created but no messages exchanged yet
//   - Active: At least one challenge has been issued
//   - Resolved: A participant has declared consensus
//
// # Usage
//
//	mb := mailbox.NewMailbox(sessionDir)
//	bus := event.NewBus()
//	sess := debate.NewSession(mb, bus, "instance-1", "instance-2", "Should we use gRPC or REST?")
//
//	sess.Challenge("instance-1", "REST is simpler for our use case", map[string]any{"confidence": 0.8})
//	sess.Defend("instance-2", "gRPC gives us type safety and streaming", map[string]any{"confidence": 0.7})
//	sess.Resolve("instance-1", "Agreed on gRPC for inter-service, REST for public API")
//
// # Thread Safety
//
// Session is safe for concurrent use. All state mutations are protected
// by an internal mutex.
package debate
