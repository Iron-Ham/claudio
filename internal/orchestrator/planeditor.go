package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
)

// PlanEditor provides mutation operations for modifying a PlanSpec.
// After each mutation, the dependency graph and execution order are recalculated,
// and the plan is validated to ensure consistency.

// ErrTaskNotFound is returned when a task ID does not exist in the plan
type ErrTaskNotFound struct {
	TaskID string
}

func (e ErrTaskNotFound) Error() string {
	return fmt.Sprintf("task not found: %s", e.TaskID)
}

// ErrInvalidDependency is returned when a dependency reference is invalid
type ErrInvalidDependency struct {
	TaskID       string
	DependencyID string
	Reason       string
}

func (e ErrInvalidDependency) Error() string {
	return fmt.Sprintf("invalid dependency %s for task %s: %s", e.DependencyID, e.TaskID, e.Reason)
}

// ErrCannotMerge is returned when tasks cannot be merged
type ErrCannotMerge struct {
	Reason string
}

func (e ErrCannotMerge) Error() string {
	return fmt.Sprintf("cannot merge tasks: %s", e.Reason)
}

// ErrCannotSplit is returned when a task cannot be split
type ErrCannotSplit struct {
	TaskID string
	Reason string
}

func (e ErrCannotSplit) Error() string {
	return fmt.Sprintf("cannot split task %s: %s", e.TaskID, e.Reason)
}

// findTaskIndex returns the index of a task in the plan's Tasks slice, or -1 if not found
func findTaskIndex(plan *PlanSpec, taskID string) int {
	for i, task := range plan.Tasks {
		if task.ID == taskID {
			return i
		}
	}
	return -1
}

// recalculatePlan rebuilds the dependency graph and execution order after a mutation
func recalculatePlan(plan *PlanSpec) error {
	// Rebuild dependency graph from tasks
	plan.DependencyGraph = make(map[string][]string)
	for _, task := range plan.Tasks {
		plan.DependencyGraph[task.ID] = task.DependsOn
	}

	// Recalculate execution order
	plan.ExecutionOrder = calculateExecutionOrder(plan.Tasks, plan.DependencyGraph)

	// Validate the resulting plan
	return ValidatePlan(plan)
}

// UpdateTaskTitle updates the title of a task
func UpdateTaskTitle(plan *PlanSpec, taskID, newTitle string) error {
	if plan == nil {
		return fmt.Errorf("plan is nil")
	}

	idx := findTaskIndex(plan, taskID)
	if idx == -1 {
		return ErrTaskNotFound{TaskID: taskID}
	}

	plan.Tasks[idx].Title = newTitle
	return nil // Title change doesn't affect dependencies or execution order
}

// UpdateTaskDescription updates the description of a task
func UpdateTaskDescription(plan *PlanSpec, taskID, newDesc string) error {
	if plan == nil {
		return fmt.Errorf("plan is nil")
	}

	idx := findTaskIndex(plan, taskID)
	if idx == -1 {
		return ErrTaskNotFound{TaskID: taskID}
	}

	plan.Tasks[idx].Description = newDesc
	return nil // Description change doesn't affect dependencies or execution order
}

// UpdateTaskFiles updates the expected files list for a task
func UpdateTaskFiles(plan *PlanSpec, taskID string, files []string) error {
	if plan == nil {
		return fmt.Errorf("plan is nil")
	}

	idx := findTaskIndex(plan, taskID)
	if idx == -1 {
		return ErrTaskNotFound{TaskID: taskID}
	}

	plan.Tasks[idx].Files = files
	return nil // Files change doesn't affect dependencies or execution order
}

