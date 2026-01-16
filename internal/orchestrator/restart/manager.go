// Package restart provides centralized step restart functionality for the coordinator.
// It encapsulates the logic for restarting various ultra-plan steps (planning,
// task execution, synthesis, revision, consolidation) without requiring the
// coordinator to manage all the state transitions directly.
package restart

import (
	"fmt"
	"sync"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// UltraPlanPhase represents the current phase of an ultra-plan session.
type UltraPlanPhase string

// Phase constants.
const (
	PhasePlanning      UltraPlanPhase = "planning"
	PhasePlanSelection UltraPlanPhase = "plan_selection"
	PhaseRefresh       UltraPlanPhase = "context_refresh"
	PhaseExecuting     UltraPlanPhase = "executing"
	PhaseSynthesis     UltraPlanPhase = "synthesis"
	PhaseRevision      UltraPlanPhase = "revision"
	PhaseConsolidating UltraPlanPhase = "consolidating"
	PhaseComplete      UltraPlanPhase = "complete"
	PhaseFailed        UltraPlanPhase = "failed"
)

// StepType identifies the type of step being restarted.
type StepType string

// Step type constants.
const (
	StepTypePlanning          StepType = "planning"
	StepTypePlanManager       StepType = "plan_manager"
	StepTypeTask              StepType = "task"
	StepTypeSynthesis         StepType = "synthesis"
	StepTypeRevision          StepType = "revision"
	StepTypeConsolidation     StepType = "consolidation"
	StepTypeGroupConsolidator StepType = "group_consolidator"
)

// StepInfo provides information about a step for restart operations.
type StepInfo struct {
	Type       StepType
	InstanceID string
	TaskID     string // Only for task steps
	GroupIndex int    // Only for group consolidators or task groups
	Label      string // Human-readable label
}

// SessionOperations defines session state operations needed by the restart manager.
// This interface abstracts the UltraPlanSession to allow independent testing.
type SessionOperations interface {
	// GetCoordinatorID returns the coordinator instance ID
	GetCoordinatorID() string
	// SetCoordinatorID sets the coordinator instance ID
	SetCoordinatorID(id string)

	// GetPlanCoordinatorIDs returns multi-pass plan coordinator IDs
	GetPlanCoordinatorIDs() []string

	// GetPlanManagerID returns the plan manager instance ID
	GetPlanManagerID() string
	// SetPlanManagerID sets the plan manager instance ID
	SetPlanManagerID(id string)

	// GetTaskToInstance returns the task-to-instance mapping
	GetTaskToInstance() map[string]string

	// GetGroupConsolidatorIDs returns group consolidator IDs
	GetGroupConsolidatorIDs() []string

	// GetSynthesisID returns the synthesis instance ID
	GetSynthesisID() string
	// SetSynthesisID sets the synthesis instance ID
	SetSynthesisID(id string)

	// GetRevisionID returns the revision instance ID
	GetRevisionID() string
	// SetRevisionID sets the revision instance ID
	SetRevisionID(id string)

	// GetConsolidationID returns the consolidation instance ID
	GetConsolidationID() string
	// SetConsolidationID sets the consolidation instance ID
	SetConsolidationID(id string)

	// GetPlan returns the plan or nil
	GetPlan() any
	// SetPlan sets the plan
	SetPlan(plan any)

	// GetPhase returns the current phase
	GetPhase() UltraPlanPhase
	// SetPhase sets the current phase
	SetPhase(phase UltraPlanPhase)

	// IsMultiPass returns true if multi-pass planning is enabled
	IsMultiPass() bool

	// GetTask returns a task by ID
	GetTask(taskID string) any

	// ClearSynthesisState clears all synthesis-related state
	ClearSynthesisState()

	// GetRevision returns the revision state
	GetRevision() any

	// ClearConsolidationState clears consolidation-related state
	ClearConsolidationState()

	// ClearPRUrls clears PR URLs
	ClearPRUrls()

	// GetExecutionOrder returns the execution order
	GetExecutionOrder() [][]string

	// ClearGroupConsolidatorID clears a specific group consolidator ID
	ClearGroupConsolidatorID(groupIndex int)

	// GetMultiPassStrategyNames returns the strategy names for multi-pass planning
	// Returns nil if not available, in which case numbered labels are used
	GetMultiPassStrategyNames() []string

	// GetTaskGroupIndex returns the group index for a task (-1 if not found)
	GetTaskGroupIndex(taskID string) int

	// ClearGroupConsolidatedBranch clears a specific consolidated branch
	ClearGroupConsolidatedBranch(groupIndex int)

	// ClearGroupConsolidationContext clears a specific consolidation context
	ClearGroupConsolidationContext(groupIndex int)

	// GetCompletedTasks returns the completed task IDs
	GetCompletedTasks() []string
	// SetCompletedTasks sets the completed tasks
	SetCompletedTasks(tasks []string)

	// GetFailedTasks returns the failed task IDs
	GetFailedTasks() []string
	// SetFailedTasks sets the failed tasks
	SetFailedTasks(tasks []string)

	// DeleteTaskFromInstance removes a task from task-to-instance mapping
	DeleteTaskFromInstance(taskID string)

	// DeleteTaskCommitCount removes a task from commit counts
	DeleteTaskCommitCount(taskID string)

	// ClearGroupDecision clears the group decision state
	ClearGroupDecision()
}

// RetryManagerOperations defines retry state operations.
type RetryManagerOperations interface {
	// Reset clears retry state for a task
	Reset(taskID string)
	// GetAllStates returns all task retry states
	GetAllStates() map[string]any
}

// InstanceOperations defines instance management operations.
type InstanceOperations interface {
	// GetInstance returns an instance by ID
	GetInstance(id string) any
	// StopInstance stops a running instance
	StopInstance(inst any) error
}

// PlanningOperations defines planning phase operations.
type PlanningOperations interface {
	// RunPlanning starts the planning phase
	RunPlanning() error
	// RunPlanManager starts the plan manager for multi-pass planning
	RunPlanManager() error
	// ResetPlanningOrchestrator resets the planning orchestrator state
	ResetPlanningOrchestrator()
}

// ExecutionOperations defines execution phase operations.
type ExecutionOperations interface {
	// StartTask starts a specific task
	StartTask(taskID string, completionChan chan<- any) error
	// ResetExecutionOrchestrator resets the execution orchestrator state
	ResetExecutionOrchestrator()
	// GetRunningCount returns the number of running tasks
	GetRunningCount() int
}

// SynthesisOperations defines synthesis phase operations.
type SynthesisOperations interface {
	// RunSynthesis starts the synthesis phase
	RunSynthesis() error
	// StartRevision starts the revision phase
	StartRevision(issues []any) error
	// ResetSynthesisOrchestrator resets the synthesis orchestrator state
	ResetSynthesisOrchestrator()
}

// ConsolidationOperations defines consolidation phase operations.
type ConsolidationOperations interface {
	// StartConsolidation starts the consolidation phase
	StartConsolidation() error
	// StartGroupConsolidatorSession starts a group consolidator
	StartGroupConsolidatorSession(groupIndex int) error
	// ResetConsolidationOrchestrator resets the consolidation orchestrator state
	ResetConsolidationOrchestrator()
}

// PersistenceOperations defines session persistence operations.
type PersistenceOperations interface {
	// SaveSession persists the session state
	SaveSession() error
	// SetTaskRetries sets the task retry states on the session
	SetTaskRetries(retries map[string]any)
}

// Context holds all the dependencies needed by the restart manager.
type Context struct {
	Session       SessionOperations
	RetryManager  RetryManagerOperations
	Instances     InstanceOperations
	Planning      PlanningOperations
	Execution     ExecutionOperations
	Synthesis     SynthesisOperations
	Consolidation ConsolidationOperations
	Persistence   PersistenceOperations
	Logger        *logging.Logger
}

// Manager handles step restart operations.
// It encapsulates the complex state management required to restart
// various ultra-plan steps while maintaining data consistency.
type Manager struct {
	ctx    *Context
	logger *logging.Logger
	mu     sync.Mutex
}

// NewManager creates a new restart manager with the given context.
func NewManager(ctx *Context) *Manager {
	logger := logging.NopLogger()
	if ctx != nil && ctx.Logger != nil {
		logger = ctx.Logger.WithPhase("restart-manager")
	}
	return &Manager{
		ctx:    ctx,
		logger: logger,
	}
}

// GetStepInfo returns information about a step given its instance ID.
// This is used to determine what kind of step is selected for restart operations.
func (m *Manager) GetStepInfo(instanceID string) *StepInfo {
	if m.ctx == nil || m.ctx.Session == nil || instanceID == "" {
		return nil
	}

	session := m.ctx.Session

	// Check planning coordinator
	if session.GetCoordinatorID() == instanceID {
		return &StepInfo{
			Type:       StepTypePlanning,
			InstanceID: instanceID,
			GroupIndex: -1,
			Label:      "Planning Coordinator",
		}
	}

	// Check multi-pass plan coordinators
	for i, coordID := range session.GetPlanCoordinatorIDs() {
		if coordID == instanceID {
			strategies := session.GetMultiPassStrategyNames()
			label := fmt.Sprintf("Plan Coordinator %d", i+1)
			if strategies != nil && i < len(strategies) {
				label = fmt.Sprintf("Plan Coordinator (%s)", strategies[i])
			}
			return &StepInfo{
				Type:       StepTypePlanning,
				InstanceID: instanceID,
				GroupIndex: i,
				Label:      label,
			}
		}
	}

	// Check plan manager
	if session.GetPlanManagerID() == instanceID {
		return &StepInfo{
			Type:       StepTypePlanManager,
			InstanceID: instanceID,
			GroupIndex: -1,
			Label:      "Plan Manager",
		}
	}

	// Check task instances
	for taskID, instID := range session.GetTaskToInstance() {
		if instID == instanceID {
			task := session.GetTask(taskID)
			label := taskID
			if task != nil {
				if labeler, ok := task.(interface{ GetTitle() string }); ok {
					label = labeler.GetTitle()
				}
			}
			groupIdx := session.GetTaskGroupIndex(taskID)
			return &StepInfo{
				Type:       StepTypeTask,
				InstanceID: instanceID,
				TaskID:     taskID,
				GroupIndex: groupIdx,
				Label:      label,
			}
		}
	}

	// Check group consolidators
	for i, consolidatorID := range session.GetGroupConsolidatorIDs() {
		if consolidatorID == instanceID {
			return &StepInfo{
				Type:       StepTypeGroupConsolidator,
				InstanceID: instanceID,
				GroupIndex: i,
				Label:      fmt.Sprintf("Group %d Consolidator", i+1),
			}
		}
	}

	// Check synthesis instance
	if session.GetSynthesisID() == instanceID {
		return &StepInfo{
			Type:       StepTypeSynthesis,
			InstanceID: instanceID,
			GroupIndex: -1,
			Label:      "Synthesis",
		}
	}

	// Check revision instance
	if session.GetRevisionID() == instanceID {
		return &StepInfo{
			Type:       StepTypeRevision,
			InstanceID: instanceID,
			GroupIndex: -1,
			Label:      "Revision",
		}
	}

	// Check consolidation instance
	if session.GetConsolidationID() == instanceID {
		return &StepInfo{
			Type:       StepTypeConsolidation,
			InstanceID: instanceID,
			GroupIndex: -1,
			Label:      "Consolidation",
		}
	}

	return nil
}

// RestartStep restarts the specified step.
// It stops any existing instance for that step and starts a fresh one.
// Returns the new instance ID or an error.
func (m *Manager) RestartStep(stepInfo *StepInfo) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if stepInfo == nil {
		return "", fmt.Errorf("step info is nil")
	}

	if m.ctx == nil || m.ctx.Session == nil {
		return "", fmt.Errorf("no session")
	}

	// Stop existing instance if it exists (best-effort)
	if stepInfo.InstanceID != "" && m.ctx.Instances != nil {
		inst := m.ctx.Instances.GetInstance(stepInfo.InstanceID)
		if inst != nil {
			if err := m.ctx.Instances.StopInstance(inst); err != nil {
				m.logger.Warn("failed to stop existing instance before restart",
					"instance_id", stepInfo.InstanceID,
					"step_type", stepInfo.Type,
					"error", err)
				// Continue with restart - stopping is best-effort
			}
		}
	}

	switch stepInfo.Type {
	case StepTypePlanning:
		return m.restartPlanning()
	case StepTypePlanManager:
		return m.restartPlanManager()
	case StepTypeTask:
		return m.restartTask(stepInfo.TaskID)
	case StepTypeSynthesis:
		return m.restartSynthesis()
	case StepTypeRevision:
		return m.restartRevision()
	case StepTypeConsolidation:
		return m.restartConsolidation()
	case StepTypeGroupConsolidator:
		return m.restartGroupConsolidator(stepInfo.GroupIndex)
	default:
		return "", fmt.Errorf("unknown step type: %s", stepInfo.Type)
	}
}

