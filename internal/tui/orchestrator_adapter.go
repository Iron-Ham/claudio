// Package tui provides the terminal user interface for Claudio.
// This file defines interfaces that decouple the TUI from the concrete orchestrator implementation.
package tui

import "time"

// OrchestratorInterface defines the contract between the TUI and the orchestrator.
// This interface exposes only what the TUI needs, allowing for better testability
// and separation of concerns.
type OrchestratorInterface interface {
	// Instance management
	GetInstance(id string) *InstanceInfo
	GetInstances() []*InstanceInfo
	GetInstanceOutput(id string) []byte
	AddInstance(task string) (*InstanceInfo, error)
	RemoveInstance(sessionID string, instanceID string, force bool) error
	StartInstance(id string) error
	StopInstance(id string) error
	StopInstanceWithAutoPR(id string) (bool, error)
	ReconnectInstance(id string) error

	// Instance interaction
	SendInput(instanceID string, input []byte) error
	GetInstanceDiff(worktreePath string) (string, error)
	GetInstanceMetrics(id string) *InstanceMetrics

	// Session management
	GetSessionName() string
	GetSessionCreated() time.Time
	SaveSession() error
	ResizeAllInstances(width, height int)
	ClearCompletedInstances(sessionID string) (int, error)
	GetSessionMetrics() *SessionMetrics

	// Conflict detection
	GetConflicts() []Conflict

	// Ultra-plan support
	IsUltraPlanMode() bool
	GetUltraPlanState() *UltraPlanInfo

	// PR workflow support
	GetPRWorkflow(instanceID string) *PRWorkflowInfo
}

// InstanceInfo contains essential instance data needed by the TUI.
// This is a TUI-specific representation of an instance that avoids
// exposing internal orchestrator implementation details.
type InstanceInfo struct {
	// Core identity
	ID           string
	WorktreePath string
	Branch       string
	Task         string

	// State
	Status        InstanceStatus
	PID           int
	FilesModified []string
	Created       time.Time
	TmuxSession   string

	// Runtime data (not persisted)
	Output []byte

	// Resource tracking
	Metrics *InstanceMetrics
}

// InstanceStatus represents the current state of a Claude instance.
// Mirrors orchestrator.InstanceStatus but defined here to avoid import cycles.
type InstanceStatus string

const (
	StatusPending      InstanceStatus = "pending"
	StatusWorking      InstanceStatus = "working"
	StatusWaitingInput InstanceStatus = "waiting_input"
	StatusPaused       InstanceStatus = "paused"
	StatusCompleted    InstanceStatus = "completed"
	StatusError        InstanceStatus = "error"
	StatusCreatingPR   InstanceStatus = "creating_pr"
	StatusStuck        InstanceStatus = "stuck"
	StatusTimeout      InstanceStatus = "timeout"
)

// InstanceMetrics tracks resource usage and costs for an instance.
type InstanceMetrics struct {
	InputTokens  int64
	OutputTokens int64
	CacheRead    int64
	CacheWrite   int64
	Cost         float64
	APICalls     int
	StartTime    *time.Time
	EndTime      *time.Time
}

// TotalTokens returns the sum of input and output tokens.
func (m *InstanceMetrics) TotalTokens() int64 {
	if m == nil {
		return 0
	}
	return m.InputTokens + m.OutputTokens
}

// Duration returns the total runtime duration if start/end times are set.
func (m *InstanceMetrics) Duration() time.Duration {
	if m == nil || m.StartTime == nil {
		return 0
	}
	if m.EndTime == nil {
		return time.Since(*m.StartTime)
	}
	return m.EndTime.Sub(*m.StartTime)
}

// SessionMetrics aggregates metrics across all instances in a session.
type SessionMetrics struct {
	TotalInputTokens  int64
	TotalOutputTokens int64
	TotalCacheRead    int64
	TotalCacheWrite   int64
	TotalCost         float64
	TotalAPICalls     int
	InstanceCount     int
}

// Conflict represents a file being modified by multiple instances.
type Conflict struct {
	RelativePath string    // Path relative to worktree root
	Instances    []string  // Instance IDs that modified this file
	LastModified time.Time // When the conflict was last detected
}