// UpdateTaskPriority updates the priority of a task
// Lower priority values indicate higher priority (executed earlier within same group)
func UpdateTaskPriority(plan *PlanSpec, taskID string, priority int) error {
	if plan == nil {
		return fmt.Errorf("plan is nil")
	}

	idx := findTaskIndex(plan, taskID)
	if idx == -1 {
		return ErrTaskNotFound{TaskID: taskID}
	}

	plan.Tasks[idx].Priority = priority

	// Priority affects execution order within groups, so recalculate
	return recalculatePlan(plan)
}

// UpdateTaskComplexity updates the estimated complexity of a task
func UpdateTaskComplexity(plan *PlanSpec, taskID string, complexity TaskComplexity) error {
	if plan == nil {
		return fmt.Errorf("plan is nil")
	}

	idx := findTaskIndex(plan, taskID)
	if idx == -1 {
		return ErrTaskNotFound{TaskID: taskID}
	}

	// Validate complexity value
	switch complexity {
	case ComplexityLow, ComplexityMedium, ComplexityHigh:
		// Valid
	default:
		return fmt.Errorf("invalid complexity value: %s (must be low, medium, or high)", complexity)
	}

	plan.Tasks[idx].EstComplexity = complexity
	return nil // Complexity doesn't affect execution order
}

// UpdateTaskDependencies updates the dependencies of a task
// This triggers a full recalculation of the dependency graph and execution order
func UpdateTaskDependencies(plan *PlanSpec, taskID string, deps []string) error {
	if plan == nil {
		return fmt.Errorf("plan is nil")
	}

	idx := findTaskIndex(plan, taskID)
	if idx == -1 {
		return ErrTaskNotFound{TaskID: taskID}
	}

	// Build task ID set for validation
	taskSet := make(map[string]bool)
	for _, task := range plan.Tasks {
		taskSet[task.ID] = true
	}

	// Validate all dependencies exist and no self-dependency
	for _, depID := range deps {
		if depID == taskID {
			return ErrInvalidDependency{
				TaskID:       taskID,
				DependencyID: depID,
				Reason:       "task cannot depend on itself",
			}
		}
		if !taskSet[depID] {
			return ErrInvalidDependency{
				TaskID:       taskID,
				DependencyID: depID,
				Reason:       "dependency task does not exist",
			}
		}
	}

	plan.Tasks[idx].DependsOn = deps
	return recalculatePlan(plan)
}

// DeleteTask removes a task from the plan
// This also removes the task from any other tasks' dependency lists
func DeleteTask(plan *PlanSpec, taskID string) error {
	if plan == nil {
		return fmt.Errorf("plan is nil")
	}

	idx := findTaskIndex(plan, taskID)
	if idx == -1 {
		return ErrTaskNotFound{TaskID: taskID}
	}

	// Remove the task
	plan.Tasks = append(plan.Tasks[:idx], plan.Tasks[idx+1:]...)

	// Remove this task from all dependency lists
	for i := range plan.Tasks {
		var newDeps []string
		for _, depID := range plan.Tasks[i].DependsOn {
			if depID != taskID {
				newDeps = append(newDeps, depID)
			}
		}
		plan.Tasks[i].DependsOn = newDeps
	}

	return recalculatePlan(plan)
}

// AddTask adds a new task to the plan
// If afterTaskID is empty, the task is appended at the end
// If afterTaskID is specified, the task is inserted after that task in the Tasks slice
func AddTask(plan *PlanSpec, afterTaskID string, task PlannedTask) error {
	if plan == nil {
		return fmt.Errorf("plan is nil")
	}

	// Check for duplicate ID
	if findTaskIndex(plan, task.ID) != -1 {
		return fmt.Errorf("task with ID %s already exists", task.ID)
	}

	// Validate dependencies
	taskSet := make(map[string]bool)
	for _, t := range plan.Tasks {
		taskSet[t.ID] = true
	}

	for _, depID := range task.DependsOn {
		if depID == task.ID {
			return ErrInvalidDependency{
				TaskID:       task.ID,
				DependencyID: depID,
				Reason:       "task cannot depend on itself",
			}
		}
		if !taskSet[depID] {
			return ErrInvalidDependency{
				TaskID:       task.ID,
				DependencyID: depID,
				Reason:       "dependency task does not exist",
			}
		}
	}

	if afterTaskID == "" {
		// Append at end
		plan.Tasks = append(plan.Tasks, task)
	} else {
		// Find insertion point
		afterIdx := findTaskIndex(plan, afterTaskID)
		if afterIdx == -1 {
			return ErrTaskNotFound{TaskID: afterTaskID}
		}

		// Insert after the specified task
		insertIdx := afterIdx + 1
		plan.Tasks = append(plan.Tasks[:insertIdx], append([]PlannedTask{task}, plan.Tasks[insertIdx:]...)...)
	}

	return recalculatePlan(plan)
}

