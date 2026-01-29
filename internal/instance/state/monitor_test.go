package state

import (
	"sync"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/instance/detect"
)

func TestNewMonitor(t *testing.T) {
	cfg := MonitorConfig{
		ActivityTimeoutMinutes:   15,
		CompletionTimeoutMinutes: 60,
		StaleDetection:           true,
		StaleThreshold:           1000,
	}

	m := NewMonitor(cfg)

	if m == nil {
		t.Fatal("NewMonitor returned nil")
	}

	if m.config.ActivityTimeoutMinutes != 15 {
		t.Errorf("ActivityTimeoutMinutes = %d, want 15", m.config.ActivityTimeoutMinutes)
	}

	if m.config.CompletionTimeoutMinutes != 60 {
		t.Errorf("CompletionTimeoutMinutes = %d, want 60", m.config.CompletionTimeoutMinutes)
	}

	if !m.config.StaleDetection {
		t.Error("StaleDetection should be true")
	}

	if m.config.StaleThreshold != 1000 {
		t.Errorf("StaleThreshold = %d, want 1000", m.config.StaleThreshold)
	}
}

func TestNewMonitor_DefaultStaleThreshold(t *testing.T) {
	cfg := MonitorConfig{
		StaleDetection: true,
		StaleThreshold: 0, // Should be set to default
	}

	m := NewMonitor(cfg)

	if m.config.StaleThreshold != 3000 {
		t.Errorf("StaleThreshold = %d, want 3000 (default)", m.config.StaleThreshold)
	}
}

func TestNewMonitorWithDefaults(t *testing.T) {
	m := NewMonitorWithDefaults()

	if m == nil {
		t.Fatal("NewMonitorWithDefaults returned nil")
	}

	defaults := DefaultMonitorConfig()
	if m.config.ActivityTimeoutMinutes != defaults.ActivityTimeoutMinutes {
		t.Errorf("ActivityTimeoutMinutes = %d, want %d", m.config.ActivityTimeoutMinutes, defaults.ActivityTimeoutMinutes)
	}
}

func TestDefaultMonitorConfig(t *testing.T) {
	cfg := DefaultMonitorConfig()

	if cfg.ActivityTimeoutMinutes != 30 {
		t.Errorf("ActivityTimeoutMinutes = %d, want 30", cfg.ActivityTimeoutMinutes)
	}

	if cfg.CompletionTimeoutMinutes != 0 {
		t.Errorf("CompletionTimeoutMinutes = %d, want 0 (disabled)", cfg.CompletionTimeoutMinutes)
	}

	if !cfg.StaleDetection {
		t.Error("StaleDetection should be true by default")
	}

	if cfg.StaleThreshold != 3000 {
		t.Errorf("StaleThreshold = %d, want 3000", cfg.StaleThreshold)
	}
}

func TestTimeoutType_String(t *testing.T) {
	tests := []struct {
		tt   TimeoutType
		want string
	}{
		{TimeoutActivity, "activity"},
		{TimeoutCompletion, "completion"},
		{TimeoutStale, "stale"},
		{TimeoutType(99), "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.tt.String(); got != tc.want {
				t.Errorf("TimeoutType(%d).String() = %q, want %q", tc.tt, got, tc.want)
			}
		})
	}
}

func TestMonitor_StartStop(t *testing.T) {
	m := NewMonitorWithDefaults()

	// Start monitoring
	m.Start("inst-1")

	if !m.IsMonitoring("inst-1") {
		t.Error("IsMonitoring should return true after Start")
	}

	// Starting again should be no-op
	m.Start("inst-1")
	if len(m.MonitoredInstances()) != 1 {
		t.Errorf("Expected 1 monitored instance, got %d", len(m.MonitoredInstances()))
	}

	// Stop monitoring
	stopped := m.Stop("inst-1")
	if !stopped {
		t.Error("Stop should return true for monitored instance")
	}

	if m.IsMonitoring("inst-1") {
		t.Error("IsMonitoring should return false after Stop")
	}

	// Stopping again should return false
	stopped = m.Stop("inst-1")
	if stopped {
		t.Error("Stop should return false for non-monitored instance")
	}
}