// UltraPlanInfo provides TUI-relevant ultra-plan state.
// This is a simplified view of the ultra-plan session for display purposes.
type UltraPlanInfo struct {
	// Core state
	ID        string
	Objective string
	Phase     UltraPlanPhase

	// Plan information
	Plan *PlanInfo

	// Multi-pass planning state
	CandidatePlans     []*PlanInfo
	SelectedPlanIndex  int
	PlanManagerID      string

	// Instance tracking
	CoordinatorID string
	SynthesisID   string
	RevisionID    string

	// Task execution state
	TaskToInstance map[string]string // PlannedTask.ID -> Instance.ID
	CompletedTasks []string
	FailedTasks    []string
	CurrentGroup   int

	// Timestamps
	Created     time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time

	// Error information
	Error string

	// Synthesis state
	SynthesisAwaitingApproval bool

	// Revision state
	Revision *RevisionInfo

	// Consolidation state
	Consolidation              *ConsolidationInfo
	PRUrls                     []string
	GroupConsolidatedBranches  []string
	GroupConsolidatorIDs       []string

	// Task verification
	TaskCommitCounts map[string]int
	TaskRetries      map[string]*TaskRetryInfo

	// Group decision state (for partial success/failure handling)
	GroupDecision *GroupDecisionInfo
}

// UltraPlanPhase represents the current phase of an ultra-plan session.
type UltraPlanPhase string

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

// PlanInfo contains the plan specification for TUI display.
type PlanInfo struct {
	ID              string
	Objective       string
	Summary         string
	Tasks           []PlannedTaskInfo
	DependencyGraph map[string][]string
	ExecutionOrder  [][]string // Groups of parallelizable tasks
	Insights        []string
	Constraints     []string
	CreatedAt       time.Time
}

// PlannedTaskInfo represents a single decomposed task.
type PlannedTaskInfo struct {
	ID            string
	Title         string
	Description   string
	Files         []string
	DependsOn     []string
	Priority      int
	EstComplexity TaskComplexity
}

// TaskComplexity represents the estimated complexity of a planned task.
type TaskComplexity string

const (
	ComplexityLow    TaskComplexity = "low"
	ComplexityMedium TaskComplexity = "medium"
	ComplexityHigh   TaskComplexity = "high"
)

// RevisionInfo tracks the state of the revision phase.
type RevisionInfo struct {
	Issues          []RevisionIssue
	RevisionRound   int
	MaxRevisions    int
	TasksToRevise   []string
	RevisedTasks    []string
	RevisionPrompts map[string]string
	StartedAt       *time.Time
	CompletedAt     *time.Time
}

// RevisionIssue represents an issue identified during synthesis.
type RevisionIssue struct {
	TaskID      string
	Description string
	Files       []string
	Severity    string // "critical", "major", "minor"
	Suggestion  string
}

// ConsolidationInfo tracks the progress of consolidation.
type ConsolidationInfo struct {
	Phase            ConsolidationPhase
	CurrentGroup     int
	TotalGroups      int
	CurrentTask      string
	GroupBranches    []string
	PRUrls           []string
	ConflictFiles    []string
	ConflictTaskID   string
	ConflictWorktree string
	Error            string
	StartedAt        *time.Time
	CompletedAt      *time.Time
}

// ConsolidationPhase represents sub-phases within consolidation.
type ConsolidationPhase string

const (
	ConsolidationIdle             ConsolidationPhase = "idle"
	ConsolidationDetecting        ConsolidationPhase = "detecting_conflicts"
	ConsolidationCreatingBranches ConsolidationPhase = "creating_branches"
	ConsolidationMergingTasks     ConsolidationPhase = "merging_tasks"
	ConsolidationPushing          ConsolidationPhase = "pushing"
	ConsolidationCreatingPRs      ConsolidationPhase = "creating_prs"
	ConsolidationPaused           ConsolidationPhase = "paused"
	ConsolidationComplete         ConsolidationPhase = "complete"
	ConsolidationFailed           ConsolidationPhase = "failed"
)

// TaskRetryInfo tracks retry attempts for a task.
type TaskRetryInfo struct {
	TaskID       string
	RetryCount   int
	MaxRetries   int
	LastError    string
	CommitCounts []int
}

// GroupDecisionInfo tracks state when a group has partial success/failure.
type GroupDecisionInfo struct {
	GroupIndex       int
	SucceededTasks   []string
	FailedTasks      []string
	AwaitingDecision bool
}

// PRWorkflowInfo provides PR creation workflow state for display.
type PRWorkflowInfo struct {
	InstanceID string
	Status     PRWorkflowStatus
	PRUrl      string
	Error      string
}

// PRWorkflowStatus represents the state of a PR creation workflow.
type PRWorkflowStatus string

const (
	PRWorkflowPending   PRWorkflowStatus = "pending"
	PRWorkflowCreating  PRWorkflowStatus = "creating"
	PRWorkflowCompleted PRWorkflowStatus = "completed"
	PRWorkflowFailed    PRWorkflowStatus = "failed"
)

// InstanceManagerInterface defines methods for direct instance interaction.
// This is used when the TUI needs to send input directly to an instance.
type InstanceManagerInterface interface {
	// SendInput sends raw input to the instance's terminal
	SendInput(input []byte) error

	// GetOutput returns the current captured output
	GetOutput() []byte

	// IsRunning returns true if the instance process is still running
	IsRunning() bool
}
