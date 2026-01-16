// Package prompt provides conversion helpers for transforming orchestrator types
// into prompt package types without creating circular dependencies.
package prompt

// PlannedTaskLike defines the interface for types that can be converted to TaskInfo.
// This interface allows the prompt package to accept orchestrator.PlannedTask (and similar types)
// without importing the orchestrator package, avoiding circular dependencies.
//
// Implement this interface by adding getter methods to your task type:
//
//	func (t PlannedTask) GetID() string            { return t.ID }
//	func (t PlannedTask) GetTitle() string         { return t.Title }
//	func (t PlannedTask) GetDescription() string   { return t.Description }
//	func (t PlannedTask) GetFiles() []string       { return t.Files }
//	func (t PlannedTask) GetDependsOn() []string   { return t.DependsOn }
//	func (t PlannedTask) GetPriority() int         { return t.Priority }
//	func (t PlannedTask) GetEstComplexity() string { return string(t.EstComplexity) }
type PlannedTaskLike interface {
	GetID() string
	GetTitle() string
	GetDescription() string
	GetFiles() []string
	GetDependsOn() []string
	GetPriority() int
	GetEstComplexity() string
}

// PlanSpecLike defines the interface for types that can be converted to PlanInfo.
// This interface allows the prompt package to accept orchestrator.PlanSpec without
// importing the orchestrator package.
//
// Note: GetTasks returns []PlannedTaskLike to maintain the interface abstraction.
// Implementations should convert their task slice to this interface type.
type PlanSpecLike interface {
	GetSummary() string
	GetTasks() []PlannedTaskLike
	GetExecutionOrder() [][]string
	GetInsights() []string
	GetConstraints() []string
}

// GroupConsolidationLike defines the interface for types that can be converted to GroupContext.
// This interface allows the prompt package to accept orchestrator.GroupConsolidationCompletionFile
// without importing the orchestrator package.
type GroupConsolidationLike interface {
	GetNotes() string
	GetIssuesForNextGroup() []string
	IsVerificationSuccess() bool
}

// ConvertPlannedTaskToTaskInfo converts a PlannedTaskLike to a TaskInfo.
// This function enables the prompt package to work with orchestrator task types
// without direct dependencies on the orchestrator package.
//
// Returns a zero-value TaskInfo if task is nil (type assertion to check for nil interface).
func ConvertPlannedTaskToTaskInfo(task PlannedTaskLike) TaskInfo {
	if task == nil {
		return TaskInfo{}
	}

	return TaskInfo{
		ID:            task.GetID(),
		Title:         task.GetTitle(),
		Description:   task.GetDescription(),
		Files:         copyStringSlice(task.GetFiles()),
		DependsOn:     copyStringSlice(task.GetDependsOn()),
		Priority:      task.GetPriority(),
		EstComplexity: task.GetEstComplexity(),
		// IssueURL and CommitCount are not part of PlannedTaskLike since they
		// are either set through other means or populated later during synthesis
	}
}

// ConvertPlanSpecToPlanInfo converts a PlanSpecLike to a PlanInfo.
// This function enables the prompt package to work with orchestrator plan types
// without direct dependencies on the orchestrator package.
//
// Returns nil if plan is nil.
func ConvertPlanSpecToPlanInfo(plan PlanSpecLike) *PlanInfo {
	if plan == nil {
		return nil
	}

	tasks := plan.GetTasks()
	taskInfos := make([]TaskInfo, len(tasks))
	for i, task := range tasks {
		taskInfos[i] = ConvertPlannedTaskToTaskInfo(task)
	}

	return &PlanInfo{
		Summary:        plan.GetSummary(),
		Tasks:          taskInfos,
		ExecutionOrder: copyStringSliceSlice(plan.GetExecutionOrder()),
		Insights:       copyStringSlice(plan.GetInsights()),
		Constraints:    copyStringSlice(plan.GetConstraints()),
	}
}

// ConvertPlanSpecsToCandidatePlans converts a slice of PlanSpecLike to CandidatePlanInfo slice.
// Each plan is paired with a strategy name from the strategies slice at the corresponding index.
// If strategies is shorter than plans, empty strings are used for the missing strategy names.
//
// This function is used during multi-pass planning when comparing multiple plans from different
// planning strategies (e.g., "maximize-parallelism", "minimize-complexity", "balanced-approach").
//
// Returns nil if plans is nil or empty.
func ConvertPlanSpecsToCandidatePlans(plans []PlanSpecLike, strategies []string) []CandidatePlanInfo {
	if len(plans) == 0 {
		return nil
	}

	result := make([]CandidatePlanInfo, len(plans))
	for i, plan := range plans {
		var strategy string
		if i < len(strategies) {
			strategy = strategies[i]
		}

		if plan == nil {
			result[i] = CandidatePlanInfo{Strategy: strategy}
			continue
		}

		tasks := plan.GetTasks()
		taskInfos := make([]TaskInfo, len(tasks))
		for j, task := range tasks {
			taskInfos[j] = ConvertPlannedTaskToTaskInfo(task)
		}

		result[i] = CandidatePlanInfo{
			Strategy:       strategy,
			Summary:        plan.GetSummary(),
			Tasks:          taskInfos,
			ExecutionOrder: copyStringSliceSlice(plan.GetExecutionOrder()),
			Insights:       copyStringSlice(plan.GetInsights()),
			Constraints:    copyStringSlice(plan.GetConstraints()),
		}
	}

	return result
}

// ConvertGroupConsolidationToGroupContext converts a GroupConsolidationLike to a GroupContext.
// The groupIndex parameter specifies which execution group this consolidation belongs to.
//
// This function enables the prompt package to work with orchestrator consolidation types
// without direct dependencies on the orchestrator package.
//
// Returns nil if gc is nil.
func ConvertGroupConsolidationToGroupContext(gc GroupConsolidationLike, groupIndex int) *GroupContext {
	if gc == nil {
		return nil
	}

	return &GroupContext{
		GroupIndex:         groupIndex,
		Notes:              gc.GetNotes(),
		IssuesForNextGroup: copyStringSlice(gc.GetIssuesForNextGroup()),
		VerificationPassed: gc.IsVerificationSuccess(),
	}
}

// copyStringSlice creates a defensive copy of a string slice.
// Returns nil if the input is nil, preserving nil vs empty slice semantics.
func copyStringSlice(s []string) []string {
	if s == nil {
		return nil
	}
	result := make([]string, len(s))
	copy(result, s)
	return result
}

// copyStringSliceSlice creates a defensive copy of a 2D string slice.
// Returns nil if the input is nil, preserving nil vs empty slice semantics.
func copyStringSliceSlice(s [][]string) [][]string {
	if s == nil {
		return nil
	}
	result := make([][]string, len(s))
	for i, inner := range s {
		result[i] = copyStringSlice(inner)
	}
	return result
}
