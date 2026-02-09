package bridge

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDynamicSemaphore_BasicAcquireRelease(t *testing.T) {
	sem := newDynamicSemaphore(2)
	ctx := context.Background()

	if err := sem.Acquire(ctx); err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	if sem.Acquired() != 1 {
		t.Errorf("Acquired() = %d, want 1", sem.Acquired())
	}

	if err := sem.Acquire(ctx); err != nil {
		t.Fatalf("second Acquire: %v", err)
	}
	if sem.Acquired() != 2 {
		t.Errorf("Acquired() = %d, want 2", sem.Acquired())
	}

	sem.Release()
	if sem.Acquired() != 1 {
		t.Errorf("after release: Acquired() = %d, want 1", sem.Acquired())
	}

	sem.Release()
	if sem.Acquired() != 0 {
		t.Errorf("after second release: Acquired() = %d, want 0", sem.Acquired())
	}
}

func TestDynamicSemaphore_UnlimitedMode(t *testing.T) {
	sem := newDynamicSemaphore(0) // unlimited
	ctx := context.Background()

	// Should be able to acquire many without blocking.
	for i := range 100 {
		if err := sem.Acquire(ctx); err != nil {
			t.Fatalf("Acquire %d: %v", i, err)
		}
	}
	if sem.Acquired() != 100 {
		t.Errorf("Acquired() = %d, want 100", sem.Acquired())
	}
	if sem.Limit() != 0 {
		t.Errorf("Limit() = %d, want 0", sem.Limit())
	}
}

func TestDynamicSemaphore_BlocksAtLimit(t *testing.T) {
	sem := newDynamicSemaphore(1)
	ctx := context.Background()

	if err := sem.Acquire(ctx); err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	// Second acquire should block. Use a channel to detect blocking.
	acquired := make(chan struct{})
	go func() {
		_ = sem.Acquire(ctx)
		close(acquired)
	}()

	select {
	case <-acquired:
		t.Fatal("second Acquire should have blocked")
	case <-time.After(50 * time.Millisecond):
		// Expected: still blocked.
	}

	// Release to unblock.
	sem.Release()
	select {
	case <-acquired:
		// Good — unblocked.
	case <-time.After(time.Second):
		t.Fatal("second Acquire did not unblock after Release")
	}
}

func TestDynamicSemaphore_ContextCancellation(t *testing.T) {
	sem := newDynamicSemaphore(1)
	ctx := context.Background()

	if err := sem.Acquire(ctx); err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	// Second acquire with a cancellable context.
	ctx2, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- sem.Acquire(ctx2)
	}()

	// Give the goroutine time to block.
	time.Sleep(20 * time.Millisecond)

	cancel()

	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Errorf("Acquire error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled Acquire did not return")
	}

	// Acquired count should still be 1 (the failed acquire should not increment).
	if sem.Acquired() != 1 {
		t.Errorf("Acquired() = %d, want 1", sem.Acquired())
	}
}

func TestDynamicSemaphore_ResizeUp(t *testing.T) {
	sem := newDynamicSemaphore(1)
	ctx := context.Background()

	if err := sem.Acquire(ctx); err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	// Second acquire should block.
	acquired := make(chan struct{})
	go func() {
		_ = sem.Acquire(ctx)
		close(acquired)
	}()

	select {
	case <-acquired:
		t.Fatal("should have blocked at limit 1")
	case <-time.After(50 * time.Millisecond):
	}

	// Resize up to 2 — should unblock.
	sem.SetLimit(2)

	select {
	case <-acquired:
		// Good.
	case <-time.After(time.Second):
		t.Fatal("did not unblock after SetLimit(2)")
	}

	if sem.Acquired() != 2 {
		t.Errorf("Acquired() = %d, want 2", sem.Acquired())
	}
}

