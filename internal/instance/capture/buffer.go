// Package capture provides output capture utilities for instance management.
//
// The primary component is RingBuffer, a thread-safe circular buffer designed
// for capturing process output streams. It efficiently stores the most recent
// N bytes of output, automatically discarding older data when the buffer fills.
package capture

import "sync"

// RingBuffer is a thread-safe circular (ring) buffer for capturing output streams.
//
// A ring buffer is a fixed-size buffer that operates as if its ends were connected
// in a circle. When the buffer fills up, new data overwrites the oldest data,
// making it ideal for capturing the "tail" of an output stream without unbounded
// memory growth.
//
// # How It Works
//
// The buffer maintains two pointers:
//   - start: points to the oldest byte in the buffer
//   - end: points to where the next byte will be written
//
// When data is written and the buffer isn't full, only 'end' advances.
// Once the buffer fills (end catches up to start), both pointers advance together,
// effectively sliding a window over the data stream.
//
// Visual example with a 5-byte buffer:
//
//	Initial:     [_, _, _, _, _]  start=0, end=0
//	Write "abc": [a, b, c, _, _]  start=0, end=3
//	Write "de":  [a, b, c, d, e]  start=0, end=0, full=true
//	Write "fg":  [f, g, c, d, e]  start=2, end=2 → Bytes() returns "cdefg"
//
// # Thread Safety
//
// All methods are safe for concurrent use. The buffer uses a sync.RWMutex:
//   - Write and Reset acquire exclusive (write) locks
//   - Bytes and Len acquire shared (read) locks
//
// # Interface Compatibility
//
// RingBuffer implements io.Writer, making it suitable for use with any
// function that accepts an io.Writer (e.g., exec.Cmd.Stdout).
type RingBuffer struct {
	data  []byte
	size  int
	start int
	end   int
	full  bool
	mu    sync.RWMutex
}

// NewRingBuffer creates a new ring buffer with the given capacity in bytes.
//
// The capacity determines how many bytes of recent output will be retained.
// For example, NewRingBuffer(1024) creates a buffer that always holds
// the most recent 1KB of written data.
func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		data: make([]byte, size),
		size: size,
	}
}

// Write writes data to the buffer, implementing io.Writer.
//
// Write always succeeds and returns len(p), nil. If the data being written
// is larger than the remaining capacity, the oldest bytes in the buffer
// are overwritten to make room. This ensures the buffer always contains
// the most recent 'size' bytes of all data written.
//
// Write is safe for concurrent use with other Write, Bytes, Len, and Reset calls.
func (r *RingBuffer) Write(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	n = len(p)

	for _, b := range p {
		r.data[r.end] = b
		r.end = (r.end + 1) % r.size

		if r.full {
			r.start = (r.start + 1) % r.size
		}

		if r.end == r.start {
			r.full = true
		}
	}

	return n, nil
}

// Bytes returns a copy of all data currently in the buffer.
//
// The returned slice is in chronological order (oldest to newest).
// The slice is a copy, so modifying it will not affect the buffer's contents.
//
// Returns an empty slice if the buffer is empty.
//
// Bytes is safe for concurrent use with other Bytes, Len, Write, and Reset calls.
func (r *RingBuffer) Bytes() []byte {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.full && r.start == 0 {
		return append([]byte(nil), r.data[:r.end]...)
	}

	result := make([]byte, 0, r.len())
	if r.full || r.end < r.start {
		result = append(result, r.data[r.start:]...)
		result = append(result, r.data[:r.end]...)
	} else {
		result = append(result, r.data[r.start:r.end]...)
	}

	return result
}

// Len returns the number of bytes currently stored in the buffer.
//
// The returned value is always between 0 and the buffer's capacity (inclusive).
// Once the buffer fills for the first time, Len will always return the capacity.
//
// Len is safe for concurrent use with other Len, Bytes, Write, and Reset calls.
func (r *RingBuffer) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.len()
}

// len returns the number of bytes in the buffer (caller must hold lock).
func (r *RingBuffer) len() int {
	if r.full {
		return r.size
	}

	if r.end >= r.start {
		return r.end - r.start
	}

	return r.size - r.start + r.end
}

// Reset clears the buffer, discarding all stored data.
//
// After Reset, the buffer behaves as if newly created with the same capacity.
// The underlying memory is retained to avoid reallocation.
//
// Reset is safe for concurrent use with other Reset, Bytes, Len, and Write calls.
func (r *RingBuffer) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.start = 0
	r.end = 0
	r.full = false
}

// ReplaceWith atomically resets the buffer and writes new data.
//
// This is equivalent to calling Reset() followed by Write(), but performs both
// operations under a single lock acquisition. This prevents race conditions where
// concurrent Bytes() calls could see an empty buffer between Reset and Write.
//
// ReplaceWith is safe for concurrent use with other methods.
func (r *RingBuffer) ReplaceWith(p []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Reset the buffer
	r.start = 0
	r.end = 0
	r.full = false

	// Write the new data (inline to avoid lock re-acquisition)
	for _, b := range p {
		r.data[r.end] = b
		r.end = (r.end + 1) % r.size

		if r.full {
			r.start = (r.start + 1) % r.size
		}

		if r.end == r.start {
			r.full = true
		}
	}
}
