// Package consolidate provides group consolidation logic for ultra-plan workflows.
// It handles merging parallel task branches, resolving conflicts, and verifying
// consolidated code before proceeding to the next execution group.
package consolidate

import (
	"github.com/Iron-Ham/claudio/internal/orchestrator/types"
)

// CoordinatorInterface defines the coordinator methods needed by group consolidation.
// This interface allows the consolidate package to be decoupled from the Coordinator.
type CoordinatorInterface interface {
	// Session returns the session interface
	Session() SessionInterface

	// Orchestrator returns the orchestrator interface for instance operations
	Orchestrator() OrchestratorInterface

	// BaseSession returns the base session for instance lookups
	BaseSession() BaseSessionInterface

	// Manager returns the manager for event emission
	Manager() ManagerInterface

	// Lock acquires the coordinator mutex
	Lock()

	// Unlock releases the coordinator mutex
	Unlock()

	// Context returns the context for cancellation
	Context() ContextInterface
}

// SessionInterface defines session methods needed by group consolidation.
type SessionInterface interface {
	// Getters
	GetID() string
	GetPlan() PlanInterface
	GetConfig() ConfigInterface
	GetTask(taskID string) TaskInterface
	GetTaskCommitCounts() map[string]int
	GetGroupConsolidatedBranches() []string
	GetGroupConsolidationContexts() []*types.GroupConsolidationCompletionFile
	GetGroupConsolidatorIDs() []string

	// Setters
	SetGroupConsolidatorID(groupIndex int, id string)
	SetGroupConsolidatedBranch(groupIndex int, branch string)
	SetGroupConsolidationContext(groupIndex int, ctx *types.GroupConsolidationCompletionFile)
	EnsureGroupArraysCapacity(groupIndex int)
}

// PlanInterface defines plan methods needed by group consolidation.
type PlanInterface interface {
	GetSummary() string
	GetExecutionOrder() [][]string
}

// ConfigInterface defines config methods needed by group consolidation.
type ConfigInterface interface {
	GetBranchPrefix() string
	IsMultiPass() bool
}

// TaskInterface defines task methods needed by group consolidation.
type TaskInterface interface {
	GetID() string
	GetTitle() string
}

// OrchestratorInterface defines orchestrator methods for instance and worktree operations.
type OrchestratorInterface interface {
	// Worktree operations
	Worktree() WorktreeInterface

	// Instance operations
	AddInstance(baseSession BaseSessionInterface, prompt string) (InstanceInterface, error)
	AddInstanceFromBranch(baseSession BaseSessionInterface, prompt, branch string) (InstanceInterface, error)
	GetInstance(id string) InstanceInterface
	StartInstance(inst InstanceInterface) error
	StopInstance(inst InstanceInterface) error
	SaveSession() error
	GetClaudioDir() string
	GetBranchPrefix() string

	// Instance manager for checking tmux sessions
	GetInstanceManager(id string) InstanceManagerInterface
}

// WorktreeInterface defines worktree operations needed for consolidation.
type WorktreeInterface interface {
	FindMainBranch() string
	CreateBranchFrom(branchName, baseBranch string) error
	CreateWorktreeFromBranch(path, branch string) error
	Remove(path string) error
	CherryPickBranch(worktreePath, sourceBranch string) error
	AbortCherryPick(worktreePath string) error
	CountCommitsBetween(worktreePath, baseBranch, head string) (int, error)
	Push(worktreePath string, force bool) error
}

// InstanceInterface defines instance methods needed by consolidation.
type InstanceInterface interface {
	GetID() string
	GetTask() string
	GetBranch() string
	GetWorktreePath() string
	GetStatus() string
}

// InstanceManagerInterface defines instance manager methods for checking sessions.
type InstanceManagerInterface interface {
	TmuxSessionExists() bool
}

// BaseSessionInterface defines base session methods for instance lookups.
type BaseSessionInterface interface {
	GetInstances() []InstanceInterface
	GetGroupBySessionType(sessionType string) GroupInterface
}

// GroupInterface defines group methods for adding instances.
type GroupInterface interface {
	AddInstance(instanceID string)
}

// ManagerInterface defines manager methods for event emission.
type ManagerInterface interface {
	EmitEvent(eventType, message string)
}

// ContextInterface provides context for cancellation.
type ContextInterface interface {
	Done() <-chan struct{}
}

// TaskWorktreeInfo contains information about a task's worktree for consolidation.
type TaskWorktreeInfo struct {
	TaskID       string
	TaskTitle    string
	WorktreePath string
	Branch       string
}

// EventType constants for coordinator events.
const (
	EventGroupComplete = "group_complete"
)

// Session type constants.
const (
	SessionTypeUltraPlan = "ultraplan"
	SessionTypePlanMulti = "planmulti"
)

// Instance status constants.
const (
	StatusError     = "error"
	StatusCompleted = "completed"
)
