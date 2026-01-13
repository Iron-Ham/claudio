// Package detect provides detection utilities for instance monitoring.
package detect

import (
	"time"
)

// TimeoutType represents the type of timeout that occurred.
type TimeoutType int

const (
	// TimeoutNone indicates no timeout has occurred.
	TimeoutNone TimeoutType = iota
	// TimeoutActivity indicates no output activity for the configured period.
	TimeoutActivity
	// TimeoutCompletion indicates total runtime exceeded the configured limit.
	TimeoutCompletion
	// TimeoutStale indicates repeated identical output (stuck in a loop).
	TimeoutStale
)

// String returns a human-readable name for the timeout type.
func (t TimeoutType) String() string {
	switch t {
	case TimeoutNone:
		return "none"
	case TimeoutActivity:
		return "activity"
	case TimeoutCompletion:
		return "completion"
	case TimeoutStale:
		return "stale"
	default:
		return "unknown"
	}
}

// TimeoutConfig holds configurable thresholds for timeout detection.
type TimeoutConfig struct {
	// ActivityTimeout is the duration of no output changes before triggering.
	// Zero disables activity timeout.
	ActivityTimeout time.Duration

	// CompletionTimeout is the maximum total runtime before triggering.
	// Zero disables completion timeout.
	CompletionTimeout time.Duration

	// StaleThreshold is the count of repeated identical outputs before triggering.
	// Zero disables stale detection.
	StaleThreshold int
}

// DefaultTimeoutConfig returns sensible default timeout thresholds.
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		ActivityTimeout:   30 * time.Minute, // 30 minutes of no activity
		CompletionTimeout: 0,                // Disabled by default (no max runtime limit)
		StaleThreshold:    3000,             // ~5 minutes at 100ms capture interval
	}
}

// TimeoutDetector checks for various timeout conditions based on time and output state.
// It is stateless - callers must track the current state (times, counters) externally.
type TimeoutDetector struct {
	config TimeoutConfig
}

// NewTimeoutDetector creates a new timeout detector with the given configuration.
func NewTimeoutDetector(cfg TimeoutConfig) *TimeoutDetector {
	return &TimeoutDetector{
		config: cfg,
	}
}

// CheckInput contains the inputs needed for timeout detection.
type CheckInput struct {
	// Now is the current time for timeout calculations.
	Now time.Time

	// StartTime is when the instance started (for completion timeout).
	// Nil if not started yet.
	StartTime *time.Time

	// LastActivityTime is when output last changed (for activity timeout).
	LastActivityTime time.Time

	// RepeatedOutputCount is how many consecutive captures had identical output.
	// Used for stale detection.
	RepeatedOutputCount int
}

// CheckTimeout evaluates all timeout conditions and returns the triggered timeout type.
// Returns TimeoutNone if no timeout condition is met.
// Checks are prioritized: CompletionTimeout > ActivityTimeout > StaleTimeout.
func (d *TimeoutDetector) CheckTimeout(input CheckInput) TimeoutType {
	// Check completion timeout (total runtime) - highest priority
	if d.config.CompletionTimeout > 0 && input.StartTime != nil {
		if input.Now.Sub(*input.StartTime) > d.config.CompletionTimeout {
			return TimeoutCompletion
		}
	}

	// Check activity timeout (no output changes)
	if d.config.ActivityTimeout > 0 {
		if input.Now.Sub(input.LastActivityTime) > d.config.ActivityTimeout {
			return TimeoutActivity
		}
	}

	// Check stale detection (repeated identical output)
	if d.config.StaleThreshold > 0 && input.RepeatedOutputCount > d.config.StaleThreshold {
		return TimeoutStale
	}

	return TimeoutNone
}

// IsActivityTimeoutEnabled returns whether activity timeout checking is enabled.
func (d *TimeoutDetector) IsActivityTimeoutEnabled() bool {
	return d.config.ActivityTimeout > 0
}

// IsCompletionTimeoutEnabled returns whether completion timeout checking is enabled.
func (d *TimeoutDetector) IsCompletionTimeoutEnabled() bool {
	return d.config.CompletionTimeout > 0
}

// IsStaleDetectionEnabled returns whether stale output detection is enabled.
func (d *TimeoutDetector) IsStaleDetectionEnabled() bool {
	return d.config.StaleThreshold > 0
}

// Config returns a copy of the detector's configuration.
func (d *TimeoutDetector) Config() TimeoutConfig {
	return d.config
}
