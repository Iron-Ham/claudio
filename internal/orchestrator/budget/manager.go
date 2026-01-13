// Package budget provides budget monitoring and enforcement for orchestrator sessions.
package budget

import (
	"time"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/logging"
)

// InstanceMetrics represents metrics for a single instance.
type InstanceMetrics struct {
	ID           string
	Status       string
	InputTokens  int64
	OutputTokens int64
	CacheRead    int64
	CacheWrite   int64
	Cost         float64
	APICalls     int
	StartTime    time.Time
	EndTime      *time.Time
}

// TotalTokens returns the sum of input and output tokens.
func (m *InstanceMetrics) TotalTokens() int64 {
	return m.InputTokens + m.OutputTokens
}

// Duration returns the duration of the instance if it has ended.
func (m *InstanceMetrics) Duration() time.Duration {
	if m.EndTime != nil {
		return m.EndTime.Sub(m.StartTime)
	}
	if !m.StartTime.IsZero() {
		return time.Since(m.StartTime)
	}
	return 0
}

// SessionMetrics holds aggregated metrics for the entire session.
type SessionMetrics struct {
	TotalInputTokens  int64
	TotalOutputTokens int64
	TotalCacheRead    int64
	TotalCacheWrite   int64
	TotalCost         float64
	TotalAPICalls     int
	TotalDuration     time.Duration
	InstanceCount     int
	ActiveCount       int
}

// InstanceProvider provides access to instance metrics.
type InstanceProvider interface {
	// GetAllInstanceMetrics returns metrics for all instances.
	GetAllInstanceMetrics() []InstanceMetrics
}

// InstancePauser can pause instances that exceed limits.
type InstancePauser interface {
	// PauseInstance pauses the instance with the given ID.
	PauseInstance(id string) error
}

// Callbacks defines callbacks for budget events.
type Callbacks struct {
	// OnBudgetLimit is called when the budget limit is exceeded.
	OnBudgetLimit func()
	// OnBudgetWarning is called when the budget warning threshold is reached.
	OnBudgetWarning func()
	// OnInstanceTokenLimit is called when an instance exceeds its token limit.
	OnInstanceTokenLimit func(instanceID string)
}

// Config holds budget configuration.
type Config struct {
	CostLimit             float64
	CostWarningThreshold  float64
	TokenLimitPerInstance int64
}

// Manager monitors and enforces budget limits.
type Manager struct {
	config    Config
	provider  InstanceProvider
	pauser    InstancePauser
	callbacks Callbacks
	logger    *logging.Logger
}

// NewManager creates a new budget manager.
func NewManager(cfg Config, provider InstanceProvider, pauser InstancePauser, callbacks Callbacks, logger *logging.Logger) *Manager {
	if logger == nil {
		logger = logging.NopLogger()
	}
	return &Manager{
		config:    cfg,
		provider:  provider,
		pauser:    pauser,
		callbacks: callbacks,
		logger:    logger,
	}
}

// NewManagerFromConfig creates a budget manager from application config.
func NewManagerFromConfig(appCfg *config.Config, provider InstanceProvider, pauser InstancePauser, callbacks Callbacks, logger *logging.Logger) *Manager {
	cfg := Config{}
	if appCfg != nil {
		cfg.CostLimit = appCfg.Resources.CostLimit
		cfg.CostWarningThreshold = appCfg.Resources.CostWarningThreshold
		cfg.TokenLimitPerInstance = appCfg.Resources.TokenLimitPerInstance
	}
	return NewManager(cfg, provider, pauser, callbacks, logger)
}

// UpdateConfig updates the budget configuration.
func (m *Manager) UpdateConfig(cfg Config) {
	m.config = cfg
}

// GetSessionMetrics aggregates metrics across all instances.
func (m *Manager) GetSessionMetrics() *SessionMetrics {
	if m.provider == nil {
		return &SessionMetrics{}
	}

	allMetrics := m.provider.GetAllInstanceMetrics()
	metrics := &SessionMetrics{
		InstanceCount: len(allMetrics),
	}

	for _, inst := range allMetrics {
		if inst.Status == "working" || inst.Status == "waiting_input" {
			metrics.ActiveCount++
		}

		metrics.TotalInputTokens += inst.InputTokens
		metrics.TotalOutputTokens += inst.OutputTokens
		metrics.TotalCacheRead += inst.CacheRead
		metrics.TotalCacheWrite += inst.CacheWrite
		metrics.TotalCost += inst.Cost
		metrics.TotalAPICalls += inst.APICalls
		metrics.TotalDuration += inst.Duration()
	}

	return metrics
}

// CheckLimits checks all budget limits and pauses instances if necessary.
// Returns true if any limit was exceeded.
func (m *Manager) CheckLimits() bool {
	if m.provider == nil {
		return false
	}

	limitExceeded := false

	// Get session totals
	sessionMetrics := m.GetSessionMetrics()

	// Check cost limit
	if m.config.CostLimit > 0 && sessionMetrics.TotalCost >= m.config.CostLimit {
		m.logger.Warn("budget limit exceeded, pausing all instances",
			"total_cost", sessionMetrics.TotalCost,
			"cost_limit", m.config.CostLimit,
		)

		// Pause all working instances
		allMetrics := m.provider.GetAllInstanceMetrics()
		for _, inst := range allMetrics {
			if inst.Status == "working" && m.pauser != nil {
				if err := m.pauser.PauseInstance(inst.ID); err != nil {
					m.logger.Error("failed to pause instance during budget limit enforcement",
						"instance_id", inst.ID,
						"error", err,
					)
				}
			}
		}

		if m.callbacks.OnBudgetLimit != nil {
			m.callbacks.OnBudgetLimit()
		}
		limitExceeded = true
	}

	// Check cost warning threshold
	if m.config.CostWarningThreshold > 0 && sessionMetrics.TotalCost >= m.config.CostWarningThreshold {
		m.logger.Warn("budget warning threshold reached",
			"total_cost", sessionMetrics.TotalCost,
			"warning_threshold", m.config.CostWarningThreshold,
		)

		if m.callbacks.OnBudgetWarning != nil {
			m.callbacks.OnBudgetWarning()
		}
	}

	// Check per-instance token limit
	if m.config.TokenLimitPerInstance > 0 {
		allMetrics := m.provider.GetAllInstanceMetrics()
		for _, inst := range allMetrics {
			if inst.Status == "working" && inst.TotalTokens() >= m.config.TokenLimitPerInstance {
				m.logger.Warn("instance token limit exceeded",
					"instance_id", inst.ID,
					"total_tokens", inst.TotalTokens(),
					"token_limit", m.config.TokenLimitPerInstance,
				)

				if m.pauser != nil {
					if err := m.pauser.PauseInstance(inst.ID); err != nil {
						m.logger.Error("failed to pause instance after token limit exceeded",
							"instance_id", inst.ID,
							"error", err,
						)
					}
				}

				if m.callbacks.OnInstanceTokenLimit != nil {
					m.callbacks.OnInstanceTokenLimit(inst.ID)
				}
				limitExceeded = true
			}
		}
	}

	return limitExceeded
}
