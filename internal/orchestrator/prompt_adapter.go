package orchestrator

import (
	"errors"

	"github.com/Iron-Ham/claudio/internal/orchestrator/prompt"
	"github.com/Iron-Ham/claudio/internal/orchestrator/types"
)

// Error sentinels for phase adapter operations.
// These are used by coordinator_phase_adapter.go for nil checks.
var (
	// ErrNilCoordinator is returned when an operation requires a
	// Coordinator but nil was provided.
	ErrNilCoordinator = errors.New("prompt adapter has nil coordinator")

	// ErrNilManager is returned when the coordinator's manager is nil.
	ErrNilManager = errors.New("prompt adapter: manager is required")

	// ErrNilSession is returned when the manager's session is nil.
	ErrNilSession = errors.New("prompt adapter: session is required")
)

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

// groupContextFromCompletion converts a GroupConsolidationCompletionFile to prompt.GroupContext.
// This enables passing context from one group's consolidation to inform subsequent groups.
// The completion file is written by the per-group consolidator session after merging
// all task branches for that group.
func groupContextFromCompletion(completion *types.GroupConsolidationCompletionFile) *prompt.GroupContext {
	if completion == nil {
		return nil
	}

	return &prompt.GroupContext{
		GroupIndex:         completion.GroupIndex,
		Notes:              completion.Notes,
		IssuesForNextGroup: completion.IssuesForNextGroup,
		VerificationPassed: completion.Verification.OverallSuccess,
	}
}

// consolidationInfoFromSession builds prompt.ConsolidationInfo from UltraPlanSession state.
// This extracts the consolidation-relevant fields (mode, branch config, worktrees, group branches)
// needed by the prompt builder to generate consolidation prompts.
func consolidationInfoFromSession(session *UltraPlanSession, mainBranch string) *prompt.ConsolidationInfo {
	if session == nil {
		return nil
	}

	// Convert orchestrator TaskWorktreeInfo to prompt TaskWorktreeInfo
	taskWorktrees := make([]prompt.TaskWorktreeInfo, len(session.TaskWorktrees))
	for i, tw := range session.TaskWorktrees {
		taskWorktrees[i] = taskWorktreeInfoFromOrchestrator(tw, session.TaskCommitCounts)
	}

	// Determine mode string from config
	mode := string(session.Config.ConsolidationMode)
	if mode == "" {
		mode = string(ModeSinglePR) // Default to single PR mode
	}

	// Determine branch prefix
	branchPrefix := session.Config.BranchPrefix
	if branchPrefix == "" {
		branchPrefix = "Iron-Ham" // Default branch prefix
	}

	// Determine main branch
	if mainBranch == "" {
		mainBranch = "main"
	}

	// Determine pre-consolidated branch (if any groups have been consolidated)
	var preConsolidatedBranch string
	if len(session.GroupConsolidatedBranches) > 0 {
		// Use the most recent consolidated branch as the base
		preConsolidatedBranch = session.GroupConsolidatedBranches[len(session.GroupConsolidatedBranches)-1]
	}

	return &prompt.ConsolidationInfo{
		Mode:                  mode,
		BranchPrefix:          branchPrefix,
		MainBranch:            mainBranch,
		TaskWorktrees:         taskWorktrees,
		GroupBranches:         session.GroupConsolidatedBranches,
		PreConsolidatedBranch: preConsolidatedBranch,
	}
}

// taskWorktreeInfoFromOrchestrator converts orchestrator.TaskWorktreeInfo to prompt.TaskWorktreeInfo.
// The CommitCount is looked up from the TaskCommitCounts map since it's stored separately
// from the basic worktree info and is populated after task completion verification.
func taskWorktreeInfoFromOrchestrator(tw TaskWorktreeInfo, commitCounts map[string]int) prompt.TaskWorktreeInfo {
	commitCount := 0
	if commitCounts != nil {
		commitCount = commitCounts[tw.TaskID]
	}

	return prompt.TaskWorktreeInfo{
		TaskID:       tw.TaskID,
		TaskTitle:    tw.TaskTitle,
		WorktreePath: tw.WorktreePath,
		Branch:       tw.Branch,
		CommitCount:  commitCount,
	}
}

