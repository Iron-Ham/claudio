package detect

import (
	"testing"
	"time"
)

func TestTimeoutType_String(t *testing.T) {
	tests := []struct {
		tt   TimeoutType
		want string
	}{
		{TimeoutNone, "none"},
		{TimeoutActivity, "activity"},
		{TimeoutCompletion, "completion"},
		{TimeoutStale, "stale"},
		{TimeoutType(99), "unknown"},
	}

	for _, tc := range tests {
		got := tc.tt.String()
		if got != tc.want {
			t.Errorf("TimeoutType(%d).String() = %q, want %q", tc.tt, got, tc.want)
		}
	}
}

func TestDefaultTimeoutConfig(t *testing.T) {
	cfg := DefaultTimeoutConfig()

	if cfg.ActivityTimeout != 30*time.Minute {
		t.Errorf("ActivityTimeout = %v, want 30m", cfg.ActivityTimeout)
	}
	if cfg.CompletionTimeout != 0 {
		t.Errorf("CompletionTimeout = %v, want 0 (disabled)", cfg.CompletionTimeout)
	}
	if cfg.StaleThreshold != 3000 {
		t.Errorf("StaleThreshold = %d, want 3000", cfg.StaleThreshold)
	}
}

func TestTimeoutDetector_CheckTimeout_NoTimeout(t *testing.T) {
	cfg := TimeoutConfig{
		ActivityTimeout:   30 * time.Minute,
		CompletionTimeout: 2 * time.Hour,
		StaleThreshold:    3000,
	}
	detector := NewTimeoutDetector(cfg)

	now := time.Now()
	startTime := now.Add(-10 * time.Minute)   // Started 10 minutes ago
	lastActivity := now.Add(-1 * time.Minute) // Activity 1 minute ago

	input := CheckInput{
		Now:                 now,
		StartTime:           &startTime,
		LastActivityTime:    lastActivity,
		RepeatedOutputCount: 10,
	}

	result := detector.CheckTimeout(input)
	if result != TimeoutNone {
		t.Errorf("CheckTimeout() = %v, want TimeoutNone", result)
	}
}

func TestTimeoutDetector_CheckTimeout_CompletionTimeout(t *testing.T) {
	cfg := TimeoutConfig{
		ActivityTimeout:   30 * time.Minute,
		CompletionTimeout: 2 * time.Hour,
		StaleThreshold:    3000,
	}
	detector := NewTimeoutDetector(cfg)

	now := time.Now()
	startTime := now.Add(-3 * time.Hour) // Started 3 hours ago (exceeds 2h limit)
	lastActivity := now.Add(-1 * time.Minute)

	input := CheckInput{
		Now:                 now,
		StartTime:           &startTime,
		LastActivityTime:    lastActivity,
		RepeatedOutputCount: 10,
	}

	result := detector.CheckTimeout(input)
	if result != TimeoutCompletion {
		t.Errorf("CheckTimeout() = %v, want TimeoutCompletion", result)
	}
}

func TestTimeoutDetector_CheckTimeout_ActivityTimeout(t *testing.T) {
	cfg := TimeoutConfig{
		ActivityTimeout:   30 * time.Minute,
		CompletionTimeout: 2 * time.Hour,
		StaleThreshold:    3000,
	}
	detector := NewTimeoutDetector(cfg)

	now := time.Now()
	startTime := now.Add(-45 * time.Minute)    // Started 45 min ago (within 2h)
	lastActivity := now.Add(-35 * time.Minute) // No activity for 35 min (exceeds 30m)

	input := CheckInput{
		Now:                 now,
		StartTime:           &startTime,
		LastActivityTime:    lastActivity,
		RepeatedOutputCount: 10,
	}

	result := detector.CheckTimeout(input)
	if result != TimeoutActivity {
		t.Errorf("CheckTimeout() = %v, want TimeoutActivity", result)
	}
}