// restartPlanning restarts the planning phase.
func (m *Manager) restartPlanning() (string, error) {
	session := m.ctx.Session

	// Reset planning orchestrator state
	if m.ctx.Planning != nil {
		m.ctx.Planning.ResetPlanningOrchestrator()
	}

	// Reset planning-related state in session
	session.SetCoordinatorID("")
	session.SetPlan(nil)
	session.SetPhase(PhasePlanning)

	// Run planning again
	if m.ctx.Planning != nil {
		if err := m.ctx.Planning.RunPlanning(); err != nil {
			return "", fmt.Errorf("failed to restart planning: %w", err)
		}
	}

	return session.GetCoordinatorID(), nil
}

// restartPlanManager restarts the plan manager in multi-pass mode.
func (m *Manager) restartPlanManager() (string, error) {
	session := m.ctx.Session

	if !session.IsMultiPass() {
		return "", fmt.Errorf("plan manager only exists in multi-pass mode")
	}

	// Reset planning orchestrator state
	if m.ctx.Planning != nil {
		m.ctx.Planning.ResetPlanningOrchestrator()
	}

	// Reset plan manager state in session
	session.SetPlanManagerID("")
	session.SetPlan(nil)
	session.SetPhase(PhasePlanSelection)

	// Run plan manager again
	if m.ctx.Planning != nil {
		if err := m.ctx.Planning.RunPlanManager(); err != nil {
			return "", fmt.Errorf("failed to restart plan manager: %w", err)
		}
	}

	return session.GetPlanManagerID(), nil
}

