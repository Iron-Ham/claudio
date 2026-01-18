// Package step provides step introspection and restart logic for ultra-plan workflows.
package step

// StepCoordinatorInterface defines the coordinator methods needed by step management.
// This interface allows the step package to be decoupled from the full Coordinator implementation.
type StepCoordinatorInterface interface {
	// Session returns the ultra-plan session interface
	Session() SessionInterface

	// PlanningOrchestrator returns the planning orchestrator for introspection
	PlanningOrchestrator() PlanningOrchestratorInterface

	// ExecutionOrchestrator returns the execution orchestrator for introspection
	ExecutionOrchestrator() ExecutionOrchestratorInterface

	// SynthesisOrchestrator returns the synthesis orchestrator for introspection
	SynthesisOrchestrator() SynthesisOrchestratorInterface

	// ConsolidationOrchestrator returns the consolidation orchestrator for introspection
	ConsolidationOrchestrator() ConsolidationOrchestratorInterface

	// GetOrchestrator returns the base orchestrator for instance operations
	GetOrchestrator() OrchestratorInterface

	// GetRetryManager returns the retry manager for task retry state
	GetRetryManager() RetryManagerInterface

	// GetTaskGroupIndex returns the group index for a task ID
	GetTaskGroupIndex(taskID string) int

	// GetRunningCount returns the number of currently running tasks
	GetRunningCount() int

	// Lock acquires the coordinator's mutex for state updates
	Lock()

	// Unlock releases the coordinator's mutex
	Unlock()

	// SaveSession saves the session state
	SaveSession() error

	// RunPlanning starts the planning phase
	RunPlanning() error

	// RunPlanManager starts the plan manager in multi-pass mode
	RunPlanManager() error

	// RunSynthesis starts the synthesis phase
	RunSynthesis() error

	// StartConsolidation starts the consolidation phase
	StartConsolidation() error

	// StartGroupConsolidatorSession starts a group consolidator for the given group
	StartGroupConsolidatorSession(groupIndex int) error

	// Logger returns the logger for the coordinator
	Logger() LoggerInterface

	// GetMultiPassStrategyNames returns the names of multi-pass planning strategies
	GetMultiPassStrategyNames() []string
}

// SessionInterface defines the session methods needed by step management.
type SessionInterface interface {
	// Getters for session state
	GetCoordinatorID() string
	GetPlanCoordinatorIDs() []string
	GetPlanManagerID() string
	GetSynthesisID() string
	GetRevisionID() string
	GetConsolidationID() string
	GetGroupConsolidatorIDs() []string
	GetTaskToInstance() map[string]string
	GetTask(taskID string) TaskInterface
	GetConfig() ConfigInterface
	GetPhase() string
	GetPlan() PlanInterface
	GetRevision() RevisionInterface

	// Setters for session state
	SetCoordinatorID(id string)
	SetPlanManagerID(id string)
	SetSynthesisID(id string)
	SetRevisionID(id string)
	SetConsolidationID(id string)
	SetPhase(phase string)
	SetPlan(plan PlanInterface)
	SetSynthesisCompletion(completion any)
	SetSynthesisAwaitingApproval(awaiting bool)
	SetConsolidation(state any)
	SetPRUrls(urls []string)
	SetGroupDecision(decision any)
	SetGroupConsolidatorID(groupIndex int, id string)
	SetGroupConsolidatedBranch(groupIndex int, branch string)
	SetGroupConsolidationContext(groupIndex int, ctx any)

	// Task list manipulation
	GetCompletedTasks() []string
	GetFailedTasks() []string
	SetCompletedTasks(tasks []string)
	SetFailedTasks(tasks []string)
	DeleteTaskToInstance(taskID string)
	DeleteTaskCommitCount(taskID string)
	GetTaskRetries() map[string]any
	SetTaskRetries(retries map[string]any)
}

// PlanningOrchestratorInterface defines planning orchestrator methods for step introspection.
type PlanningOrchestratorInterface interface {
	GetInstanceID() string
	GetPlanCoordinatorIDs() []string
	Reset()
}

// ExecutionOrchestratorInterface defines execution orchestrator methods for step introspection.
type ExecutionOrchestratorInterface interface {
	State() ExecutionStateInterface
	Reset()
	StartSingleTask(taskID string) (string, error)
}

// ExecutionStateInterface provides access to execution state for step introspection.
type ExecutionStateInterface interface {
	GetRunningTasks() map[string]string
}

// SynthesisOrchestratorInterface defines synthesis orchestrator methods for step introspection.
type SynthesisOrchestratorInterface interface {
	GetInstanceID() string
	State() SynthesisStateInterface
	Reset()
	StartRevision(issues []RevisionIssue) error
}

// SynthesisStateInterface provides access to synthesis state for step introspection.
type SynthesisStateInterface interface {
	GetRunningRevisionTasks() map[string]string
}

// ConsolidationOrchestratorInterface defines consolidation orchestrator methods for step introspection.
type ConsolidationOrchestratorInterface interface {
	GetInstanceID() string
	Reset()
	ClearStateForRestart()
}

// OrchestratorInterface defines base orchestrator methods for instance operations.
type OrchestratorInterface interface {
	GetInstance(id string) InstanceInterface
	StopInstance(inst InstanceInterface) error
	SaveSession() error
}

// InstanceInterface defines instance methods needed by step operations.
type InstanceInterface interface {
	GetID() string
}

// TaskInterface defines task methods needed by step operations.
type TaskInterface interface {
	GetID() string
	GetTitle() string
}

// ConfigInterface defines config methods needed by step operations.
type ConfigInterface interface {
	IsMultiPass() bool
}

// PlanInterface defines plan methods needed by step operations.
type PlanInterface interface {
	GetExecutionOrder() [][]string
}

// RevisionInterface defines revision methods needed by step operations.
type RevisionInterface interface {
	GetIssues() []RevisionIssue
	GetRevisedTasks() []string
	GetTasksToRevise() []string
	SetRevisedTasks(tasks []string)
	SetTasksToRevise(tasks []string)
}

// RevisionIssue represents an issue identified during synthesis.
type RevisionIssue struct {
	TaskID      string
	Description string
	Files       []string
	Severity    string
	Suggestion  string
}

// RetryManagerInterface defines retry manager methods needed by step operations.
type RetryManagerInterface interface {
	Reset(taskID string)
	GetAllStates() map[string]any
}

// LoggerInterface defines logging methods needed by step operations.
type LoggerInterface interface {
	Warn(msg string, keysAndValues ...any)
	Error(msg string, keysAndValues ...any)
}
