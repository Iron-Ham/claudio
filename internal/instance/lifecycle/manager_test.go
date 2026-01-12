package lifecycle

import (
	"sync"
	"testing"
	"time"
)

// mockInstance implements the Instance interface for testing.
type mockInstance struct {
	mu          sync.Mutex
	id          string
	sessionName string
	workDir     string
	task        string
	config      InstanceConfig
	running     bool
	startTime   time.Time
	startCalled bool
	stopCalled  bool
}

func newMockInstance(id string) *mockInstance {
	return &mockInstance{
		id:          id,
		sessionName: "claudio-" + id,
		workDir:     "/tmp/test",
		task:        "test task",
		config: InstanceConfig{
			TmuxWidth:  200,
			TmuxHeight: 30,
		},
	}
}

func (m *mockInstance) ID() string          { return m.id }
func (m *mockInstance) SessionName() string { return m.sessionName }
func (m *mockInstance) WorkDir() string     { return m.workDir }
func (m *mockInstance) Task() string        { return m.task }
func (m *mockInstance) Config() InstanceConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.config
}

func (m *mockInstance) SetRunning(running bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = running
}

func (m *mockInstance) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func (m *mockInstance) SetStartTime(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startTime = t
}

func (m *mockInstance) OnStarted() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startCalled = true
}

func (m *mockInstance) OnStopped() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopCalled = true
}

func (m *mockInstance) wasStopCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopCalled
}

func TestNewManager(t *testing.T) {
	mgr := NewManager(nil)
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.instanceStates == nil {
		t.Error("instanceStates map should be initialized")
	}
	if mgr.gracefulStopTimeout != 500*time.Millisecond {
		t.Errorf("gracefulStopTimeout = %v, want %v", mgr.gracefulStopTimeout, 500*time.Millisecond)
	}
}

