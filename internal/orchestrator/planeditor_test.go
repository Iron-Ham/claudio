package orchestrator

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// createTestPlan creates a sample plan for testing
func createTestPlan() *PlanSpec {
	tasks := []PlannedTask{
		{
			ID:            "task-1",
			Title:         "Setup",
			Description:   "Initialize the project",
			Files:         []string{"main.go", "go.mod"},
			DependsOn:     []string{},
			Priority:      1,
			EstComplexity: ComplexityLow,
		},
		{
			ID:            "task-2",
			Title:         "Core Features",
			Description:   "Implement core features",
			Files:         []string{"core.go"},
			DependsOn:     []string{"task-1"},
			Priority:      2,
			EstComplexity: ComplexityMedium,
		},
		{
			ID:            "task-3",
			Title:         "Tests",
			Description:   "Write tests",
			Files:         []string{"core_test.go"},
			DependsOn:     []string{"task-2"},
			Priority:      3,
			EstComplexity: ComplexityLow,
		},
		{
			ID:            "task-4",
			Title:         "Documentation",
			Description:   "Write documentation",
			Files:         []string{"README.md"},
			DependsOn:     []string{"task-1"},
			Priority:      2,
			EstComplexity: ComplexityLow,
		},
	}

	deps := make(map[string][]string)
	for _, t := range tasks {
		deps[t.ID] = t.DependsOn
	}

	plan := &PlanSpec{
		ID:              "test-plan",
		Objective:       "Test objective",
		Summary:         "Test summary",
		Tasks:           tasks,
		DependencyGraph: deps,
		ExecutionOrder:  calculateExecutionOrder(tasks, deps),
		CreatedAt:       time.Now(),
	}

	return plan
}

func TestUpdateTaskTitle(t *testing.T) {
	plan := createTestPlan()

	err := UpdateTaskTitle(plan, "task-1", "New Title")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	task := GetTaskByID(plan, "task-1")
	if task.Title != "New Title" {
		t.Errorf("expected title 'New Title', got '%s'", task.Title)
	}

	// Test with non-existent task
	err = UpdateTaskTitle(plan, "nonexistent", "Title")
	if _, ok := err.(ErrTaskNotFound); !ok {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}

	// Test with nil plan
	err = UpdateTaskTitle(nil, "task-1", "Title")
	if err == nil {
		t.Error("expected error for nil plan")
	}
}

func TestUpdateTaskDescription(t *testing.T) {
	plan := createTestPlan()

	err := UpdateTaskDescription(plan, "task-1", "New description")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	task := GetTaskByID(plan, "task-1")
	if task.Description != "New description" {
		t.Errorf("expected description 'New description', got '%s'", task.Description)
	}
}

func TestUpdateTaskFiles(t *testing.T) {
	plan := createTestPlan()
	newFiles := []string{"new1.go", "new2.go", "new3.go"}

	err := UpdateTaskFiles(plan, "task-1", newFiles)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	task := GetTaskByID(plan, "task-1")
	if len(task.Files) != 3 {
		t.Errorf("expected 3 files, got %d", len(task.Files))
	}
	if task.Files[0] != "new1.go" {
		t.Errorf("expected first file 'new1.go', got '%s'", task.Files[0])
	}
}