// MoveTaskUp moves a task earlier in the Tasks slice order
// This affects display order but not execution order (which is determined by dependencies)
func MoveTaskUp(plan *PlanSpec, taskID string) error {
	if plan == nil {
		return fmt.Errorf("plan is nil")
	}

	idx := findTaskIndex(plan, taskID)
	if idx == -1 {
		return ErrTaskNotFound{TaskID: taskID}
	}

	if idx == 0 {
		// Already at the top
		return nil
	}

	// Swap with previous task
	plan.Tasks[idx], plan.Tasks[idx-1] = plan.Tasks[idx-1], plan.Tasks[idx]
	return nil // No need to recalculate, just display order changed
}

// MoveTaskDown moves a task later in the Tasks slice order
// This affects display order but not execution order (which is determined by dependencies)
func MoveTaskDown(plan *PlanSpec, taskID string) error {
	if plan == nil {
		return fmt.Errorf("plan is nil")
	}

	idx := findTaskIndex(plan, taskID)
	if idx == -1 {
		return ErrTaskNotFound{TaskID: taskID}
	}

	if idx == len(plan.Tasks)-1 {
		// Already at the bottom
		return nil
	}

	// Swap with next task
	plan.Tasks[idx], plan.Tasks[idx+1] = plan.Tasks[idx+1], plan.Tasks[idx]
	return nil // No need to recalculate, just display order changed
}

