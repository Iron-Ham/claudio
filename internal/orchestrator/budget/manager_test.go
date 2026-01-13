package budget

import (
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/config"
)

// mockProvider implements InstanceProvider for testing.
type mockProvider struct {
	metrics []InstanceMetrics
}

func (m *mockProvider) GetAllInstanceMetrics() []InstanceMetrics {
	return m.metrics
}

// mockPauser implements InstancePauser for testing.
type mockPauser struct {
	paused []string
}

func (m *mockPauser) PauseInstance(id string) error {
	m.paused = append(m.paused, id)
	return nil
}

func TestNewManager(t *testing.T) {
	cfg := Config{
		CostLimit:             10.0,
		CostWarningThreshold:  5.0,
		TokenLimitPerInstance: 1000,
	}

	mgr := NewManager(cfg, nil, nil, Callbacks{}, nil)
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}

	if mgr.config.CostLimit != 10.0 {
		t.Errorf("CostLimit = %v, want 10.0", mgr.config.CostLimit)
	}
}

func TestNewManagerFromConfig(t *testing.T) {
	appCfg := &config.Config{
		Resources: config.ResourceConfig{
			CostLimit:             15.0,
			CostWarningThreshold:  8.0,
			TokenLimitPerInstance: 2000,
		},
	}

	mgr := NewManagerFromConfig(appCfg, nil, nil, Callbacks{}, nil)
	if mgr == nil {
		t.Fatal("NewManagerFromConfig returned nil")
	}

	if mgr.config.CostLimit != 15.0 {
		t.Errorf("CostLimit = %v, want 15.0", mgr.config.CostLimit)
	}
	if mgr.config.TokenLimitPerInstance != 2000 {
		t.Errorf("TokenLimitPerInstance = %v, want 2000", mgr.config.TokenLimitPerInstance)
	}
}

func TestNewManagerFromConfig_NilConfig(t *testing.T) {
	mgr := NewManagerFromConfig(nil, nil, nil, Callbacks{}, nil)
	if mgr == nil {
		t.Fatal("NewManagerFromConfig returned nil with nil config")
	}

	if mgr.config.CostLimit != 0 {
		t.Errorf("CostLimit = %v, want 0 for nil config", mgr.config.CostLimit)
	}
}

func TestGetSessionMetrics(t *testing.T) {
	provider := &mockProvider{
		metrics: []InstanceMetrics{
			{
				ID:           "inst-1",
				Status:       "working",
				InputTokens:  100,
				OutputTokens: 50,
				CacheRead:    10,
				CacheWrite:   5,
				Cost:         0.50,
				APICalls:     3,
			},
			{
				ID:           "inst-2",
				Status:       "waiting_input",
				InputTokens:  200,
				OutputTokens: 100,
				CacheRead:    20,
				CacheWrite:   10,
				Cost:         1.00,
				APICalls:     5,
			},
			{
				ID:           "inst-3",
				Status:       "completed",
				InputTokens:  50,
				OutputTokens: 25,
				Cost:         0.25,
				APICalls:     2,
			},
		},
	}

	mgr := NewManager(Config{}, provider, nil, Callbacks{}, nil)
	metrics := mgr.GetSessionMetrics()

	if metrics.InstanceCount != 3 {
		t.Errorf("InstanceCount = %d, want 3", metrics.InstanceCount)
	}
	if metrics.ActiveCount != 2 {
		t.Errorf("ActiveCount = %d, want 2", metrics.ActiveCount)
	}
	if metrics.TotalInputTokens != 350 {
		t.Errorf("TotalInputTokens = %d, want 350", metrics.TotalInputTokens)
	}
	if metrics.TotalOutputTokens != 175 {
		t.Errorf("TotalOutputTokens = %d, want 175", metrics.TotalOutputTokens)
	}
	if metrics.TotalCost != 1.75 {
		t.Errorf("TotalCost = %v, want 1.75", metrics.TotalCost)
	}
	if metrics.TotalAPICalls != 10 {
		t.Errorf("TotalAPICalls = %d, want 10", metrics.TotalAPICalls)
	}
}

func TestGetSessionMetrics_NilProvider(t *testing.T) {
	mgr := NewManager(Config{}, nil, nil, Callbacks{}, nil)
	metrics := mgr.GetSessionMetrics()

	if metrics.InstanceCount != 0 {
		t.Errorf("InstanceCount = %d, want 0 for nil provider", metrics.InstanceCount)
	}
}

func TestCheckLimits_CostLimit(t *testing.T) {
	provider := &mockProvider{
		metrics: []InstanceMetrics{
			{ID: "inst-1", Status: "working", Cost: 8.0},
			{ID: "inst-2", Status: "working", Cost: 5.0},
		},
	}
	pauser := &mockPauser{}

	var budgetLimitCalled bool
	callbacks := Callbacks{
		OnBudgetLimit: func() {
			budgetLimitCalled = true
		},
	}

	cfg := Config{CostLimit: 10.0}
	mgr := NewManager(cfg, provider, pauser, callbacks, nil)

	exceeded := mgr.CheckLimits()

	if !exceeded {
		t.Error("CheckLimits should return true when cost limit exceeded")
	}
	if !budgetLimitCalled {
		t.Error("OnBudgetLimit callback should be called")
	}
	if len(pauser.paused) != 2 {
		t.Errorf("Should pause 2 instances, paused %d", len(pauser.paused))
	}
}

