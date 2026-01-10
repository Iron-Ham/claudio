package ultraplan

import (
	"testing"
)

func TestNewSession(t *testing.T) {
	config := DefaultConfig()
	session := NewSession("Test objective", config)

	if session == nil {
		t.Fatal("NewSession returned nil")
	}

	if session.Objective != "Test objective" {
		t.Errorf("Expected objective 'Test objective', got '%s'", session.Objective)
	}

	if session.Phase != PhasePlanning {
		t.Errorf("Expected phase %s, got %s", PhasePlanning, session.Phase)
	}

	if session.ID == "" {
		t.Error("Session ID should not be empty")
	}

	if session.TaskToInstance == nil {
		t.Error("TaskToInstance should be initialized")
	}
}

func TestSession_GetTask(t *testing.T) {
	session := NewSession("Test", DefaultConfig())
	session.Plan = &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Task 1"},
			{ID: "task-2", Title: "Task 2"},
		},
	}

	task := session.GetTask("task-1")
	if task == nil {
		t.Fatal("GetTask returned nil for existing task")
	}
	if task.ID != "task-1" {
		t.Errorf("Expected task ID 'task-1', got '%s'", task.ID)
	}

	task = session.GetTask("nonexistent")
	if task != nil {
		t.Error("GetTask should return nil for nonexistent task")
	}

	// Test with nil plan
	session.Plan = nil
	task = session.GetTask("task-1")
	if task != nil {
		t.Error("GetTask should return nil when plan is nil")
	}
}

func TestSession_IsTaskReady(t *testing.T) {
	session := NewSession("Test", DefaultConfig())
	session.Plan = &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Task 1", DependsOn: []string{}},
			{ID: "task-2", Title: "Task 2", DependsOn: []string{"task-1"}},
			{ID: "task-3", Title: "Task 3", DependsOn: []string{"task-1", "task-2"}},
		},
	}

	// Task with no dependencies should be ready
	if !session.IsTaskReady("task-1") {
		t.Error("task-1 should be ready (no dependencies)")
	}

	// Task with unfulfilled dependency should not be ready
	if session.IsTaskReady("task-2") {
		t.Error("task-2 should not be ready (task-1 not complete)")
	}

	// Complete task-1
	session.CompletedTasks = []string{"task-1"}

	if !session.IsTaskReady("task-2") {
		t.Error("task-2 should be ready (task-1 complete)")
	}

	// task-3 still not ready (needs task-2)
	if session.IsTaskReady("task-3") {
		t.Error("task-3 should not be ready (task-2 not complete)")
	}

	// Complete task-2
	session.CompletedTasks = []string{"task-1", "task-2"}

	if !session.IsTaskReady("task-3") {
		t.Error("task-3 should be ready (all deps complete)")
	}
}

func TestSession_GetReadyTasks(t *testing.T) {
	session := NewSession("Test", DefaultConfig())
	session.Plan = &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Task 1"},
			{ID: "task-2", Title: "Task 2"},
			{ID: "task-3", Title: "Task 3", DependsOn: []string{"task-1"}},
		},
		ExecutionOrder: [][]string{
			{"task-1", "task-2"},
			{"task-3"},
		},
	}

	// Initially, tasks in group 0 should be ready
	ready := session.GetReadyTasks()
	if len(ready) != 2 {
		t.Errorf("Expected 2 ready tasks, got %d", len(ready))
	}

	// Start task-1
	session.TaskToInstance["task-1"] = "instance-1"

	ready = session.GetReadyTasks()
	if len(ready) != 1 {
		t.Errorf("Expected 1 ready task, got %d", len(ready))
	}

	// Complete both tasks in group 0
	session.CompletedTasks = []string{"task-1", "task-2"}
	delete(session.TaskToInstance, "task-1")
	session.CurrentGroup = 1

	// Now task-3 from group 1 should be ready
	ready = session.GetReadyTasks()
	if len(ready) != 1 {
		t.Errorf("Expected 1 ready task in group 1, got %d", len(ready))
	}
	if len(ready) > 0 && ready[0] != "task-3" {
		t.Errorf("Expected task-3, got %s", ready[0])
	}
}

func TestSession_GroupCompletion(t *testing.T) {
	session := NewSession("Test", DefaultConfig())
	session.Plan = &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Task 1"},
			{ID: "task-2", Title: "Task 2"},
		},
		ExecutionOrder: [][]string{
			{"task-1", "task-2"},
		},
	}

	if session.IsCurrentGroupComplete() {
		t.Error("Group should not be complete initially")
	}

	session.CompletedTasks = []string{"task-1"}
	if session.IsCurrentGroupComplete() {
		t.Error("Group should not be complete with only task-1 done")
	}

	session.CompletedTasks = []string{"task-1", "task-2"}
	if !session.IsCurrentGroupComplete() {
		t.Error("Group should be complete")
	}

	advanced, prevGroup := session.AdvanceGroupIfComplete()
	if !advanced {
		t.Error("Group should advance")
	}
	if prevGroup != 0 {
		t.Errorf("Previous group should be 0, got %d", prevGroup)
	}
	if session.CurrentGroup != 1 {
		t.Errorf("Current group should be 1, got %d", session.CurrentGroup)
	}
}

func TestSession_HasMoreGroups(t *testing.T) {
	session := NewSession("Test", DefaultConfig())
	session.Plan = &PlanSpec{
		ExecutionOrder: [][]string{
			{"task-1"},
			{"task-2"},
		},
	}

	if !session.HasMoreGroups() {
		t.Error("Should have more groups at start")
	}

	session.CurrentGroup = 1
	if !session.HasMoreGroups() {
		t.Error("Should still have groups when on group 1")
	}

	session.CurrentGroup = 2
	if session.HasMoreGroups() {
		t.Error("Should not have more groups past end")
	}
}

func TestSession_Progress(t *testing.T) {
	session := NewSession("Test", DefaultConfig())
	session.Plan = &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1"},
			{ID: "task-2"},
			{ID: "task-3"},
			{ID: "task-4"},
		},
	}

	if session.Progress() != 0 {
		t.Error("Progress should be 0 initially")
	}

	session.CompletedTasks = []string{"task-1", "task-2"}
	progress := session.Progress()
	if progress != 50 {
		t.Errorf("Progress should be 50%%, got %v%%", progress)
	}

	session.CompletedTasks = []string{"task-1", "task-2", "task-3", "task-4"}
	if session.Progress() != 100 {
		t.Errorf("Progress should be 100%%, got %v%%", session.Progress())
	}
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()

	if id1 == "" {
		t.Error("GenerateID returned empty string")
	}

	if len(id1) != 8 {
		t.Errorf("Expected 8-char ID, got %d chars", len(id1))
	}

	if id1 == id2 {
		t.Error("GenerateID should return unique IDs")
	}
}
