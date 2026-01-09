// Package instance provides interfaces and implementations for managing
// Claude Code instances. The interfaces abstract the underlying execution
// mechanism (currently tmux) to enable testing and alternative implementations.
package instance

import (
	"io"
	"time"
)

// Runner abstracts the execution of a Claude Code instance.
// The current implementation uses tmux sessions, but this interface
// allows for alternative backends (e.g., direct process management,
// containers, or mock implementations for testing).
//
// Implementations must be safe for concurrent use from multiple goroutines.
// All callback functions are invoked outside of any locks to prevent deadlocks.
type Runner interface {
	// Start launches the Claude Code instance with the configured task.
	// Returns an error if the instance is already running or if startup fails.
	// After Start returns successfully, Running() will return true.
	Start() error

	// Stop terminates the running instance gracefully.
	// It first attempts to send an interrupt signal, then forcefully terminates
	// if the instance doesn't stop within a reasonable time.
	// Stop is idempotent - calling it on a stopped instance is a no-op.
	Stop() error

	// SendInput sends user input to the running instance.
	// Special characters (newlines, tabs, etc.) are translated appropriately.
	// Returns an error if the instance is not running.
	SendInput(data []byte) error

	// SendKey sends a special key sequence (e.g., "Enter", "Tab", "C-c").
	// The key format depends on the underlying implementation.
	SendKey(key string) error

	// GetOutput returns the current contents of the output buffer.
	// The returned slice is a copy and safe to modify.
	GetOutput() []byte

	// Running returns true if the instance is currently running.
	Running() bool

	// ID returns the unique identifier for this instance.
	ID() string
}

// RunnerLifecycle extends Runner with pause/resume and reconnection capabilities.
// Not all runners support these operations; implementations may return errors
// for unsupported operations.
type RunnerLifecycle interface {
	Runner

	// Pause temporarily suspends output capture without stopping the instance.
	// The underlying process continues running.
	// Returns an error if pausing is not supported or the instance is not running.
	Pause() error

	// Resume resumes output capture after a Pause.
	// Returns an error if the instance is not paused.
	Resume() error

	// Paused returns true if output capture is currently paused.
	Paused() bool

	// Reconnect re-establishes connection to an existing instance.
	// This is useful for recovering from crashes or resuming sessions.
	// Returns an error if no existing session is found.
	Reconnect() error
}

// RunnerResizable extends Runner with terminal resize capabilities.
type RunnerResizable interface {
	Runner

	// Resize changes the terminal dimensions of the running instance.
	// Width and height are in character cells.
	Resize(width, height int) error
}

// StateDetector analyzes output to determine the current waiting state
// of a Claude Code instance. The detector examines patterns in the output
// to identify whether Claude is working, waiting for input, asking questions,
// requesting permissions, or has encountered an error.
//
// Implementations should focus on recent output (last few lines/kilobytes)
// as Claude's state is determined by its current activity, not historical output.
type StateDetector interface {
	// Detect analyzes the provided output and returns the detected state.
	// The output parameter contains the full buffered output; implementations
	// should focus on the most recent portion for accurate state detection.
	Detect(output []byte) WaitingState
}

// StateObserver provides callback-based state change notifications.
// This is typically implemented by the Runner to enable reactive handling
// of state transitions.
type StateObserver interface {
	// SetStateCallback registers a callback invoked when the detected state changes.
	// Only one callback can be registered; subsequent calls replace the previous callback.
	// Pass nil to unregister the callback.
	SetStateCallback(cb StateChangeCallback)

	// CurrentState returns the most recently detected waiting state.
	CurrentState() WaitingState
}

// MetricsSource provides access to parsed metrics from Claude Code output.
// Metrics include token usage, cost estimates, and API call counts.
type MetricsSource interface {
	// CurrentMetrics returns the most recently parsed metrics.
	// Returns nil if no metrics have been detected yet.
	CurrentMetrics() *ParsedMetrics

	// SetMetricsCallback registers a callback invoked when metrics change.
	// Pass nil to unregister the callback.
	SetMetricsCallback(cb MetricsChangeCallback)
}

// MetricsParsing extracts metrics from Claude Code output text.
// The parser looks for token counts, costs, and API call information
// in the status line output from Claude Code.
//
// Note: The concrete MetricsParser struct in metrics.go implements this interface.
type MetricsParsing interface {
	// Parse extracts metrics from the provided output.
	// Returns nil if no metrics are found in the output.
	Parse(output []byte) *ParsedMetrics
}

// TimeoutObserver provides callback-based timeout notifications.
// Timeouts can occur due to inactivity, excessive runtime, or
// detecting stuck/stale output patterns.
type TimeoutObserver interface {
	// SetTimeoutCallback registers a callback invoked when a timeout occurs.
	// Pass nil to unregister the callback.
	SetTimeoutCallback(cb TimeoutCallback)

	// TimedOut returns whether a timeout has occurred and its type.
	TimedOut() (timedOut bool, timeoutType TimeoutType)

	// ClearTimeout resets the timeout state, allowing the instance to continue.
	ClearTimeout()

	// LastActivityTime returns when output was last observed to change.
	LastActivityTime() time.Time
}

// OutputBuffer provides a bounded buffer for capturing instance output.
// The buffer automatically discards old content when capacity is exceeded,
// keeping the most recent output.
//
// All methods must be safe for concurrent use.
type OutputBuffer interface {
	io.Writer

	// Bytes returns a copy of all data currently in the buffer.
	// The returned slice is safe to modify without affecting the buffer.
	Bytes() []byte

	// Len returns the number of bytes currently in the buffer.
	Len() int

	// Reset clears all data from the buffer.
	Reset()
}

// OutputSubscriber provides real-time streaming of instance output.
// Subscribers receive output as it arrives rather than polling.
type OutputSubscriber interface {
	// Subscribe returns a channel that receives output chunks as they arrive.
	// The channel is closed when the instance stops or Unsubscribe is called.
	// Multiple subscribers are supported.
	Subscribe() <-chan []byte

	// Unsubscribe stops delivery to the given channel and closes it.
	Unsubscribe(ch <-chan []byte)
}

// BellObserver provides notification when the terminal bell is triggered.
// This is useful for alerting users when Claude needs attention.
type BellObserver interface {
	// SetBellCallback registers a callback invoked when a terminal bell is detected.
	// Pass nil to unregister the callback.
	SetBellCallback(cb BellCallback)
}

// InstanceInfo provides read-only access to instance metadata.
type InstanceInfo interface {
	// ID returns the unique instance identifier.
	ID() string

	// SessionName returns the underlying session name (e.g., tmux session).
	SessionName() string

	// StartTime returns when the instance was started.
	// Returns nil if the instance has never been started.
	StartTime() *time.Time

	// AttachCommand returns a command string that can be used to
	// manually attach to the underlying session for debugging.
	AttachCommand() string
}

// FullRunner combines all runner-related interfaces for implementations
// that provide complete functionality (like the tmux-based Manager).
// This is a convenience interface; consumers should prefer the smaller
// interfaces when possible to reduce coupling.
type FullRunner interface {
	Runner
	RunnerLifecycle
	RunnerResizable
	StateObserver
	MetricsSource
	TimeoutObserver
	BellObserver
	InstanceInfo
}
