package prworkflow

import (
	"sync"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/event"
)

func TestNewManager(t *testing.T) {
	cfg := Config{
		UseAI:      true,
		Draft:      true,
		AutoRebase: false,
		TmuxWidth:  120,
		TmuxHeight: 40,
	}
	eventBus := event.NewBus()

	m := NewManager(cfg, "test-session", eventBus)

	if m == nil {
		t.Fatal("NewManager returned nil")
	}

	if m.config.UseAI != cfg.UseAI {
		t.Errorf("UseAI = %v, want %v", m.config.UseAI, cfg.UseAI)
	}
	if m.config.Draft != cfg.Draft {
		t.Errorf("Draft = %v, want %v", m.config.Draft, cfg.Draft)
	}
	if m.sessionID != "test-session" {
		t.Errorf("sessionID = %q, want %q", m.sessionID, "test-session")
	}
	if m.eventBus != eventBus {
		t.Error("eventBus not set correctly")
	}
	if m.workflows == nil {
		t.Error("workflows map not initialized")
	}
}

func TestNewConfigFromConfig(t *testing.T) {
	globalCfg := &config.Config{
		PR: config.PRConfig{
			UseAI:      true,
			Draft:      false,
			AutoRebase: true,
		},
		Instance: config.InstanceConfig{
			TmuxWidth:  160,
			TmuxHeight: 50,
		},
	}

	cfg := NewConfigFromConfig(globalCfg)

	if cfg.UseAI != globalCfg.PR.UseAI {
		t.Errorf("UseAI = %v, want %v", cfg.UseAI, globalCfg.PR.UseAI)
	}
	if cfg.Draft != globalCfg.PR.Draft {
		t.Errorf("Draft = %v, want %v", cfg.Draft, globalCfg.PR.Draft)
	}
	if cfg.AutoRebase != globalCfg.PR.AutoRebase {
		t.Errorf("AutoRebase = %v, want %v", cfg.AutoRebase, globalCfg.PR.AutoRebase)
	}
	if cfg.TmuxWidth != globalCfg.Instance.TmuxWidth {
		t.Errorf("TmuxWidth = %v, want %v", cfg.TmuxWidth, globalCfg.Instance.TmuxWidth)
	}
	if cfg.TmuxHeight != globalCfg.Instance.TmuxHeight {
		t.Errorf("TmuxHeight = %v, want %v", cfg.TmuxHeight, globalCfg.Instance.TmuxHeight)
	}
}

func TestSetDisplayDimensions(t *testing.T) {
	m := NewManager(Config{TmuxWidth: 80, TmuxHeight: 24}, "", nil)

	m.SetDisplayDimensions(200, 60)

	m.mu.RLock()
	width := m.displayWidth
	height := m.displayHeight
	m.mu.RUnlock()

	if width != 200 {
		t.Errorf("displayWidth = %d, want %d", width, 200)
	}
	if height != 60 {
		t.Errorf("displayHeight = %d, want %d", height, 60)
	}
}

func TestSetCompleteCallback(t *testing.T) {
	m := NewManager(Config{}, "", nil)

	called := false
	cb := func(instanceID string, success bool) {
		called = true
	}

	m.SetCompleteCallback(cb)

	m.mu.RLock()
	hasCallback := m.completeCallback != nil
	m.mu.RUnlock()

	if !hasCallback {
		t.Error("completeCallback not set")
	}

	// Verify it can be called
	m.completeCallback("test", true)
	if !called {
		t.Error("callback was not invoked")
	}
}

func TestSetOpenedCallback(t *testing.T) {
	m := NewManager(Config{}, "", nil)

	called := false
	cb := func(instanceID string) {
		called = true
	}

	m.SetOpenedCallback(cb)

	m.mu.RLock()
	hasCallback := m.openedCallback != nil
	m.mu.RUnlock()

	if !hasCallback {
		t.Error("openedCallback not set")
	}

	// Verify it can be called
	m.openedCallback("test")
	if !called {
		t.Error("callback was not invoked")
	}
}

func TestGet_NoWorkflow(t *testing.T) {
	m := NewManager(Config{}, "", nil)

	workflow := m.Get("nonexistent")

	if workflow != nil {
		t.Error("Get() should return nil for nonexistent workflow")
	}
}

func TestRunning_NoWorkflow(t *testing.T) {
	m := NewManager(Config{}, "", nil)

	running := m.Running("nonexistent")

	if running {
		t.Error("Running() should return false for nonexistent workflow")
	}
}

func TestCount_Empty(t *testing.T) {
	m := NewManager(Config{}, "", nil)

	count := m.Count()

	if count != 0 {
		t.Errorf("Count() = %d, want 0", count)
	}
}

