package instance

import (
	"sync"
	"time"
)

// TimeoutType represents the type of timeout that occurred
type TimeoutType int

const (
	TimeoutActivity   TimeoutType = iota // No activity for configured period
	TimeoutCompletion                    // Total runtime exceeded limit
	TimeoutStale                         // Repeated output detected (stuck in loop)
)

// TimeoutCallback is called when a timeout condition is detected
type TimeoutCallback func(instanceID string, timeoutType TimeoutType)

// TimeoutConfig holds configuration for timeout detection
type TimeoutConfig struct {
	ActivityTimeoutMinutes   int  // 0 = disabled
	CompletionTimeoutMinutes int  // 0 = disabled
	StaleDetection           bool // Enable repeated output detection
}

// TimeoutHandler manages timeout detection for an instance.
// It tracks activity, completion time, and stale output conditions,
// triggering callbacks when timeout thresholds are exceeded.
type TimeoutHandler struct {
	mu sync.RWMutex

	// Configuration
	config TimeoutConfig

	// Timeout tracking state
	lastActivityTime    time.Time   // Last time output changed
	lastOutputHash      string      // Hash of last output for change detection
	repeatedOutputCount int         // Count of consecutive identical outputs (for stale detection)
	timedOut            bool        // Whether a timeout has been triggered
	timeoutType         TimeoutType // Type of timeout that was triggered

	// Reference to start time (owned by Manager, passed in for checks)
	startTime *time.Time

	// Callback
	callback TimeoutCallback
}

// NewTimeoutHandler creates a new TimeoutHandler with the given configuration
func NewTimeoutHandler(config TimeoutConfig) *TimeoutHandler {
	return &TimeoutHandler{
		config:           config,
		lastActivityTime: time.Now(),
	}
}

// SetCallback sets the callback that will be invoked when a timeout is detected
func (h *TimeoutHandler) SetCallback(cb TimeoutCallback) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.callback = cb
}

// SetStartTime sets the reference to the start time for completion timeout checks
func (h *TimeoutHandler) SetStartTime(startTime *time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.startTime = startTime
}

// Reset resets the timeout state (for recovery/restart scenarios)
func (h *TimeoutHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.timedOut = false
	h.repeatedOutputCount = 0
	h.lastActivityTime = time.Now()
}

// RecordActivity updates the last activity time and resets stale detection.
// Called when output changes.
func (h *TimeoutHandler) RecordActivity(outputHash string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastActivityTime = time.Now()
	h.lastOutputHash = outputHash
	h.repeatedOutputCount = 0
}

// RecordRepeatedOutput increments the repeated output counter.
// Called when output hasn't changed (for stale detection).
func (h *TimeoutHandler) RecordRepeatedOutput() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.config.StaleDetection {
		h.repeatedOutputCount++
	}
}

// TimedOut returns whether a timeout has been triggered and the type of timeout
func (h *TimeoutHandler) TimedOut() (bool, TimeoutType) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.timedOut, h.timeoutType
}

// LastActivityTime returns when the instance last had output activity
func (h *TimeoutHandler) LastActivityTime() time.Time {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastActivityTime
}

// CheckTimeouts checks for various timeout conditions and triggers callbacks.
// Returns true if a timeout was triggered.
func (h *TimeoutHandler) CheckTimeouts(instanceID string, running, paused bool) bool {
	h.mu.Lock()
	if h.timedOut || !running || paused {
		h.mu.Unlock()
		return false
	}

	now := time.Now()
	callback := h.callback
	var triggeredTimeout *TimeoutType

	// Check completion timeout (total runtime)
	if h.config.CompletionTimeoutMinutes > 0 && h.startTime != nil {
		completionTimeout := time.Duration(h.config.CompletionTimeoutMinutes) * time.Minute
		if now.Sub(*h.startTime) > completionTimeout {
			t := TimeoutCompletion
			triggeredTimeout = &t
			h.timedOut = true
			h.timeoutType = TimeoutCompletion
		}
	}

	// Check activity timeout (no output changes)
	if triggeredTimeout == nil && h.config.ActivityTimeoutMinutes > 0 {
		activityTimeout := time.Duration(h.config.ActivityTimeoutMinutes) * time.Minute
		if now.Sub(h.lastActivityTime) > activityTimeout {
			t := TimeoutActivity
			triggeredTimeout = &t
			h.timedOut = true
			h.timeoutType = TimeoutActivity
		}
	}

	// Check for stale detection (repeated identical output)
	// Trigger if we've seen the same output 3000 times (5 minutes at 100ms interval)
	// This catches stuck loops producing identical output while allowing time for
	// legitimate long-running operations like planning and exploration
	if triggeredTimeout == nil && h.config.StaleDetection && h.repeatedOutputCount > 3000 {
		t := TimeoutStale
		triggeredTimeout = &t
		h.timedOut = true
		h.timeoutType = TimeoutStale
	}

	h.mu.Unlock()

	// Invoke callback outside of lock to prevent deadlocks
	if triggeredTimeout != nil && callback != nil {
		callback(instanceID, *triggeredTimeout)
	}

	return triggeredTimeout != nil
}
