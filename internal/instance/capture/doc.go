// Package capture provides output buffering functionality for Claude instances
// running in tmux sessions.
//
// # Main Types
//
//   - [RingBuffer]: Bounded circular buffer for memory-efficient output storage
//
// # Thread Safety
//
// All types in this package are safe for concurrent use. The [RingBuffer]
// uses internal synchronization to protect against concurrent reads and writes.
//
// # Basic Usage
//
//	buf := capture.NewRingBuffer(200000) // 200KB buffer
//	buf.Write([]byte("some output"))
//	data := buf.Read()
//	buf.Clear()
package capture
