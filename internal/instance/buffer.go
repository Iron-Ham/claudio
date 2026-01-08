package instance

import "sync"

// RingBuffer is a thread-safe ring buffer for output
type RingBuffer struct {
	data  []byte
	size  int
	start int
	end   int
	full  bool
	mu    sync.RWMutex
}

// NewRingBuffer creates a new ring buffer with the given capacity
func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		data: make([]byte, size),
		size: size,
	}
}

// Write writes data to the buffer
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

// Bytes returns all data in the buffer
func (r *RingBuffer) Bytes() []byte {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.full && r.start == 0 {
		return append([]byte(nil), r.data[:r.end]...)
	}

	result := make([]byte, 0, r.Len())
	if r.full || r.end < r.start {
		result = append(result, r.data[r.start:]...)
		result = append(result, r.data[:r.end]...)
	} else {
		result = append(result, r.data[r.start:r.end]...)
	}

	return result
}

// Len returns the number of bytes in the buffer
func (r *RingBuffer) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.full {
		return r.size
	}

	if r.end >= r.start {
		return r.end - r.start
	}

	return r.size - r.start + r.end
}

// Reset clears the buffer
func (r *RingBuffer) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.start = 0
	r.end = 0
	r.full = false
}