func TestIDs_Empty(t *testing.T) {
	m := NewManager(Config{}, "", nil)

	ids := m.IDs()

	if len(ids) != 0 {
		t.Errorf("IDs() returned %d items, want 0", len(ids))
	}
}

func TestStop_NoWorkflow(t *testing.T) {
	m := NewManager(Config{}, "", nil)

	err := m.Stop("nonexistent")

	if err != nil {
		t.Errorf("Stop() returned unexpected error: %v", err)
	}
}

func TestStopAll_Empty(t *testing.T) {
	m := NewManager(Config{}, "", nil)

	// Should not panic
	m.StopAll()

	count := m.Count()
	if count != 0 {
		t.Errorf("Count() after StopAll = %d, want 0", count)
	}
}

func TestHandleComplete_WithCallback(t *testing.T) {
	eventBus := event.NewBus()
	m := NewManager(Config{}, "", eventBus)

	var capturedID string
	var capturedSuccess bool
	var callbackCalled bool

	m.SetCompleteCallback(func(instanceID string, success bool) {
		callbackCalled = true
		capturedID = instanceID
		capturedSuccess = success
	})

	// Simulate having a workflow (add it directly to test cleanup)
	m.mu.Lock()
	m.workflows["test-id"] = nil // We just need the key to test cleanup
	m.mu.Unlock()

	m.HandleComplete("test-id", true, "output text")

	if !callbackCalled {
		t.Error("callback was not called")
	}
	if capturedID != "test-id" {
		t.Errorf("capturedID = %q, want %q", capturedID, "test-id")
	}
	if !capturedSuccess {
		t.Error("capturedSuccess should be true")
	}

	// Verify workflow was cleaned up
	if m.Get("test-id") != nil {
		t.Error("workflow should have been removed")
	}
}

func TestHandleComplete_WithEventBus(t *testing.T) {
	eventBus := event.NewBus()
	m := NewManager(Config{}, "", eventBus)

	// Subscribe to events
	var receivedEvent event.Event
	var wg sync.WaitGroup
	wg.Add(1)
	eventBus.Subscribe("pr.completed", func(e event.Event) {
		receivedEvent = e
		wg.Done()
	})

	m.HandleComplete("test-id", true, "output text")

	// Wait for event with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Event received
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}

	if receivedEvent.EventType() != "pr.completed" {
		t.Errorf("event type = %v, want %v", receivedEvent.EventType(), "pr.completed")
	}

	// Type assert to get the specific event type
	prEvent, ok := receivedEvent.(event.PRCompleteEvent)
	if !ok {
		t.Fatal("event is not a PRCompleteEvent")
	}
	if prEvent.InstanceID != "test-id" {
		t.Errorf("event instanceID = %q, want %q", prEvent.InstanceID, "test-id")
	}
}

func TestHandleComplete_NoCallbackOrEventBus(t *testing.T) {
	m := NewManager(Config{}, "", nil) // nil eventBus

	// Should not panic
	m.HandleComplete("test-id", false, "")
}

func TestConcurrentAccess(t *testing.T) {
	m := NewManager(Config{}, "session", nil)

	// Test concurrent reads and writes
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(3)
		id := string(rune('a' + i))

		go func(id string) {
			defer wg.Done()
			m.SetDisplayDimensions(100, 50)
		}(id)

		go func(id string) {
			defer wg.Done()
			_ = m.Get(id)
		}(id)

		go func(id string) {
			defer wg.Done()
			_ = m.Running(id)
		}(id)
	}

	wg.Wait()
}

func TestManagerWithNilEventBus(t *testing.T) {
	m := NewManager(Config{
		TmuxWidth:  120,
		TmuxHeight: 40,
	}, "", nil)

	// Ensure nil eventBus doesn't cause panic during HandleComplete
	m.HandleComplete("test-id", true, "output")

	// Should complete without panic
	if m.Count() != 0 {
		t.Errorf("Count() = %d, want 0", m.Count())
	}
}

func TestNewManager_EmptySessionID(t *testing.T) {
	m := NewManager(Config{}, "", nil)

	if m.sessionID != "" {
		t.Errorf("sessionID = %q, want empty string", m.sessionID)
	}
}

func TestSetLogger(t *testing.T) {
	m := NewManager(Config{}, "", nil)

	// Just verify it doesn't panic with nil logger
	m.SetLogger(nil)

	m.mu.RLock()
	logger := m.logger
	m.mu.RUnlock()

	if logger != nil {
		t.Error("logger should be nil")
	}
}