func TestState_String(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateStopped, "stopped"},
		{StateStarting, "starting"},
		{StateRunning, "running"},
		{StateReady, "ready"},
		{StateStopping, "stopping"},
		{State(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("State.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestManager_GetState_NilInstance(t *testing.T) {
	mgr := NewManager(nil)
	state := mgr.GetState(nil)
	if state != StateStopped {
		t.Errorf("GetState(nil) = %v, want StateStopped", state)
	}
}

func TestManager_GetState_UnknownInstance(t *testing.T) {
	mgr := NewManager(nil)
	inst := newMockInstance("unknown")
	state := mgr.GetState(inst)
	if state != StateStopped {
		t.Errorf("GetState for unknown instance = %v, want StateStopped", state)
	}
}

func TestManager_Start_NilInstance(t *testing.T) {
	mgr := NewManager(nil)
	err := mgr.Start(nil)
	if err != ErrInvalidInstance {
		t.Errorf("Start(nil) = %v, want ErrInvalidInstance", err)
	}
}

func TestManager_Stop_NilInstance(t *testing.T) {
	mgr := NewManager(nil)
	err := mgr.Stop(nil)
	if err != ErrInvalidInstance {
		t.Errorf("Stop(nil) = %v, want ErrInvalidInstance", err)
	}
}

func TestManager_Stop_NotRunning(t *testing.T) {
	mgr := NewManager(nil)
	inst := newMockInstance("test")

	// Stop should succeed (no-op) when not running
	err := mgr.Stop(inst)
	if err != nil {
		t.Errorf("Stop on non-running instance should not error, got: %v", err)
	}
}

func TestManager_Restart_NilInstance(t *testing.T) {
	mgr := NewManager(nil)
	err := mgr.Restart(nil)
	if err != ErrInvalidInstance {
		t.Errorf("Restart(nil) = %v, want ErrInvalidInstance", err)
	}
}

func TestManager_WaitForReady_NilInstance(t *testing.T) {
	mgr := NewManager(nil)
	err := mgr.WaitForReady(nil, time.Second)
	if err != ErrInvalidInstance {
		t.Errorf("WaitForReady(nil) = %v, want ErrInvalidInstance", err)
	}
}

func TestManager_WaitForReady_NotRunning(t *testing.T) {
	mgr := NewManager(nil)
	inst := newMockInstance("test")
	inst.SetRunning(false)

	err := mgr.WaitForReady(inst, 100*time.Millisecond)
	if err != ErrNotRunning {
		t.Errorf("WaitForReady on non-running instance = %v, want ErrNotRunning", err)
	}
}

func TestManager_WaitForReady_Timeout(t *testing.T) {
	mgr := NewManager(nil)
	inst := newMockInstance("test")
	inst.SetRunning(true)

	// Set a checker that always returns false
	mgr.SetReadinessChecker(func(Instance) bool {
		return false
	})

	err := mgr.WaitForReady(inst, 100*time.Millisecond)
	if err != ErrReadyTimeout {
		t.Errorf("WaitForReady with never-ready checker = %v, want ErrReadyTimeout", err)
	}
}

func TestManager_WaitForReady_ImmediatelyReady(t *testing.T) {
	mgr := NewManager(nil)
	inst := newMockInstance("test")
	inst.SetRunning(true)

	// Set a checker that always returns true
	mgr.SetReadinessChecker(func(Instance) bool {
		return true
	})

	err := mgr.WaitForReady(inst, time.Second)
	if err != nil {
		t.Errorf("WaitForReady with always-ready checker should succeed, got: %v", err)
	}

	// State should be Ready
	if state := mgr.GetState(inst); state != StateReady {
		t.Errorf("State after WaitForReady = %v, want StateReady", state)
	}
}

func TestManager_WaitForReady_BecomesReady(t *testing.T) {
	mgr := NewManager(nil)
	inst := newMockInstance("test")
	inst.SetRunning(true)

	// Set a checker that becomes ready after a few calls
	callCount := 0
	mgr.SetReadinessChecker(func(Instance) bool {
		callCount++
		return callCount >= 3
	})

	err := mgr.WaitForReady(inst, time.Second)
	if err != nil {
		t.Errorf("WaitForReady should succeed when instance becomes ready, got: %v", err)
	}
	if callCount < 3 {
		t.Errorf("Readiness checker should have been called at least 3 times, got %d", callCount)
	}
}

func TestManager_Reconnect_NilInstance(t *testing.T) {
	mgr := NewManager(nil)
	err := mgr.Reconnect(nil)
	if err != ErrInvalidInstance {
		t.Errorf("Reconnect(nil) = %v, want ErrInvalidInstance", err)
	}
}

func TestManager_Reconnect_SessionNotFound(t *testing.T) {
	mgr := NewManager(nil)
	inst := newMockInstance("nonexistent-session-xyz123")

	err := mgr.Reconnect(inst)
	if err != ErrSessionNotFound {
		t.Errorf("Reconnect to non-existent session = %v, want ErrSessionNotFound", err)
	}
}

func TestManager_SessionExists_NonExistent(t *testing.T) {
	mgr := NewManager(nil)
	if mgr.SessionExists("nonexistent-test-session-abc") {
		t.Error("SessionExists should return false for non-existent session")
	}
}

func TestManager_SetGracefulStopTimeout(t *testing.T) {
	mgr := NewManager(nil)
	newTimeout := 2 * time.Second
	mgr.SetGracefulStopTimeout(newTimeout)

	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if mgr.gracefulStopTimeout != newTimeout {
		t.Errorf("gracefulStopTimeout = %v, want %v", mgr.gracefulStopTimeout, newTimeout)
	}
}

func TestManager_SetReadinessChecker(t *testing.T) {
	mgr := NewManager(nil)
	called := false
	mgr.SetReadinessChecker(func(Instance) bool {
		called = true
		return true
	})

	inst := newMockInstance("test")
	inst.SetRunning(true)
	_ = mgr.isReady(inst)

	if !called {
		t.Error("Custom readiness checker should have been called")
	}
}

func TestManager_ClearState(t *testing.T) {
	mgr := NewManager(nil)
	inst := newMockInstance("test")

	// Set a state
	mgr.mu.Lock()
	mgr.instanceStates[inst.ID()] = StateRunning
	mgr.mu.Unlock()

	// Clear it
	mgr.ClearState(inst)

	// Should be gone
	mgr.mu.Lock()
	_, exists := mgr.instanceStates[inst.ID()]
	mgr.mu.Unlock()

	if exists {
		t.Error("ClearState should remove instance from instanceStates map")
	}
}

func TestManager_ClearState_NilInstance(t *testing.T) {
	mgr := NewManager(nil)
	// Should not panic
	mgr.ClearState(nil)
}

func TestManager_StateTransitions_StopFromStopped(t *testing.T) {
	mgr := NewManager(nil)
	inst := newMockInstance("test")

	// Instance starts in Stopped state
	if state := mgr.GetState(inst); state != StateStopped {
		t.Errorf("Initial state = %v, want StateStopped", state)
	}

	// Stop should be no-op
	err := mgr.Stop(inst)
	if err != nil {
		t.Errorf("Stop from Stopped state should succeed, got: %v", err)
	}

	// OnStopped should not be called since we weren't running
	if inst.wasStopCalled() {
		t.Error("OnStopped should not be called when stopping from Stopped state")
	}
}

func TestManager_DefaultReadinessChecker(t *testing.T) {
	mgr := NewManager(nil)
	// Use a unique session name to avoid conflicts with any existing tmux sessions
	inst := newMockInstance("nonexistent-readiness-check-xyz789")

	// Not running -> not ready
	inst.SetRunning(false)
	if mgr.isReady(inst) {
		t.Error("isReady should return false when instance is not running")
	}

	// Running but session doesn't exist -> not ready (in real tmux scenario)
	inst.SetRunning(true)
	// The default checker checks SessionExists which will fail for our mock
	// Since we're using a non-existent session name, this should return false
	if mgr.isReady(inst) {
		t.Error("isReady should return false when tmux session doesn't exist")
	}
}

func TestManager_ConcurrentGetState(t *testing.T) {
	mgr := NewManager(nil)
	inst := newMockInstance("concurrent-test")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = mgr.GetState(inst)
		}()
	}
	wg.Wait()
}

