package ultraplan

import (
	"testing"
)

func TestValidatePlan_NilPlan(t *testing.T) {
	result, err := ValidatePlan(nil)
	if err != nil {
		t.Fatalf("ValidatePlan returned error: %v", err)
	}

	if result.IsValid {
		t.Error("Expected invalid result for nil plan")
	}
	if result.ErrorCount != 1 {
		t.Errorf("Expected 1 error, got %d", result.ErrorCount)
	}
}

func TestValidatePlan_EmptyPlan(t *testing.T) {
	spec := &PlanSpec{
		Tasks: []PlannedTask{},
	}

	result, err := ValidatePlan(spec)
	if err != nil {
		t.Fatalf("ValidatePlan returned error: %v", err)
	}

	if result.IsValid {
		t.Error("Expected invalid result for empty plan")
	}
	if result.ErrorCount != 1 {
		t.Errorf("Expected 1 error, got %d", result.ErrorCount)
	}
}

func TestValidatePlan_ValidPlan(t *testing.T) {
	spec := &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Task 1", Description: "First task"},
			{ID: "task-2", Title: "Task 2", Description: "Second task", DependsOn: []string{"task-1"}},
		},
	}

	result, err := ValidatePlan(spec)
	if err != nil {
		t.Fatalf("ValidatePlan returned error: %v", err)
	}

	if !result.IsValid {
		t.Error("Expected valid result for valid plan")
	}
	if result.ErrorCount != 0 {
		t.Errorf("Expected 0 errors, got %d", result.ErrorCount)
	}
}

func TestValidateTaskDependencies_MissingDescription(t *testing.T) {
	tasks := []PlannedTask{
		{ID: "task-1", Title: "Task 1", Description: ""},
	}

	messages := ValidateTaskDependencies(tasks)

	hasWarning := false
	for _, msg := range messages {
		if msg.TaskID == "task-1" && msg.Field == "description" && msg.Severity == SeverityWarning {
			hasWarning = true
			break
		}
	}

	if !hasWarning {
		t.Error("Expected warning for missing description")
	}
}

func TestValidateTaskDependencies_MissingTitle(t *testing.T) {
	tasks := []PlannedTask{
		{ID: "task-1", Title: "", Description: "A task"},
	}

	messages := ValidateTaskDependencies(tasks)

	hasWarning := false
	for _, msg := range messages {
		if msg.TaskID == "task-1" && msg.Field == "title" && msg.Severity == SeverityWarning {
			hasWarning = true
			break
		}
	}

	if !hasWarning {
		t.Error("Expected warning for missing title")
	}
}

func TestValidateTaskDependencies_SelfDependency(t *testing.T) {
	tasks := []PlannedTask{
		{ID: "task-1", Title: "Task 1", Description: "A task", DependsOn: []string{"task-1"}},
	}

	messages := ValidateTaskDependencies(tasks)

	hasError := false
	for _, msg := range messages {
		if msg.TaskID == "task-1" && msg.IsError() && msg.Message == "Task depends on itself" {
			hasError = true
			break
		}
	}

	if !hasError {
		t.Error("Expected error for self-dependency")
	}
}

func TestValidateTaskDependencies_InvalidDependency(t *testing.T) {
	tasks := []PlannedTask{
		{ID: "task-1", Title: "Task 1", Description: "A task", DependsOn: []string{"nonexistent"}},
	}

	messages := ValidateTaskDependencies(tasks)

	hasError := false
	for _, msg := range messages {
		if msg.TaskID == "task-1" && msg.IsError() && msg.Field == "depends_on" {
			hasError = true
			break
		}
	}

	if !hasError {
		t.Error("Expected error for invalid dependency")
	}
}

func TestValidateTaskDependencies_HighComplexity(t *testing.T) {
	tasks := []PlannedTask{
		{ID: "task-1", Title: "Task 1", Description: "A task", EstComplexity: ComplexityHigh},
	}

	messages := ValidateTaskDependencies(tasks)

	hasWarning := false
	for _, msg := range messages {
		if msg.TaskID == "task-1" && msg.IsWarning() && msg.Field == "est_complexity" {
			hasWarning = true
			break
		}
	}

	if !hasWarning {
		t.Error("Expected warning for high complexity")
	}
}

