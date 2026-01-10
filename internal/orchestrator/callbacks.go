package orchestrator

import (
	"time"

	"github.com/Iron-Ham/claudio/internal/instance"
)

// SetPRCompleteCallback sets the callback for PR workflow completion.
// This callback is invoked when a PR workflow finishes (success or failure),
// allowing the TUI to update its state and remove completed instances.
func (o *Orchestrator) SetPRCompleteCallback(cb PRCompleteCallback) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.prCompleteCallback = cb
}

// SetPROpenedCallback sets the callback for when a PR URL is detected in instance output.
// This is used for inline PR creation detection, where Claude creates a PR directly
// without going through the PR workflow.
func (o *Orchestrator) SetPROpenedCallback(cb PROpenedCallback) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.prOpenedCallback = cb
}

// SetTimeoutCallback sets the callback for when an instance timeout is detected.
// The callback receives the instance ID and the type of timeout that occurred
// (activity timeout, completion timeout, or stale detection).
func (o *Orchestrator) SetTimeoutCallback(cb TimeoutCallback) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.timeoutCallback = cb
}

// SetBellCallback sets the callback for when a terminal bell is detected in an instance.
// Terminal bells are typically used by Claude to signal that it needs attention
// (e.g., waiting for user input or permission).
func (o *Orchestrator) SetBellCallback(cb BellCallback) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.bellCallback = cb
}

// handlePRWorkflowComplete handles PR workflow completion.
// It cleans up the PR workflow from tracking and notifies via callback if set.
func (o *Orchestrator) handlePRWorkflowComplete(instanceID string, success bool, output string) {
	o.mu.Lock()
	// Clean up PR workflow
	delete(o.prWorkflows, instanceID)

	// Get the callback before unlocking
	callback := o.prCompleteCallback
	o.mu.Unlock()

	// Notify via callback if set
	if callback != nil {
		callback(instanceID, success)
	}
}

// handleInstanceExit handles when a Claude instance process exits.
// It updates the instance status to completed, records end time metrics,
// and executes any configured completion notifications.
func (o *Orchestrator) handleInstanceExit(id string) {
	inst := o.GetInstance(id)
	if inst != nil {
		inst.Status = StatusCompleted
		inst.PID = 0
		// Record end time for metrics
		if inst.Metrics != nil {
			now := time.Now()
			inst.Metrics.EndTime = &now
		}
		_ = o.saveSession()
		o.executeNotification("notifications.on_completion", inst)
	}
}

// handleInstanceWaitingInput handles when a Claude instance is waiting for input.
// It updates the instance status and executes any configured waiting input notifications.
func (o *Orchestrator) handleInstanceWaitingInput(id string) {
	inst := o.GetInstance(id)
	if inst != nil {
		inst.Status = StatusWaitingInput
		_ = o.saveSession()
		o.executeNotification("notifications.on_waiting_input", inst)
	}
}

// handleInstancePROpened handles when a PR URL is detected in instance output.
// This indicates that Claude has created a PR inline (not through the PR workflow).
// The TUI callback handles instance removal if auto-remove is configured.
func (o *Orchestrator) handleInstancePROpened(id string) {
	o.mu.RLock()
	callback := o.prOpenedCallback
	o.mu.RUnlock()

	// Notify via callback if set (TUI will handle the removal)
	if callback != nil {
		callback(id)
	}
}

// handleInstanceTimeout handles when an instance timeout is detected.
// It updates the instance status based on the timeout type:
//   - TimeoutActivity/TimeoutStale: StatusStuck
//   - TimeoutCompletion: StatusTimeout
//
// It also records end time metrics and notifies via callback.
func (o *Orchestrator) handleInstanceTimeout(id string, timeoutType instance.TimeoutType) {
	inst := o.GetInstance(id)
	if inst == nil {
		return
	}

	// Update status based on timeout type
	switch timeoutType {
	case instance.TimeoutActivity, instance.TimeoutStale:
		inst.Status = StatusStuck
	case instance.TimeoutCompletion:
		inst.Status = StatusTimeout
	}

	// Record end time for metrics
	if inst.Metrics != nil {
		now := time.Now()
		inst.Metrics.EndTime = &now
	}

	_ = o.saveSession()

	// Notify via callback if set (TUI will handle the display)
	o.mu.RLock()
	callback := o.timeoutCallback
	o.mu.RUnlock()

	if callback != nil {
		callback(id, timeoutType)
	}
}

// handleInstanceBell handles when a terminal bell is detected in an instance.
// Terminal bells are forwarded to the TUI via callback for user notification.
func (o *Orchestrator) handleInstanceBell(id string) {
	o.mu.RLock()
	callback := o.bellCallback
	o.mu.RUnlock()

	if callback != nil {
		callback(id)
	}
}

// handleInstanceMetrics updates instance metrics when they change.
// This is called periodically by the instance manager when new metrics are parsed
// from Claude's output. It also checks budget limits after updating metrics.
func (o *Orchestrator) handleInstanceMetrics(id string, metrics *instance.ParsedMetrics) {
	inst := o.GetInstance(id)
	if inst == nil || metrics == nil {
		return
	}

	// Update instance metrics
	if inst.Metrics == nil {
		inst.Metrics = &Metrics{}
	}

	inst.Metrics.InputTokens = metrics.InputTokens
	inst.Metrics.OutputTokens = metrics.OutputTokens
	inst.Metrics.CacheRead = metrics.CacheRead
	inst.Metrics.CacheWrite = metrics.CacheWrite
	inst.Metrics.APICalls = metrics.APICalls

	// Use parsed cost if available, otherwise calculate from tokens
	if metrics.Cost > 0 {
		inst.Metrics.Cost = metrics.Cost
	} else {
		inst.Metrics.Cost = instance.CalculateCost(
			metrics.InputTokens,
			metrics.OutputTokens,
			metrics.CacheRead,
			metrics.CacheWrite,
		)
	}

	// Check budget limits
	o.checkBudgetLimits()

	// Save session periodically (not on every metric update to avoid excessive I/O)
	// The session will be saved when status changes occur
}
