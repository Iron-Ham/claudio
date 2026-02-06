package pipeline

import (
	"github.com/Iron-Ham/claudio/internal/coordination"
	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/team"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

// PipelinePhase represents a phase of the orchestration pipeline.
type PipelinePhase string

const (
	// PhasePlanning indicates the pipeline is running a planning team.
	PhasePlanning PipelinePhase = "planning"

	// PhaseExecution indicates the pipeline is running execution teams.
	PhaseExecution PipelinePhase = "execution"

	// PhaseReview indicates the pipeline is running a review team.
	PhaseReview PipelinePhase = "review"

	// PhaseConsolidation indicates the pipeline is running a consolidation team.
	PhaseConsolidation PipelinePhase = "consolidation"

	// PhaseDone indicates the pipeline has completed successfully.
	PhaseDone PipelinePhase = "done"

	// PhaseFailed indicates the pipeline has failed.
	PhaseFailed PipelinePhase = "failed"
)

// String returns the string representation of the phase.
func (p PipelinePhase) String() string {
	return string(p)
}

// IsTerminal returns true if this phase represents a final state.
func (p PipelinePhase) IsTerminal() bool {
	return p == PhaseDone || p == PhaseFailed
}

// PipelineConfig holds required dependencies for creating a Pipeline.
type PipelineConfig struct {
	Bus     *event.Bus          // Shared event bus for all phases
	BaseDir string              // Base directory; per-phase subdirs created under this
	Plan    *ultraplan.PlanSpec // The plan to decompose and execute
}

// DecomposeConfig configures how a plan is decomposed into teams.
type DecomposeConfig struct {
	MaxTeamSize       int  // Max tasks per team (0 = unlimited)
	MinTeamSize       int  // Min tasks per team before merging (default: 1)
	PlanningTeam      bool // Create a planning team phase
	ReviewTeam        bool // Create a review team phase
	ConsolidationTeam bool // Create a consolidation team phase
}

// defaults returns a copy of the config with defaults applied.
func (c DecomposeConfig) defaults() DecomposeConfig {
	if c.MinTeamSize < 1 {
		c.MinTeamSize = 1
	}
	return c
}

// DecomposeResult holds the output of plan decomposition.
type DecomposeResult struct {
	ExecutionTeams    []team.Spec // Teams for the execution phase
	PlanningTeam      *team.Spec  // Optional planning team (nil if disabled)
	ReviewTeam        *team.Spec  // Optional review team (nil if disabled)
	ConsolidationTeam *team.Spec  // Optional consolidation team (nil if disabled)
}

// pipelineConfig holds optional settings for the Pipeline.
type pipelineConfig struct {
	hubOpts []coordination.Option
}