func TestMonitor_StartWithTime(t *testing.T) {
	m := NewMonitorWithDefaults()

	customTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	m.StartWithTime("inst-1", customTime)

	startTime := m.GetStartTime("inst-1")
	if startTime == nil {
		t.Fatal("GetStartTime returned nil")
	}

	if !startTime.Equal(customTime) {
		t.Errorf("GetStartTime = %v, want %v", startTime, customTime)
	}
}

func TestMonitor_GetState(t *testing.T) {
	m := NewMonitorWithDefaults()

	// Non-monitored instance should return StateWorking
	state := m.GetState("nonexistent")
	if state != detect.StateWorking {
		t.Errorf("GetState for non-monitored = %v, want StateWorking", state)
	}

	// Monitored instance should start with StateWorking
	m.Start("inst-1")
	state = m.GetState("inst-1")
	if state != detect.StateWorking {
		t.Errorf("GetState after Start = %v, want StateWorking", state)
	}
}

func TestMonitor_GetTimedOut(t *testing.T) {
	m := NewMonitorWithDefaults()

	// Non-monitored instance
	timedOut, _ := m.GetTimedOut("nonexistent")
	if timedOut {
		t.Error("GetTimedOut for non-monitored should return false")
	}

	// Monitored instance starts not timed out
	m.Start("inst-1")
	timedOut, _ = m.GetTimedOut("inst-1")
	if timedOut {
		t.Error("GetTimedOut after Start should return false")
	}
}

func TestMonitor_GetLastActivityTime(t *testing.T) {
	m := NewMonitorWithDefaults()

	// Non-monitored instance should return zero time
	actTime := m.GetLastActivityTime("nonexistent")
	if !actTime.IsZero() {
		t.Errorf("GetLastActivityTime for non-monitored = %v, want zero", actTime)
	}

	// Monitored instance should have recent activity time
	before := time.Now()
	m.Start("inst-1")
	after := time.Now()

	actTime = m.GetLastActivityTime("inst-1")
	if actTime.Before(before) || actTime.After(after) {
		t.Errorf("GetLastActivityTime = %v, should be between %v and %v", actTime, before, after)
	}
}

func TestMonitor_GetStartTime(t *testing.T) {
	m := NewMonitorWithDefaults()

	// Non-monitored instance should return nil
	startTime := m.GetStartTime("nonexistent")
	if startTime != nil {
		t.Error("GetStartTime for non-monitored should return nil")
	}

	// Monitored instance should have start time
	before := time.Now()
	m.Start("inst-1")
	after := time.Now()

	startTime = m.GetStartTime("inst-1")
	if startTime == nil {
		t.Fatal("GetStartTime returned nil for monitored instance")
	}
	if startTime.Before(before) || startTime.After(after) {
		t.Errorf("GetStartTime = %v, should be between %v and %v", startTime, before, after)
	}
}

func TestMonitor_ClearTimeout(t *testing.T) {
	m := NewMonitor(MonitorConfig{
		ActivityTimeoutMinutes: 0, // Disable so we can manually trigger
		StaleDetection:         true,
		StaleThreshold:         2, // Low threshold for testing
	})

	m.Start("inst-1")

	// Trigger stale timeout by repeated output
	for i := 0; i < 5; i++ {
		m.ProcessOutput("inst-1", []byte("same output"), "samehash")
	}
	m.CheckTimeouts("inst-1")

	timedOut, _ := m.GetTimedOut("inst-1")
	if !timedOut {
		t.Error("Instance should be timed out after repeated output")
	}

	// Clear timeout
	m.ClearTimeout("inst-1")

	timedOut, _ = m.GetTimedOut("inst-1")
	if timedOut {
		t.Error("Instance should not be timed out after ClearTimeout")
	}
}

