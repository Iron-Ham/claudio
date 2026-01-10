package orchestrator

import "github.com/Iron-Ham/claudio/internal/instance"

// Callback type definitions for orchestrator events.
// These types define the signatures for callbacks that external components
// (like the TUI) can register to receive notifications about instance events.

// PRCompleteCallback is called when a PR workflow finishes.
// Parameters:
//   - instanceID: The ID of the instance whose PR workflow completed
//   - success: Whether the PR was successfully created
//
// This allows the TUI to update its state and potentially remove
// completed instances from the display.
type PRCompleteCallback func(instanceID string, success bool)

// PROpenedCallback is called when a PR URL is detected in instance output.
// This handles inline PR creation, where Claude creates a PR directly
// without going through the dedicated PR workflow.
// Parameters:
//   - instanceID: The ID of the instance that created the PR
//
// The TUI uses this to detect PR completion and optionally auto-remove
// the instance based on configuration.
type PROpenedCallback func(instanceID string)

// TimeoutCallback is called when an instance timeout is detected.
// Parameters:
//   - instanceID: The ID of the instance that timed out
//   - timeoutType: The type of timeout that occurred:
//   - TimeoutActivity: No output activity for the configured duration
//   - TimeoutCompletion: Instance exceeded maximum allowed run time
//   - TimeoutStale: Repeated identical output detected (stuck state)
//
// The TUI uses this to display timeout warnings and update instance status.
type TimeoutCallback func(instanceID string, timeoutType instance.TimeoutType)

// BellCallback is called when a terminal bell character is detected in instance output.
// Parameters:
//   - instanceID: The ID of the instance that triggered the bell
//
// Terminal bells are typically used by Claude to signal that it needs attention,
// such as waiting for user input or permission approval. The TUI can use this
// to highlight the instance or show a notification.
type BellCallback func(instanceID string)
