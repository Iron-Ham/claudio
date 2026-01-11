// Package detect provides state detection for Claude instances by analyzing
// their terminal output.
//
// This package uses pattern matching to determine whether a Claude instance
// is actively working, waiting for user input, completed, or in an error state.
// It enables the orchestrator to react appropriately to instance state changes.
//
// # Main Types
//
//   - [WaitingState]: Enum representing detected instance states
//   - [Detector]: Pattern matcher that analyzes output to determine state
//   - [TimeoutType]: Types of timeout conditions (Activity, Completion, Stale)
//
// # Waiting States
//
// The detector can identify these states:
//
//   - [StateWorking]: Instance is actively processing (has working indicators)
//   - [StateWaitingPermission]: Waiting for Y/N confirmation or permission
//   - [StateWaitingQuestion]: Waiting for user to answer a question
//   - [StateWaitingInput]: General input waiting state
//   - [StateCompleted]: Task completed (sentinel file detected)
//   - [StateError]: CLI error encountered
//   - [StatePROpened]: PR URL detected in output
//
// # Detection Priority
//
// When multiple patterns match, the detector uses priority ordering:
// 1. Working indicators override historical waiting states
// 2. Completion detection takes precedence when sentinel file exists
// 3. More specific states (permission, question) override generic waiting
//
// # Basic Usage
//
//	detector := detect.NewDetector()
//
//	// Analyze output to determine state
//	state := detector.Detect(output)
//
//	switch state {
//	case detect.StateWaitingPermission:
//	    // Prompt user for permission
//	case detect.StateCompleted:
//	    // Handle completion
//	case detect.StateWorking:
//	    // Instance is still active
//	}
//
// # Timeout Detection
//
// The package also provides timeout detection for stuck instances:
//
//   - Activity timeout: No new output for configured duration
//   - Completion timeout: Instance hasn't completed within time limit
//   - Stale detection: Repeated identical output indicating a loop
package detect