// SplitTask splits one task into multiple tasks
// The splitPoints slice indicates character offsets in the description where to split
// Returns the IDs of the newly created tasks (including the modified original)
func SplitTask(plan *PlanSpec, taskID string, splitPoints []int) ([]string, error) {
	if plan == nil {
		return nil, fmt.Errorf("plan is nil")
	}

	idx := findTaskIndex(plan, taskID)
	if idx == -1 {
		return nil, ErrTaskNotFound{TaskID: taskID}
	}

	if len(splitPoints) == 0 {
		return nil, ErrCannotSplit{
			TaskID: taskID,
			Reason: "no split points provided",
		}
	}

	originalTask := plan.Tasks[idx]
	descLen := len(originalTask.Description)

	// Validate and sort split points
	for _, sp := range splitPoints {
		if sp <= 0 || sp >= descLen {
			return nil, ErrCannotSplit{
				TaskID: taskID,
				Reason: fmt.Sprintf("split point %d is out of bounds (description length: %d)", sp, descLen),
			}
		}
	}

	// Sort split points and add boundaries
	sortedPoints := make([]int, len(splitPoints))
	copy(sortedPoints, splitPoints)
	for i := 0; i < len(sortedPoints)-1; i++ {
		for j := i + 1; j < len(sortedPoints); j++ {
			if sortedPoints[i] > sortedPoints[j] {
				sortedPoints[i], sortedPoints[j] = sortedPoints[j], sortedPoints[i]
			}
		}
	}

	// Create boundaries: [0, sp1, sp2, ..., len]
	boundaries := append([]int{0}, sortedPoints...)
	boundaries = append(boundaries, descLen)

	// Generate new task IDs
	var newTaskIDs []string
	baseID := originalTask.ID
	for i := 0; i < len(boundaries)-1; i++ {
		if i == 0 {
			newTaskIDs = append(newTaskIDs, baseID) // Keep original ID for first part
		} else {
			newTaskIDs = append(newTaskIDs, fmt.Sprintf("%s-part%d", baseID, i+1))
		}
	}

	// Check for ID conflicts
	for _, newID := range newTaskIDs[1:] { // Skip first (original ID)
		if findTaskIndex(plan, newID) != -1 {
			return nil, ErrCannotSplit{
				TaskID: taskID,
				Reason: fmt.Sprintf("generated task ID %s already exists", newID),
			}
		}
	}

	// Create new tasks
	var newTasks []PlannedTask
	for i := 0; i < len(boundaries)-1; i++ {
		start := boundaries[i]
		end := boundaries[i+1]
		descPart := strings.TrimSpace(originalTask.Description[start:end])

		newTask := PlannedTask{
			ID:            newTaskIDs[i],
			Title:         fmt.Sprintf("%s (Part %d)", originalTask.Title, i+1),
			Description:   descPart,
			Files:         nil, // Files will need to be reassigned manually
			Priority:      originalTask.Priority,
			EstComplexity: originalTask.EstComplexity,
		}

		if i == 0 {
			// First part inherits original dependencies
			newTask.DependsOn = originalTask.DependsOn
		} else {
			// Subsequent parts depend on the previous part (sequential execution)
			newTask.DependsOn = []string{newTaskIDs[i-1]}
		}

		newTasks = append(newTasks, newTask)
	}

	// Replace original task with new tasks
	newTasksList := make([]PlannedTask, 0, len(plan.Tasks)-1+len(newTasks))
	newTasksList = append(newTasksList, plan.Tasks[:idx]...)
	newTasksList = append(newTasksList, newTasks...)
	newTasksList = append(newTasksList, plan.Tasks[idx+1:]...)
	plan.Tasks = newTasksList

	// Update dependencies: any task that depended on the original should depend on the last part
	lastNewID := newTaskIDs[len(newTaskIDs)-1]
	for i := range plan.Tasks {
		var updatedDeps []string
		for _, depID := range plan.Tasks[i].DependsOn {
			if depID == taskID && plan.Tasks[i].ID != newTaskIDs[0] {
				// Replace original ID with last part ID (if not one of the new split tasks)
				if !slices.Contains(newTaskIDs, plan.Tasks[i].ID) {
					updatedDeps = append(updatedDeps, lastNewID)
				} else {
					updatedDeps = append(updatedDeps, depID)
				}
			} else {
				updatedDeps = append(updatedDeps, depID)
			}
		}
		plan.Tasks[i].DependsOn = updatedDeps
	}

	if err := recalculatePlan(plan); err != nil {
		return nil, err
	}

	return newTaskIDs, nil
}

