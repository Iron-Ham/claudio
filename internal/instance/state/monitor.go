// Package state provides instance state monitoring for Claude Code sessions.
// It tracks state changes, timeouts, and bell detection for managed instances.
package state

import (
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/instance/detect"
	"github.com/Iron-Ham/claudio/internal/logging"
)

// MonitorConfig holds configuration for state monitoring.
type MonitorConfig struct {
	// ActivityTimeoutMinutes is how long to wait with no output before triggering.
	// Zero disables activity timeout.
	ActivityTimeoutMinutes int

	// CompletionTimeoutMinutes is the maximum total runtime before triggering.
	// Zero disables completion timeout.
	CompletionTimeoutMinutes int

	// StaleDetection enables detection of repeated identical output.
	StaleDetection bool

	// StaleThreshold is the count of repeated outputs before triggering (default: 3000).
	// Only used if StaleDetection is true.
	StaleThreshold int
}

// DefaultMonitorConfig returns sensible default monitoring configuration.
func DefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{
		ActivityTimeoutMinutes:   30,   // 30 minutes of no activity
		CompletionTimeoutMinutes: 0,    // Disabled by default (no max runtime limit)
		StaleDetection:           true, // Enable stale detection
		StaleThreshold:           3000, // ~5 minutes at 100ms capture interval
	}
}

// TimeoutType is an alias for detect.TimeoutType for backwards compatibility.
// New code should import detect.TimeoutType directly.
type TimeoutType = detect.TimeoutType

// Re-export timeout type constants for backwards compatibility.
// New code should import these from detect package directly.
const (
	TimeoutActivity   = detect.TimeoutActivity
	TimeoutCompletion = detect.TimeoutCompletion
	TimeoutStale      = detect.TimeoutStale
)

// StateChangeCallback is called when an instance's detected state changes.
type StateChangeCallback func(instanceID string, oldState, newState detect.WaitingState)

// TimeoutCallback is called when a timeout condition is detected.
type TimeoutCallback func(instanceID string, timeoutType TimeoutType)

// BellCallback is called when a terminal bell is detected.
type BellCallback func(instanceID string)

// instanceState tracks the state for a single monitored instance.
type instanceState struct {
	instanceID          string
	startTime           *time.Time
	lastActivityTime    time.Time
	lastOutputHash      string
	repeatedOutputCount int
	currentState        detect.WaitingState
	timedOut            bool
	timeoutType         TimeoutType
	lastBellState       bool
}

// Monitor tracks state changes, timeouts, and bell events for instances.
// It provides a centralized state monitoring service that can be used by
// instance managers without coupling state tracking to instance lifecycle.
//
// Monitor is safe for concurrent use.
type Monitor struct {
	mu        sync.RWMutex
	config    MonitorConfig
	detector  *detect.Detector
	instances map[string]*instanceState

	// Callbacks
	stateCallback   StateChangeCallback
	timeoutCallback TimeoutCallback
	bellCallback    BellCallback

	// Logger for structured logging
	logger *logging.Logger
}

// NewMonitor creates a new state monitor with the given configuration.
func NewMonitor(cfg MonitorConfig) *Monitor {
	// Apply default stale threshold if not set
	if cfg.StaleDetection && cfg.StaleThreshold <= 0 {
		cfg.StaleThreshold = 3000
	}

	return &Monitor{
		config:    cfg,
		detector:  detect.NewDetector(),
		instances: make(map[string]*instanceState),
	}
}

// NewMonitorWithDefaults creates a new state monitor with default configuration.
func NewMonitorWithDefaults() *Monitor {
	return NewMonitor(DefaultMonitorConfig())
}

// SetLogger sets the logger for the monitor.
func (m *Monitor) SetLogger(logger *logging.Logger) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger = logger
}

// OnStateChange sets a callback for state changes.
// The callback receives the instance ID and both old and new states.
func (m *Monitor) OnStateChange(cb StateChangeCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateCallback = cb
}

// OnTimeout sets a callback for timeout events.
func (m *Monitor) OnTimeout(cb TimeoutCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.timeoutCallback = cb
}

// OnBell sets a callback for bell events.
func (m *Monitor) OnBell(cb BellCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bellCallback = cb
}

// Start begins monitoring a new instance.
// If the instance is already being monitored, this is a no-op.
func (m *Monitor) Start(instanceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.instances[instanceID]; exists {
		return
	}

	now := time.Now()
	m.instances[instanceID] = &instanceState{
		instanceID:       instanceID,
		startTime:        &now,
		lastActivityTime: now,
		currentState:     detect.StateWorking,
	}

	if m.logger != nil {
		m.logger.Info("started monitoring instance",
			"instance_id", instanceID)
	}
}

