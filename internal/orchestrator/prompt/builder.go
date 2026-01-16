// Package prompt provides interfaces and types for building prompts
// in the ultra-plan orchestration system.
package prompt

import (
	"errors"
	"fmt"
)

// Builder defines the interface for building prompts from context.
// Different prompt types (planning, task, synthesis, consolidation, revision)
// implement this interface to transform context into their specific prompt format.
type Builder interface {
	// Build generates a prompt string from the given context.
	// Returns an error if the context is invalid for this prompt type.
	Build(ctx *Context) (string, error)
}

// PhaseType identifies the ultra-plan phase for which a prompt is being built.
type PhaseType string

const (
	PhasePlanning      PhaseType = "planning"
	PhaseTask          PhaseType = "task"
	PhaseSynthesis     PhaseType = "synthesis"
	PhaseRevision      PhaseType = "revision"
	PhaseConsolidation PhaseType = "consolidation"
	PhasePlanSelection PhaseType = "plan_selection"
)

// Context provides all the information needed to build any prompt type.
// Not all fields are required for every prompt type; builders should validate
// that the fields they need are present.
type Context struct {
	// Phase identifies which type of prompt is being built
	Phase PhaseType

	// SessionID is the unique identifier for the ultra-plan session
	SessionID string

	// Objective is the original user request/goal
	Objective string

	// Plan contains the decomposed tasks and execution order
	Plan *PlanInfo

	// Task contains details about a specific task (for task/revision prompts)
	Task *TaskInfo

	// GroupIndex identifies which execution group is being processed (0-indexed)
	GroupIndex int

	// BaseDir is the base directory for the repository
	BaseDir string

	// Revision contains state for revision phase prompts
	Revision *RevisionInfo

	// Synthesis contains context from synthesis phase
	Synthesis *SynthesisInfo

	// Consolidation contains context for consolidation prompts
	Consolidation *ConsolidationInfo

	// PreviousGroupContext contains context from prior group consolidations (simple notes)
	// Used to inform subsequent groups about changes made in earlier groups
	PreviousGroupContext []string

	// PreviousGroup contains detailed context from the previous group's consolidation
	// Used by task prompts for tasks in groups after group 0
	PreviousGroup *GroupContext

	// CompletedTasks lists task IDs that have been completed
	CompletedTasks []string

	// FailedTasks lists task IDs that have failed
	FailedTasks []string

	// CandidatePlans contains multiple plans for plan selection phase (multi-pass planning)
	CandidatePlans []CandidatePlanInfo

	// StrategyNames provides fallback strategy names when CandidatePlanInfo.Strategy is empty.
	// Used by plan selection phase to name plans when individual plans don't have strategy names set.
	StrategyNames []string
}

// CandidatePlanInfo contains information about a candidate plan for comparison.
type CandidatePlanInfo struct {
	Strategy       string
	Summary        string
	Tasks          []TaskInfo
	ExecutionOrder [][]string
	Insights       []string
	Constraints    []string
}

// PlanInfo contains plan-level information for prompt building.
type PlanInfo struct {
	ID             string
	Summary        string
	Tasks          []TaskInfo
	ExecutionOrder [][]string
	Insights       []string
	Constraints    []string
}

// TaskInfo contains task-level information for prompt building.
type TaskInfo struct {
	ID            string
	Title         string
	Description   string
	Files         []string
	DependsOn     []string
	Priority      int
	EstComplexity string
	IssueURL      string
	CommitCount   int // Number of commits made by this task (for synthesis)
}

// RevisionInfo contains revision phase context.
type RevisionInfo struct {
	Round         int
	MaxRounds     int
	Issues        []RevisionIssue
	TasksToRevise []string
	RevisedTasks  []string
}

// RevisionIssue represents an issue identified during synthesis.
type RevisionIssue struct {
	TaskID      string
	Description string
	Files       []string
	Severity    string
	Suggestion  string
}

// SynthesisInfo contains synthesis phase context.
type SynthesisInfo struct {
	Notes           []string
	Recommendations []string
	Issues          []string
}

// ConsolidationInfo contains consolidation phase context.
type ConsolidationInfo struct {
	Mode                  string
	BranchPrefix          string
	MainBranch            string
	TaskWorktrees         []TaskWorktreeInfo
	GroupBranches         []string
	PreConsolidatedBranch string
}

// TaskWorktreeInfo contains information about a task's worktree.
type TaskWorktreeInfo struct {
	TaskID       string
	TaskTitle    string
	WorktreePath string
	Branch       string
	CommitCount  int
}

// GroupContext contains context from a previous group's consolidation.
// This is used to inform tasks in subsequent groups about what was done.
type GroupContext struct {
	GroupIndex         int
	Notes              string
	IssuesForNextGroup []string
	VerificationPassed bool
}

// Validation errors
var (
	ErrNilContext            = errors.New("prompt context is nil")
	ErrEmptyObjective        = errors.New("objective is required")
	ErrEmptySessionID        = errors.New("session ID is required")
	ErrInvalidPhase          = errors.New("invalid or empty phase")
	ErrMissingPlan           = errors.New("plan info is required for this phase")
	ErrMissingTask           = errors.New("task info is required for this phase")
	ErrMissingRevision       = errors.New("revision info is required for revision phase")
	ErrMissingSynthesis      = errors.New("synthesis info is required")
	ErrMissingCandidatePlans = errors.New("candidate plans are required for plan selection phase")
	ErrMissingConsolidation  = errors.New("consolidation info is required for consolidation phase")
)

// Validate checks that the context has all required fields for its phase.
// Returns nil if valid, or an error describing what's missing.
func (c *Context) Validate() error {
	if c == nil {
		return ErrNilContext
	}

	if c.Phase == "" {
		return ErrInvalidPhase
	}

	if c.SessionID == "" {
		return ErrEmptySessionID
	}

	if c.Objective == "" {
		return ErrEmptyObjective
	}

	// Phase-specific validation
	switch c.Phase {
	case PhasePlanning:
		// Planning only needs base fields (objective)
		return nil

	case PhaseTask:
		if c.Plan == nil {
			return ErrMissingPlan
		}
		if c.Task == nil {
			return ErrMissingTask
		}
		return nil

	case PhaseSynthesis:
		if c.Plan == nil {
			return ErrMissingPlan
		}
		return nil

	case PhaseRevision:
		if c.Plan == nil {
			return ErrMissingPlan
		}
		if c.Task == nil {
			return ErrMissingTask
		}
		if c.Revision == nil {
			return ErrMissingRevision
		}
		return nil

	case PhaseConsolidation:
		if c.Plan == nil {
			return ErrMissingPlan
		}
		return nil

	case PhasePlanSelection:
		// Plan selection needs candidate plans to choose from
		if len(c.CandidatePlans) == 0 {
			return ErrMissingCandidatePlans
		}
		return nil

	default:
		return fmt.Errorf("%w: %s", ErrInvalidPhase, c.Phase)
	}
}

// ValidPhases returns all valid phase types.
func ValidPhases() []PhaseType {
	return []PhaseType{
		PhasePlanning,
		PhaseTask,
		PhaseSynthesis,
		PhaseRevision,
		PhaseConsolidation,
		PhasePlanSelection,
	}
}
