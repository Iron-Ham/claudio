// Package filelock provides advisory file ownership tracking for Claudio sessions.
//
// When multiple Claude Code instances run in parallel, they may attempt to edit
// the same file simultaneously, leading to conflicts. The filelock package
// prevents this by maintaining an in-memory registry of file ownership claims.
// Instances claim files before editing and release them when done.
//
// # Architecture
//
// The [Registry] maintains a map of file path to owner (instance ID). Claims
// are broadcast to other instances via the mailbox using [mailbox.MessageClaim]
// and [mailbox.MessageRelease] message types. Claim and release events are
// published to the event bus for TUI observability.
//
// # Claim Scopes
//
// Claims support scoped ownership via the Scope field on [FileClaim]:
//   - "file" (default): The entire file is claimed
//   - "function": A specific function within the file is claimed (advisory)
//
// # Basic Usage
//
//	reg := filelock.NewRegistry(mb, bus)
//
//	// Claim a file before editing
//	err := reg.Claim("instance-1", "pkg/foo.go")
//
//	// Check ownership
//	owner, ok := reg.Owner("pkg/foo.go")
//
//	// Release when done
//	err = reg.Release("instance-1", "pkg/foo.go")
//
//	// Release all on shutdown
//	err = reg.ReleaseAll("instance-1")
//
// # Thread Safety
//
// All [Registry] methods are safe for concurrent use via an internal sync.RWMutex.
package filelock
