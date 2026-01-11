// Package process provides abstractions for managing Claude Code processes
// running in tmux sessions.
//
// This package defines interfaces and implementations for starting, stopping,
// and interacting with Claude instances. It abstracts the underlying process
// management (tmux) to enable testing and potential alternative backends.
//
// # Main Types
//
// Interfaces:
//   - [Process]: Core process lifecycle (Start, Stop, IsRunning, Wait, SendInput)
//   - [OutputProvider]: Access to process output (GetOutput)
//   - [Resizable]: Terminal dimension changes (Resize)
//   - [Reconnectable]: Reconnect to existing sessions (Reconnect)
//   - [StateObserver]: Monitor process state changes
//
// Implementations:
//   - [TmuxProcess]: Implementation using tmux for session management
//
// # Design Philosophy
//
// The package uses interface-based design to:
//   - Enable unit testing with mock implementations
//   - Support potential future backends beyond tmux
//   - Separate concerns between process management and output capture
//
// # Thread Safety
//
// [TmuxProcess] is safe for concurrent use. Methods can be called from
// multiple goroutines. The implementation uses appropriate synchronization
// for state management.
//
// # Basic Usage
//
//	config := process.DefaultConfig()
//	config.TmuxSession = "claudio-inst1"
//	config.WorkDir = "/path/to/worktree"
//	config.InitialPrompt = "task description"
//	config.Width = 200
//	config.Height = 30
//
//	proc := process.NewTmuxProcess(config)
//
//	if err := proc.Start(ctx); err != nil {
//	    return err
//	}
//	defer proc.Stop()
//
//	// Send input to the process
//	proc.SendInput("y\n")
//
//	// Resize terminal (implements Resizable interface)
//	proc.Resize(250, 40)
//
//	// Check if running and get output (implements OutputProvider interface)
//	if proc.IsRunning() {
//	    output := proc.GetOutput()
//	}
//
// # Session Naming
//
// Tmux sessions are named with a configurable prefix followed by the instance ID:
//   - Default: "claudio-{instanceID}"
//   - Multi-session: "{sessionID}-claudio-{instanceID}"
package process
