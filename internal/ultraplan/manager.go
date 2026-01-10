// Package ultraplan provides types and utilities for Ultra-Plan orchestration.
package ultraplan

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/issue"
	"github.com/Iron-Ham/claudio/internal/logging"
)

// Manager manages the execution of an ultra-plan session.
//
// Manager is responsible for:
//   - Tracking session state and phase transitions
//   - Emitting events for UI updates
//   - Managing candidate plans during multi-pass planning
//   - Validating and parsing plans with logging
//   - Coordinating issue auto-close on task completion
//
// Manager is designed to be used by the Coordinator, which handles the
// higher-level orchestration of planning, execution, and consolidation phases.
type Manager struct {
	session      *Session
	logger       *logging.Logger
	issueService *issue.Service

	// Event handling
	eventChan     chan CoordinatorEvent
	eventCallback func(CoordinatorEvent)

	// Synchronization
	mu sync.RWMutex
	wg sync.WaitGroup

	// Cancellation
	stopChan chan struct{}
	stopped  bool
}

// NewManager creates a new ultra-plan manager.
//
// The logger parameter should be passed from the Coordinator and will be used
// for structured logging throughout plan lifecycle events. If nil, a no-op
// logger is used.
func NewManager(session *Session, logger *logging.Logger) *Manager {
	if logger == nil {
		logger = logging.NopLogger()
	}
	planLogger := logger.WithPhase("ultraplan")
	return &Manager{
		session:      session,
		logger:       planLogger,
		issueService: issue.NewService(planLogger),
		eventChan:    make(chan CoordinatorEvent, 100),
		stopChan:     make(chan struct{}),
	}
}

// SetEventCallback sets the callback for coordinator events.
func (m *Manager) SetEventCallback(cb func(CoordinatorEvent)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventCallback = cb
}

// Session returns the ultra-plan session.
func (m *Manager) Session() *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.session
}

// emitEvent sends an event to the event channel and callback.
func (m *Manager) emitEvent(event CoordinatorEvent) {
	event.Timestamp = time.Now()

	// Non-blocking send to channel
	select {
	case m.eventChan <- event:
	default:
		// Channel full - log the dropped event for debugging
		m.logger.Warn("event channel full, event dropped",
			"event_type", string(event.Type),
			"event_message", event.Message,
		)
	}

	// Call callback if set
	m.mu.RLock()
	cb := m.eventCallback
	m.mu.RUnlock()
	if cb != nil {
		cb(event)
	}
}

// EmitEvent is a public method to emit events from external callers.
func (m *Manager) EmitEvent(event CoordinatorEvent) {
	m.emitEvent(event)
}

// Events returns the event channel for monitoring.
func (m *Manager) Events() <-chan CoordinatorEvent {
	return m.eventChan
}

// Stop stops the ultra-plan execution.
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.stopped {
		m.stopped = true
		close(m.stopChan)
	}
	m.mu.Unlock()

	// Wait for any running goroutines
	m.wg.Wait()
}

// IsStopped returns whether the manager has been stopped.
func (m *Manager) IsStopped() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stopped
}

// SetPhase updates the session phase and emits an event.
func (m *Manager) SetPhase(phase UltraPlanPhase) {
	m.mu.Lock()
	m.session.Phase = phase
	m.mu.Unlock()

	m.emitEvent(CoordinatorEvent{
		Type:    EventPhaseChange,
		Message: string(phase),
	})
}

// SetPlan sets the plan on the session and logs the plan creation.
// The objective is truncated to 100 characters in the log for readability.
func (m *Manager) SetPlan(plan *PlanSpec) {
	m.mu.Lock()
	m.session.Plan = plan
	m.mu.Unlock()

	if plan != nil {
		// Truncate objective for logging
		objective := plan.Objective
		if len(objective) > 100 {
			objective = objective[:97] + "..."
		}

		m.logger.Info("plan created",
			"plan_id", plan.ID,
			"task_count", len(plan.Tasks),
			"objective", objective,
		)
	}

	m.emitEvent(CoordinatorEvent{
		Type:    EventPlanReady,
		Message: "plan ready",
	})
}