// MergeTasks combines multiple tasks into a single task
// The merged task inherits the union of all files and dependencies
// Returns the ID of the merged task
func MergeTasks(plan *PlanSpec, taskIDs []string, mergedTitle string) (string, error) {
	if plan == nil {
		return "", fmt.Errorf("plan is nil")
	}

	if len(taskIDs) < 2 {
		return "", ErrCannotMerge{Reason: "need at least 2 tasks to merge"}
	}

	// Find all tasks and validate they exist
	var tasksToMerge []PlannedTask
	var indices []int
	for _, tid := range taskIDs {
		idx := findTaskIndex(plan, tid)
		if idx == -1 {
			return "", ErrTaskNotFound{TaskID: tid}
		}
		tasksToMerge = append(tasksToMerge, plan.Tasks[idx])
		indices = append(indices, idx)
	}

	// Build the merged task
	mergeSet := make(map[string]bool)
	for _, tid := range taskIDs {
		mergeSet[tid] = true
	}

	// Collect all files (deduplicated)
	fileSet := make(map[string]bool)
	for _, task := range tasksToMerge {
		for _, f := range task.Files {
			fileSet[f] = true
		}
	}
	var mergedFiles []string
	for f := range fileSet {
		mergedFiles = append(mergedFiles, f)
	}

	// Collect all external dependencies (not pointing to tasks being merged)
	depSet := make(map[string]bool)
	for _, task := range tasksToMerge {
		for _, depID := range task.DependsOn {
			if !mergeSet[depID] {
				depSet[depID] = true
			}
		}
	}
	var mergedDeps []string
	for d := range depSet {
		mergedDeps = append(mergedDeps, d)
	}

	// Combine descriptions
	var descParts []string
	for _, task := range tasksToMerge {
		descParts = append(descParts, fmt.Sprintf("## %s\n%s", task.Title, task.Description))
	}
	mergedDesc := strings.Join(descParts, "\n\n")

	// Use the first task's ID as the merged task ID
	mergedID := taskIDs[0]

	// Determine complexity (take the highest)
	var mergedComplexity TaskComplexity = ComplexityLow
	for _, task := range tasksToMerge {
		switch task.EstComplexity {
		case ComplexityHigh:
			mergedComplexity = ComplexityHigh
		case ComplexityMedium:
			if mergedComplexity != ComplexityHigh {
				mergedComplexity = ComplexityMedium
			}
		}
	}

	// Use lowest (highest priority) priority value
	mergedPriority := tasksToMerge[0].Priority
	for _, task := range tasksToMerge {
		if task.Priority < mergedPriority {
			mergedPriority = task.Priority
		}
	}

	mergedTask := PlannedTask{
		ID:            mergedID,
		Title:         mergedTitle,
		Description:   mergedDesc,
		Files:         mergedFiles,
		DependsOn:     mergedDeps,
		Priority:      mergedPriority,
		EstComplexity: mergedComplexity,
	}

	// Remove all tasks being merged and add the merged task
	// Sort indices in descending order to remove from back to front
	for i := 0; i < len(indices)-1; i++ {
		for j := i + 1; j < len(indices); j++ {
			if indices[i] < indices[j] {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}

	// Remove tasks (from back to front to preserve indices)
	for _, idx := range indices {
		plan.Tasks = append(plan.Tasks[:idx], plan.Tasks[idx+1:]...)
	}

	// Add merged task
	plan.Tasks = append(plan.Tasks, mergedTask)

	// Update dependencies: any task that depended on any merged task should depend on the merged task
	for i := range plan.Tasks {
		if plan.Tasks[i].ID == mergedID {
			continue
		}
		var updatedDeps []string
		addedMerged := false
		for _, depID := range plan.Tasks[i].DependsOn {
			if mergeSet[depID] {
				// This dependency was merged
				if !addedMerged {
					updatedDeps = append(updatedDeps, mergedID)
					addedMerged = true
				}
			} else {
				updatedDeps = append(updatedDeps, depID)
			}
		}
		plan.Tasks[i].DependsOn = updatedDeps
	}

	if err := recalculatePlan(plan); err != nil {
		return "", err
	}

	return mergedID, nil
}

// SavePlanToFile writes the plan to a JSON file
func SavePlanToFile(plan *PlanSpec, filepath string) error {
	if plan == nil {
		return fmt.Errorf("plan is nil")
	}

	// Marshal with indentation for readability
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write plan file: %w", err)
	}

	return nil
}

// ClonePlan creates a deep copy of a PlanSpec
// Useful for creating a backup before mutations
func ClonePlan(plan *PlanSpec) (*PlanSpec, error) {
	if plan == nil {
		return nil, fmt.Errorf("plan is nil")
	}

	// Use JSON marshaling for a clean deep copy
	data, err := json.Marshal(plan)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal plan: %w", err)
	}

	var cloned PlanSpec
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, fmt.Errorf("failed to unmarshal plan: %w", err)
	}

	return &cloned, nil
}