// restartTask restarts a specific task.
func (m *Manager) restartTask(taskID string) (string, error) {
	if taskID == "" {
		return "", fmt.Errorf("task ID is required")
	}

	session := m.ctx.Session
	task := session.GetTask(taskID)
	if task == nil {
		return "", fmt.Errorf("task %s not found", taskID)
	}

	// Check if any tasks are currently running
	if m.ctx.Execution != nil && m.ctx.Execution.GetRunningCount() > 0 {
		return "", fmt.Errorf("cannot restart task while tasks are running")
	}

	// Reset execution orchestrator state
	if m.ctx.Execution != nil {
		m.ctx.Execution.ResetExecutionOrchestrator()
	}

	// Reset task state in session
	// Remove from completed tasks
	completed := session.GetCompletedTasks()
	newCompleted := make([]string, 0, len(completed))
	for _, t := range completed {
		if t != taskID {
			newCompleted = append(newCompleted, t)
		}
	}
	session.SetCompletedTasks(newCompleted)

	// Remove from failed tasks
	failed := session.GetFailedTasks()
	newFailed := make([]string, 0, len(failed))
	for _, t := range failed {
		if t != taskID {
			newFailed = append(newFailed, t)
		}
	}
	session.SetFailedTasks(newFailed)

	// Remove from TaskToInstance
	session.DeleteTaskFromInstance(taskID)

	// Reset retry state
	if m.ctx.RetryManager != nil {
		m.ctx.RetryManager.Reset(taskID)
		if m.ctx.Persistence != nil {
			m.ctx.Persistence.SetTaskRetries(m.ctx.RetryManager.GetAllStates())
		}
	}

	// Reset commit count
	session.DeleteTaskCommitCount(taskID)

	// Ensure we're in executing phase
	session.SetPhase(PhaseExecuting)

	// Clear any group decision state
	session.ClearGroupDecision()

	// Start the task
	if m.ctx.Execution != nil {
		completionChan := make(chan any, 1)
		if err := m.ctx.Execution.StartTask(taskID, completionChan); err != nil {
			return "", fmt.Errorf("failed to restart task: %w", err)
		}
	}

	// Get the new instance ID
	newInstanceID := session.GetTaskToInstance()[taskID]

	// Save session
	if m.ctx.Persistence != nil {
		if err := m.ctx.Persistence.SaveSession(); err != nil {
			m.logger.Error("failed to save session after task restart",
				"task_id", taskID,
				"new_instance_id", newInstanceID,
				"error", err)
		}
	}

	return newInstanceID, nil
}