func TestCheckLimits_CostWarning(t *testing.T) {
	provider := &mockProvider{
		metrics: []InstanceMetrics{
			{ID: "inst-1", Status: "working", Cost: 6.0},
		},
	}

	var warningCalled bool
	callbacks := Callbacks{
		OnBudgetWarning: func() {
			warningCalled = true
		},
	}

	cfg := Config{
		CostLimit:            10.0,
		CostWarningThreshold: 5.0,
	}
	mgr := NewManager(cfg, provider, nil, callbacks, nil)

	exceeded := mgr.CheckLimits()

	if exceeded {
		t.Error("CheckLimits should return false when only warning threshold reached")
	}
	if !warningCalled {
		t.Error("OnBudgetWarning callback should be called")
	}
}

func TestCheckLimits_TokenLimit(t *testing.T) {
	provider := &mockProvider{
		metrics: []InstanceMetrics{
			{ID: "inst-1", Status: "working", InputTokens: 800, OutputTokens: 300},
			{ID: "inst-2", Status: "working", InputTokens: 400, OutputTokens: 200},
		},
	}
	pauser := &mockPauser{}

	var tokenLimitInstances []string
	callbacks := Callbacks{
		OnInstanceTokenLimit: func(id string) {
			tokenLimitInstances = append(tokenLimitInstances, id)
		},
	}

	cfg := Config{TokenLimitPerInstance: 1000}
	mgr := NewManager(cfg, provider, pauser, callbacks, nil)

	exceeded := mgr.CheckLimits()

	if !exceeded {
		t.Error("CheckLimits should return true when token limit exceeded")
	}
	if len(tokenLimitInstances) != 1 {
		t.Errorf("Should trigger token limit for 1 instance, got %d", len(tokenLimitInstances))
	}
	if tokenLimitInstances[0] != "inst-1" {
		t.Errorf("Token limit triggered for %s, want inst-1", tokenLimitInstances[0])
	}
	if len(pauser.paused) != 1 {
		t.Errorf("Should pause 1 instance, paused %d", len(pauser.paused))
	}
}

func TestCheckLimits_NoLimitsConfigured(t *testing.T) {
	provider := &mockProvider{
		metrics: []InstanceMetrics{
			{ID: "inst-1", Status: "working", Cost: 100.0, InputTokens: 10000},
		},
	}

	cfg := Config{} // No limits set
	mgr := NewManager(cfg, provider, nil, Callbacks{}, nil)

	exceeded := mgr.CheckLimits()

	if exceeded {
		t.Error("CheckLimits should return false when no limits configured")
	}
}

func TestCheckLimits_NilProvider(t *testing.T) {
	cfg := Config{CostLimit: 10.0}
	mgr := NewManager(cfg, nil, nil, Callbacks{}, nil)

	exceeded := mgr.CheckLimits()

	if exceeded {
		t.Error("CheckLimits should return false with nil provider")
	}
}

func TestUpdateConfig(t *testing.T) {
	mgr := NewManager(Config{CostLimit: 5.0}, nil, nil, Callbacks{}, nil)

	mgr.UpdateConfig(Config{CostLimit: 20.0})

	if mgr.config.CostLimit != 20.0 {
		t.Errorf("CostLimit = %v, want 20.0 after update", mgr.config.CostLimit)
	}
}

func TestInstanceMetrics_TotalTokens(t *testing.T) {
	m := InstanceMetrics{
		InputTokens:  100,
		OutputTokens: 50,
	}

	if m.TotalTokens() != 150 {
		t.Errorf("TotalTokens() = %d, want 150", m.TotalTokens())
	}
}

func TestInstanceMetrics_Duration(t *testing.T) {
	start := time.Now().Add(-time.Hour)
	end := time.Now()

	tests := []struct {
		name    string
		metrics InstanceMetrics
		want    time.Duration
	}{
		{
			name: "with end time",
			metrics: InstanceMetrics{
				StartTime: start,
				EndTime:   &end,
			},
			want: time.Hour,
		},
		{
			name: "zero start time",
			metrics: InstanceMetrics{
				StartTime: time.Time{},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.metrics.Duration()
			// Allow small tolerance for "with end time" case
			if tt.name == "with end time" {
				diff := got - tt.want
				if diff < -time.Second || diff > time.Second {
					t.Errorf("Duration() = %v, want approximately %v", got, tt.want)
				}
			} else if got != tt.want {
				t.Errorf("Duration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckLimits_SkipsNonWorkingInstances(t *testing.T) {
	provider := &mockProvider{
		metrics: []InstanceMetrics{
			{ID: "inst-1", Status: "completed", InputTokens: 2000},
			{ID: "inst-2", Status: "paused", InputTokens: 2000},
		},
	}
	pauser := &mockPauser{}

	cfg := Config{TokenLimitPerInstance: 1000}
	mgr := NewManager(cfg, provider, pauser, Callbacks{}, nil)

	exceeded := mgr.CheckLimits()

	if exceeded {
		t.Error("CheckLimits should not trigger for non-working instances")
	}
	if len(pauser.paused) != 0 {
		t.Errorf("Should not pause any instances, paused %d", len(pauser.paused))
	}
}
