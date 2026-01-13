package namer

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// mockClient implements Client for testing.
type mockClient struct {
	summarizeFunc func(ctx context.Context, task, output string) (string, error)
	callCount     int
	mu            sync.Mutex
}

func (m *mockClient) Summarize(ctx context.Context, task, output string) (string, error) {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()
	if m.summarizeFunc != nil {
		return m.summarizeFunc(ctx, task, output)
	}
	return "Generated name", nil
}

func TestNamer_RequestRename_Success(t *testing.T) {
	client := &mockClient{}
	namer := New(client, nil)

	var callbackCalled bool
	var receivedID, receivedName string
	var wg sync.WaitGroup
	wg.Add(1)

	namer.OnRename(func(instanceID, newName string) {
		receivedID = instanceID
		receivedName = newName
		callbackCalled = true
		wg.Done()
	})

	namer.Start()
	defer namer.Stop()

	namer.RequestRename("inst-1", "Fix authentication bug", "Reading auth.go...")

	// Wait for callback with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for callback")
	}

	if !callbackCalled {
		t.Error("expected callback to be called")
	}
	if receivedID != "inst-1" {
		t.Errorf("expected instance ID 'inst-1', got '%s'", receivedID)
	}
	if receivedName != "Generated name" {
		t.Errorf("expected name 'Generated name', got '%s'", receivedName)
	}
}

func TestNamer_RequestRename_SkipsDuplicate(t *testing.T) {
	client := &mockClient{}
	namer := New(client, nil)

	var callCount int
	var mu sync.Mutex
	namer.OnRename(func(_, _ string) {
		mu.Lock()
		callCount++
		mu.Unlock()
	})

	namer.Start()
	defer namer.Stop()

	// Request rename for same instance multiple times
	namer.RequestRename("inst-1", "Task 1", "Output 1")
	namer.RequestRename("inst-1", "Task 1", "Output 2")
	namer.RequestRename("inst-1", "Task 1", "Output 3")

	// Wait for processing
	time.Sleep(time.Second)

	mu.Lock()
	count := callCount
	mu.Unlock()

	// Should only be called once
	if count != 1 {
		t.Errorf("expected callback to be called once, got %d times", count)
	}
}

func TestNamer_RequestRename_APIError(t *testing.T) {
	client := &mockClient{
		summarizeFunc: func(_ context.Context, _, _ string) (string, error) {
			return "", errors.New("API error")
		},
	}
	namer := New(client, nil)

	var callbackCalled bool
	namer.OnRename(func(_, _ string) {
		callbackCalled = true
	})

	namer.Start()
	defer namer.Stop()

	namer.RequestRename("inst-1", "Task", "Output")

	// Wait for processing
	time.Sleep(time.Second)

	// Callback should NOT be called on error
	if callbackCalled {
		t.Error("expected callback NOT to be called on API error")
	}

	// But instance should be marked as renamed to prevent retries
	if !namer.IsRenamed("inst-1") {
		t.Error("expected instance to be marked as renamed after error")
	}
}

func TestNamer_IsRenamed(t *testing.T) {
	client := &mockClient{}
	namer := New(client, nil)
	namer.Start()
	defer namer.Stop()

	// Initially not renamed
	if namer.IsRenamed("inst-1") {
		t.Error("expected instance NOT to be renamed initially")
	}

	// Request rename
	namer.RequestRename("inst-1", "Task", "Output")

	// Wait for processing
	time.Sleep(time.Second)

	// Now should be renamed
	if !namer.IsRenamed("inst-1") {
		t.Error("expected instance to be renamed after processing")
	}
}