// restartSynthesis restarts the synthesis phase.
func (m *Manager) restartSynthesis() (string, error) {
	session := m.ctx.Session

	// Reset synthesis orchestrator state
	if m.ctx.Synthesis != nil {
		m.ctx.Synthesis.ResetSynthesisOrchestrator()
	}

	// Reset synthesis state in session
	session.ClearSynthesisState()
	session.SetPhase(PhaseSynthesis)

	// Run synthesis again
	if m.ctx.Synthesis != nil {
		if err := m.ctx.Synthesis.RunSynthesis(); err != nil {
			return "", fmt.Errorf("failed to restart synthesis: %w", err)
		}
	}

	return session.GetSynthesisID(), nil
}

// restartRevision restarts the revision phase.
func (m *Manager) restartRevision() (string, error) {
	session := m.ctx.Session

	revision := session.GetRevision()
	if revision == nil {
		return "", fmt.Errorf("no revision issues to address")
	}

	// Try to get issues from the revision
	var issues []any
	if issueGetter, ok := revision.(interface{ GetIssues() []any }); ok {
		issues = issueGetter.GetIssues()
	}
	if len(issues) == 0 {
		return "", fmt.Errorf("no revision issues to address")
	}

	// Reset synthesis orchestrator state (revision is a sub-phase of synthesis)
	if m.ctx.Synthesis != nil {
		m.ctx.Synthesis.ResetSynthesisOrchestrator()
	}

	// Reset revision state in session (keep issues but reset progress)
	session.SetRevisionID("")
	session.SetPhase(PhaseRevision)

	// Run revision again
	if m.ctx.Synthesis != nil {
		if err := m.ctx.Synthesis.StartRevision(issues); err != nil {
			return "", fmt.Errorf("failed to restart revision: %w", err)
		}
	}

	return session.GetRevisionID(), nil
}