// GetTaskByID returns a pointer to a task by ID, or nil if not found
func GetTaskByID(plan *PlanSpec, taskID string) *PlannedTask {
	if plan == nil {
		return nil
	}
	idx := findTaskIndex(plan, taskID)
	if idx == -1 {
		return nil
	}
	return &plan.Tasks[idx]
}

// GetTasksInExecutionGroup returns the task IDs for a specific execution group
func GetTasksInExecutionGroup(plan *PlanSpec, groupIndex int) []string {
	if plan == nil || groupIndex < 0 || groupIndex >= len(plan.ExecutionOrder) {
		return nil
	}
	return plan.ExecutionOrder[groupIndex]
}

// GetExecutionGroupForTask returns the execution group index for a task, or -1 if not found
func GetExecutionGroupForTask(plan *PlanSpec, taskID string) int {
	if plan == nil {
		return -1
	}
	for i, group := range plan.ExecutionOrder {
		if slices.Contains(group, taskID) {
			return i
		}
	}
	return -1
}

// GetDependents returns all tasks that depend on the given task
func GetDependents(plan *PlanSpec, taskID string) []string {
	if plan == nil {
		return nil
	}

	var dependents []string
	for _, task := range plan.Tasks {
		if slices.Contains(task.DependsOn, taskID) {
			dependents = append(dependents, task.ID)
		}
	}
	return dependents
}

// HasCircularDependency checks if adding a dependency would create a cycle
// Returns true if the dependency would create a cycle
func HasCircularDependency(plan *PlanSpec, taskID, newDepID string) bool {
	if plan == nil || taskID == newDepID {
		return true
	}

	// Check if newDepID transitively depends on taskID
	visited := make(map[string]bool)
	var checkCycle func(current string) bool
	checkCycle = func(current string) bool {
		if current == taskID {
			return true
		}
		if visited[current] {
			return false
		}
		visited[current] = true

		task := GetTaskByID(plan, current)
		if task == nil {
			return false
		}

		for _, depID := range task.DependsOn {
			if checkCycle(depID) {
				return true
			}
		}
		return false
	}

	return checkCycle(newDepID)
}

// ValidationSeverity represents the severity level of a validation message
type ValidationSeverity string

const (
	SeverityError   ValidationSeverity = "error"
	SeverityWarning ValidationSeverity = "warning"
	SeverityInfo    ValidationSeverity = "info"
)

// ValidationMessage represents a single validation issue with structured information
type ValidationMessage struct {
	Severity    ValidationSeverity `json:"severity"`
	Message     string             `json:"message"`
	TaskID      string             `json:"task_id,omitempty"`      // The task this message relates to (empty for plan-level issues)
	Field       string             `json:"field,omitempty"`        // The field causing the issue (e.g., "depends_on", "description")
	Suggestion  string             `json:"suggestion,omitempty"`   // A suggested fix
	RelatedIDs  []string           `json:"related_ids,omitempty"`  // Related task IDs (for cycles, conflicts, etc.)
}

// ValidationResult contains the complete validation results for a plan
type ValidationResult struct {
	IsValid   bool                `json:"is_valid"`   // True if there are no errors (warnings allowed)
	Messages  []ValidationMessage `json:"messages"`
	ErrorCount   int              `json:"error_count"`
	WarningCount int              `json:"warning_count"`
	InfoCount    int              `json:"info_count"`
}

// HasErrors returns true if there are any error-level messages
func (v *ValidationResult) HasErrors() bool {
	return v.ErrorCount > 0
}

// HasWarnings returns true if there are any warning-level messages
func (v *ValidationResult) HasWarnings() bool {
	return v.WarningCount > 0
}