// StartWithTime begins monitoring an instance with a specific start time.
// This is useful for resuming monitoring of an instance that was started earlier.
func (m *Monitor) StartWithTime(instanceID string, startTime time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.instances[instanceID]; exists {
		return
	}

	m.instances[instanceID] = &instanceState{
		instanceID:       instanceID,
		startTime:        &startTime,
		lastActivityTime: time.Now(),
		currentState:     detect.StateWorking,
	}

	if m.logger != nil {
		m.logger.Info("started monitoring instance with custom start time",
			"instance_id", instanceID,
			"start_time", startTime)
	}
}

// Stop stops monitoring an instance.
// Returns false if the instance was not being monitored.
func (m *Monitor) Stop(instanceID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.instances[instanceID]; !exists {
		return false
	}

	delete(m.instances, instanceID)

	if m.logger != nil {
		m.logger.Info("stopped monitoring instance",
			"instance_id", instanceID)
	}

	return true
}

// GetState returns the current state for an instance.
// Returns detect.StateWorking if the instance is not being monitored.
func (m *Monitor) GetState(instanceID string) detect.WaitingState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if inst, exists := m.instances[instanceID]; exists {
		return inst.currentState
	}
	return detect.StateWorking
}

// GetTimedOut returns whether an instance has timed out and the timeout type.
// Returns (false, TimeoutActivity) if the instance is not being monitored.
func (m *Monitor) GetTimedOut(instanceID string) (bool, TimeoutType) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if inst, exists := m.instances[instanceID]; exists {
		return inst.timedOut, inst.timeoutType
	}
	return false, TimeoutActivity
}

// GetLastActivityTime returns when the instance last had output activity.
// Returns zero time if the instance is not being monitored.
func (m *Monitor) GetLastActivityTime(instanceID string) time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if inst, exists := m.instances[instanceID]; exists {
		return inst.lastActivityTime
	}
	return time.Time{}
}

// GetStartTime returns when the instance started.
// Returns nil if the instance is not being monitored.
func (m *Monitor) GetStartTime(instanceID string) *time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if inst, exists := m.instances[instanceID]; exists {
		return inst.startTime
	}
	return nil
}

// ClearTimeout resets the timeout state for an instance.
// This is useful for recovery/restart scenarios.
func (m *Monitor) ClearTimeout(instanceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if inst, exists := m.instances[instanceID]; exists {
		inst.timedOut = false
		inst.repeatedOutputCount = 0
		inst.lastActivityTime = time.Now()

		if m.logger != nil {
			m.logger.Debug("cleared timeout for instance",
				"instance_id", instanceID)
		}
	}
}

// ProcessOutput processes new output for an instance, detecting state changes.
// This should be called periodically with the instance's terminal output.
// Returns the detected state.
func (m *Monitor) ProcessOutput(instanceID string, output []byte, outputHash string) detect.WaitingState {
	m.mu.Lock()

	inst, exists := m.instances[instanceID]
	if !exists {
		m.mu.Unlock()
		return detect.StateWorking
	}

	// Skip processing if already timed out
	if inst.timedOut {
		currentState := inst.currentState
		m.mu.Unlock()
		return currentState
	}

	// Detect new state
	newState := m.detector.Detect(output)
	oldState := inst.currentState
	stateChanged := newState != oldState

	// Track output changes and working indicators
	outputChanged := outputHash != inst.lastOutputHash
	hasWorkingIndicators := m.detector.HasWorkingIndicators(output)

	if outputChanged {
		inst.lastActivityTime = time.Now()
		inst.lastOutputHash = outputHash
		inst.repeatedOutputCount = 0
	} else if m.config.StaleDetection {
		// Only increment stale counter if no working indicators are present.
		// If Claude is showing spinners, "Reading...", etc., it's actively working
		// even if the output hash hasn't changed (e.g., spinner is static).
		// This prevents false positives during Claude's thinking phase.
		if !hasWorkingIndicators {
			inst.repeatedOutputCount++
		}
	}

	// Update state
	if stateChanged {
		inst.currentState = newState
	}

	// Get callback and logger for use outside lock
	callback := m.stateCallback
	logger := m.logger
	m.mu.Unlock()

	// Log and invoke callback outside of lock to prevent deadlocks
	if stateChanged {
		if logger != nil {
			logger.Info("instance state changed",
				"instance_id", instanceID,
				"old_state", oldState.String(),
				"new_state", newState.String())
		}
		if callback != nil {
			callback(instanceID, oldState, newState)
		}
	}

	return newState
}