func TestDetectDependencyCycle_NoCycle(t *testing.T) {
	spec := &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Task 1", Description: "First"},
			{ID: "task-2", Title: "Task 2", Description: "Second", DependsOn: []string{"task-1"}},
			{ID: "task-3", Title: "Task 3", Description: "Third", DependsOn: []string{"task-2"}},
		},
	}

	cycle := DetectDependencyCycle(spec)
	if cycle != nil {
		t.Errorf("Expected no cycle, got: %v", cycle)
	}
}

func TestDetectDependencyCycle_DirectCycle(t *testing.T) {
	spec := &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Task 1", Description: "First", DependsOn: []string{"task-2"}},
			{ID: "task-2", Title: "Task 2", Description: "Second", DependsOn: []string{"task-1"}},
		},
	}

	cycle := DetectDependencyCycle(spec)
	if cycle == nil {
		t.Error("Expected cycle to be detected")
	}
}

func TestDetectDependencyCycle_IndirectCycle(t *testing.T) {
	spec := &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Task 1", Description: "First", DependsOn: []string{"task-3"}},
			{ID: "task-2", Title: "Task 2", Description: "Second", DependsOn: []string{"task-1"}},
			{ID: "task-3", Title: "Task 3", Description: "Third", DependsOn: []string{"task-2"}},
		},
	}

	cycle := DetectDependencyCycle(spec)
	if cycle == nil {
		t.Error("Expected cycle to be detected")
	}
}

func TestValidateTaskFiles_NoConflict(t *testing.T) {
	spec := &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Task 1", Description: "First", Files: []string{"file1.go"}},
			{ID: "task-2", Title: "Task 2", Description: "Second", Files: []string{"file2.go"}},
		},
	}

	messages := ValidateTaskFiles(spec)
	if len(messages) != 0 {
		t.Errorf("Expected no file conflict warnings, got %d", len(messages))
	}
}

func TestValidateTaskFiles_ConflictParallel(t *testing.T) {
	spec := &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Task 1", Description: "First", Files: []string{"shared.go"}},
			{ID: "task-2", Title: "Task 2", Description: "Second", Files: []string{"shared.go"}},
		},
	}

	messages := ValidateTaskFiles(spec)

	hasWarning := false
	for _, msg := range messages {
		if msg.IsWarning() && msg.Field == "files" {
			hasWarning = true
			break
		}
	}

	if !hasWarning {
		t.Error("Expected warning for parallel file conflict")
	}
}

func TestValidateTaskFiles_ConflictWithDependency(t *testing.T) {
	spec := &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Task 1", Description: "First", Files: []string{"shared.go"}},
			{ID: "task-2", Title: "Task 2", Description: "Second", Files: []string{"shared.go"}, DependsOn: []string{"task-1"}},
		},
	}

	messages := ValidateTaskFiles(spec)

	// Should not warn because task-2 depends on task-1
	for _, msg := range messages {
		if msg.IsWarning() && msg.Field == "files" {
			t.Error("Should not warn about file conflict when tasks are in dependency chain")
		}
	}
}

func TestAreTasksInDependencyChain_InChain(t *testing.T) {
	spec := &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Task 1", Description: "First"},
			{ID: "task-2", Title: "Task 2", Description: "Second", DependsOn: []string{"task-1"}},
			{ID: "task-3", Title: "Task 3", Description: "Third", DependsOn: []string{"task-2"}},
		},
	}

	if !AreTasksInDependencyChain(spec, []string{"task-1", "task-2"}) {
		t.Error("Expected tasks to be in dependency chain")
	}

	if !AreTasksInDependencyChain(spec, []string{"task-1", "task-3"}) {
		t.Error("Expected tasks to be in dependency chain (transitive)")
	}
}

