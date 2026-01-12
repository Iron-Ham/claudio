// Package lifecycle provides instance lifecycle management operations.
//
// The LifecycleManager handles the start/stop/restart lifecycle of Claude Code
// instances running in tmux sessions. It separates lifecycle concerns from
// output capture, state detection, and metrics parsing.
//
// Usage:
//
//	lm := lifecycle.NewManager(logger)
//	if err := lm.Start(inst); err != nil {
//	    return err
//	}
//	defer lm.Stop(inst)
//
//	// Wait for the instance to be ready
//	if err := lm.WaitForReady(inst, 30*time.Second); err != nil {
//	    return err
//	}
//
// The LifecycleManager coordinates with the Instance type to manage:
//   - tmux session creation and cleanup
//   - Process starting and graceful stopping
//   - State transitions (stopped -> running -> ready)
//   - Health checking and readiness detection
package lifecycle