func TestManager_ConcurrentStateUpdates(t *testing.T) {
	mgr := NewManager(nil)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			inst := newMockInstance("concurrent-" + string(rune('a'+id%26)))
			mgr.mu.Lock()
			mgr.instanceStates[inst.ID()] = StateRunning
			mgr.mu.Unlock()
			_ = mgr.GetState(inst)
			mgr.ClearState(inst)
		}(i)
	}
	wg.Wait()
}

func TestInstanceConfig_Defaults(t *testing.T) {
	cfg := InstanceConfig{}
	if cfg.TmuxWidth != 0 || cfg.TmuxHeight != 0 {
		t.Error("Default InstanceConfig should have zero values")
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"ErrAlreadyRunning", ErrAlreadyRunning, "instance already running"},
		{"ErrNotRunning", ErrNotRunning, "instance not running"},
		{"ErrSessionNotFound", ErrSessionNotFound, "tmux session not found"},
		{"ErrReadyTimeout", ErrReadyTimeout, "timeout waiting for instance to be ready"},
		{"ErrInvalidInstance", ErrInvalidInstance, "invalid instance: nil"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("%s.Error() = %q, want %q", tt.name, tt.err.Error(), tt.expected)
			}
		})
	}
}

func TestManager_WaitForReady_InstanceStopsDuringWait(t *testing.T) {
	mgr := NewManager(nil)
	inst := newMockInstance("test")
	inst.SetRunning(true)

	checkCount := 0
	mgr.SetReadinessChecker(func(i Instance) bool {
		checkCount++
		if checkCount >= 2 {
			// Simulate instance stopping
			inst.SetRunning(false)
		}
		return false
	})

	err := mgr.WaitForReady(inst, time.Second)
	if err != ErrNotRunning {
		t.Errorf("WaitForReady when instance stops = %v, want ErrNotRunning", err)
	}
}

func TestMockInstance_ThreadSafety(t *testing.T) {
	inst := newMockInstance("thread-safe")

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			inst.SetRunning(true)
			_ = inst.IsRunning()
		}()
		go func() {
			defer wg.Done()
			inst.SetRunning(false)
			_ = inst.IsRunning()
		}()
	}
	wg.Wait()
}