func TestAreTasksInDependencyChain_NotInChain(t *testing.T) {
	spec := &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Task 1", Description: "First"},
			{ID: "task-2", Title: "Task 2", Description: "Second"},
		},
	}

	if AreTasksInDependencyChain(spec, []string{"task-1", "task-2"}) {
		t.Error("Expected tasks NOT to be in dependency chain")
	}
}

func TestGetAllDependencies(t *testing.T) {
	spec := &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Task 1", Description: "First"},
			{ID: "task-2", Title: "Task 2", Description: "Second", DependsOn: []string{"task-1"}},
			{ID: "task-3", Title: "Task 3", Description: "Third", DependsOn: []string{"task-2"}},
		},
	}

	deps := GetAllDependencies(spec, "task-3")

	if !deps["task-1"] {
		t.Error("Expected task-3 to transitively depend on task-1")
	}
	if !deps["task-2"] {
		t.Error("Expected task-3 to directly depend on task-2")
	}
	if deps["task-3"] {
		t.Error("Task should not depend on itself")
	}
}

func TestGetTaskByID(t *testing.T) {
	spec := &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Task 1", Description: "First"},
			{ID: "task-2", Title: "Task 2", Description: "Second"},
		},
	}

	task := GetTaskByID(spec, "task-1")
	if task == nil {
		t.Error("Expected to find task-1")
	}
	if task.Title != "Task 1" {
		t.Errorf("Expected title 'Task 1', got %q", task.Title)
	}

	task = GetTaskByID(spec, "nonexistent")
	if task != nil {
		t.Error("Expected nil for nonexistent task")
	}

	task = GetTaskByID(nil, "task-1")
	if task != nil {
		t.Error("Expected nil for nil spec")
	}
}

func TestGetTasksInCycle(t *testing.T) {
	spec := &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Task 1", Description: "First", DependsOn: []string{"task-2"}},
			{ID: "task-2", Title: "Task 2", Description: "Second", DependsOn: []string{"task-1"}},
		},
	}

	cycleIDs := GetTasksInCycle(spec)
	if cycleIDs == nil {
		t.Error("Expected to find cycle IDs")
	}

	// Both tasks should be in the cycle
	hasTask1 := false
	hasTask2 := false
	for _, id := range cycleIDs {
		if id == "task-1" {
			hasTask1 = true
		}
		if id == "task-2" {
			hasTask2 = true
		}
	}

	if !hasTask1 || !hasTask2 {
		t.Error("Expected both tasks to be in cycle")
	}
}

func TestGetTasksInCycle_NoCycle(t *testing.T) {
	spec := &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Task 1", Description: "First"},
			{ID: "task-2", Title: "Task 2", Description: "Second", DependsOn: []string{"task-1"}},
		},
	}

	cycleIDs := GetTasksInCycle(spec)
	if cycleIDs != nil {
		t.Errorf("Expected no cycle, got: %v", cycleIDs)
	}
}

func TestValidationResult_Methods(t *testing.T) {
	result := &ValidationResult{
		IsValid:      false,
		ErrorCount:   2,
		WarningCount: 1,
		Messages: []ValidationMessage{
			{Severity: SeverityError, TaskID: "task-1", Message: "Error 1"},
			{Severity: SeverityError, TaskID: "task-2", Message: "Error 2"},
			{Severity: SeverityWarning, TaskID: "task-1", Message: "Warning 1"},
		},
	}

	if !result.HasErrors() {
		t.Error("Expected HasErrors() to be true")
	}
	if !result.HasWarnings() {
		t.Error("Expected HasWarnings() to be true")
	}

	task1Messages := result.GetMessagesForTask("task-1")
	if len(task1Messages) != 2 {
		t.Errorf("Expected 2 messages for task-1, got %d", len(task1Messages))
	}

	errorMessages := result.GetMessagesBySeverity(SeverityError)
	if len(errorMessages) != 2 {
		t.Errorf("Expected 2 error messages, got %d", len(errorMessages))
	}
}