func TestUpdateTaskPriority(t *testing.T) {
	plan := createTestPlan()

	err := UpdateTaskPriority(plan, "task-2", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	task := GetTaskByID(plan, "task-2")
	if task.Priority != 0 {
		t.Errorf("expected priority 0, got %d", task.Priority)
	}

	// Verify execution order was recalculated
	if len(plan.ExecutionOrder) == 0 {
		t.Error("execution order should not be empty after priority change")
	}
}

func TestUpdateTaskComplexity(t *testing.T) {
	plan := createTestPlan()

	err := UpdateTaskComplexity(plan, "task-1", ComplexityHigh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	task := GetTaskByID(plan, "task-1")
	if task.EstComplexity != ComplexityHigh {
		t.Errorf("expected complexity high, got '%s'", task.EstComplexity)
	}

	// Test invalid complexity
	err = UpdateTaskComplexity(plan, "task-1", "invalid")
	if err == nil {
		t.Error("expected error for invalid complexity")
	}
}

func TestUpdateTaskDependencies(t *testing.T) {
	plan := createTestPlan()

	// Update task-3 to also depend on task-4
	err := UpdateTaskDependencies(plan, "task-3", []string{"task-2", "task-4"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	task := GetTaskByID(plan, "task-3")
	if len(task.DependsOn) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(task.DependsOn))
	}

	// Test self-dependency
	err = UpdateTaskDependencies(plan, "task-1", []string{"task-1"})
	if _, ok := err.(ErrInvalidDependency); !ok {
		t.Errorf("expected ErrInvalidDependency for self-dependency, got %v", err)
	}

	// Test non-existent dependency
	err = UpdateTaskDependencies(plan, "task-1", []string{"nonexistent"})
	if _, ok := err.(ErrInvalidDependency); !ok {
		t.Errorf("expected ErrInvalidDependency for non-existent dependency, got %v", err)
	}
}

func TestDeleteTask(t *testing.T) {
	plan := createTestPlan()
	originalCount := len(plan.Tasks)

	err := DeleteTask(plan, "task-4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(plan.Tasks) != originalCount-1 {
		t.Errorf("expected %d tasks, got %d", originalCount-1, len(plan.Tasks))
	}

	if GetTaskByID(plan, "task-4") != nil {
		t.Error("task-4 should have been deleted")
	}

	// Verify execution order was recalculated
	if err := ValidatePlan(plan); err != nil {
		t.Errorf("plan should be valid after deletion: %v", err)
	}
}

func TestDeleteTaskRemovesDependencies(t *testing.T) {
	plan := createTestPlan()

	// Delete task-1 which is a dependency of task-2 and task-4
	err := DeleteTask(plan, "task-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// task-2 should no longer depend on task-1
	task2 := GetTaskByID(plan, "task-2")
	for _, dep := range task2.DependsOn {
		if dep == "task-1" {
			t.Error("task-2 should not depend on deleted task-1")
		}
	}
}

func TestAddTask(t *testing.T) {
	plan := createTestPlan()
	originalCount := len(plan.Tasks)

	newTask := PlannedTask{
		ID:            "task-5",
		Title:         "New Task",
		Description:   "A new task",
		Files:         []string{"new.go"},
		DependsOn:     []string{"task-1"},
		Priority:      5,
		EstComplexity: ComplexityMedium,
	}

	err := AddTask(plan, "", newTask)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(plan.Tasks) != originalCount+1 {
		t.Errorf("expected %d tasks, got %d", originalCount+1, len(plan.Tasks))
	}

	if GetTaskByID(plan, "task-5") == nil {
		t.Error("task-5 should exist")
	}

	// Verify execution order was recalculated
	if err := ValidatePlan(plan); err != nil {
		t.Errorf("plan should be valid after addition: %v", err)
	}
}

func TestAddTaskAfter(t *testing.T) {
	plan := createTestPlan()

	newTask := PlannedTask{
		ID:            "task-5",
		Title:         "New Task",
		Description:   "A new task",
		Files:         []string{"new.go"},
		DependsOn:     []string{},
		Priority:      5,
		EstComplexity: ComplexityMedium,
	}

	err := AddTask(plan, "task-1", newTask)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify task-5 is at index 1 (after task-1)
	if plan.Tasks[1].ID != "task-5" {
		t.Errorf("expected task-5 at index 1, got %s", plan.Tasks[1].ID)
	}
}

func TestAddTaskDuplicateID(t *testing.T) {
	plan := createTestPlan()

	newTask := PlannedTask{
		ID:    "task-1", // Duplicate
		Title: "Duplicate",
	}

	err := AddTask(plan, "", newTask)
	if err == nil {
		t.Error("expected error for duplicate task ID")
	}
}

func TestMoveTaskUp(t *testing.T) {
	plan := createTestPlan()

	// Move task-2 up (it's at index 1)
	err := MoveTaskUp(plan, "task-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.Tasks[0].ID != "task-2" {
		t.Errorf("expected task-2 at index 0, got %s", plan.Tasks[0].ID)
	}

	// Try to move first task up (should be no-op)
	err = MoveTaskUp(plan, "task-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Tasks[0].ID != "task-2" {
		t.Errorf("first task should stay at index 0")
	}
}

func TestMoveTaskDown(t *testing.T) {
	plan := createTestPlan()

	// Move task-1 down (it's at index 0)
	err := MoveTaskDown(plan, "task-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.Tasks[1].ID != "task-1" {
		t.Errorf("expected task-1 at index 1, got %s", plan.Tasks[1].ID)
	}

	// Move last task down (should be no-op)
	lastIdx := len(plan.Tasks) - 1
	lastTask := plan.Tasks[lastIdx]
	err = MoveTaskDown(plan, lastTask.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Tasks[lastIdx].ID != lastTask.ID {
		t.Error("last task should stay at last index")
	}
}

func TestSplitTask(t *testing.T) {
	plan := createTestPlan()

	// Set up a task with longer description for splitting
	UpdateTaskDescription(plan, "task-2", "Part 1: Setup the foundation. Part 2: Implement the logic. Part 3: Add error handling.")

	// Split at two points
	newIDs, err := SplitTask(plan, "task-2", []int{30, 60})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(newIDs) != 3 {
		t.Errorf("expected 3 new task IDs, got %d", len(newIDs))
	}

	// First part should keep original ID
	if newIDs[0] != "task-2" {
		t.Errorf("expected first part to be 'task-2', got '%s'", newIDs[0])
	}

	// Verify all parts exist
	for _, id := range newIDs {
		if GetTaskByID(plan, id) == nil {
			t.Errorf("task %s should exist", id)
		}
	}

	// Verify plan is still valid
	if err := ValidatePlan(plan); err != nil {
		t.Errorf("plan should be valid after split: %v", err)
	}
}

func TestSplitTaskInvalidPoints(t *testing.T) {
	plan := createTestPlan()

	// Empty split points
	_, err := SplitTask(plan, "task-1", []int{})
	if _, ok := err.(ErrCannotSplit); !ok {
		t.Errorf("expected ErrCannotSplit for empty split points, got %v", err)
	}

	// Out of bounds split point
	_, err = SplitTask(plan, "task-1", []int{1000})
	if _, ok := err.(ErrCannotSplit); !ok {
		t.Errorf("expected ErrCannotSplit for out of bounds split point, got %v", err)
	}
}

func TestMergeTasks(t *testing.T) {
	plan := createTestPlan()

	// Merge task-2 and task-3 (sequential tasks)
	mergedID, err := MergeTasks(plan, []string{"task-2", "task-3"}, "Merged Core and Tests")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mergedID != "task-2" {
		t.Errorf("expected merged ID 'task-2', got '%s'", mergedID)
	}

	merged := GetTaskByID(plan, mergedID)
	if merged == nil {
		t.Fatal("merged task should exist")
	}

	if merged.Title != "Merged Core and Tests" {
		t.Errorf("expected merged title, got '%s'", merged.Title)
	}

	// task-3 should no longer exist
	if GetTaskByID(plan, "task-3") != nil {
		t.Error("task-3 should have been merged and removed")
	}

	// Verify plan is still valid
	if err := ValidatePlan(plan); err != nil {
		t.Errorf("plan should be valid after merge: %v", err)
	}
}

func TestMergeTasksInsufficientTasks(t *testing.T) {
	plan := createTestPlan()

	_, err := MergeTasks(plan, []string{"task-1"}, "Single Task")
	if _, ok := err.(ErrCannotMerge); !ok {
		t.Errorf("expected ErrCannotMerge for single task, got %v", err)
	}
}

func TestMergeTasksNonExistent(t *testing.T) {
	plan := createTestPlan()

	_, err := MergeTasks(plan, []string{"task-1", "nonexistent"}, "Merge")
	if _, ok := err.(ErrTaskNotFound); !ok {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestSavePlanToFile(t *testing.T) {
	plan := createTestPlan()

	// Create temp dir
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test-plan.json")

	err := SavePlanToFile(plan, filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file exists and can be read
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}

	if len(data) == 0 {
		t.Error("saved file should not be empty")
	}

	// Test nil plan
	err = SavePlanToFile(nil, filePath)
	if err == nil {
		t.Error("expected error for nil plan")
	}
}

func TestClonePlan(t *testing.T) {
	plan := createTestPlan()

	cloned, err := ClonePlan(plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it's a deep copy
	if cloned == plan {
		t.Error("cloned plan should be a different object")
	}

	if cloned.ID != plan.ID {
		t.Error("cloned plan should have same ID")
	}

	// Modify cloned and verify original is unchanged
	cloned.Tasks[0].Title = "Modified Title"
	if plan.Tasks[0].Title == "Modified Title" {
		t.Error("original plan should not be modified")
	}
}

func TestGetTaskByID(t *testing.T) {
	plan := createTestPlan()

	task := GetTaskByID(plan, "task-1")
	if task == nil {
		t.Error("task-1 should exist")
	}
	if task.ID != "task-1" {
		t.Errorf("expected ID 'task-1', got '%s'", task.ID)
	}

	// Non-existent task
	task = GetTaskByID(plan, "nonexistent")
	if task != nil {
		t.Error("nonexistent task should return nil")
	}

	// Nil plan
	task = GetTaskByID(nil, "task-1")
	if task != nil {
		t.Error("nil plan should return nil")
	}
}

func TestGetTasksInExecutionGroup(t *testing.T) {
	plan := createTestPlan()

	// Group 0 should contain task-1 (no dependencies)
	group0 := GetTasksInExecutionGroup(plan, 0)
	if len(group0) == 0 {
		t.Error("group 0 should not be empty")
	}

	if !slices.Contains(group0, "task-1") {
		t.Error("task-1 should be in group 0")
	}

	// Invalid group index
	invalid := GetTasksInExecutionGroup(plan, 999)
	if invalid != nil {
		t.Error("invalid group index should return nil")
	}
}

func TestGetExecutionGroupForTask(t *testing.T) {
	plan := createTestPlan()

	// task-1 should be in group 0
	group := GetExecutionGroupForTask(plan, "task-1")
	if group != 0 {
		t.Errorf("expected task-1 in group 0, got %d", group)
	}

	// task-3 depends on task-2 which depends on task-1, so should be in later group
	group3 := GetExecutionGroupForTask(plan, "task-3")
	if group3 <= 0 {
		t.Errorf("task-3 should be in a later group, got %d", group3)
	}

	// Non-existent task
	group = GetExecutionGroupForTask(plan, "nonexistent")
	if group != -1 {
		t.Errorf("expected -1 for nonexistent task, got %d", group)
	}
}

func TestGetDependents(t *testing.T) {
	plan := createTestPlan()

	// task-1 is depended on by task-2 and task-4
	dependents := GetDependents(plan, "task-1")
	if len(dependents) != 2 {
		t.Errorf("expected 2 dependents for task-1, got %d", len(dependents))
	}

	// task-3 has no dependents
	dependents = GetDependents(plan, "task-3")
	if len(dependents) != 0 {
		t.Errorf("expected 0 dependents for task-3, got %d", len(dependents))
	}
}

func TestHasCircularDependency(t *testing.T) {
	plan := createTestPlan()

	// Self-dependency
	if !HasCircularDependency(plan, "task-1", "task-1") {
		t.Error("self-dependency should be detected as circular")
	}

	// task-1 -> task-2 -> task-3
	// Adding task-1 depending on task-3 would create a cycle
	if !HasCircularDependency(plan, "task-1", "task-3") {
		t.Error("cycle task-1 -> task-3 -> task-2 -> task-1 should be detected")
	}

	// Adding task-4 depending on task-3 should NOT create a cycle
	// (task-4 depends on task-1, task-3 depends on task-2 which depends on task-1)
	if HasCircularDependency(plan, "task-4", "task-3") {
		t.Error("task-4 -> task-3 should not create a cycle")
	}

	// Nil plan
	if !HasCircularDependency(nil, "task-1", "task-2") {
		t.Error("nil plan should return true (circular)")
	}
}

func TestRecalculatePlanAfterMutations(t *testing.T) {
	plan := createTestPlan()

	// Make multiple mutations
	UpdateTaskTitle(plan, "task-1", "Updated Setup")
	UpdateTaskPriority(plan, "task-2", 10)
	UpdateTaskDependencies(plan, "task-4", []string{"task-2"})

	// Plan should still be valid
	if err := ValidatePlan(plan); err != nil {
		t.Errorf("plan should be valid after mutations: %v", err)
	}

	// Execution order should reflect the new dependencies
	group := GetExecutionGroupForTask(plan, "task-4")
	if group <= GetExecutionGroupForTask(plan, "task-2") {
		t.Error("task-4 should be in a later group than task-2 after dependency change")
	}
}

func TestErrorTypes(t *testing.T) {
	// Test ErrTaskNotFound
	err := ErrTaskNotFound{TaskID: "test-id"}
	if err.Error() != "task not found: test-id" {
		t.Errorf("unexpected error message: %s", err.Error())
	}

	// Test ErrInvalidDependency
	err2 := ErrInvalidDependency{TaskID: "task-1", DependencyID: "task-2", Reason: "test reason"}
	expected := "invalid dependency task-2 for task task-1: test reason"
	if err2.Error() != expected {
		t.Errorf("unexpected error message: %s", err2.Error())
	}

	// Test ErrCannotMerge
	err3 := ErrCannotMerge{Reason: "test reason"}
	if err3.Error() != "cannot merge tasks: test reason" {
		t.Errorf("unexpected error message: %s", err3.Error())
	}

	// Test ErrCannotSplit
	err4 := ErrCannotSplit{TaskID: "task-1", Reason: "test reason"}
	if err4.Error() != "cannot split task task-1: test reason" {
		t.Errorf("unexpected error message: %s", err4.Error())
	}
}

// Tests for ValidatePlanForEditor and related validation functions

func TestValidatePlanForEditor_ValidPlan(t *testing.T) {
	plan := createTestPlan()

	result := ValidatePlanForEditor(plan)

	// Valid plan should have no errors
	if result.HasErrors() {
		t.Errorf("expected no errors, got %d errors", result.ErrorCount)
		for _, msg := range result.GetMessagesBySeverity(SeverityError) {
			t.Logf("  Error: %s", msg.Message)
		}
	}

	if !result.IsValid {
		t.Error("expected IsValid to be true")
	}
}

func TestValidatePlanForEditor_NilPlan(t *testing.T) {
	result := ValidatePlanForEditor(nil)

	if !result.HasErrors() {
		t.Error("expected errors for nil plan")
	}

	if result.IsValid {
		t.Error("expected IsValid to be false for nil plan")
	}
}

func TestValidatePlanForEditor_EmptyPlan(t *testing.T) {
	plan := &PlanSpec{
		ID:    "empty-plan",
		Tasks: []PlannedTask{},
	}

	result := ValidatePlanForEditor(plan)

	if !result.HasErrors() {
		t.Error("expected errors for empty plan")
	}

	if result.IsValid {
		t.Error("expected IsValid to be false for empty plan")
	}
}

func TestValidatePlanForEditor_MissingDescription(t *testing.T) {
	plan := &PlanSpec{
		ID: "test-plan",
		Tasks: []PlannedTask{
			{
				ID:          "task-1",
				Title:       "Test Task",
				Description: "", // Missing description
			},
		},
	}
	plan.ExecutionOrder = [][]string{{"task-1"}}

	result := ValidatePlanForEditor(plan)

	// Should have a warning for missing description
	if !result.HasWarnings() {
		t.Error("expected warning for missing description")
	}

	// Should still be valid (warnings allowed)
	if !result.IsValid {
		t.Error("expected IsValid to be true (missing description is just a warning)")
	}

	// Check the warning message
	found := false
	for _, msg := range result.Messages {
		if msg.TaskID == "task-1" && msg.Field == "description" && msg.Severity == SeverityWarning {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning message for task-1 description field")
	}
}

func TestValidatePlanForEditor_SelfDependency(t *testing.T) {
	plan := &PlanSpec{
		ID: "test-plan",
		Tasks: []PlannedTask{
			{
				ID:          "task-1",
				Title:       "Test Task",
				Description: "A task",
				DependsOn:   []string{"task-1"}, // Self-dependency
			},
		},
	}

	result := ValidatePlanForEditor(plan)

	if !result.HasErrors() {
		t.Error("expected error for self-dependency")
	}

	if result.IsValid {
		t.Error("expected IsValid to be false for self-dependency")
	}

	// Check the error message
	found := false
	for _, msg := range result.Messages {
		if msg.TaskID == "task-1" && msg.Severity == SeverityError && msg.Field == "depends_on" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error message for self-dependency")
	}
}

func TestValidatePlanForEditor_InvalidDependency(t *testing.T) {
	plan := &PlanSpec{
		ID: "test-plan",
		Tasks: []PlannedTask{
			{
				ID:          "task-1",
				Title:       "Test Task",
				Description: "A task",
				DependsOn:   []string{"nonexistent-task"}, // Invalid reference
			},
		},
	}

	result := ValidatePlanForEditor(plan)

	if !result.HasErrors() {
		t.Error("expected error for invalid dependency")
	}

	if result.IsValid {
		t.Error("expected IsValid to be false for invalid dependency")
	}
}

func TestValidatePlanForEditor_DependencyCycle(t *testing.T) {
	plan := &PlanSpec{
		ID: "test-plan",
		Tasks: []PlannedTask{
			{
				ID:          "task-1",
				Title:       "Task 1",
				Description: "A task",
				DependsOn:   []string{"task-2"},
			},
			{
				ID:          "task-2",
				Title:       "Task 2",
				Description: "Another task",
				DependsOn:   []string{"task-1"}, // Creates cycle
			},
		},
		DependencyGraph: map[string][]string{
			"task-1": {"task-2"},
			"task-2": {"task-1"},
		},
	}
	// Don't set ExecutionOrder - it should be incomplete due to cycle

	result := ValidatePlanForEditor(plan)

	if !result.HasErrors() {
		t.Error("expected error for dependency cycle")
	}

	if result.IsValid {
		t.Error("expected IsValid to be false for dependency cycle")
	}

	// Should have a cycle-related error message
	found := false
	for _, msg := range result.Messages {
		if msg.Severity == SeverityError && len(msg.RelatedIDs) >= 2 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected cycle error message with related IDs")
	}
}

func TestValidatePlanForEditor_HighComplexityWarning(t *testing.T) {
	plan := &PlanSpec{
		ID: "test-plan",
		Tasks: []PlannedTask{
			{
				ID:            "task-1",
				Title:         "Complex Task",
				Description:   "A complex task",
				EstComplexity: ComplexityHigh,
			},
		},
	}
	plan.ExecutionOrder = [][]string{{"task-1"}}

	result := ValidatePlanForEditor(plan)

	if !result.HasWarnings() {
		t.Error("expected warning for high complexity task")
	}

	// Should still be valid (warnings allowed)
	if !result.IsValid {
		t.Error("expected IsValid to be true (high complexity is just a warning)")
	}
}

func TestValidatePlanForEditor_FileConflict(t *testing.T) {
	plan := &PlanSpec{
		ID: "test-plan",
		Tasks: []PlannedTask{
			{
				ID:          "task-1",
				Title:       "Task 1",
				Description: "A task",
				Files:       []string{"main.go"},
				DependsOn:   []string{},
			},
			{
				ID:          "task-2",
				Title:       "Task 2",
				Description: "Another task",
				Files:       []string{"main.go"}, // Same file, parallel task
				DependsOn:   []string{},          // No dependency - can run in parallel
			},
		},
	}
	plan.ExecutionOrder = [][]string{{"task-1", "task-2"}} // Both in same group

	result := ValidatePlanForEditor(plan)

	if !result.HasWarnings() {
		t.Error("expected warning for file conflict")
	}

	// Should still be valid (file conflicts are warnings)
	if !result.IsValid {
		t.Error("expected IsValid to be true (file conflicts are just warnings)")
	}
}

func TestValidatePlanForEditor_FileConflictWithDependency(t *testing.T) {
	plan := &PlanSpec{
		ID: "test-plan",
		Tasks: []PlannedTask{
			{
				ID:          "task-1",
				Title:       "Task 1",
				Description: "A task",
				Files:       []string{"main.go"},
				DependsOn:   []string{},
			},
			{
				ID:          "task-2",
				Title:       "Task 2",
				Description: "Another task",
				Files:       []string{"main.go"}, // Same file, but depends on task-1
				DependsOn:   []string{"task-1"},  // Sequential - OK
			},
		},
	}
	plan.ExecutionOrder = [][]string{{"task-1"}, {"task-2"}}

	result := ValidatePlanForEditor(plan)

	// Should NOT have a warning for file conflict when tasks are in dependency chain
	hasFileConflictWarning := false
	for _, msg := range result.Messages {
		if msg.Field == "files" && msg.Severity == SeverityWarning {
			hasFileConflictWarning = true
			break
		}
	}
	if hasFileConflictWarning {
		t.Error("should not warn about file conflict when tasks have dependency relationship")
	}
}

func TestGetTasksInCycle(t *testing.T) {
	plan := &PlanSpec{
		ID: "test-plan",
		Tasks: []PlannedTask{
			{ID: "task-1", DependsOn: []string{"task-2"}},
			{ID: "task-2", DependsOn: []string{"task-1"}},
			{ID: "task-3", DependsOn: []string{}}, // Not in cycle
		},
	}

	cycleTasks := GetTasksInCycle(plan)

	if len(cycleTasks) == 0 {
		t.Error("expected cycle tasks to be detected")
	}

	// Both task-1 and task-2 should be in the cycle
	hasTask1 := slices.Contains(cycleTasks, "task-1")
	hasTask2 := slices.Contains(cycleTasks, "task-2")

	if !hasTask1 || !hasTask2 {
		t.Errorf("expected task-1 and task-2 in cycle, got %v", cycleTasks)
	}
}

func TestGetTasksInCycle_NoCycle(t *testing.T) {
	plan := createTestPlan()

	cycleTasks := GetTasksInCycle(plan)

	if len(cycleTasks) != 0 {
		t.Errorf("expected no cycle tasks, got %v", cycleTasks)
	}
}

func TestValidationResultMethods(t *testing.T) {
	result := &ValidationResult{
		IsValid: true,
		Messages: []ValidationMessage{
			{Severity: SeverityError, TaskID: "task-1", Message: "Error 1"},
			{Severity: SeverityWarning, TaskID: "task-1", Message: "Warning 1"},
			{Severity: SeverityWarning, TaskID: "task-2", Message: "Warning 2"},
			{Severity: SeverityInfo, TaskID: "task-2", Message: "Info 1"},
		},
		ErrorCount:   1,
		WarningCount: 2,
		InfoCount:    1,
	}

	// Test GetMessagesForTask
	task1Msgs := result.GetMessagesForTask("task-1")
	if len(task1Msgs) != 2 {
		t.Errorf("expected 2 messages for task-1, got %d", len(task1Msgs))
	}

	task2Msgs := result.GetMessagesForTask("task-2")
	if len(task2Msgs) != 2 {
		t.Errorf("expected 2 messages for task-2, got %d", len(task2Msgs))
	}

	// Test GetMessagesBySeverity
	errorMsgs := result.GetMessagesBySeverity(SeverityError)
	if len(errorMsgs) != 1 {
		t.Errorf("expected 1 error message, got %d", len(errorMsgs))
	}

	warningMsgs := result.GetMessagesBySeverity(SeverityWarning)
	if len(warningMsgs) != 2 {
		t.Errorf("expected 2 warning messages, got %d", len(warningMsgs))
	}

	// Test HasErrors and HasWarnings
	if !result.HasErrors() {
		t.Error("expected HasErrors to return true")
	}

	if !result.HasWarnings() {
		t.Error("expected HasWarnings to return true")
	}
}

// Table-driven tests for edge cases in mutation operations

func TestUpdateTaskTitle_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		taskID      string
		newTitle    string
		wantErr     bool
		errType     any
		checkResult func(t *testing.T, plan *PlanSpec)
	}{
		{
			name:     "update existing task title",
			taskID:   "task-1",
			newTitle: "New Setup Title",
			wantErr:  false,
			checkResult: func(t *testing.T, plan *PlanSpec) {
				task := GetTaskByID(plan, "task-1")
				if task.Title != "New Setup Title" {
					t.Errorf("expected title 'New Setup Title', got '%s'", task.Title)
				}
			},
		},
		{
			name:     "update with empty title",
			taskID:   "task-1",
			newTitle: "",
			wantErr:  false,
			checkResult: func(t *testing.T, plan *PlanSpec) {
				task := GetTaskByID(plan, "task-1")
				if task.Title != "" {
					t.Errorf("expected empty title, got '%s'", task.Title)
				}
			},
		},
		{
			name:     "update with unicode title",
			taskID:   "task-2",
			newTitle: "任务二 - Task Two 🚀",
			wantErr:  false,
			checkResult: func(t *testing.T, plan *PlanSpec) {
				task := GetTaskByID(plan, "task-2")
				if task.Title != "任务二 - Task Two 🚀" {
					t.Errorf("expected unicode title, got '%s'", task.Title)
				}
			},
		},
		{
			name:    "update non-existent task",
			taskID:  "nonexistent",
			newTitle: "Title",
			wantErr: true,
			errType: ErrTaskNotFound{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := createTestPlan()
			err := UpdateTaskTitle(plan, tt.taskID, tt.newTitle)

			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateTaskTitle() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errType != nil {
				switch tt.errType.(type) {
				case ErrTaskNotFound:
					if _, ok := err.(ErrTaskNotFound); !ok {
						t.Errorf("expected ErrTaskNotFound, got %T", err)
					}
				}
			}

			if tt.checkResult != nil && !tt.wantErr {
				tt.checkResult(t, plan)
			}
		})
	}
}

func TestUpdateTaskDependencies_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		taskID      string
		deps        []string
		wantErr     bool
		errType     any
		checkResult func(t *testing.T, plan *PlanSpec)
	}{
		{
			name:    "add valid dependency",
			taskID:  "task-3",
			deps:    []string{"task-1", "task-2"},
			wantErr: false,
			checkResult: func(t *testing.T, plan *PlanSpec) {
				task := GetTaskByID(plan, "task-3")
				if len(task.DependsOn) != 2 {
					t.Errorf("expected 2 dependencies, got %d", len(task.DependsOn))
				}
			},
		},
		{
			name:    "clear all dependencies",
			taskID:  "task-3",
			deps:    []string{},
			wantErr: false,
			checkResult: func(t *testing.T, plan *PlanSpec) {
				task := GetTaskByID(plan, "task-3")
				if len(task.DependsOn) != 0 {
					t.Errorf("expected 0 dependencies, got %d", len(task.DependsOn))
				}
			},
		},
		{
			name:    "self-dependency error",
			taskID:  "task-1",
			deps:    []string{"task-1"},
			wantErr: true,
			errType: ErrInvalidDependency{},
		},
		{
			name:    "non-existent dependency error",
			taskID:  "task-1",
			deps:    []string{"ghost-task"},
			wantErr: true,
			errType: ErrInvalidDependency{},
		},
		{
			name:    "duplicate dependencies are allowed",
			taskID:  "task-3",
			deps:    []string{"task-1", "task-1"},
			wantErr: false,
			checkResult: func(t *testing.T, plan *PlanSpec) {
				task := GetTaskByID(plan, "task-3")
				// Duplicates are passed through - validation may flag this as a warning
				if len(task.DependsOn) != 2 {
					t.Errorf("expected 2 dependencies, got %d", len(task.DependsOn))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := createTestPlan()
			err := UpdateTaskDependencies(plan, tt.taskID, tt.deps)

			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateTaskDependencies() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errType != nil {
				switch tt.errType.(type) {
				case ErrInvalidDependency:
					if _, ok := err.(ErrInvalidDependency); !ok {
						t.Errorf("expected ErrInvalidDependency, got %T", err)
					}
				}
			}

			if tt.checkResult != nil && !tt.wantErr {
				tt.checkResult(t, plan)
			}
		})
	}
}

func TestCircularDependencyDetection_TableDriven(t *testing.T) {
	tests := []struct {
		name      string
		setup     func() *PlanSpec
		taskID    string
		newDepID  string
		wantCycle bool
	}{
		{
			name: "simple two-node cycle",
			setup: func() *PlanSpec {
				return &PlanSpec{
					Tasks: []PlannedTask{
						{ID: "A", DependsOn: []string{}},
						{ID: "B", DependsOn: []string{"A"}},
					},
				}
			},
			taskID:    "A",
			newDepID:  "B",
			wantCycle: true, // A -> B -> A would create cycle
		},
		{
			name: "three-node cycle",
			setup: func() *PlanSpec {
				return &PlanSpec{
					Tasks: []PlannedTask{
						{ID: "A", DependsOn: []string{}},
						{ID: "B", DependsOn: []string{"A"}},
						{ID: "C", DependsOn: []string{"B"}},
					},
				}
			},
			taskID:    "A",
			newDepID:  "C",
			wantCycle: true, // A -> C -> B -> A would create cycle
		},
		{
			name: "no cycle - independent tasks",
			setup: func() *PlanSpec {
				return &PlanSpec{
					Tasks: []PlannedTask{
						{ID: "A", DependsOn: []string{}},
						{ID: "B", DependsOn: []string{}},
						{ID: "C", DependsOn: []string{}},
					},
				}
			},
			taskID:    "A",
			newDepID:  "B",
			wantCycle: false,
		},
		{
			name: "no cycle - valid DAG",
			setup: func() *PlanSpec {
				return &PlanSpec{
					Tasks: []PlannedTask{
						{ID: "A", DependsOn: []string{}},
						{ID: "B", DependsOn: []string{"A"}},
						{ID: "C", DependsOn: []string{"A"}},
						{ID: "D", DependsOn: []string{"B", "C"}},
					},
				}
			},
			taskID:    "D",
			newDepID:  "A",
			wantCycle: false, // D can depend on A directly (already transitive)
		},
		{
			name: "diamond dependency - no cycle",
			setup: func() *PlanSpec {
				return &PlanSpec{
					Tasks: []PlannedTask{
						{ID: "root", DependsOn: []string{}},
						{ID: "left", DependsOn: []string{"root"}},
						{ID: "right", DependsOn: []string{"root"}},
						{ID: "bottom", DependsOn: []string{"left", "right"}},
					},
				}
			},
			taskID:    "bottom",
			newDepID:  "root",
			wantCycle: false, // Adding explicit dep doesn't create cycle
		},
		{
			name: "self-dependency",
			setup: func() *PlanSpec {
				return &PlanSpec{
					Tasks: []PlannedTask{
						{ID: "A", DependsOn: []string{}},
					},
				}
			},
			taskID:    "A",
			newDepID:  "A",
			wantCycle: true, // Self-dependency is always a cycle
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := tt.setup()
			hasCycle := HasCircularDependency(plan, tt.taskID, tt.newDepID)

			if hasCycle != tt.wantCycle {
				t.Errorf("HasCircularDependency(%s, %s) = %v, want %v",
					tt.taskID, tt.newDepID, hasCycle, tt.wantCycle)
			}
		})
	}
}

func TestExecutionOrderAfterMutations_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		mutations   func(plan *PlanSpec) error
		checkResult func(t *testing.T, plan *PlanSpec)
	}{
		{
			name: "delete task updates execution order",
			mutations: func(plan *PlanSpec) error {
				return DeleteTask(plan, "task-2")
			},
			checkResult: func(t *testing.T, plan *PlanSpec) {
				// task-3 should now be earlier (since its dependency task-2 is gone)
				group := GetExecutionGroupForTask(plan, "task-3")
				if group > 1 {
					t.Errorf("task-3 should be in earlier group after dependency removed, got %d", group)
				}
			},
		},
		{
			name: "add dependency moves task later",
			mutations: func(plan *PlanSpec) error {
				return UpdateTaskDependencies(plan, "task-4", []string{"task-3"})
			},
			checkResult: func(t *testing.T, plan *PlanSpec) {
				group3 := GetExecutionGroupForTask(plan, "task-3")
				group4 := GetExecutionGroupForTask(plan, "task-4")
				if group4 <= group3 {
					t.Errorf("task-4 should be after task-3, got groups %d and %d", group4, group3)
				}
			},
		},
		{
			name: "remove dependency moves task earlier",
			mutations: func(plan *PlanSpec) error {
				return UpdateTaskDependencies(plan, "task-3", []string{})
			},
			checkResult: func(t *testing.T, plan *PlanSpec) {
				group := GetExecutionGroupForTask(plan, "task-3")
				if group != 0 {
					t.Errorf("task-3 with no deps should be in group 0, got %d", group)
				}
			},
		},
		{
			name: "adding task with dependency affects order",
			mutations: func(plan *PlanSpec) error {
				newTask := PlannedTask{
					ID:        "task-5",
					Title:     "New Task",
					DependsOn: []string{"task-3"},
				}
				return AddTask(plan, "", newTask)
			},
			checkResult: func(t *testing.T, plan *PlanSpec) {
				group3 := GetExecutionGroupForTask(plan, "task-3")
				group5 := GetExecutionGroupForTask(plan, "task-5")
				if group5 <= group3 {
					t.Errorf("task-5 should be after task-3, got groups %d and %d", group5, group3)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := createTestPlan()

			if err := tt.mutations(plan); err != nil {
				t.Fatalf("mutation failed: %v", err)
			}

			// Verify plan is still valid
			if err := ValidatePlan(plan); err != nil {
				t.Errorf("plan should be valid after mutation: %v", err)
			}

			tt.checkResult(t, plan)
		})
	}
}

func TestSplitTask_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		taskID      string
		description string
		splitPoints []int
		wantErr     bool
		wantParts   int
		checkResult func(t *testing.T, plan *PlanSpec, newIDs []string)
	}{
		{
			name:        "split into two parts",
			taskID:      "task-1",
			description: "First part. Second part.",
			splitPoints: []int{12},
			wantParts:   2,
			checkResult: func(t *testing.T, plan *PlanSpec, newIDs []string) {
				if len(newIDs) != 2 {
					t.Errorf("expected 2 new IDs, got %d", len(newIDs))
				}
				// First part keeps original ID
				if newIDs[0] != "task-1" {
					t.Errorf("expected first part to be 'task-1', got '%s'", newIDs[0])
				}
			},
		},
		{
			name:        "split into three parts",
			taskID:      "task-2",
			description: "Part A. Part B. Part C.",
			splitPoints: []int{8, 16},
			wantParts:   3,
			checkResult: func(t *testing.T, plan *PlanSpec, newIDs []string) {
				if len(newIDs) != 3 {
					t.Errorf("expected 3 new IDs, got %d", len(newIDs))
				}
			},
		},
		{
			name:        "split with out of bounds point",
			taskID:      "task-1",
			description: "Short",
			splitPoints: []int{100},
			wantErr:     true,
		},
		{
			name:        "split with zero point",
			taskID:      "task-1",
			description: "Some text",
			splitPoints: []int{0},
			wantErr:     true,
		},
		{
			name:        "split with no points",
			taskID:      "task-1",
			description: "Some text",
			splitPoints: []int{},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := createTestPlan()
			UpdateTaskDescription(plan, tt.taskID, tt.description)

			newIDs, err := SplitTask(plan, tt.taskID, tt.splitPoints)

			if (err != nil) != tt.wantErr {
				t.Errorf("SplitTask() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(newIDs) != tt.wantParts {
					t.Errorf("expected %d parts, got %d", tt.wantParts, len(newIDs))
				}

				if tt.checkResult != nil {
					tt.checkResult(t, plan, newIDs)
				}

				// Verify plan is valid after split
				if err := ValidatePlan(plan); err != nil {
					t.Errorf("plan should be valid after split: %v", err)
				}
			}
		})
	}
}

