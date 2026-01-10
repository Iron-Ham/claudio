// Package process defines interfaces and types for managing Claude Code processes.
//
// This package provides a clean abstraction over the underlying process execution
// mechanism (currently tmux sessions) to enable better testability, separation of
// concerns, and potential alternative implementations in the future.
package process

import (
	"context"
	"errors"
)

// Common errors returned by Process implementations.
var (
	// ErrAlreadyRunning is returned when Start is called on an already running process.
	ErrAlreadyRunning = errors.New("process already running")

	// ErrNotRunning is returned when an operation requires a running process but none exists.
	ErrNotRunning = errors.New("process not running")

	// ErrSessionNotFound is returned when attempting to reconnect to a non-existent session.
	ErrSessionNotFound = errors.New("session not found")
)

// Config holds the configuration for creating a new process.
// These settings define how the process should be initialized and managed.
type Config struct {
	// TmuxSession is the name of the tmux session to use for this process.
	// If empty, a name will be generated automatically.
	TmuxSession string

	// WorkDir is the working directory for the process.
	// This is where the Claude Code instance will execute.
	WorkDir string

	// InitialPrompt is the task/prompt to send to Claude Code when starting.
	// This is the instruction that Claude will work on.
	InitialPrompt string

	// Width is the terminal width in columns (default: 200).
	Width int

	// Height is the terminal height in rows (default: 30).
	Height int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Width:  200,
		Height: 30,
	}
}

// Validate checks that the Config has all required fields set.
func (c *Config) Validate() error {
	if c.WorkDir == "" {
		return errors.New("WorkDir is required")
	}
	if c.InitialPrompt == "" {
		return errors.New("InitialPrompt is required")
	}
	return nil
}

// Process defines the interface for managing a Claude Code process.
//
// Implementations of this interface handle the lifecycle of a single Claude Code
// instance, including starting, stopping, monitoring, and communicating with it.
//
// The typical lifecycle is:
//  1. Create a Process implementation with NewXxxProcess(config)
//  2. Start the process with Start(ctx)
//  3. Monitor status with IsRunning() and optionally Wait()
//  4. Send input as needed with SendInput(input)
//  5. Stop the process with Stop() when done
//
// Example usage:
//
//	proc := NewTmuxProcess(config)
//	if err := proc.Start(ctx); err != nil {
//	    return err
//	}
//	defer proc.Stop()
//
//	// Send follow-up input
//	proc.SendInput("yes\n")
//
//	// Wait for completion
//	if err := proc.Wait(); err != nil {
//	    return err
//	}
type Process interface {
	// Start launches the Claude Code process.
	//
	// The provided context controls the startup operation. If the context is
	// cancelled before startup completes, Start should return the context error.
	//
	// Returns ErrAlreadyRunning if the process is already running.
	// Returns an error if the process cannot be started (e.g., tmux unavailable).
	Start(ctx context.Context) error

	// Stop terminates the process gracefully.
	//
	// Stop should attempt a graceful shutdown (e.g., sending Ctrl+C) before
	// forcefully terminating the process. It is safe to call Stop multiple times
	// or on a process that is not running.
	//
	// Returns nil on success or if the process is not running.
	// Returns an error if the termination fails.
	Stop() error

	// IsRunning returns true if the process is currently running.
	//
	// This should reflect the actual state of the underlying process, not just
	// whether Start() was called. For example, if the process exits on its own,
	// IsRunning should return false.
	IsRunning() bool

	// Wait blocks until the process exits.
	//
	// Returns nil if the process exits successfully (exit code 0).
	// Returns ErrNotRunning if the process is not running.
	// Returns an error if the process exits with a non-zero exit code or fails.
	//
	// Wait can be called concurrently from multiple goroutines.
	Wait() error

	// SendInput sends input to the running process.
	//
	// The input string is sent to the process's stdin. Newlines in the input
	// are handled appropriately (converted to Enter key presses for terminal-based
	// processes).
	//
	// Returns ErrNotRunning if the process is not running.
	// Returns an error if the input cannot be sent.
	SendInput(input string) error
}

// OutputProvider is an optional interface for processes that provide output capture.
//
// Implementations that capture process output (e.g., from tmux pane) should
// implement this interface in addition to Process.
type OutputProvider interface {
	// GetOutput returns the captured output from the process.
	//
	// The returned bytes represent the current buffer of captured output.
	// This may be truncated if the output exceeds the buffer size.
	GetOutput() []byte
}

// Resizable is an optional interface for processes that support terminal resizing.
//
// Terminal-based implementations (like tmux) should implement this interface
// to allow dynamic resizing of the process terminal.
type Resizable interface {
	// Resize changes the terminal dimensions.
	//
	// Width and height are specified in character columns and rows respectively.
	// Returns an error if the resize operation fails.
	Resize(width, height int) error
}

// Reconnectable is an optional interface for processes that support session recovery.
//
// Implementations that persist across restarts (like tmux sessions) should
// implement this interface to enable reconnection to existing sessions.
type Reconnectable interface {
	// Reconnect attempts to reconnect to an existing session.
	//
	// This is used for session recovery after a restart. The implementation
	// should verify the session exists and resume monitoring it.
	//
	// Returns ErrSessionNotFound if the session does not exist.
	// Returns an error if reconnection fails.
	Reconnect() error

	// SessionExists checks if the session exists and can be reconnected to.
	SessionExists() bool
}

// StateObserver is an optional interface for processes that provide state monitoring.
//
// Implementations that can detect process state (working, waiting, completed, etc.)
// should implement this interface.
type StateObserver interface {
	// OnStateChange registers a callback to be invoked when the process state changes.
	//
	// The callback receives the process ID and the new state.
	// Only one callback can be registered at a time; subsequent calls replace
	// the previous callback.
	OnStateChange(callback func(processID string, state State))

	// CurrentState returns the current detected state of the process.
	CurrentState() State
}

// State represents the current operational state of a process.
// This mirrors the WaitingState enum from the detector package but is defined
// here to avoid circular dependencies.
type State int

const (
	// StateWorking indicates the process is actively working.
	StateWorking State = iota

	// StateWaitingPermission indicates the process is waiting for permission approval.
	StateWaitingPermission

	// StateWaitingQuestion indicates the process is waiting for an answer to a question.
	StateWaitingQuestion

	// StateWaitingInput indicates the process is waiting for general input.
	StateWaitingInput

	// StateCompleted indicates the process has finished its task.
	StateCompleted

	// StateError indicates the process encountered an error.
	StateError
)

// String returns a human-readable string for the state.
func (s State) String() string {
	switch s {
	case StateWorking:
		return "working"
	case StateWaitingPermission:
		return "waiting_permission"
	case StateWaitingQuestion:
		return "waiting_question"
	case StateWaitingInput:
		return "waiting_input"
	case StateCompleted:
		return "completed"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// IsWaiting returns true if the state represents any waiting condition.
func (s State) IsWaiting() bool {
	return s == StateWaitingPermission || s == StateWaitingQuestion || s == StateWaitingInput
}
