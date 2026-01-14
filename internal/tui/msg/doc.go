// Package msg defines the message types used by the TUI's Bubbletea event loop.
//
// This package contains all [tea.Msg] types that represent events the TUI can receive,
// such as timer ticks, instance output, errors, and completion signals. By centralizing
// these types, we achieve:
//
//   - Clear separation between message types and their handlers
//   - Single source of truth for message definitions
//   - Easier testing and documentation of the event system
//
// Message types are exported so they can be used by both the main TUI package
// and any subpackages that need to produce or handle these messages.
package msg