func TestDynamicSemaphore_ResizeDown(t *testing.T) {
	sem := newDynamicSemaphore(3)
	ctx := context.Background()

	// Acquire 2 slots.
	if err := sem.Acquire(ctx); err != nil {
		t.Fatalf("Acquire 1: %v", err)
	}
	if err := sem.Acquire(ctx); err != nil {
		t.Fatalf("Acquire 2: %v", err)
	}

	// Resize down to 1 — existing acquires stay, new ones block.
	sem.SetLimit(1)

	acquired := make(chan struct{})
	go func() {
		_ = sem.Acquire(ctx)
		close(acquired)
	}()

	select {
	case <-acquired:
		t.Fatal("should have blocked at new limit 1 with 2 acquired")
	case <-time.After(50 * time.Millisecond):
	}

	// Release one — still at 1/1, should still block.
	sem.Release()

	select {
	case <-acquired:
		t.Fatal("should still block at 1/1")
	case <-time.After(50 * time.Millisecond):
	}

	// Release another — 0/1, should unblock.
	sem.Release()

	select {
	case <-acquired:
		// Good.
	case <-time.After(time.Second):
		t.Fatal("did not unblock after releases")
	}
}

func TestDynamicSemaphore_ResizeToUnlimited(t *testing.T) {
	sem := newDynamicSemaphore(1)
	ctx := context.Background()

	if err := sem.Acquire(ctx); err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	// Block on second acquire.
	acquired := make(chan struct{})
	go func() {
		_ = sem.Acquire(ctx)
		close(acquired)
	}()

	select {
	case <-acquired:
		t.Fatal("should have blocked")
	case <-time.After(50 * time.Millisecond):
	}

	// Resize to unlimited — should unblock.
	sem.SetLimit(0)

	select {
	case <-acquired:
		// Good.
	case <-time.After(time.Second):
		t.Fatal("did not unblock after SetLimit(0)")
	}
}

func TestDynamicSemaphore_ReleaseNeverNegative(t *testing.T) {
	sem := newDynamicSemaphore(1)

	// Release without acquire — should not go negative.
	sem.Release()
	if sem.Acquired() != 0 {
		t.Errorf("Acquired() = %d, want 0 (not negative)", sem.Acquired())
	}
}

func TestDynamicSemaphore_ConcurrentStress(t *testing.T) {
	sem := newDynamicSemaphore(5)
	ctx := context.Background()

	var completed atomic.Int32
	var wg sync.WaitGroup
	const goroutines = 50

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := sem.Acquire(ctx); err != nil {
				t.Errorf("Acquire: %v", err)
				return
			}
			completed.Add(1)
			// Simulate some work.
			time.Sleep(time.Millisecond)
			sem.Release()
		}()
	}

	wg.Wait()
	if completed.Load() != goroutines {
		t.Errorf("completed = %d, want %d", completed.Load(), goroutines)
	}
	if sem.Acquired() != 0 {
		t.Errorf("Acquired() = %d after all releases, want 0", sem.Acquired())
	}
}

func TestDynamicSemaphore_NegativeLimitClampedToUnlimited(t *testing.T) {
	// Constructor clamps negative to 0 (unlimited).
	sem := newDynamicSemaphore(-5)
	if sem.Limit() != 0 {
		t.Errorf("newDynamicSemaphore(-5).Limit() = %d, want 0", sem.Limit())
	}

	ctx := context.Background()
	// Should behave as unlimited.
	for i := range 10 {
		if err := sem.Acquire(ctx); err != nil {
			t.Fatalf("Acquire %d: %v", i, err)
		}
	}
	if sem.Acquired() != 10 {
		t.Errorf("Acquired() = %d, want 10", sem.Acquired())
	}

	// SetLimit also clamps negative to 0.
	sem2 := newDynamicSemaphore(2)
	sem2.SetLimit(-3)
	if sem2.Limit() != 0 {
		t.Errorf("SetLimit(-3).Limit() = %d, want 0", sem2.Limit())
	}
}

func TestDynamicSemaphore_ConcurrentResizeAndAcquire(t *testing.T) {
	sem := newDynamicSemaphore(1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var completed atomic.Int32
	var wg sync.WaitGroup

	// 20 goroutines trying to acquire/release.
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := sem.Acquire(ctx); err != nil {
				return
			}
			completed.Add(1)
			time.Sleep(time.Millisecond)
			sem.Release()
		}()
	}

	// Concurrently resize.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range 10 {
			sem.SetLimit(i%5 + 1) // cycle through 1..5
			time.Sleep(2 * time.Millisecond)
		}
	}()

	wg.Wait()
	if completed.Load() == 0 {
		t.Error("no goroutines completed")
	}
}
