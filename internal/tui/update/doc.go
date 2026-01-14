// Package update provides message handlers for the TUI Update function.
//
// This package separates message routing logic from the actual handling,
// enabling easier testing and reducing the complexity of the main app.go file.
// Each handler operates on a Context interface, allowing the TUI Model to
// implement the required methods without circular dependencies.
//
// The handlers in this package process Bubbletea messages such as:
//   - TickMsg: periodic updates for UI refresh and polling
//   - OutputMsg: output data from Claude instances
//   - ErrMsg: error notifications
//   - PRCompleteMsg/PROpenedMsg: PR workflow events
//   - TimeoutMsg: instance timeout notifications
//   - BellMsg: terminal bell forwarding
//   - TaskAddedMsg/DependentTaskAddedMsg: async task creation results
package update