func TestNamer_Reset(t *testing.T) {
	client := &mockClient{}
	namer := New(client, nil)

	var callCount int
	var mu sync.Mutex
	namer.OnRename(func(_, _ string) {
		mu.Lock()
		callCount++
		mu.Unlock()
	})

	namer.Start()
	defer namer.Stop()

	// First rename
	namer.RequestRename("inst-1", "Task 1", "Output 1")
	time.Sleep(500 * time.Millisecond)

	// Reset the renamed state
	namer.Reset("inst-1")

	// Should not be marked as renamed anymore
	if namer.IsRenamed("inst-1") {
		t.Error("expected instance NOT to be renamed after reset")
	}

	// Second rename should work
	namer.RequestRename("inst-1", "Task 2", "Output 2")
	time.Sleep(time.Second)

	mu.Lock()
	count := callCount
	mu.Unlock()

	if count != 2 {
		t.Errorf("expected callback to be called twice after reset, got %d", count)
	}
}

func TestNamer_MultipleInstances(t *testing.T) {
	client := &mockClient{
		summarizeFunc: func(_ context.Context, task, _ string) (string, error) {
			// Return different names based on task
			return "Name for: " + task[:10], nil
		},
	}
	namer := New(client, nil)

	results := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(3)

	namer.OnRename(func(instanceID, newName string) {
		mu.Lock()
		results[instanceID] = newName
		mu.Unlock()
		wg.Done()
	})

	namer.Start()
	defer namer.Stop()

	namer.RequestRename("inst-1", "First task description", "Output 1")
	namer.RequestRename("inst-2", "Second task description", "Output 2")
	namer.RequestRename("inst-3", "Third task description", "Output 3")

	// Wait for all callbacks
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for callbacks")
	}

	mu.Lock()
	defer mu.Unlock()

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	// Each instance should have a unique name
	if results["inst-1"] == results["inst-2"] || results["inst-2"] == results["inst-3"] {
		t.Error("expected unique names for each instance")
	}
}

func TestNamer_StopGracefully(t *testing.T) {
	client := &mockClient{
		summarizeFunc: func(_ context.Context, _, _ string) (string, error) {
			time.Sleep(100 * time.Millisecond)
			return "Name", nil
		},
	}
	namer := New(client, nil)
	namer.Start()

	// Queue some requests
	namer.RequestRename("inst-1", "Task 1", "Output 1")

	// Stop should not hang
	done := make(chan struct{})
	go func() {
		namer.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return in time")
	}
}

func TestNamer_NoCallback(t *testing.T) {
	client := &mockClient{}
	namer := New(client, nil)
	// Don't set a callback

	namer.Start()
	defer namer.Stop()

	// Should not panic when callback is nil
	namer.RequestRename("inst-1", "Task", "Output")

	time.Sleep(500 * time.Millisecond)

	// Instance should still be marked as renamed
	if !namer.IsRenamed("inst-1") {
		t.Error("expected instance to be renamed even without callback")
	}
}

func TestNamer_QueueFull(t *testing.T) {
	// Create a slow client to back up the queue
	client := &mockClient{
		summarizeFunc: func(_ context.Context, _, _ string) (string, error) {
			time.Sleep(time.Second)
			return "Name", nil
		},
	}
	namer := New(client, nil)
	namer.Start()
	defer namer.Stop()

	// Fill the queue (pendingQueueSize is 20)
	for range 30 {
		namer.RequestRename("inst-overflow", "Task", "Output")
	}

	// Should not block or panic
	// Some requests will be dropped, which is expected behavior
}

func TestNamer_New_NilClientPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil client, got none")
		}
	}()

	New(nil, nil)
}

func TestNamer_Start_DoubleStartSafe(t *testing.T) {
	client := &mockClient{}
	namer := New(client, nil)

	// Start twice should not spawn multiple goroutines
	namer.Start()
	namer.Start() // Should be a no-op
	namer.Start() // Should be a no-op

	defer namer.Stop()

	// Should still work correctly
	var wg sync.WaitGroup
	wg.Add(1)
	namer.OnRename(func(instanceID, newName string) {
		wg.Done()
	})

	namer.RequestRename("inst-1", "Task", "Output")

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - only one callback fired
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for callback")
	}
}

func TestNamer_Stop_DoubleStopSafe(t *testing.T) {
	client := &mockClient{}
	namer := New(client, nil)
	namer.Start()

	// Stop twice should not panic
	namer.Stop()
	namer.Stop()
	namer.Stop()
}