// SetSelectedPlanIndex sets the selected plan index and logs the selection.
// This is used during multi-pass planning when a plan is chosen from candidates.
func (m *Manager) SetSelectedPlanIndex(index int, action string) {
	m.mu.Lock()
	m.session.SelectedPlanIndex = index
	m.mu.Unlock()

	m.logger.Info("plan selected",
		"selected_index", index,
		"action", action,
	)

	m.emitEvent(CoordinatorEvent{
		Type:      EventPlanSelected,
		PlanIndex: index,
		Message:   action,
	})
}

// ValidatePlanWithLogging validates a plan and logs DEBUG-level information about the validation.
// It logs the dependency graph structure, execution order calculation, and any validation errors.
// Returns an error if validation fails.
func (m *Manager) ValidatePlanWithLogging(plan *PlanSpec) error {
	if plan == nil {
		m.logger.Error("plan validation failed", "error", "plan is nil")
		return fmt.Errorf("plan is nil")
	}

	// Log dependency graph construction at DEBUG level
	m.logger.Debug("dependency graph constructed",
		"task_count", len(plan.Tasks),
		"dependency_count", countDependencies(plan.DependencyGraph),
	)

	// Log execution order calculation at DEBUG level
	if len(plan.ExecutionOrder) > 0 {
		groupSizes := make([]int, len(plan.ExecutionOrder))
		for i, group := range plan.ExecutionOrder {
			groupSizes[i] = len(group)
		}
		m.logger.Debug("execution order calculated",
			"group_count", len(plan.ExecutionOrder),
			"group_sizes", groupSizes,
		)
	}

	// Perform comprehensive validation
	result, err := ValidatePlan(plan)
	if err != nil {
		m.logger.Error("plan validation error", "error", err.Error())
		return err
	}

	if !result.IsValid {
		// Build error message from validation messages
		var errMsgs []string
		for _, msg := range result.Messages {
			if msg.IsError() {
				errMsgs = append(errMsgs, msg.Message)
				// Check for specific error types
				if strings.Contains(msg.Message, "unknown task") {
					m.logger.Error("invalid dependency error", "error", msg.Message)
				}
			}
		}
		if len(errMsgs) > 0 {
			m.logger.Debug("plan validation failed",
				"error_count", result.ErrorCount,
				"warning_count", result.WarningCount,
			)
			return fmt.Errorf("plan validation failed: %s", strings.Join(errMsgs, "; "))
		}
	}

	m.logger.Debug("plan validation completed",
		"valid", result.IsValid,
		"warning_count", result.WarningCount,
	)
	return nil
}

// countDependencies counts the total number of dependencies in the graph.
func countDependencies(deps map[string][]string) int {
	count := 0
	for _, d := range deps {
		count += len(d)
	}
	return count
}

// ParsePlanFromOutputWithLogging parses a plan from Claude's output and logs any errors.
// On success, it logs DEBUG-level information about the parsed plan.
// On failure, it logs ERROR-level information about the parsing failure.
func (m *Manager) ParsePlanFromOutputWithLogging(output string, objective string) (*PlanSpec, error) {
	plan, err := ParsePlanFromOutput(output, objective)
	if err != nil {
		m.logger.Error("plan parsing failed",
			"error", err.Error(),
			"output_length", len(output),
		)
		return nil, err
	}

	m.logger.Debug("plan parsed successfully",
		"plan_id", plan.ID,
		"task_count", len(plan.Tasks),
	)
	return plan, nil
}

// ParsePlanFromFileWithLogging parses a plan from a file and logs any errors.
// On success, it logs DEBUG-level information about the parsed plan.
// On failure, it logs ERROR-level information about the parsing failure.
func (m *Manager) ParsePlanFromFileWithLogging(filepath string, objective string) (*PlanSpec, error) {
	plan, err := ParsePlanFromFile(filepath, objective)
	if err != nil {
		m.logger.Error("plan parsing failed",
			"error", err.Error(),
			"filepath", filepath,
		)
		return nil, err
	}

	m.logger.Debug("plan parsed successfully",
		"plan_id", plan.ID,
		"task_count", len(plan.Tasks),
		"filepath", filepath,
	)
	return plan, nil
}