// GetMessagesForTask returns all validation messages for a specific task
func (v *ValidationResult) GetMessagesForTask(taskID string) []ValidationMessage {
	var messages []ValidationMessage
	for _, msg := range v.Messages {
		if msg.TaskID == taskID {
			messages = append(messages, msg)
		}
	}
	return messages
}

// GetMessagesBySeverity returns all messages of a specific severity
func (v *ValidationResult) GetMessagesBySeverity(severity ValidationSeverity) []ValidationMessage {
	var messages []ValidationMessage
	for _, msg := range v.Messages {
		if msg.Severity == severity {
			messages = append(messages, msg)
		}
	}
	return messages
}

// ValidatePlanForEditor performs comprehensive validation of a plan for the editor UI.
// It returns structured validation results including errors, warnings, and informational messages.
// This is more comprehensive than ValidatePlan() which only returns basic validity.
func ValidatePlanForEditor(plan *PlanSpec) *ValidationResult {
	result := &ValidationResult{
		IsValid:  true,
		Messages: make([]ValidationMessage, 0),
	}

	if plan == nil {
		result.IsValid = false
		result.Messages = append(result.Messages, ValidationMessage{
			Severity: SeverityError,
			Message:  "Plan is nil",
		})
		result.ErrorCount++
		return result
	}

	if len(plan.Tasks) == 0 {
		result.IsValid = false
		result.Messages = append(result.Messages, ValidationMessage{
			Severity:   SeverityError,
			Message:    "Plan has no tasks",
			Suggestion: "Add at least one task to the plan",
		})
		result.ErrorCount++
		return result
	}

	// Build task ID set for validation
	taskSet := make(map[string]bool)
	for _, task := range plan.Tasks {
		taskSet[task.ID] = true
	}

	// Build a map of files to tasks for conflict detection
	fileToTasks := make(map[string][]string)
	for _, task := range plan.Tasks {
		for _, file := range task.Files {
			fileToTasks[file] = append(fileToTasks[file], task.ID)
		}
	}

	// Validate each task
	for _, task := range plan.Tasks {
		// Check for missing description (warning)
		if strings.TrimSpace(task.Description) == "" {
			result.Messages = append(result.Messages, ValidationMessage{
				Severity:   SeverityWarning,
				Message:    "Task has no description",
				TaskID:     task.ID,
				Field:      "description",
				Suggestion: "Add a detailed description for the task",
			})
			result.WarningCount++
		}

		// Check for missing title (warning)
		if strings.TrimSpace(task.Title) == "" {
			result.Messages = append(result.Messages, ValidationMessage{
				Severity:   SeverityWarning,
				Message:    "Task has no title",
				TaskID:     task.ID,
				Field:      "title",
				Suggestion: "Add a descriptive title for the task",
			})
			result.WarningCount++
		}

		// Check for self-dependency (error)
		for _, depID := range task.DependsOn {
			if depID == task.ID {
				result.IsValid = false
				result.Messages = append(result.Messages, ValidationMessage{
					Severity:   SeverityError,
					Message:    "Task depends on itself",
					TaskID:     task.ID,
					Field:      "depends_on",
					RelatedIDs: []string{task.ID},
					Suggestion: "Remove the self-dependency",
				})
				result.ErrorCount++
			}
		}

		// Check for invalid dependency references (error)
		for _, depID := range task.DependsOn {
			if depID != task.ID && !taskSet[depID] {
				result.IsValid = false
				result.Messages = append(result.Messages, ValidationMessage{
					Severity:   SeverityError,
					Message:    fmt.Sprintf("Depends on unknown task '%s'", depID),
					TaskID:     task.ID,
					Field:      "depends_on",
					RelatedIDs: []string{depID},
					Suggestion: fmt.Sprintf("Remove '%s' from dependencies or create a task with that ID", depID),
				})
				result.ErrorCount++
			}
		}

		// Check for high complexity tasks (warning - might benefit from splitting)
		if task.EstComplexity == ComplexityHigh {
			result.Messages = append(result.Messages, ValidationMessage{
				Severity:   SeverityWarning,
				Message:    "High complexity task may benefit from splitting",
				TaskID:     task.ID,
				Field:      "est_complexity",
				Suggestion: "Consider splitting into smaller, more manageable subtasks",
			})
			result.WarningCount++
		}
	}

	// Check for dependency cycles
	cycleInfo := detectDependencyCycle(plan)
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

	// Check for file conflicts (warning - tasks with overlapping files not in same dependency chain)
	for file, taskIDs := range fileToTasks {
		if len(taskIDs) > 1 {
			// Check if tasks are in a dependency chain (which is OK)
			if !areTasksInDependencyChain(plan, taskIDs) {
				result.Messages = append(result.Messages, ValidationMessage{
					Severity:   SeverityWarning,
					Message:    fmt.Sprintf("File '%s' is modified by multiple parallel tasks", file),
					RelatedIDs: taskIDs,
					Field:      "files",
					Suggestion: "Add dependencies between these tasks or assign different files",
				})
				result.WarningCount++
			}
		}
	}

	// Verify execution order coverage matches task count
	if plan.ExecutionOrder != nil {
		scheduledTasks := 0
		for _, group := range plan.ExecutionOrder {
			scheduledTasks += len(group)
		}
		if scheduledTasks < len(plan.Tasks) {
			result.IsValid = false
			result.Messages = append(result.Messages, ValidationMessage{
				Severity:   SeverityError,
				Message:    fmt.Sprintf("Execution order incomplete: only %d of %d tasks scheduled (indicates a cycle)", scheduledTasks, len(plan.Tasks)),
				Suggestion: "Fix the dependency cycle to allow all tasks to be scheduled",
			})
			result.ErrorCount++
		}
	}

	return result
}

