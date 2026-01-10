package orchestrator

import (
	"time"
)

// SessionMetrics holds aggregated metrics for the entire session
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

// GetSessionMetrics aggregates metrics across all instances in the session
func (o *Orchestrator) GetSessionMetrics() *SessionMetrics {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.session == nil {
		return &SessionMetrics{}
	}

	metrics := &SessionMetrics{
		InstanceCount: len(o.session.Instances),
	}

	for _, inst := range o.session.Instances {
		if inst.Status == StatusWorking || inst.Status == StatusWaitingInput {
			metrics.ActiveCount++
		}

		if inst.Metrics != nil {
			metrics.TotalInputTokens += inst.Metrics.InputTokens
			metrics.TotalOutputTokens += inst.Metrics.OutputTokens
			metrics.TotalCacheRead += inst.Metrics.CacheRead
			metrics.TotalCacheWrite += inst.Metrics.CacheWrite
			metrics.TotalCost += inst.Metrics.Cost
			metrics.TotalAPICalls += inst.Metrics.APICalls
			metrics.TotalDuration += inst.Metrics.Duration()
		}
	}

	return metrics
}

// checkBudgetLimits checks if any budget limits have been exceeded
func (o *Orchestrator) checkBudgetLimits() {
	if o.config == nil || o.session == nil {
		return
	}

	// Get session totals
	sessionMetrics := o.GetSessionMetrics()

	// Check cost limit
	if o.config.Resources.CostLimit > 0 && sessionMetrics.TotalCost >= o.config.Resources.CostLimit {
		// Pause all running instances
		for _, inst := range o.session.Instances {
			if inst.Status == StatusWorking {
				if mgr, ok := o.instances[inst.ID]; ok {
					_ = mgr.Pause()
					inst.Status = StatusPaused
				}
			}
		}
		o.executeNotification("notifications.on_budget_limit", nil)
	}

	// Check cost warning threshold
	if o.config.Resources.CostWarningThreshold > 0 && sessionMetrics.TotalCost >= o.config.Resources.CostWarningThreshold {
		o.executeNotification("notifications.on_budget_warning", nil)
	}

	// Check per-instance token limit
	if o.config.Resources.TokenLimitPerInstance > 0 {
		for _, inst := range o.session.Instances {
			if inst.Metrics != nil && inst.Status == StatusWorking {
				if inst.Metrics.TotalTokens() >= o.config.Resources.TokenLimitPerInstance {
					if mgr, ok := o.instances[inst.ID]; ok {
						_ = mgr.Pause()
						inst.Status = StatusPaused
					}
				}
			}
		}
	}
}