// StoreCandidatePlan stores a candidate plan at the given index with proper mutex protection.
// It initializes the CandidatePlans slice if needed, marks the coordinator as processed,
// and returns the count of non-nil plans collected.
// This method is safe for concurrent access from multiple goroutines.
// Pass nil for plan to mark a coordinator as completed but failed to produce a valid plan.
func (m *Manager) StoreCandidatePlan(planIndex int, plan *PlanSpec) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Initialize CandidatePlans slice if needed
	numCoordinators := len(m.session.PlanCoordinatorIDs)
	if len(m.session.CandidatePlans) < numCoordinators {
		newPlans := make([]*PlanSpec, numCoordinators)
		copy(newPlans, m.session.CandidatePlans)
		m.session.CandidatePlans = newPlans
	}

	// Initialize ProcessedCoordinators map if needed
	if m.session.ProcessedCoordinators == nil {
		m.session.ProcessedCoordinators = make(map[int]bool)
	}

	// Store the plan (or nil if parsing failed)
	if planIndex >= 0 && planIndex < len(m.session.CandidatePlans) {
		m.session.CandidatePlans[planIndex] = plan
		m.session.ProcessedCoordinators[planIndex] = true
	}

	// Count non-nil plans
	count := 0
	for _, p := range m.session.CandidatePlans {
		if p != nil {
			count++
		}
	}
	return count
}

// CountCandidatePlans returns the number of non-nil candidate plans collected.
func (m *Manager) CountCandidatePlans() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, p := range m.session.CandidatePlans {
		if p != nil {
			count++
		}
	}
	return count
}

// CountCoordinatorsCompleted returns the number of coordinators that have completed
// (regardless of whether they produced a valid plan or not).
func (m *Manager) CountCoordinatorsCompleted() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.session.ProcessedCoordinators)
}

// AssignTaskToInstance maps a task ID to its executing instance ID.
func (m *Manager) AssignTaskToInstance(taskID, instanceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session.TaskToInstance == nil {
		m.session.TaskToInstance = make(map[string]string)
	}
	m.session.TaskToInstance[taskID] = instanceID
}

// MarkTaskComplete marks a task as completed and handles auto-close of linked issues.
func (m *Manager) MarkTaskComplete(taskID string) {
	m.mu.Lock()
	// Add to completed list
	m.session.CompletedTasks = append(m.session.CompletedTasks, taskID)

	// Remove from TaskToInstance (task is done)
	delete(m.session.TaskToInstance, taskID)

	// Get task data for issue auto-close while holding lock
	// Copy the IssueURL to avoid data race after unlock
	task := m.session.GetTask(taskID)
	var issueURL string
	if task != nil {
		issueURL = task.IssueURL
	}
	m.mu.Unlock()

	// Auto-close linked issue if configured
	if issueURL != "" {
		m.autoCloseIssueByURL(taskID, issueURL)
	}
}

// autoCloseIssueByURL attempts to close the linked issue for a completed task.
// This variant takes the issue URL directly to avoid holding locks during the API call.
func (m *Manager) autoCloseIssueByURL(taskID, issueURL string) {
	if issueURL == "" || m.issueService == nil {
		return
	}

	ctx := context.Background()
	if err := m.issueService.Close(ctx, issueURL); err != nil {
		m.logger.Warn("failed to auto-close issue",
			"task_id", taskID,
			"issue_url", issueURL,
			"error", err.Error(),
		)
	} else {
		m.logger.Info("auto-closed linked issue",
			"task_id", taskID,
			"issue_url", issueURL,
		)
	}
}

// MarkTaskFailed marks a task as failed with a reason.
func (m *Manager) MarkTaskFailed(taskID, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Add to failed list
	m.session.FailedTasks = append(m.session.FailedTasks, taskID)

	// Remove from TaskToInstance
	delete(m.session.TaskToInstance, taskID)

	m.logger.Warn("task failed",
		"task_id", taskID,
		"reason", reason,
	)
}

// Logger returns the manager's logger for use by external components.
func (m *Manager) Logger() *logging.Logger {
	return m.logger
}

// IssueService returns the issue service for external use.
func (m *Manager) IssueService() *issue.Service {
	return m.issueService
}