// CheckTimeouts checks for timeout conditions on an instance.
// Returns the detected timeout type, or nil if no timeout occurred.
// The callback is invoked if a timeout is detected.
func (m *Monitor) CheckTimeouts(instanceID string) *TimeoutType {
	m.mu.Lock()

	inst, exists := m.instances[instanceID]
	if !exists || inst.timedOut {
		m.mu.Unlock()
		return nil
	}

	now := time.Now()
	var triggeredTimeout *TimeoutType

	// Check completion timeout (total runtime) - highest priority
	if m.config.CompletionTimeoutMinutes > 0 && inst.startTime != nil {
		completionTimeout := time.Duration(m.config.CompletionTimeoutMinutes) * time.Minute
		if now.Sub(*inst.startTime) > completionTimeout {
			t := TimeoutCompletion
			triggeredTimeout = &t
			inst.timedOut = true
			inst.timeoutType = TimeoutCompletion
		}
	}

	// Check activity timeout (no output changes)
	if triggeredTimeout == nil && m.config.ActivityTimeoutMinutes > 0 {
		activityTimeout := time.Duration(m.config.ActivityTimeoutMinutes) * time.Minute
		if now.Sub(inst.lastActivityTime) > activityTimeout {
			t := TimeoutActivity
			triggeredTimeout = &t
			inst.timedOut = true
			inst.timeoutType = TimeoutActivity
		}
	}

	// Check stale detection (repeated identical output)
	// Note: The stale counter is only incremented in ProcessOutput when no
	// working indicators are present, so this check is already filtered to
	// cases where Claude is not actively showing working patterns.
	if triggeredTimeout == nil && m.config.StaleDetection && inst.repeatedOutputCount > m.config.StaleThreshold {
		t := TimeoutStale
		triggeredTimeout = &t
		inst.timedOut = true
		inst.timeoutType = TimeoutStale
	}

	repeatCount := inst.repeatedOutputCount
	callback := m.timeoutCallback
	logger := m.logger
	m.mu.Unlock()

	// Log and invoke callback outside of lock
	if triggeredTimeout != nil {
		if logger != nil {
			if *triggeredTimeout == TimeoutStale {
				logger.Warn("stale output detected",
					"instance_id", instanceID,
					"repeat_count", repeatCount)
			} else {
				logger.Warn("timeout triggered",
					"instance_id", instanceID,
					"timeout_type", triggeredTimeout.String())
			}
		}
		if callback != nil {
			callback(instanceID, *triggeredTimeout)
		}
	}

	return triggeredTimeout
}

// CheckBell checks for a terminal bell transition and invokes the callback if detected.
// The bellActive parameter indicates whether the bell flag is currently set.
// Returns true if a bell was detected (transition from inactive to active).
func (m *Monitor) CheckBell(instanceID string, bellActive bool) bool {
	m.mu.Lock()

	inst, exists := m.instances[instanceID]
	if !exists {
		m.mu.Unlock()
		return false
	}

	lastBellState := inst.lastBellState
	inst.lastBellState = bellActive

	// Detect edge transition (no bell -> bell)
	bellDetected := bellActive && !lastBellState

	callback := m.bellCallback
	logger := m.logger
	m.mu.Unlock()

	if bellDetected {
		if logger != nil {
			logger.Debug("bell detected",
				"instance_id", instanceID)
		}
		if callback != nil {
			callback(instanceID)
		}
	}

	return bellDetected
}

// SetState directly sets the state for an instance.
// This is useful for external state updates (e.g., completion detected via sentinel file).
// Invokes the state change callback if the state changed.
func (m *Monitor) SetState(instanceID string, newState detect.WaitingState) {
	m.mu.Lock()

	inst, exists := m.instances[instanceID]
	if !exists {
		m.mu.Unlock()
		return
	}

	oldState := inst.currentState
	if oldState == newState {
		m.mu.Unlock()
		return
	}

	inst.currentState = newState
	callback := m.stateCallback
	logger := m.logger
	m.mu.Unlock()

	if logger != nil {
		logger.Info("instance state set externally",
			"instance_id", instanceID,
			"old_state", oldState.String(),
			"new_state", newState.String())
	}
	if callback != nil {
		callback(instanceID, oldState, newState)
	}
}

// IsMonitoring returns whether an instance is currently being monitored.
func (m *Monitor) IsMonitoring(instanceID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.instances[instanceID]
	return exists
}

// MonitoredInstances returns a list of currently monitored instance IDs.
func (m *Monitor) MonitoredInstances() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.instances))
	for id := range m.instances {
		ids = append(ids, id)
	}
	return ids
}

// Config returns a copy of the monitor's configuration.
func (m *Monitor) Config() MonitorConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}