func TestTimeoutDetector_CheckTimeout_StaleTimeout(t *testing.T) {
	cfg := TimeoutConfig{
		ActivityTimeout:   30 * time.Minute,
		CompletionTimeout: 2 * time.Hour,
		StaleThreshold:    3000,
	}
	detector := NewTimeoutDetector(cfg)

	now := time.Now()
	startTime := now.Add(-10 * time.Minute)
	lastActivity := now.Add(-1 * time.Minute)

	input := CheckInput{
		Now:                 now,
		StartTime:           &startTime,
		LastActivityTime:    lastActivity,
		RepeatedOutputCount: 3500, // Exceeds 3000 threshold
	}

	result := detector.CheckTimeout(input)
	if result != TimeoutStale {
		t.Errorf("CheckTimeout() = %v, want TimeoutStale", result)
	}
}

func TestTimeoutDetector_CheckTimeout_Priority(t *testing.T) {
	// When multiple timeouts would trigger, CompletionTimeout should win
	cfg := TimeoutConfig{
		ActivityTimeout:   30 * time.Minute,
		CompletionTimeout: 2 * time.Hour,
		StaleThreshold:    3000,
	}
	detector := NewTimeoutDetector(cfg)

	now := time.Now()
	startTime := now.Add(-3 * time.Hour)       // Completion timeout triggered
	lastActivity := now.Add(-35 * time.Minute) // Activity timeout also triggered

	input := CheckInput{
		Now:                 now,
		StartTime:           &startTime,
		LastActivityTime:    lastActivity,
		RepeatedOutputCount: 5000, // Stale also triggered
	}

	result := detector.CheckTimeout(input)
	if result != TimeoutCompletion {
		t.Errorf("CheckTimeout() = %v, want TimeoutCompletion (highest priority)", result)
	}
}

func TestTimeoutDetector_CheckTimeout_ActivityBeatsStale(t *testing.T) {
	// When Activity and Stale would both trigger, Activity should win
	cfg := TimeoutConfig{
		ActivityTimeout:   30 * time.Minute,
		CompletionTimeout: 2 * time.Hour,
		StaleThreshold:    3000,
	}
	detector := NewTimeoutDetector(cfg)

	now := time.Now()
	startTime := now.Add(-45 * time.Minute)    // Within completion limit
	lastActivity := now.Add(-35 * time.Minute) // Activity timeout triggered

	input := CheckInput{
		Now:                 now,
		StartTime:           &startTime,
		LastActivityTime:    lastActivity,
		RepeatedOutputCount: 5000, // Stale also triggered
	}

	result := detector.CheckTimeout(input)
	if result != TimeoutActivity {
		t.Errorf("CheckTimeout() = %v, want TimeoutActivity (higher priority than stale)", result)
	}
}

func TestTimeoutDetector_CheckTimeout_DisabledTimeouts(t *testing.T) {
	// All timeouts disabled (zero values)
	cfg := TimeoutConfig{
		ActivityTimeout:   0,
		CompletionTimeout: 0,
		StaleThreshold:    0,
	}
	detector := NewTimeoutDetector(cfg)

	now := time.Now()
	startTime := now.Add(-100 * time.Hour) // Would trigger if enabled
	lastActivity := now.Add(-100 * time.Hour)

	input := CheckInput{
		Now:                 now,
		StartTime:           &startTime,
		LastActivityTime:    lastActivity,
		RepeatedOutputCount: 100000,
	}

	result := detector.CheckTimeout(input)
	if result != TimeoutNone {
		t.Errorf("CheckTimeout() = %v, want TimeoutNone (all timeouts disabled)", result)
	}
}