// detectDependencyCycle detects if there's a dependency cycle in the plan.
// Returns the task IDs forming the cycle if found, nil otherwise.
func detectDependencyCycle(plan *PlanSpec) []string {
	if plan == nil {
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

		task := GetTaskByID(plan, taskID)
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

	for _, task := range plan.Tasks {
		if !visited[task.ID] {
			if cycle := dfs(task.ID); cycle != nil {
				return cycle
			}
		}
	}

	return nil
}

// areTasksInDependencyChain checks if a set of tasks are all in a linear dependency chain
// (meaning they can't execute in parallel and file conflicts are acceptable)
func areTasksInDependencyChain(plan *PlanSpec, taskIDs []string) bool {
	if len(taskIDs) < 2 {
		return true
	}

	// Build a dependency set for each task (all tasks it directly or indirectly depends on)
	allDeps := make(map[string]map[string]bool)
	for _, taskID := range taskIDs {
		allDeps[taskID] = getAllDependencies(plan, taskID)
	}

	// Check if for each pair, one depends on the other
	for i := 0; i < len(taskIDs); i++ {
		for j := i + 1; j < len(taskIDs); j++ {
			taskA := taskIDs[i]
			taskB := taskIDs[j]

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

// getAllDependencies returns all direct and transitive dependencies for a task
func getAllDependencies(plan *PlanSpec, taskID string) map[string]bool {
	deps := make(map[string]bool)
	visited := make(map[string]bool)

	var collectDeps func(id string)
	collectDeps = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true

		task := GetTaskByID(plan, id)
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

// GetTasksInCycle returns a list of task IDs that are part of a dependency cycle, if any.
// This is useful for highlighting cyclic tasks in the UI.
func GetTasksInCycle(plan *PlanSpec) []string {
	cycle := detectDependencyCycle(plan)
	if cycle == nil {
		return nil
	}

	// Return unique task IDs from the cycle
	seen := make(map[string]bool)
	var unique []string
	for _, taskID := range cycle {
		if !seen[taskID] {
			seen[taskID] = true
			unique = append(unique, taskID)
		}
	}
	return unique
}