func TestMergeTasks_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		taskIDs     []string
		mergedTitle string
		wantErr     bool
		checkResult func(t *testing.T, plan *PlanSpec, mergedID string)
	}{
		{
			name:        "merge two sequential tasks",
			taskIDs:     []string{"task-2", "task-3"},
			mergedTitle: "Combined Task",
			wantErr:     false,
			checkResult: func(t *testing.T, plan *PlanSpec, mergedID string) {
				merged := GetTaskByID(plan, mergedID)
				if merged == nil {
					t.Fatal("merged task should exist")
				}
				if merged.Title != "Combined Task" {
					t.Errorf("expected title 'Combined Task', got '%s'", merged.Title)
				}
				// Original task-3 should be gone
				if GetTaskByID(plan, "task-3") != nil {
					t.Error("task-3 should have been merged away")
				}
			},
		},
		{
			name:        "merge tasks inherits highest complexity",
			taskIDs:     []string{"task-1", "task-2"},
			mergedTitle: "Merged Low+Medium",
			wantErr:     false,
			checkResult: func(t *testing.T, plan *PlanSpec, mergedID string) {
				merged := GetTaskByID(plan, mergedID)
				// task-1 is Low, task-2 is Medium - should get Medium
				if merged.EstComplexity != ComplexityMedium {
					t.Errorf("expected complexity medium, got %s", merged.EstComplexity)
				}
			},
		},
		{
			name:        "merge single task fails",
			taskIDs:     []string{"task-1"},
			mergedTitle: "Single",
			wantErr:     true,
		},
		{
			name:        "merge with non-existent task fails",
			taskIDs:     []string{"task-1", "ghost"},
			mergedTitle: "Ghost Merge",
			wantErr:     true,
		},
		{
			name:        "merge empty list fails",
			taskIDs:     []string{},
			mergedTitle: "Empty",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := createTestPlan()

			mergedID, err := MergeTasks(plan, tt.taskIDs, tt.mergedTitle)

			if (err != nil) != tt.wantErr {
				t.Errorf("MergeTasks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if tt.checkResult != nil {
					tt.checkResult(t, plan, mergedID)
				}

				// Verify plan is valid after merge
				if err := ValidatePlan(plan); err != nil {
					t.Errorf("plan should be valid after merge: %v", err)
				}
			}
		})
	}
}