func TestTimeoutDetector_CheckTimeout_NilStartTime(t *testing.T) {
	cfg := TimeoutConfig{
		ActivityTimeout:   30 * time.Minute,
		CompletionTimeout: 2 * time.Hour,
		StaleThreshold:    3000,
	}
	detector := NewTimeoutDetector(cfg)

	now := time.Now()
	lastActivity := now.Add(-1 * time.Minute)

	input := CheckInput{
		Now:                 now,
		StartTime:           nil, // Not started yet
		LastActivityTime:    lastActivity,
		RepeatedOutputCount: 10,
	}

	// Should not panic with nil StartTime
	result := detector.CheckTimeout(input)
	if result != TimeoutNone {
		t.Errorf("CheckTimeout() = %v, want TimeoutNone", result)
	}
}

func TestTimeoutDetector_CheckTimeout_ExactThreshold(t *testing.T) {
	cfg := TimeoutConfig{
		ActivityTimeout:   30 * time.Minute,
		CompletionTimeout: 2 * time.Hour,
		StaleThreshold:    3000,
	}
	detector := NewTimeoutDetector(cfg)

	now := time.Now()
	startTime := now.Add(-10 * time.Minute)
	lastActivity := now.Add(-1 * time.Minute)

	// Exactly at threshold should NOT trigger (need to exceed)
	input := CheckInput{
		Now:                 now,
		StartTime:           &startTime,
		LastActivityTime:    lastActivity,
		RepeatedOutputCount: 3000, // Exactly at threshold
	}

	result := detector.CheckTimeout(input)
	if result != TimeoutNone {
		t.Errorf("CheckTimeout() at exact threshold = %v, want TimeoutNone", result)
	}

	// One more than threshold should trigger
	input.RepeatedOutputCount = 3001
	result = detector.CheckTimeout(input)
	if result != TimeoutStale {
		t.Errorf("CheckTimeout() above threshold = %v, want TimeoutStale", result)
	}
}

func TestTimeoutDetector_EnabledChecks(t *testing.T) {
	tests := []struct {
		name       string
		cfg        TimeoutConfig
		wantActive bool
		wantComp   bool
		wantStale  bool
	}{
		{
			name:       "default config (completion disabled)",
			cfg:        DefaultTimeoutConfig(),
			wantActive: true,
			wantComp:   false, // CompletionTimeout disabled by default
			wantStale:  true,
		},
		{
			name: "all enabled",
			cfg: TimeoutConfig{
				ActivityTimeout:   30 * time.Minute,
				CompletionTimeout: 120 * time.Minute,
				StaleThreshold:    3000,
			},
			wantActive: true,
			wantComp:   true,
			wantStale:  true,
		},
		{
			name: "all disabled",
			cfg: TimeoutConfig{
				ActivityTimeout:   0,
				CompletionTimeout: 0,
				StaleThreshold:    0,
			},
			wantActive: false,
			wantComp:   false,
			wantStale:  false,
		},
		{
			name: "only activity",
			cfg: TimeoutConfig{
				ActivityTimeout:   10 * time.Minute,
				CompletionTimeout: 0,
				StaleThreshold:    0,
			},
			wantActive: true,
			wantComp:   false,
			wantStale:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			detector := NewTimeoutDetector(tc.cfg)

			if got := detector.IsActivityTimeoutEnabled(); got != tc.wantActive {
				t.Errorf("IsActivityTimeoutEnabled() = %v, want %v", got, tc.wantActive)
			}
			if got := detector.IsCompletionTimeoutEnabled(); got != tc.wantComp {
				t.Errorf("IsCompletionTimeoutEnabled() = %v, want %v", got, tc.wantComp)
			}
			if got := detector.IsStaleDetectionEnabled(); got != tc.wantStale {
				t.Errorf("IsStaleDetectionEnabled() = %v, want %v", got, tc.wantStale)
			}
		})
	}
}

func TestTimeoutDetector_Config(t *testing.T) {
	cfg := TimeoutConfig{
		ActivityTimeout:   15 * time.Minute,
		CompletionTimeout: 90 * time.Minute,
		StaleThreshold:    5000,
	}
	detector := NewTimeoutDetector(cfg)

	got := detector.Config()
	if got != cfg {
		t.Errorf("Config() = %+v, want %+v", got, cfg)
	}
}