// candidatePlanInfoFromPlanSpec converts a PlanSpec to a prompt.CandidatePlanInfo for
// multi-pass planning. The strategyIndex parameter identifies which planning strategy
// produced this plan (0=maximize-parallelism, 1=minimize-complexity, 2=balanced-approach).
// This enables the plan selection phase to compare plans from different strategic perspectives.
func candidatePlanInfoFromPlanSpec(spec *PlanSpec, strategyIndex int) prompt.CandidatePlanInfo {
	if spec == nil {
		return prompt.CandidatePlanInfo{}
	}

	// Map strategy index to strategy name from MultiPassPlanningPrompts
	var strategy string
	if strategyIndex >= 0 && strategyIndex < len(MultiPassPlanningPrompts) {
		strategy = MultiPassPlanningPrompts[strategyIndex].Strategy
	}

	return prompt.CandidatePlanInfo{
		Strategy:       strategy,
		Summary:        spec.Summary,
		Tasks:          tasksFromPlanSpec(spec.Tasks),
		ExecutionOrder: spec.ExecutionOrder,
		Insights:       spec.Insights,
		Constraints:    spec.Constraints,
	}
}

// planInfoWithCommitCounts creates a PlanInfo from a PlanSpec, enriching task info
// with commit counts from the provided map. This is used during synthesis to include
// the number of commits each task made in its work.
func planInfoWithCommitCounts(spec *PlanSpec, commitCounts map[string]int) *prompt.PlanInfo {
	if spec == nil {
		return nil
	}

	planInfo := &prompt.PlanInfo{
		ID:             spec.ID,
		Summary:        spec.Summary,
		ExecutionOrder: spec.ExecutionOrder,
		Insights:       spec.Insights,
		Constraints:    spec.Constraints,
	}

	// Convert tasks with commit counts
	if spec.Tasks != nil {
		planInfo.Tasks = make([]prompt.TaskInfo, len(spec.Tasks))
		for i, task := range spec.Tasks {
			taskInfo := taskInfoFromPlannedTask(task)
			if commitCounts != nil {
				if count, ok := commitCounts[task.ID]; ok {
					taskInfo.CommitCount = count
				}
			}
			planInfo.Tasks[i] = taskInfo
		}
	}

	return planInfo
}

// buildPreviousGroupContextStrings extracts context strings from GroupConsolidationCompletionFile entries.
// For each completed group, it formats the notes and issues into a readable string.
func buildPreviousGroupContextStrings(contexts []*types.GroupConsolidationCompletionFile) []string {
	if contexts == nil {
		return nil
	}

	result := make([]string, 0, len(contexts))
	for _, ctx := range contexts {
		if ctx == nil {
			continue
		}

		var contextStr string
		if ctx.Notes != "" {
			contextStr = ctx.Notes
		}
		if len(ctx.IssuesForNextGroup) > 0 {
			if contextStr != "" {
				contextStr += " | Issues: "
			} else {
				contextStr = "Issues: "
			}
			for i, issue := range ctx.IssuesForNextGroup {
				if i > 0 {
					contextStr += "; "
				}
				contextStr += issue
			}
		}
		if contextStr != "" {
			result = append(result, contextStr)
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// findTaskByID searches for a PlannedTask by its ID in the given plan.
// Returns nil if the plan is nil or the task is not found.
func findTaskByID(plan *PlanSpec, taskID string) *PlannedTask {
	if plan == nil || taskID == "" {
		return nil
	}

	for i := range plan.Tasks {
		if plan.Tasks[i].ID == taskID {
			return &plan.Tasks[i]
		}
	}
	return nil
}
