// Package contextprop provides context propagation between Claude Code instances.
//
// When instances make discoveries, encounter warnings, or generate insights,
// the Propagator enables sharing that context with other instances via the
// mailbox. It also provides formatted context suitable for prompt injection,
// allowing instances to benefit from each other's findings.
//
// # Usage
//
//	mb := mailbox.NewMailbox(sessionDir)
//	bus := event.NewBus()
//	prop := contextprop.NewPropagator(mb, bus)
//
//	// Share a discovery with all instances
//	prop.ShareDiscovery("instance-1", "Found shared types in pkg/models", map[string]any{
//	    "file": "pkg/models/types.go",
//	})
//
//	// Share a warning with all instances
//	prop.ShareWarning("instance-2", "API rate limit approaching")
//
//	// Get formatted context for prompt injection
//	ctx := prop.GetContextForInstance("instance-3", mailbox.FilterOptions{
//	    Types: []mailbox.MessageType{mailbox.MessageDiscovery},
//	})
//
// # Thread Safety
//
// Propagator delegates to [mailbox.Mailbox] for thread safety. The Propagator
// itself holds no mutable state.
package contextprop