func TestMonitor_ProcessOutput_StateChange(t *testing.T) {
	m := NewMonitorWithDefaults()

	var callbackCalls []struct {
		instanceID string
		oldState   detect.WaitingState
		newState   detect.WaitingState
	}

	m.OnStateChange(func(id string, old, new detect.WaitingState) {
		callbackCalls = append(callbackCalls, struct {
			instanceID string
			oldState   detect.WaitingState
			newState   detect.WaitingState
		}{id, old, new})
	})

	m.Start("inst-1")

	// Process output that triggers a question state
	output := []byte("What file would you like me to edit?")
	state := m.ProcessOutput("inst-1", output, "hash1")

	if state != detect.StateWaitingQuestion {
		t.Errorf("ProcessOutput state = %v, want StateWaitingQuestion", state)
	}

	if len(callbackCalls) != 1 {
		t.Fatalf("Expected 1 callback call, got %d", len(callbackCalls))
	}

	if callbackCalls[0].instanceID != "inst-1" {
		t.Errorf("Callback instanceID = %q, want %q", callbackCalls[0].instanceID, "inst-1")
	}

	if callbackCalls[0].oldState != detect.StateWorking {
		t.Errorf("Callback oldState = %v, want StateWorking", callbackCalls[0].oldState)
	}

	if callbackCalls[0].newState != detect.StateWaitingQuestion {
		t.Errorf("Callback newState = %v, want StateWaitingQuestion", callbackCalls[0].newState)
	}
}

func TestMonitor_ProcessOutput_NoStateChange(t *testing.T) {
	m := NewMonitorWithDefaults()

	callbackCount := 0
	m.OnStateChange(func(id string, old, new detect.WaitingState) {
		callbackCount++
	})

	m.Start("inst-1")

	// Process output that keeps working state
	output := []byte("Processing files...")
	m.ProcessOutput("inst-1", output, "hash1")
	initialCallbackCount := callbackCount

	// Process more working output with different hash
	output2 := []byte("Reading directory...")
	m.ProcessOutput("inst-1", output2, "hash2")

	// Working to working shouldn't trigger additional callbacks
	if callbackCount > initialCallbackCount {
		t.Errorf("Expected no additional callbacks for working->working, got %d after initial", callbackCount-initialCallbackCount)
	}
}

func TestMonitor_ProcessOutput_ActivityTracking(t *testing.T) {
	m := NewMonitorWithDefaults()
	m.Start("inst-1")

	// Get initial activity time
	initialTime := m.GetLastActivityTime("inst-1")

	// Small delay to ensure time difference
	time.Sleep(10 * time.Millisecond)

	// Process output with new hash (output changed)
	m.ProcessOutput("inst-1", []byte("new output"), "hash1")

	newTime := m.GetLastActivityTime("inst-1")
	if !newTime.After(initialTime) {
		t.Error("LastActivityTime should be updated when output changes")
	}

	// Process output with same hash (output unchanged)
	previousTime := m.GetLastActivityTime("inst-1")
	time.Sleep(10 * time.Millisecond)
	m.ProcessOutput("inst-1", []byte("new output"), "hash1") // Same hash

	unchangedTime := m.GetLastActivityTime("inst-1")
	if !unchangedTime.Equal(previousTime) {
		t.Error("LastActivityTime should not change when output hash is same")
	}
}

func TestMonitor_ProcessOutput_NonMonitored(t *testing.T) {
	m := NewMonitorWithDefaults()

	// Should not panic and return StateWorking
	state := m.ProcessOutput("nonexistent", []byte("output"), "hash")
	if state != detect.StateWorking {
		t.Errorf("ProcessOutput for non-monitored = %v, want StateWorking", state)
	}
}

func TestMonitor_ProcessOutput_TimedOut(t *testing.T) {
	m := NewMonitor(MonitorConfig{
		StaleDetection: true,
		StaleThreshold: 1, // Low threshold
	})

	m.Start("inst-1")

	// Trigger timeout
	m.ProcessOutput("inst-1", []byte("same"), "hash")
	m.ProcessOutput("inst-1", []byte("same"), "hash")
	m.CheckTimeouts("inst-1")

	// Process output should still work but state is preserved
	state := m.ProcessOutput("inst-1", []byte("new"), "newhash")
	// State detection still works even when timed out - working state is expected
	_ = state // Verify it doesn't panic; state value doesn't matter for this test
}

