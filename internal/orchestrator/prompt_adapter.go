package orchestrator

import (
	"errors"

	"github.com/Iron-Ham/claudio/internal/orchestrator/prompt"
)

// Error sentinels for PromptAdapter operations
var (
	// ErrNilCoordinator is returned when a PromptAdapter method requires a
	// Coordinator but the adapter was created with a nil coordinator.
	ErrNilCoordinator = errors.New("prompt adapter has nil coordinator")

	// ErrNilUltraPlanSession is returned when a PromptAdapter method requires
	// an UltraPlanSession but the coordinator's manager returns nil.
	ErrNilUltraPlanSession = errors.New("coordinator has nil ultra-plan session")
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

// groupContextFromCompletion converts a GroupConsolidationCompletionFile to prompt.GroupContext.
// This enables passing context from one group's consolidation to inform subsequent groups.
// The completion file is written by the per-group consolidator session after merging
// all task branches for that group.
func groupContextFromCompletion(completion *GroupConsolidationCompletionFile) *prompt.GroupContext {
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

// BuildPlanningContext creates a prompt.Context configured for the planning phase.
// It populates the context with session ID, objective, and base directory from
// the Coordinator's UltraPlanSession and base Session respectively.
//
// Returns an error if the adapter has no coordinator, no UltraPlanSession,
// or if the resulting context fails validation.
func (a *PromptAdapter) BuildPlanningContext() (*prompt.Context, error) {
	if a.coordinator == nil {
		return nil, ErrNilCoordinator
	}

	session := a.coordinator.manager.Session()
	if session == nil {
		return nil, ErrNilUltraPlanSession
	}

	baseDir := ""
	if a.coordinator.baseSession != nil {
		baseDir = a.coordinator.baseSession.BaseRepo
	}

	ctx := &prompt.Context{
		Phase:     prompt.PhasePlanning,
		SessionID: session.ID,
		Objective: session.Objective,
		BaseDir:   baseDir,
	}

	if err := ctx.Validate(); err != nil {
		return nil, err
	}

	return ctx, nil
}
