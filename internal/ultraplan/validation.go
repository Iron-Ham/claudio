package ultraplan

import (
	"fmt"
	"strings"
)

// ValidatePlan performs comprehensive validation of a PlanSpec.
// It checks for structural issues, dependency problems, and file conflicts.
// Returns a ValidationResult containing all issues found.
func ValidatePlan(spec *PlanSpec) (*ValidationResult, error) {
	result := &ValidationResult{
		IsValid:  true,
		Messages: make([]ValidationMessage, 0),
	}

	if spec == nil {
		result.IsValid = false
		result.Messages = append(result.Messages, ValidationMessage{
			Severity: SeverityError,
			Message:  "Plan is nil",
		})
		result.ErrorCount++
		return result, nil
	}

	if len(spec.Tasks) == 0 {
		result.IsValid = false
		result.Messages = append(result.Messages, ValidationMessage{
			Severity:   SeverityError,
			Message:    "Plan has no tasks",
			Suggestion: "Add at least one task to the plan",
		})
		result.ErrorCount++
		return result, nil
	}

	// Validate task dependencies
	depMessages := ValidateTaskDependencies(spec.Tasks)
	for _, msg := range depMessages {
		if msg.IsError() {
			result.IsValid = false
			result.ErrorCount++
		} else if msg.IsWarning() {
			result.WarningCount++
		}
		result.Messages = append(result.Messages, msg)
	}

	// Validate task files for conflicts
	fileMessages := ValidateTaskFiles(spec)
	for _, msg := range fileMessages {
		if msg.IsError() {
			result.IsValid = false
			result.ErrorCount++
		} else if msg.IsWarning() {
			result.WarningCount++
		}
		result.Messages = append(result.Messages, msg)
	}

	// Check for dependency cycles
	cycleInfo := DetectDependencyCycle(spec)
	if cycleInfo != nil {
		result.IsValid = false
		result.Messages = append(result.Messages, ValidationMessage{
			Severity:   SeverityError,
			Message:    fmt.Sprintf("Dependency cycle detected: %s", strings.Join(cycleInfo, " → ")),
			RelatedIDs: cycleInfo,
			Suggestion: "Remove one of the dependencies to break the cycle",
		})
		result.ErrorCount++
	}

	// Verify execution order coverage matches task count
	if spec.ExecutionOrder != nil {
		scheduledTasks := 0
		for _, group := range spec.ExecutionOrder {
			scheduledTasks += len(group)
		}
		if scheduledTasks < len(spec.Tasks) {
			result.IsValid = false
			result.Messages = append(result.Messages, ValidationMessage{
				Severity:   SeverityError,
				Message:    fmt.Sprintf("Execution order incomplete: only %d of %d tasks scheduled (indicates a cycle)", scheduledTasks, len(spec.Tasks)),
				Suggestion: "Fix the dependency cycle to allow all tasks to be scheduled",
			})
			result.ErrorCount++
		}
	}

	return result, nil
}

// ValidateTaskDependencies validates the dependencies of a list of tasks.
// It checks for:
// - Missing task descriptions/titles (warnings)
// - Self-dependencies (errors)
// - References to non-existent tasks (errors)
// - High complexity tasks (warnings)
func ValidateTaskDependencies(tasks []PlannedTask) []ValidationMessage {
	var messages []ValidationMessage

	// Build task ID set for validation
	taskSet := make(map[string]bool)
	for _, task := range tasks {
		taskSet[task.ID] = true
	}

	for _, task := range tasks {
		// Check for missing description (warning)
		if strings.TrimSpace(task.Description) == "" {
			messages = append(messages, ValidationMessage{
				Severity:   SeverityWarning,
				Message:    "Task has no description",
				TaskID:     task.ID,
				Field:      "description",
				Suggestion: "Add a detailed description for the task",
			})
		}

		// Check for missing title (warning)
		if strings.TrimSpace(task.Title) == "" {
			messages = append(messages, ValidationMessage{
				Severity:   SeverityWarning,
				Message:    "Task has no title",
				TaskID:     task.ID,
				Field:      "title",
				Suggestion: "Add a descriptive title for the task",
			})
		}

		// Check for self-dependency (error)
		for _, depID := range task.DependsOn {
			if depID == task.ID {
				messages = append(messages, ValidationMessage{
					Severity:   SeverityError,
					Message:    "Task depends on itself",
					TaskID:     task.ID,
					Field:      "depends_on",
					RelatedIDs: []string{task.ID},
					Suggestion: "Remove the self-dependency",
				})
			}
		}

		// Check for invalid dependency references (error)
		for _, depID := range task.DependsOn {
			if depID != task.ID && !taskSet[depID] {
				messages = append(messages, ValidationMessage{
					Severity:   SeverityError,
					Message:    fmt.Sprintf("Depends on unknown task '%s'", depID),
					TaskID:     task.ID,
					Field:      "depends_on",
					RelatedIDs: []string{depID},
					Suggestion: fmt.Sprintf("Remove '%s' from dependencies or create a task with that ID", depID),
				})
			}
		}

		// Check for high complexity tasks (warning - might benefit from splitting)
		if task.EstComplexity == ComplexityHigh {
			messages = append(messages, ValidationMessage{
				Severity:   SeverityWarning,
				Message:    "High complexity task may benefit from splitting",
				TaskID:     task.ID,
				Field:      "est_complexity",
				Suggestion: "Consider splitting into smaller, more manageable subtasks",
			})
		}
	}

	return messages
}

