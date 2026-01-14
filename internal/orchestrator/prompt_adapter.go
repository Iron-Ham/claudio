package orchestrator

import (
	"github.com/Iron-Ham/claudio/internal/orchestrator/prompt"
)

// PlanInfoFromPlanSpec converts a PlanSpec to prompt.PlanInfo.
// This adapter bridges the orchestrator's plan domain type to the prompt package's
// representation, enabling prompt builders to work with plan data.
// Returns nil if spec is nil.
func PlanInfoFromPlanSpec(spec *PlanSpec) *prompt.PlanInfo {
	if spec == nil {
		return nil
	}

	// Convert tasks
	tasks := make([]prompt.TaskInfo, len(spec.Tasks))
	for i, task := range spec.Tasks {
		tasks[i] = prompt.TaskInfo{
			ID:            task.ID,
			Title:         task.Title,
			Description:   task.Description,
			Files:         task.Files,
			DependsOn:     task.DependsOn,
			Priority:      task.Priority,
			EstComplexity: string(task.EstComplexity),
			IssueURL:      task.IssueURL,
			// CommitCount is not available from PlannedTask, leave as zero
		}
	}

	return &prompt.PlanInfo{
		ID:             spec.ID,
		Summary:        spec.Summary,
		Tasks:          tasks,
		ExecutionOrder: spec.ExecutionOrder,
		Insights:       spec.Insights,
		Constraints:    spec.Constraints,
	}
}

// TaskInfoFromPlannedTask converts a PlannedTask to prompt.TaskInfo.
// The commitCount parameter allows callers to provide the number of commits
// made by this task, which is useful for synthesis prompts.
// Returns nil if task is nil.
func TaskInfoFromPlannedTask(task *PlannedTask, commitCount int) *prompt.TaskInfo {
	if task == nil {
		return nil
	}

	return &prompt.TaskInfo{
		ID:            task.ID,
		Title:         task.Title,
		Description:   task.Description,
		Files:         task.Files,
		DependsOn:     task.DependsOn,
		Priority:      task.Priority,
		EstComplexity: string(task.EstComplexity),
		IssueURL:      task.IssueURL,
		CommitCount:   commitCount,
	}
}

// TaskInfoSliceFromPlannedTasks converts a slice of PlannedTask to []prompt.TaskInfo.
// The commitCounts map provides commit counts keyed by task ID. If a task's ID
// is not present in the map, its commit count will be zero.
// Returns an empty slice (not nil) if tasks is empty or nil.
func TaskInfoSliceFromPlannedTasks(tasks []PlannedTask, commitCounts map[string]int) []prompt.TaskInfo {
	if len(tasks) == 0 {
		return []prompt.TaskInfo{}
	}

	result := make([]prompt.TaskInfo, len(tasks))
	for i := range tasks {
		commitCount := 0
		if commitCounts != nil {
			commitCount = commitCounts[tasks[i].ID]
		}
		result[i] = prompt.TaskInfo{
			ID:            tasks[i].ID,
			Title:         tasks[i].Title,
			Description:   tasks[i].Description,
			Files:         tasks[i].Files,
			DependsOn:     tasks[i].DependsOn,
			Priority:      tasks[i].Priority,
			EstComplexity: string(tasks[i].EstComplexity),
			IssueURL:      tasks[i].IssueURL,
			CommitCount:   commitCount,
		}
	}

	return result
}