func TestDependencyGraphRecalculation(t *testing.T) {
	plan := createTestPlan()

	// Verify initial dependency graph matches task dependencies
	for _, task := range plan.Tasks {
		graphDeps := plan.DependencyGraph[task.ID]
		if len(graphDeps) != len(task.DependsOn) {
			t.Errorf("initial graph mismatch for %s: graph has %d deps, task has %d",
				task.ID, len(graphDeps), len(task.DependsOn))
		}
	}

	// Modify dependencies
	UpdateTaskDependencies(plan, "task-4", []string{"task-2", "task-3"})

	// Verify graph was recalculated
	graphDeps := plan.DependencyGraph["task-4"]
	if len(graphDeps) != 2 {
		t.Errorf("expected 2 deps in graph after update, got %d", len(graphDeps))
	}

	// Verify execution order was recalculated
	group4 := GetExecutionGroupForTask(plan, "task-4")
	group3 := GetExecutionGroupForTask(plan, "task-3")
	if group4 <= group3 {
		t.Errorf("task-4 should be after task-3 in execution order")
	}
}

func TestGetAllDependencies(t *testing.T) {
	plan := createTestPlan()

	// task-3 depends on task-2, which depends on task-1
	allDeps := getAllDependencies(plan, "task-3")

	if !allDeps["task-2"] {
		t.Error("task-3 should have task-2 as dependency")
	}
	if !allDeps["task-1"] {
		t.Error("task-3 should have task-1 as transitive dependency")
	}

	// task-1 has no dependencies
	allDeps = getAllDependencies(plan, "task-1")
	if len(allDeps) != 0 {
		t.Errorf("task-1 should have no dependencies, got %d", len(allDeps))
	}
}