func TestMonitor_CheckTimeouts_ActivityTimeout(t *testing.T) {
	m := NewMonitor(MonitorConfig{
		ActivityTimeoutMinutes: 1, // 1 minute
		StaleDetection:         false,
	})

	callbackCalled := false
	var callbackType TimeoutType
	m.OnTimeout(func(id string, tt TimeoutType) {
		callbackCalled = true
		callbackType = tt
	})

	// Start with a past time to simulate timeout
	pastTime := time.Now().Add(-2 * time.Minute)
	m.StartWithTime("inst-1", pastTime)

	// Manually set last activity time to past
	m.mu.Lock()
	m.instances["inst-1"].lastActivityTime = pastTime
	m.mu.Unlock()

	result := m.CheckTimeouts("inst-1")

	if result == nil {
		t.Fatal("Expected timeout to be detected")
	}

	if *result != TimeoutActivity {
		t.Errorf("Timeout type = %v, want TimeoutActivity", *result)
	}

	if !callbackCalled {
		t.Error("Timeout callback should have been called")
	}

	if callbackType != TimeoutActivity {
		t.Errorf("Callback timeout type = %v, want TimeoutActivity", callbackType)
	}

	timedOut, tt := m.GetTimedOut("inst-1")
	if !timedOut {
		t.Error("GetTimedOut should return true")
	}
	if tt != TimeoutActivity {
		t.Errorf("GetTimedOut type = %v, want TimeoutActivity", tt)
	}
}

func TestMonitor_CheckTimeouts_CompletionTimeout(t *testing.T) {
	m := NewMonitor(MonitorConfig{
		CompletionTimeoutMinutes: 1,  // 1 minute
		ActivityTimeoutMinutes:   60, // Long activity timeout
		StaleDetection:           false,
	})

	var callbackType TimeoutType
	m.OnTimeout(func(id string, tt TimeoutType) {
		callbackType = tt
	})

	// Start with a past time to simulate completion timeout
	pastTime := time.Now().Add(-2 * time.Minute)
	m.StartWithTime("inst-1", pastTime)

	result := m.CheckTimeouts("inst-1")

	if result == nil {
		t.Fatal("Expected timeout to be detected")
	}

	// Completion timeout has higher priority than activity timeout
	if *result != TimeoutCompletion {
		t.Errorf("Timeout type = %v, want TimeoutCompletion", *result)
	}

	if callbackType != TimeoutCompletion {
		t.Errorf("Callback timeout type = %v, want TimeoutCompletion", callbackType)
	}
}

func TestMonitor_CheckTimeouts_StaleTimeout(t *testing.T) {
	m := NewMonitor(MonitorConfig{
		StaleDetection: true,
		StaleThreshold: 3,
	})

	var callbackType TimeoutType
	m.OnTimeout(func(id string, tt TimeoutType) {
		callbackType = tt
	})

	m.Start("inst-1")

	// Simulate repeated identical outputs
	for i := 0; i < 5; i++ {
		m.ProcessOutput("inst-1", []byte("same output"), "samehash")
	}

	result := m.CheckTimeouts("inst-1")

	if result == nil {
		t.Fatal("Expected stale timeout to be detected")
	}

	if *result != TimeoutStale {
		t.Errorf("Timeout type = %v, want TimeoutStale", *result)
	}

	if callbackType != TimeoutStale {
		t.Errorf("Callback timeout type = %v, want TimeoutStale", callbackType)
	}
}

