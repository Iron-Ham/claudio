package orchestrator

import (
	"github.com/Iron-Ham/claudio/internal/orchestrator/prompt"
)

// PromptAdapter bridges orchestrator types to prompt.Context, enabling the
// prompt.Builder infrastructure to be used for prompt generation instead of
// manual string concatenation. It holds a reference to a Coordinator to access
// session state and plan information.
type PromptAdapter struct {
	coordinator *Coordinator
}

// NewPromptAdapter creates a new PromptAdapter with the given Coordinator.
// The Coordinator provides access to the underlying session, plan, and task state
// needed to build prompt contexts.
func NewPromptAdapter(coordinator *Coordinator) *PromptAdapter {
	return &PromptAdapter{
		coordinator: coordinator,
	}
}

// planInfoFromPlanSpec converts a PlanSpec to a prompt.PlanInfo.
// This enables the prompt builder to work with plan data without depending
// directly on orchestrator types.
func planInfoFromPlanSpec(spec *PlanSpec) *prompt.PlanInfo {
	if spec == nil {
		return nil
	}

	return &prompt.PlanInfo{
		ID:             spec.ID,
		Summary:        spec.Summary,
		Tasks:          tasksFromPlanSpec(spec.Tasks),
		ExecutionOrder: spec.ExecutionOrder,
		Insights:       spec.Insights,
		Constraints:    spec.Constraints,
	}
}

// taskInfoFromPlannedTask converts a PlannedTask to a prompt.TaskInfo.
// The EstComplexity field is converted from TaskComplexity to string since
// prompt.TaskInfo uses a string representation.
func taskInfoFromPlannedTask(task PlannedTask) prompt.TaskInfo {
	return prompt.TaskInfo{
		ID:            task.ID,
		Title:         task.Title,
		Description:   task.Description,
		Files:         task.Files,
		DependsOn:     task.DependsOn,
		Priority:      task.Priority,
		EstComplexity: string(task.EstComplexity),
		IssueURL:      task.IssueURL,
		// CommitCount is not available from PlannedTask - it's populated later
		// during synthesis when we know how many commits a task made
	}
}

// tasksFromPlanSpec converts a slice of PlannedTask to a slice of prompt.TaskInfo.
// This is used by planInfoFromPlanSpec to convert all tasks in a plan.
func tasksFromPlanSpec(tasks []PlannedTask) []prompt.TaskInfo {
	if tasks == nil {
		return nil
	}

	result := make([]prompt.TaskInfo, len(tasks))
	for i, task := range tasks {
		result[i] = taskInfoFromPlannedTask(task)
	}
	return result
}

// revisionInfoFromState converts a RevisionState to a prompt.RevisionInfo.
// RevisionState tracks the revision process during ultra-plan execution, while
// RevisionInfo provides the subset of data needed for prompt generation.
func revisionInfoFromState(state *RevisionState) *prompt.RevisionInfo {
	if state == nil {
		return nil
	}

	return &prompt.RevisionInfo{
		Round:         state.RevisionRound,
		MaxRounds:     state.MaxRevisions,
		Issues:        revisionIssuesFromOrchestrator(state.Issues),
		TasksToRevise: state.TasksToRevise,
		RevisedTasks:  state.RevisedTasks,
	}
}

// revisionIssueFromOrchestratorIssue converts an orchestrator.RevisionIssue to
// a prompt.RevisionIssue. The two types have the same fields but exist in
// different packages to maintain separation between orchestration logic and
// prompt building.
func revisionIssueFromOrchestratorIssue(issue RevisionIssue) prompt.RevisionIssue {
	return prompt.RevisionIssue{
		TaskID:      issue.TaskID,
		Description: issue.Description,
		Files:       issue.Files,
		Severity:    issue.Severity,
		Suggestion:  issue.Suggestion,
	}
}

// revisionIssuesFromOrchestrator converts a slice of orchestrator.RevisionIssue
// to a slice of prompt.RevisionIssue. This is a helper used by revisionInfoFromState.
func revisionIssuesFromOrchestrator(issues []RevisionIssue) []prompt.RevisionIssue {
	if issues == nil {
		return nil
	}

	result := make([]prompt.RevisionIssue, len(issues))
	for i, issue := range issues {
		result[i] = revisionIssueFromOrchestratorIssue(issue)
	}
	return result
}

// synthesisInfoFromCompletion converts a SynthesisCompletionFile to a prompt.SynthesisInfo.
// The SynthesisCompletionFile contains the full synthesis output including structured
// issues, while SynthesisInfo provides a simpler representation for prompt generation
// with issues converted to string descriptions.
func synthesisInfoFromCompletion(completion *SynthesisCompletionFile) *prompt.SynthesisInfo {
	if completion == nil {
		return nil
	}

	// Convert structured issues to simple string descriptions
	issueDescriptions := make([]string, len(completion.IssuesFound))
	for i, issue := range completion.IssuesFound {
		issueDescriptions[i] = issue.Description
	}

	// IntegrationNotes becomes a single-element Notes slice if non-empty
	var notes []string
	if completion.IntegrationNotes != "" {
		notes = []string{completion.IntegrationNotes}
	}

	return &prompt.SynthesisInfo{
		Notes:           notes,
		Recommendations: completion.Recommendations,
		Issues:          issueDescriptions,
	}
}
