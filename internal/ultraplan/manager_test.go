package ultraplan

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	session := NewSession("Test objective", DefaultConfig())
	manager := NewManager(session, nil)

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.Session() != session {
		t.Error("Manager session mismatch")
	}

	if manager.IsStopped() {
		t.Error("New manager should not be stopped")
	}
}

func TestManager_SetPhase(t *testing.T) {
	session := NewSession("Test objective", DefaultConfig())
	manager := NewManager(session, nil)

	manager.SetPhase(PhaseExecuting)

	if manager.Session().Phase != PhaseExecuting {
		t.Errorf("Expected phase %s, got %s", PhaseExecuting, manager.Session().Phase)
	}
}

func TestManager_SetPlan(t *testing.T) {
	session := NewSession("Test objective", DefaultConfig())
	manager := NewManager(session, nil)

	plan := &PlanSpec{
		ID:        "test-plan",
		Objective: "Test objective",
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Task 1", Description: "Do task 1"},
			{ID: "task-2", Title: "Task 2", Description: "Do task 2", DependsOn: []string{"task-1"}},
		},
	}

	manager.SetPlan(plan)

	if manager.Session().Plan != plan {
		t.Error("Plan not set correctly")
	}
}

func TestManager_StoreCandidatePlan(t *testing.T) {
	session := NewSession("Test objective", DefaultConfig())
	session.PlanCoordinatorIDs = []string{"coord-1", "coord-2", "coord-3"}
	manager := NewManager(session, nil)

	plan1 := &PlanSpec{ID: "plan-1"}
	plan2 := &PlanSpec{ID: "plan-2"}

	count := manager.StoreCandidatePlan(0, plan1)
	if count != 1 {
		t.Errorf("Expected count 1, got %d", count)
	}

	count = manager.StoreCandidatePlan(1, plan2)
	if count != 2 {
		t.Errorf("Expected count 2, got %d", count)
	}

	// Store nil plan (coordinator failed)
	count = manager.StoreCandidatePlan(2, nil)
	if count != 2 {
		t.Errorf("Expected count 2 after nil plan, got %d", count)
	}

	if manager.CountCoordinatorsCompleted() != 3 {
		t.Errorf("Expected 3 coordinators completed, got %d", manager.CountCoordinatorsCompleted())
	}
}

func TestManager_TaskManagement(t *testing.T) {
	session := NewSession("Test objective", DefaultConfig())
	session.Plan = &PlanSpec{
		ID: "test-plan",
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Task 1", Description: "Do task 1"},
			{ID: "task-2", Title: "Task 2", Description: "Do task 2"},
		},
	}
	manager := NewManager(session, nil)

	// Assign task to instance
	manager.AssignTaskToInstance("task-1", "instance-1")

	if session.TaskToInstance["task-1"] != "instance-1" {
		t.Error("Task not assigned to instance")
	}

	// Mark task complete
	manager.MarkTaskComplete("task-1")

	if len(session.CompletedTasks) != 1 || session.CompletedTasks[0] != "task-1" {
		t.Error("Task not marked as completed")
	}

	if _, exists := session.TaskToInstance["task-1"]; exists {
		t.Error("Task should be removed from TaskToInstance after completion")
	}

	// Mark task failed
	manager.MarkTaskFailed("task-2", "test failure")

	if len(session.FailedTasks) != 1 || session.FailedTasks[0] != "task-2" {
		t.Error("Task not marked as failed")
	}
}

func TestManager_Stop(t *testing.T) {
	session := NewSession("Test objective", DefaultConfig())
	manager := NewManager(session, nil)

	if manager.IsStopped() {
		t.Error("Manager should not be stopped initially")
	}

	manager.Stop()

	if !manager.IsStopped() {
		t.Error("Manager should be stopped after Stop()")
	}
}