func TestMonitor_ProcessOutput_WorkingIndicatorsPreventsStale(t *testing.T) {
	m := NewMonitor(MonitorConfig{
		StaleDetection: true,
		StaleThreshold: 3, // Low threshold for quick test
	})

	m.Start("inst-1")

	// Output with working indicators (spinner) - should NOT increment stale counter
	// even when output hash doesn't change
	workingOutput := []byte("Analyzing the code... ⠋")
	for i := 0; i < 5; i++ {
		m.ProcessOutput("inst-1", workingOutput, "samehash")
	}

	// Should not trigger stale timeout because working indicators are present
	result := m.CheckTimeouts("inst-1")
	if result != nil {
		t.Errorf("Expected no timeout when working indicators present, got %v", *result)
	}

	// Now test without working indicators - should increment stale counter
	m2 := NewMonitor(MonitorConfig{
		StaleDetection: true,
		StaleThreshold: 3,
	})
	m2.Start("inst-2")

	// Output without working indicators - should increment stale counter
	normalOutput := []byte("Some static output")
	for i := 0; i < 5; i++ {
		m2.ProcessOutput("inst-2", normalOutput, "samehash2")
	}

	// Should trigger stale timeout because no working indicators
	result2 := m2.CheckTimeouts("inst-2")
	if result2 == nil {
		t.Error("Expected stale timeout when no working indicators present")
	} else if *result2 != TimeoutStale {
		t.Errorf("Expected TimeoutStale, got %v", *result2)
	}
}

func TestMonitor_CheckTimeouts_NoTimeout(t *testing.T) {
	m := NewMonitor(MonitorConfig{
		ActivityTimeoutMinutes:   60,  // 1 hour
		CompletionTimeoutMinutes: 120, // 2 hours
		StaleDetection:           true,
		StaleThreshold:           3000, // High threshold
	})

	callbackCount := 0
	m.OnTimeout(func(id string, tt TimeoutType) {
		callbackCount++
	})

	m.Start("inst-1")

	result := m.CheckTimeouts("inst-1")

	if result != nil {
		t.Errorf("Expected no timeout, got %v", *result)
	}

	if callbackCount > 0 {
		t.Errorf("Timeout callback should not have been called, got %d calls", callbackCount)
	}
}

func TestMonitor_CheckTimeouts_NonMonitored(t *testing.T) {
	m := NewMonitorWithDefaults()

	result := m.CheckTimeouts("nonexistent")
	if result != nil {
		t.Errorf("Expected nil for non-monitored instance, got %v", *result)
	}
}

func TestMonitor_CheckTimeouts_AlreadyTimedOut(t *testing.T) {
	m := NewMonitor(MonitorConfig{
		StaleDetection: true,
		StaleThreshold: 1, // Timeout triggers when repeatedOutputCount > 1
	})

	callbackCount := 0
	m.OnTimeout(func(id string, tt TimeoutType) {
		callbackCount++
	})

	m.Start("inst-1")

	// Trigger timeout by exceeding stale threshold
	// First call: sets lastOutputHash, count stays 0
	// Second call: same hash, count becomes 1
	// Third call: same hash, count becomes 2 (> threshold of 1)
	m.ProcessOutput("inst-1", []byte("same"), "hash")
	m.ProcessOutput("inst-1", []byte("same"), "hash")
	m.ProcessOutput("inst-1", []byte("same"), "hash")
	m.CheckTimeouts("inst-1")

	if callbackCount != 1 {
		t.Errorf("Expected 1 callback, got %d", callbackCount)
	}

	// Second check should not trigger callback again
	result := m.CheckTimeouts("inst-1")
	if result != nil {
		t.Error("Already timed out instance should return nil")
	}

	if callbackCount != 1 {
		t.Errorf("Callback count should still be 1, got %d", callbackCount)
	}
}

