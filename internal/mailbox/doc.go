// Package mailbox provides inter-instance communication for Claudio sessions.
//
// When Claudio runs multiple Claude Code instances in parallel (via git worktrees),
// those instances are currently isolated. The mailbox package enables instances to
// exchange messages during execution, allowing coordination like sharing discoveries,
// claiming file ownership, asking questions, and broadcasting status updates.
//
// # Architecture
//
// Messages are persisted to the filesystem under the session directory using an
// append-only JSONL (JSON Lines) format. Each instance has a dedicated inbox
// directory, and a shared broadcast directory enables messages intended for all
// instances.
//
//	.claudio/mailbox/{sessionID}/
//	    broadcast/index.jsonl    -- messages to all instances
//	    {instanceID}/index.jsonl -- messages to a specific instance
//
// # Main Types
//
//   - [Message]: A single message with sender, recipient, type, body, and metadata
//   - [MessageType]: Enumeration of supported message kinds (discovery, claim, etc.)
//   - [Store]: Low-level file-based storage with atomic writes
//   - [Mailbox]: High-level facade combining broadcast and targeted delivery
//
// # Message Types
//
//   - [MessageDiscovery]: Share findings with other instances
//   - [MessageClaim]: Claim ownership of a file or module
//   - [MessageRelease]: Relinquish previously claimed ownership
//   - [MessageWarning]: Alert others about potential issues
//   - [MessageQuestion]: Request help from other instances
//   - [MessageAnswer]: Respond to a question
//   - [MessageStatus]: Provide a progress update
//
// # Basic Usage
//
//	mb := mailbox.NewMailbox(sessionDir)
//
//	// Send a broadcast message
//	msg := mailbox.Message{
//	    From: "instance-1",
//	    To:   "broadcast",
//	    Type: mailbox.MessageDiscovery,
//	    Body: "Found a shared utility in pkg/utils",
//	}
//	mb.Send(msg)
//
//	// Receive messages (broadcast + targeted)
//	messages, err := mb.Receive("instance-2")
//
//	// Watch for new messages
//	cancel := mb.Watch("instance-2", func(msg mailbox.Message) {
//	    log.Printf("New message from %s: %s", msg.From, msg.Body)
//	})
//	defer cancel()
//
// # Thread Safety
//
// The [Store] and [Mailbox] types are safe for concurrent use within a single
// process via an internal mutex. File writes use O_APPEND for POSIX atomicity
// on small JSONL lines.
package mailbox