// restartConsolidation restarts the consolidation phase.
func (m *Manager) restartConsolidation() (string, error) {
	session := m.ctx.Session

	// Reset consolidation orchestrator state
	if m.ctx.Consolidation != nil {
		m.ctx.Consolidation.ResetConsolidationOrchestrator()
	}

	// Reset consolidation state in session
	session.ClearConsolidationState()
	session.ClearPRUrls()
	session.SetPhase(PhaseConsolidating)

	// Start consolidation again
	if m.ctx.Consolidation != nil {
		if err := m.ctx.Consolidation.StartConsolidation(); err != nil {
			return "", fmt.Errorf("failed to restart consolidation: %w", err)
		}
	}

	return session.GetConsolidationID(), nil
}

// restartGroupConsolidator restarts a specific group consolidator.
func (m *Manager) restartGroupConsolidator(groupIndex int) (string, error) {
	session := m.ctx.Session

	executionOrder := session.GetExecutionOrder()
	if groupIndex < 0 || groupIndex >= len(executionOrder) {
		return "", fmt.Errorf("invalid group index: %d", groupIndex)
	}

	// Reset consolidation orchestrator state for restart
	if m.ctx.Consolidation != nil {
		m.ctx.Consolidation.ResetConsolidationOrchestrator()
	}

	// Reset group consolidator state in session
	session.ClearGroupConsolidatorID(groupIndex)
	session.ClearGroupConsolidatedBranch(groupIndex)
	session.ClearGroupConsolidationContext(groupIndex)

	// Start group consolidation again
	if m.ctx.Consolidation != nil {
		if err := m.ctx.Consolidation.StartGroupConsolidatorSession(groupIndex); err != nil {
			return "", fmt.Errorf("failed to restart group consolidator: %w", err)
		}
	}

	// Get the new instance ID
	consolidatorIDs := session.GetGroupConsolidatorIDs()
	var newInstanceID string
	if groupIndex < len(consolidatorIDs) {
		newInstanceID = consolidatorIDs[groupIndex]
	}

	// Save session
	if m.ctx.Persistence != nil {
		if err := m.ctx.Persistence.SaveSession(); err != nil {
			m.logger.Error("failed to save session after group consolidator restart",
				"group_index", groupIndex,
				"new_instance_id", newInstanceID,
				"error", err)
		}
	}

	return newInstanceID, nil
}
