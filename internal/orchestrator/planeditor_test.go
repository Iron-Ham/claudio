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