// ValidateTaskFiles checks for file conflicts between tasks.
// Returns warnings for files modified by multiple parallel tasks.
func ValidateTaskFiles(spec *PlanSpec) []ValidationMessage {
	var messages []ValidationMessage

	// Build a map of files to tasks for conflict detection
	fileToTasks := make(map[string][]string)
	for _, task := range spec.Tasks {
		for _, file := range task.Files {
			fileToTasks[file] = append(fileToTasks[file], task.ID)
		}
	}

	// Check for file conflicts (tasks with overlapping files not in same dependency chain)
	for file, taskIDs := range fileToTasks {
		if len(taskIDs) > 1 {
			// Check if tasks are in a dependency chain (which is OK)
			if !AreTasksInDependencyChain(spec, taskIDs) {
				messages = append(messages, ValidationMessage{
					Severity:   SeverityWarning,
					Message:    fmt.Sprintf("File '%s' is modified by multiple parallel tasks", file),
					RelatedIDs: taskIDs,
					Field:      "files",
					Suggestion: "Add dependencies between these tasks or assign different files",
				})
			}
		}
	}

	return messages
}

// DetectDependencyCycle detects if there's a dependency cycle in the plan.
// Returns the task IDs forming the cycle if found, nil otherwise.
func DetectDependencyCycle(spec *PlanSpec) []string {
	if spec == nil {
		return nil
	}

	// Track visited and recursion stack
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	parent := make(map[string]string)

	var dfs func(taskID string) []string
	dfs = func(taskID string) []string {
		visited[taskID] = true
		recStack[taskID] = true

		task := GetTaskByID(spec, taskID)
		if task == nil {
			recStack[taskID] = false
			return nil
		}

		for _, depID := range task.DependsOn {
			if !visited[depID] {
				parent[depID] = taskID
				if cycle := dfs(depID); cycle != nil {
					return cycle
				}
			} else if recStack[depID] {
				// Found a cycle - reconstruct it
				cycle := []string{depID}
				current := taskID
				for current != depID {
					cycle = append([]string{current}, cycle...)
					current = parent[current]
				}
				cycle = append([]string{depID}, cycle...)
				return cycle
			}
		}

		recStack[taskID] = false
		return nil
	}

	for _, task := range spec.Tasks {
		if !visited[task.ID] {
			if cycle := dfs(task.ID); cycle != nil {
				return cycle
			}
		}
	}

	return nil
}

// AreTasksInDependencyChain checks if a set of tasks are all in a linear dependency chain
// (meaning they can't execute in parallel and file conflicts are acceptable).
func AreTasksInDependencyChain(spec *PlanSpec, taskIDs []string) bool {
	if len(taskIDs) < 2 {
		return true
	}

	// Build a dependency set for each task (all tasks it directly or indirectly depends on)
	allDeps := make(map[string]map[string]bool)
	for _, taskID := range taskIDs {
		allDeps[taskID] = GetAllDependencies(spec, taskID)
	}

	// Check if for each pair, one depends on the other
	for i, taskA := range taskIDs {
		for _, taskB := range taskIDs[i+1:] {
			// Check if A depends on B or B depends on A
			aOnB := allDeps[taskA][taskB]
			bOnA := allDeps[taskB][taskA]

			if !aOnB && !bOnA {
				// Neither depends on the other - they can run in parallel
				return false
			}
		}
	}

	return true
}

// GetAllDependencies returns all direct and transitive dependencies for a task.
func GetAllDependencies(spec *PlanSpec, taskID string) map[string]bool {
	deps := make(map[string]bool)
	visited := make(map[string]bool)

	var collectDeps func(id string)
	collectDeps = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true

		task := GetTaskByID(spec, id)
		if task == nil {
			return
		}

		for _, depID := range task.DependsOn {
			deps[depID] = true
			collectDeps(depID)
		}
	}

	collectDeps(taskID)
	return deps
}

// GetTaskByID returns a pointer to a task by ID, or nil if not found.
func GetTaskByID(spec *PlanSpec, taskID string) *PlannedTask {
	if spec == nil {
		return nil
	}

	for i := range spec.Tasks {
		if spec.Tasks[i].ID == taskID {
			return &spec.Tasks[i]
		}
	}
	return nil
}

// GetTasksInCycle returns a list of task IDs that are part of a dependency cycle, if any.
// This is useful for highlighting cyclic tasks in the UI.
func GetTasksInCycle(spec *PlanSpec) []string {
	cycle := DetectDependencyCycle(spec)
	if cycle == nil {
		return nil
	}

	// Return unique task IDs from the cycle
	seen := make(map[string]bool)
	var unique []string
	for _, id := range cycle {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	return unique
}