func TestMonitor_CheckBell(t *testing.T) {
	m := NewMonitorWithDefaults()

	callbackCalls := 0
	var lastInstanceID string
	m.OnBell(func(id string) {
		callbackCalls++
		lastInstanceID = id
	})

	m.Start("inst-1")

	// No bell initially
	detected := m.CheckBell("inst-1", false)
	if detected {
		t.Error("Should not detect bell when bellActive=false")
	}

	// Bell activates (edge transition)
	detected = m.CheckBell("inst-1", true)
	if !detected {
		t.Error("Should detect bell on transition to active")
	}
	if callbackCalls != 1 {
		t.Errorf("Expected 1 callback call, got %d", callbackCalls)
	}
	if lastInstanceID != "inst-1" {
		t.Errorf("Callback instanceID = %q, want %q", lastInstanceID, "inst-1")
	}

	// Bell stays active (no new edge)
	detected = m.CheckBell("inst-1", true)
	if detected {
		t.Error("Should not detect bell when already active")
	}
	if callbackCalls != 1 {
		t.Errorf("Callback count should still be 1, got %d", callbackCalls)
	}

	// Bell deactivates
	detected = m.CheckBell("inst-1", false)
	if detected {
		t.Error("Should not detect bell on deactivation")
	}

	// Bell activates again (new edge)
	detected = m.CheckBell("inst-1", true)
	if !detected {
		t.Error("Should detect second bell activation")
	}
	if callbackCalls != 2 {
		t.Errorf("Expected 2 callback calls, got %d", callbackCalls)
	}
}

func TestMonitor_CheckBell_NonMonitored(t *testing.T) {
	m := NewMonitorWithDefaults()

	detected := m.CheckBell("nonexistent", true)
	if detected {
		t.Error("Should not detect bell for non-monitored instance")
	}
}

func TestMonitor_SetState(t *testing.T) {
	m := NewMonitorWithDefaults()

	var callbackCalls []struct {
		id       string
		oldState detect.WaitingState
		newState detect.WaitingState
	}
	m.OnStateChange(func(id string, old, new detect.WaitingState) {
		callbackCalls = append(callbackCalls, struct {
			id       string
			oldState detect.WaitingState
			newState detect.WaitingState
		}{id, old, new})
	})

	m.Start("inst-1")

	// Set new state
	m.SetState("inst-1", detect.StateCompleted)

	state := m.GetState("inst-1")
	if state != detect.StateCompleted {
		t.Errorf("GetState = %v, want StateCompleted", state)
	}

	if len(callbackCalls) != 1 {
		t.Fatalf("Expected 1 callback call, got %d", len(callbackCalls))
	}

	if callbackCalls[0].oldState != detect.StateWorking {
		t.Errorf("Callback oldState = %v, want StateWorking", callbackCalls[0].oldState)
	}

	if callbackCalls[0].newState != detect.StateCompleted {
		t.Errorf("Callback newState = %v, want StateCompleted", callbackCalls[0].newState)
	}

	// Setting same state should not trigger callback
	m.SetState("inst-1", detect.StateCompleted)
	if len(callbackCalls) != 1 {
		t.Errorf("Setting same state should not trigger callback, got %d calls", len(callbackCalls))
	}
}

func TestMonitor_SetState_NonMonitored(t *testing.T) {
	m := NewMonitorWithDefaults()

	// Should not panic
	m.SetState("nonexistent", detect.StateCompleted)
}

func TestMonitor_MonitoredInstances(t *testing.T) {
	m := NewMonitorWithDefaults()

	// Empty initially
	instances := m.MonitoredInstances()
	if len(instances) != 0 {
		t.Errorf("Expected 0 instances, got %d", len(instances))
	}

	// Add some instances
	m.Start("inst-1")
	m.Start("inst-2")
	m.Start("inst-3")

	instances = m.MonitoredInstances()
	if len(instances) != 3 {
		t.Errorf("Expected 3 instances, got %d", len(instances))
	}

	// Check all are present
	found := make(map[string]bool)
	for _, id := range instances {
		found[id] = true
	}
	for _, expected := range []string{"inst-1", "inst-2", "inst-3"} {
		if !found[expected] {
			t.Errorf("Instance %q not found in MonitoredInstances", expected)
		}
	}

	// Stop one
	m.Stop("inst-2")
	instances = m.MonitoredInstances()
	if len(instances) != 2 {
		t.Errorf("Expected 2 instances after stop, got %d", len(instances))
	}
}

