// Package capture provides output capture functionality for Claude instances
// running in tmux sessions.
//
// This package handles the continuous polling and buffering of output from
// tmux panes, providing a bounded memory buffer for efficient output storage
// and retrieval.
//
// # Main Types
//
//   - [OutputCapture]: Interface defining ReadOutput, Start, Stop, and Clear operations
//   - [TmuxCapture]: Implementation that polls tmux panes at configurable intervals
//   - [TmuxCaptureConfig]: Configuration for TmuxCapture (session name, buffer size, interval)
//   - [RingBuffer]: Bounded circular buffer for memory-efficient output storage
//
// # Design
//
// The capture system uses a polling approach to read output from tmux panes.
// Output is stored in a ring buffer that automatically discards old content
// when the buffer reaches capacity, preventing unbounded memory growth during
// long-running sessions.
//
// # Thread Safety
//
// All types in this package are safe for concurrent use. The [RingBuffer]
// uses internal synchronization to protect against concurrent reads and writes.
//
// # Basic Usage
//
//	config := capture.DefaultTmuxCaptureConfig("claudio-inst1")
//	config.BufferSize = 200000  // 200KB buffer
//	config.CaptureInterval = 50 * time.Millisecond
//
//	cap := capture.NewTmuxCapture(config)
//
//	if err := cap.Start(); err != nil {
//	    return err
//	}
//	defer cap.Stop()
//
//	// Read captured output
//	output, _ := cap.ReadOutput()
//
//	// Clear buffer
//	cap.Clear()
package capture
