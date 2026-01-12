package display

import (
	"sync"
	"testing"
)

// mockObserver is a test implementation of ResizeObserver
type mockObserver struct {
	running     bool
	resizeCalls []struct{ width, height int }
	resizeErr   error
	mu          sync.Mutex
}

func newMockObserver(running bool) *mockObserver {
	return &mockObserver{running: running}
}

func (m *mockObserver) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func (m *mockObserver) SetRunning(running bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = running
}

func (m *mockObserver) Resize(width, height int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resizeCalls = append(m.resizeCalls, struct{ width, height int }{width, height})
	return m.resizeErr
}

func (m *mockObserver) ResizeCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.resizeCalls)
}

func (m *mockObserver) LastResize() (width, height int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.resizeCalls) == 0 {
		return 0, 0
	}
	last := m.resizeCalls[len(m.resizeCalls)-1]
	return last.width, last.height
}

func TestNewManager(t *testing.T) {
	cfg := Config{
		DefaultWidth:  150,
		DefaultHeight: 40,
	}
	m := NewManager(cfg)

	if m == nil {
		t.Fatal("NewManager returned nil")
	}

	width, height := m.GetDimensions()
	if width != 150 {
		t.Errorf("expected default width 150, got %d", width)
	}
	if height != 40 {
		t.Errorf("expected default height 40, got %d", height)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.DefaultWidth != 200 {
		t.Errorf("expected DefaultWidth 200, got %d", cfg.DefaultWidth)
	}
	if cfg.DefaultHeight != 30 {
		t.Errorf("expected DefaultHeight 30, got %d", cfg.DefaultHeight)
	}
}

func TestSetDimensions(t *testing.T) {
	m := NewManager(DefaultConfig())

	m.SetDimensions(100, 50)

	width, height := m.GetDimensions()
	if width != 100 {
		t.Errorf("expected width 100, got %d", width)
	}
	if height != 50 {
		t.Errorf("expected height 50, got %d", height)
	}
}

func TestGetDimensions_ReturnsDefaultsWhenNotSet(t *testing.T) {
	cfg := Config{
		DefaultWidth:  120,
		DefaultHeight: 35,
	}
	m := NewManager(cfg)

	width, height := m.GetDimensions()
	if width != 120 {
		t.Errorf("expected width 120 (default), got %d", width)
	}
	if height != 35 {
		t.Errorf("expected height 35 (default), got %d", height)
	}
}

func TestGetDimensions_PartialDefaults(t *testing.T) {
	cfg := Config{
		DefaultWidth:  120,
		DefaultHeight: 35,
	}
	m := NewManager(cfg)

	// Set only width explicitly
	m.mu.Lock()
	m.width = 80
	m.mu.Unlock()

	width, height := m.GetDimensions()
	if width != 80 {
		t.Errorf("expected width 80, got %d", width)
	}
	if height != 35 {
		t.Errorf("expected height 35 (default), got %d", height)
	}
}

func TestNotifyResize_UpdatesDimensions(t *testing.T) {
	m := NewManager(DefaultConfig())

	m.NotifyResize(160, 45)

	width, height := m.GetDimensions()
	if width != 160 {
		t.Errorf("expected width 160, got %d", width)
	}
	if height != 45 {
		t.Errorf("expected height 45, got %d", height)
	}
}

func TestNotifyResize_NotifiesRunningObservers(t *testing.T) {
	m := NewManager(DefaultConfig())

	obs1 := newMockObserver(true)
	obs2 := newMockObserver(true)
	obs3 := newMockObserver(false) // Not running

	m.AddObserver(obs1)
	m.AddObserver(obs2)
	m.AddObserver(obs3)

	m.NotifyResize(180, 60)

	// Running observers should be notified
	if obs1.ResizeCallCount() != 1 {
		t.Errorf("expected obs1 to be called once, got %d", obs1.ResizeCallCount())
	}
	if obs2.ResizeCallCount() != 1 {
		t.Errorf("expected obs2 to be called once, got %d", obs2.ResizeCallCount())
	}

	// Non-running observer should NOT be notified
	if obs3.ResizeCallCount() != 0 {
		t.Errorf("expected obs3 (not running) to not be called, got %d", obs3.ResizeCallCount())
	}

	// Check correct dimensions were passed
	w, h := obs1.LastResize()
	if w != 180 || h != 60 {
		t.Errorf("expected obs1 resize(180, 60), got resize(%d, %d)", w, h)
	}
}

func TestAddObserver(t *testing.T) {
	m := NewManager(DefaultConfig())

	if m.ObserverCount() != 0 {
		t.Errorf("expected 0 observers initially, got %d", m.ObserverCount())
	}

	obs := newMockObserver(true)
	m.AddObserver(obs)

	if m.ObserverCount() != 1 {
		t.Errorf("expected 1 observer after add, got %d", m.ObserverCount())
	}
}

func TestAddObserver_NilIsIgnored(t *testing.T) {
	m := NewManager(DefaultConfig())

	m.AddObserver(nil)

	if m.ObserverCount() != 0 {
		t.Errorf("expected nil observer to be ignored, got count %d", m.ObserverCount())
	}
}

func TestRemoveObserver(t *testing.T) {
	m := NewManager(DefaultConfig())

	obs1 := newMockObserver(true)
	obs2 := newMockObserver(true)

	m.AddObserver(obs1)
	m.AddObserver(obs2)

	if m.ObserverCount() != 2 {
		t.Errorf("expected 2 observers, got %d", m.ObserverCount())
	}

	m.RemoveObserver(obs1)

	if m.ObserverCount() != 1 {
		t.Errorf("expected 1 observer after remove, got %d", m.ObserverCount())
	}

	// Verify obs2 still receives notifications
	m.NotifyResize(100, 50)
	if obs2.ResizeCallCount() != 1 {
		t.Errorf("expected remaining observer to be notified, got %d calls", obs2.ResizeCallCount())
	}
}

func TestRemoveObserver_NilIsIgnored(t *testing.T) {
	m := NewManager(DefaultConfig())
	obs := newMockObserver(true)
	m.AddObserver(obs)

	// Should not panic
	m.RemoveObserver(nil)

	if m.ObserverCount() != 1 {
		t.Errorf("expected observer count unchanged, got %d", m.ObserverCount())
	}
}

func TestRemoveObserver_NonExistent(t *testing.T) {
	m := NewManager(DefaultConfig())
	obs1 := newMockObserver(true)
	obs2 := newMockObserver(true)

	m.AddObserver(obs1)

	// Removing non-existent observer should be safe
	m.RemoveObserver(obs2)

	if m.ObserverCount() != 1 {
		t.Errorf("expected observer count unchanged, got %d", m.ObserverCount())
	}
}

func TestNotifyResize_ObserverBecomesStopped(t *testing.T) {
	m := NewManager(DefaultConfig())

	obs := newMockObserver(true)
	m.AddObserver(obs)

	// First resize while running
	m.NotifyResize(100, 50)
	if obs.ResizeCallCount() != 1 {
		t.Errorf("expected 1 resize call while running, got %d", obs.ResizeCallCount())
	}

	// Stop the observer
	obs.SetRunning(false)

	// Second resize while stopped
	m.NotifyResize(120, 60)
	if obs.ResizeCallCount() != 1 {
		t.Errorf("expected still 1 resize call after stopping, got %d", obs.ResizeCallCount())
	}
}

func TestSetDimensions_DoesNotNotifyObservers(t *testing.T) {
	m := NewManager(DefaultConfig())

	obs := newMockObserver(true)
	m.AddObserver(obs)

	m.SetDimensions(150, 75)

	if obs.ResizeCallCount() != 0 {
		t.Errorf("SetDimensions should not notify observers, got %d calls", obs.ResizeCallCount())
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := NewManager(DefaultConfig())

	const goroutines = 10
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 3) // readers, writers, resizers

	// Concurrent readers
	for range goroutines {
		go func() {
			defer wg.Done()
			for range iterations {
				m.GetDimensions()
			}
		}()
	}

	// Concurrent writers
	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				m.SetDimensions(id*10+j, id*10+j)
			}
		}(i)
	}

	// Concurrent resizers
	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				m.NotifyResize(id*10+j, id*10+j)
			}
		}(i)
	}

	wg.Wait()
}

func TestConcurrentObserverManagement(t *testing.T) {
	m := NewManager(DefaultConfig())

	var wg sync.WaitGroup
	const iterations = 50

	// Concurrent observer additions and removals
	for range iterations {
		wg.Add(2)
		obs := newMockObserver(true)

		go func() {
			defer wg.Done()
			m.AddObserver(obs)
		}()

		go func() {
			defer wg.Done()
			m.RemoveObserver(obs)
		}()
	}

	wg.Wait()
}

func TestNotifyResize_ConcurrentWithObserverChanges(t *testing.T) {
	m := NewManager(DefaultConfig())

	obs := newMockObserver(true)

	// Start continuous resizing in a separate goroutine
	done := make(chan struct{})
	started := make(chan struct{})
	go func() {
		close(started) // Signal that we've started
		for {
			select {
			case <-done:
				return
			default:
				m.NotifyResize(100, 50)
			}
		}
	}()

	// Wait for the goroutine to start
	<-started

	// Concurrently add and remove the observer
	for range 100 {
		m.AddObserver(obs)
		m.RemoveObserver(obs)
	}

	close(done)

	// The test passes if we didn't deadlock or panic.
}