func TestMonitor_Config(t *testing.T) {
	cfg := MonitorConfig{
		ActivityTimeoutMinutes:   10,
		CompletionTimeoutMinutes: 50,
		StaleDetection:           false,
	}

	m := NewMonitor(cfg)
	returnedCfg := m.Config()

	if returnedCfg.ActivityTimeoutMinutes != 10 {
		t.Errorf("Config ActivityTimeoutMinutes = %d, want 10", returnedCfg.ActivityTimeoutMinutes)
	}

	if returnedCfg.CompletionTimeoutMinutes != 50 {
		t.Errorf("Config CompletionTimeoutMinutes = %d, want 50", returnedCfg.CompletionTimeoutMinutes)
	}

	if returnedCfg.StaleDetection {
		t.Error("Config StaleDetection should be false")
	}
}

func TestMonitor_ConcurrentAccess(t *testing.T) {
	m := NewMonitorWithDefaults()

	var wg sync.WaitGroup
	const goroutines = 10
	const operations = 100

	// Start/Stop operations
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			instanceID := "inst-concurrent"
			for j := 0; j < operations; j++ {
				m.Start(instanceID)
				m.GetState(instanceID)
				m.ProcessOutput(instanceID, []byte("output"), "hash")
				m.CheckTimeouts(instanceID)
				m.CheckBell(instanceID, j%2 == 0)
				m.Stop(instanceID)
			}
		}(i)
	}

	wg.Wait()

	// Should not deadlock - the test passes if we reach this point without hanging
	// Final monitoring state is non-deterministic due to concurrent Start/Stop calls
	_ = m.IsMonitoring("inst-concurrent") // Check doesn't panic
}

func TestMonitor_CallbacksOutsideLock(t *testing.T) {
	m := NewMonitorWithDefaults()

	// This test verifies that callbacks don't cause deadlocks
	// by trying to access the monitor from within callbacks

	m.OnStateChange(func(id string, old, new detect.WaitingState) {
		// Try to access monitor from callback (would deadlock if lock held)
		_ = m.GetState(id)
		_ = m.IsMonitoring(id)
		_ = m.MonitoredInstances()
	})

	m.OnTimeout(func(id string, tt TimeoutType) {
		_ = m.GetState(id)
		timedOut, _ := m.GetTimedOut(id)
		_ = timedOut
	})

	m.OnBell(func(id string) {
		_ = m.GetState(id)
	})

	m.Start("inst-1")

	// Process output that triggers state change
	m.ProcessOutput("inst-1", []byte("What would you like to do?"), "hash1")

	// Should not deadlock
}

func TestMonitor_MultipleInstances(t *testing.T) {
	m := NewMonitorWithDefaults()

	stateChanges := make(map[string]int)
	var mu sync.Mutex

	m.OnStateChange(func(id string, old, new detect.WaitingState) {
		mu.Lock()
		stateChanges[id]++
		mu.Unlock()
	})

	// Start multiple instances
	m.Start("inst-1")
	m.Start("inst-2")
	m.Start("inst-3")

	// Process different outputs for different instances
	m.ProcessOutput("inst-1", []byte("What file?"), "h1")                    // Question
	m.ProcessOutput("inst-2", []byte("Working on it..."), "h2")              // Working
	m.ProcessOutput("inst-3", []byte("Do you want to proceed? [Y/N]"), "h3") // Permission

	// Check states
	if m.GetState("inst-1") != detect.StateWaitingQuestion {
		t.Errorf("inst-1 state = %v, want StateWaitingQuestion", m.GetState("inst-1"))
	}

	if m.GetState("inst-2") != detect.StateWorking {
		t.Errorf("inst-2 state = %v, want StateWorking", m.GetState("inst-2"))
	}

	if m.GetState("inst-3") != detect.StateWaitingPermission {
		t.Errorf("inst-3 state = %v, want StateWaitingPermission", m.GetState("inst-3"))
	}
}