func TestAreTasksInDependencyChain(t *testing.T) {
	plan := createTestPlan()

	// task-1 -> task-2 -> task-3 are in a chain
	if !areTasksInDependencyChain(plan, []string{"task-1", "task-2", "task-3"}) {
		t.Error("task-1, task-2, task-3 should be in dependency chain")
	}

	// task-2 and task-4 both depend on task-1 but not on each other (parallel)
	if areTasksInDependencyChain(plan, []string{"task-2", "task-4"}) {
		t.Error("task-2 and task-4 are parallel, not in a chain")
	}

	// Single task is trivially in a chain
	if !areTasksInDependencyChain(plan, []string{"task-1"}) {
		t.Error("single task should be considered in a chain")
	}

	// Empty list is trivially in a chain
	if !areTasksInDependencyChain(plan, []string{}) {
		t.Error("empty list should be considered in a chain")
	}
}

func TestValidatePlanForEditor_MissingTitle(t *testing.T) {
	plan := &PlanSpec{
		ID: "test-plan",
		Tasks: []PlannedTask{
			{
				ID:          "task-1",
				Title:       "", // Missing title
				Description: "Has description",
			},
		},
	}
	plan.ExecutionOrder = [][]string{{"task-1"}}

	result := ValidatePlanForEditor(plan)

	if !result.HasWarnings() {
		t.Error("expected warning for missing title")
	}

	// Check that we have a title-related warning
	found := false
	for _, msg := range result.Messages {
		if msg.TaskID == "task-1" && msg.Field == "title" && msg.Severity == SeverityWarning {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning message for missing title")
	}
}

func TestFindTaskIndex(t *testing.T) {
	plan := createTestPlan()

	// Find existing tasks
	idx := findTaskIndex(plan, "task-1")
	if idx != 0 {
		t.Errorf("expected task-1 at index 0, got %d", idx)
	}

	idx = findTaskIndex(plan, "task-3")
	if idx != 2 {
		t.Errorf("expected task-3 at index 2, got %d", idx)
	}

	// Non-existent task
	idx = findTaskIndex(plan, "nonexistent")
	if idx != -1 {
		t.Errorf("expected -1 for nonexistent task, got %d", idx)
	}

	// Empty task list
	emptyPlan := &PlanSpec{Tasks: []PlannedTask{}}
	idx = findTaskIndex(emptyPlan, "task-1")
	if idx != -1 {
		t.Errorf("expected -1 for empty plan, got %d", idx)
	}
}
