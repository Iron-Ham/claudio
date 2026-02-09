package bridge

import (
	"context"
	"sync"
)

// dynamicSemaphore is a context-aware, dynamically-resizable concurrency limiter.
//
// A limit of 0 means unlimited — Acquire always succeeds immediately.
// Use SetLimit to adjust capacity at runtime; blocked goroutines are notified
// via Cond.Broadcast so they can re-evaluate.
type dynamicSemaphore struct {
	mu       sync.Mutex
	cond     *sync.Cond
	limit    int // 0 = unlimited
	acquired int
}

// newDynamicSemaphore creates a semaphore with the given initial limit.
// A limit of 0 means unlimited. Negative values are clamped to 0.
func newDynamicSemaphore(limit int) *dynamicSemaphore {
	if limit < 0 {
		limit = 0
	}
	s := &dynamicSemaphore{limit: limit}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// Acquire blocks until a slot is available or the context is cancelled.
// Returns nil on success, or the context error if cancelled.
func (s *dynamicSemaphore) Acquire(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Unlimited mode: always grant immediately.
	if s.limit == 0 {
		s.acquired++
		return nil
	}

	// Spin on the condition variable, checking context between waits.
	// We use a goroutine to broadcast on context cancellation so that
	// blocked waiters wake up and can return the context error.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			s.cond.Broadcast()
		case <-done:
		}
	}()

	for s.acquired >= s.limit && s.limit > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.cond.Wait()
	}

	// Re-check context after waking — the wake may have been from cancellation.
	if err := ctx.Err(); err != nil {
		return err
	}

	s.acquired++
	return nil
}

// Release frees a slot and signals one waiting goroutine.
func (s *dynamicSemaphore) Release() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.acquired > 0 {
		s.acquired--
	}
	s.cond.Signal()
}

// SetLimit adjusts the capacity. Negative values are clamped to 0 (unlimited).
// Broadcasts to wake all blocked goroutines so they can re-evaluate against the
// new limit.
func (s *dynamicSemaphore) SetLimit(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if n < 0 {
		n = 0
	}
	s.limit = n
	s.cond.Broadcast()
}

// Limit returns the current limit (0 = unlimited).
func (s *dynamicSemaphore) Limit() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.limit
}

// Acquired returns the number of currently acquired slots.
func (s *dynamicSemaphore) Acquired() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.acquired
}
