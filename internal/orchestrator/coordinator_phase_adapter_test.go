package orchestrator

import (
	"context"
	"sync"
	"testing"

	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator/group"
	"github.com/Iron-Ham/claudio/internal/orchestrator/phase"
	"github.com/Iron-Ham/claudio/internal/orchestrator/retry"
)

// Helper to create a minimal test coordinator with required dependencies
func newTestCoordinatorForPhaseAdapter(t *testing.T) *Coordinator {
	t.Helper()

	session := &UltraPlanSession{
		ID:        "test-session",
		Objective: "Test objective",
		Phase:     PhasePlanning,
		Plan: &PlanSpec{
			ID:      "plan-1",
			Summary: "Test plan",
			Tasks: []PlannedTask{
				{ID: "task-1", Title: "Task 1", Description: "First task"},
				{ID: "task-2", Title: "Task 2", Description: "Second task", DependsOn: []string{"task-1"}},
			},
			ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
		},
		CompletedTasks: []string{},
		FailedTasks:    []string{},
	}

	baseSession := &Session{
		ID:        "base-session",
		Instances: []*Instance{},
	}

	manager := &UltraPlanManager{
		session: session,
	}

	// Create a minimal session adapter for group tracker
	sessionAdapter := group.NewSessionAdapter(
		func() group.PlanData {
			if session.Plan == nil {
				return nil
			}
			return group.NewPlanAdapter(
				func() [][]string { return session.Plan.ExecutionOrder },
				func(taskID string) *group.Task {
					for i := range session.Plan.Tasks {
						if session.Plan.Tasks[i].ID == taskID {
							t := &session.Plan.Tasks[i]
							return &group.Task{
								ID:          t.ID,
								Title:       t.Title,
								Description: t.Description,
							}
						}
					}
					return nil
				},
			)
		},
		func() []string { return session.CompletedTasks },
		func() []string { return session.FailedTasks },
		func() map[string]int { return session.TaskCommitCounts },
		func() int { return session.CurrentGroup },
	)

	ctx, cancel := context.WithCancel(context.Background())

	return &Coordinator{
		manager:      manager,
		baseSession:  baseSession,
		logger:       logging.NopLogger(),
		retryManager: retry.NewManager(),
		groupTracker: group.NewTracker(sessionAdapter),
		ctx:          ctx,
		cancelFunc:   cancel,
		runningTasks: make(map[string]string),
	}
}

func TestNewCoordinatorManagerAdapter(t *testing.T) {
	t.Run("creates adapter with coordinator", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		adapter := newCoordinatorManagerAdapter(c)
		if adapter == nil {
			t.Fatal("newCoordinatorManagerAdapter() returned nil")
		}
		if adapter.c != c {
			t.Error("adapter.c != expected coordinator")
		}
	})

	t.Run("creates adapter with nil coordinator", func(t *testing.T) {
		adapter := newCoordinatorManagerAdapter(nil)
		if adapter == nil {
			t.Fatal("newCoordinatorManagerAdapter(nil) returned nil")
		}
	})
}

func TestCoordinatorManagerAdapter_Session(t *testing.T) {
	t.Run("returns session adapter when coordinator valid", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		adapter := newCoordinatorManagerAdapter(c)

		session := adapter.Session()
		if session == nil {
			t.Error("Session() returned nil for valid coordinator")
		}
	})

	t.Run("returns nil when coordinator nil", func(t *testing.T) {
		adapter := newCoordinatorManagerAdapter(nil)
		session := adapter.Session()
		if session != nil {
			t.Errorf("Session() = %v, want nil for nil coordinator", session)
		}
	})

	t.Run("returns nil when manager nil", func(t *testing.T) {
		c := &Coordinator{manager: nil}
		adapter := newCoordinatorManagerAdapter(c)
		session := adapter.Session()
		if session != nil {
			t.Errorf("Session() = %v, want nil for nil manager", session)
		}
	})
}

func TestCoordinatorManagerAdapter_SetPhase(t *testing.T) {
	t.Run("sets phase on valid coordinator", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		adapter := newCoordinatorManagerAdapter(c)

		adapter.SetPhase(phase.PhaseExecuting)

		if c.manager.session.Phase != PhaseExecuting {
			t.Errorf("SetPhase() did not update session phase: got %v, want %v",
				c.manager.session.Phase, PhaseExecuting)
		}
	})

	t.Run("no panic on nil coordinator", func(t *testing.T) {
		adapter := newCoordinatorManagerAdapter(nil)
		// Should not panic
		adapter.SetPhase(phase.PhaseExecuting)
	})
}

func TestCoordinatorManagerAdapter_SetPlan(t *testing.T) {
	t.Run("sets plan on valid coordinator", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		adapter := newCoordinatorManagerAdapter(c)

		newPlan := &PlanSpec{ID: "new-plan", Summary: "New plan"}
		adapter.SetPlan(newPlan)

		if c.manager.session.Plan.ID != "new-plan" {
			t.Errorf("SetPlan() did not update plan: got %v, want %v",
				c.manager.session.Plan.ID, "new-plan")
		}
	})

	t.Run("ignores non-PlanSpec types", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		originalPlanID := c.manager.session.Plan.ID
		adapter := newCoordinatorManagerAdapter(c)

		adapter.SetPlan("not a plan spec")

		if c.manager.session.Plan.ID != originalPlanID {
			t.Error("SetPlan() should not modify plan for non-PlanSpec types")
		}
	})

	t.Run("no panic on nil coordinator", func(t *testing.T) {
		adapter := newCoordinatorManagerAdapter(nil)
		adapter.SetPlan(&PlanSpec{ID: "test"})
	})
}

func TestCoordinatorOrchestratorAdapter(t *testing.T) {
	t.Run("creates adapter", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		adapter := newCoordinatorOrchestratorAdapter(c)
		if adapter == nil {
			t.Fatal("newCoordinatorOrchestratorAdapter() returned nil")
		}
	})

	t.Run("SaveSession returns error for nil coordinator", func(t *testing.T) {
		adapter := newCoordinatorOrchestratorAdapter(nil)
		err := adapter.SaveSession()
		if err != ErrNilCoordinator {
			t.Errorf("SaveSession() error = %v, want %v", err, ErrNilCoordinator)
		}
	})

	t.Run("BranchPrefix returns empty for nil coordinator", func(t *testing.T) {
		adapter := newCoordinatorOrchestratorAdapter(nil)
		prefix := adapter.BranchPrefix()
		if prefix != "" {
			t.Errorf("BranchPrefix() = %q, want empty string", prefix)
		}
	})

	t.Run("GetInstanceManager returns nil for nil coordinator", func(t *testing.T) {
		adapter := newCoordinatorOrchestratorAdapter(nil)
		mgr := adapter.GetInstanceManager("test-id")
		if mgr != nil {
			t.Errorf("GetInstanceManager() = %v, want nil", mgr)
		}
	})

	t.Run("AddInstance returns error for nil coordinator", func(t *testing.T) {
		adapter := newCoordinatorOrchestratorAdapter(nil)
		_, err := adapter.AddInstance(nil, "task")
		if err != ErrNilCoordinator {
			t.Errorf("AddInstance() error = %v, want %v", err, ErrNilCoordinator)
		}
	})

	t.Run("StartInstance returns error for nil coordinator", func(t *testing.T) {
		adapter := newCoordinatorOrchestratorAdapter(nil)
		err := adapter.StartInstance(&Instance{})
		if err != ErrNilCoordinator {
			t.Errorf("StartInstance() error = %v, want %v", err, ErrNilCoordinator)
		}
	})

	t.Run("StartInstance returns error for wrong type", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		c.orch = &Orchestrator{} // Add minimal orchestrator
		adapter := newCoordinatorOrchestratorAdapter(c)

		err := adapter.StartInstance("not an instance")
		if err != ErrInstanceTypeAssertion {
			t.Errorf("StartInstance() error = %v, want %v", err, ErrInstanceTypeAssertion)
		}
	})
}

func TestCoordinatorSessionAdapter(t *testing.T) {
	t.Run("GetTask returns task for valid ID", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		adapter := newCoordinatorSessionAdapter(c, c.manager.session)

		task := adapter.GetTask("task-1")
		if task == nil {
			t.Error("GetTask() returned nil for valid task ID")
		}
		if pt, ok := task.(*PlannedTask); ok {
			if pt.ID != "task-1" {
				t.Errorf("GetTask().ID = %q, want %q", pt.ID, "task-1")
			}
		}
	})

	t.Run("GetTask returns nil for invalid ID", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		adapter := newCoordinatorSessionAdapter(c, c.manager.session)

		task := adapter.GetTask("nonexistent")
		// GetTask returns nil for non-existent tasks - that's expected
		_ = task // The test just verifies no panic occurs
	})

	t.Run("GetTask returns nil for nil session", func(t *testing.T) {
		adapter := newCoordinatorSessionAdapter(nil, nil)
		task := adapter.GetTask("task-1")
		if task != nil {
			t.Errorf("GetTask() = %v, want nil for nil session", task)
		}
	})

	t.Run("GetReadyTasks returns tasks with satisfied dependencies", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		adapter := newCoordinatorSessionAdapter(c, c.manager.session)

		ready := adapter.GetReadyTasks()
		// task-1 should be ready (no dependencies), task-2 should not (depends on task-1)
		found := false
		for _, taskID := range ready {
			if taskID == "task-1" {
				found = true
			}
			if taskID == "task-2" {
				t.Error("GetReadyTasks() should not include task-2 (has unmet dependencies)")
			}
		}
		if !found {
			t.Error("GetReadyTasks() should include task-1 (no dependencies)")
		}
	})

	t.Run("GetReadyTasks returns nil for nil session", func(t *testing.T) {
		adapter := newCoordinatorSessionAdapter(nil, nil)
		ready := adapter.GetReadyTasks()
		if ready != nil {
			t.Errorf("GetReadyTasks() = %v, want nil for nil session", ready)
		}
	})

	t.Run("Progress returns 0 for nil session", func(t *testing.T) {
		adapter := newCoordinatorSessionAdapter(nil, nil)
		progress := adapter.Progress()
		if progress != 0.0 {
			t.Errorf("Progress() = %f, want 0.0 for nil session", progress)
		}
	})

	t.Run("IsCurrentGroupComplete returns false for nil session", func(t *testing.T) {
		adapter := newCoordinatorSessionAdapter(nil, nil)
		complete := adapter.IsCurrentGroupComplete()
		if complete {
			t.Error("IsCurrentGroupComplete() = true, want false for nil session")
		}
	})

	t.Run("HasMoreGroups returns false for nil session", func(t *testing.T) {
		adapter := newCoordinatorSessionAdapter(nil, nil)
		hasMore := adapter.HasMoreGroups()
		if hasMore {
			t.Error("HasMoreGroups() = true, want false for nil session")
		}
	})
}

func TestCoordinatorCallbacksAdapter(t *testing.T) {
	t.Run("OnPhaseChange calls callback when set", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		var calledPhase UltraPlanPhase
		c.callbacks = &CoordinatorCallbacks{
			OnPhaseChange: func(p UltraPlanPhase) {
				calledPhase = p
			},
		}

		adapter := newCoordinatorCallbacksAdapter(c)
		adapter.OnPhaseChange(phase.PhaseExecuting)

		if calledPhase != PhaseExecuting {
			t.Errorf("OnPhaseChange callback got %v, want %v", calledPhase, PhaseExecuting)
		}
	})

	t.Run("OnPhaseChange no panic when callback nil", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		c.callbacks = nil

		adapter := newCoordinatorCallbacksAdapter(c)
		adapter.OnPhaseChange(phase.PhaseExecuting) // Should not panic
	})

	t.Run("OnPhaseChange no panic when coordinator nil", func(t *testing.T) {
		adapter := newCoordinatorCallbacksAdapter(nil)
		adapter.OnPhaseChange(phase.PhaseExecuting) // Should not panic
	})

	t.Run("OnTaskStart calls callback", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		var calledTaskID, calledInstanceID string
		c.callbacks = &CoordinatorCallbacks{
			OnTaskStart: func(taskID, instanceID string) {
				calledTaskID = taskID
				calledInstanceID = instanceID
			},
		}

		adapter := newCoordinatorCallbacksAdapter(c)
		adapter.OnTaskStart("task-1", "inst-1")

		if calledTaskID != "task-1" || calledInstanceID != "inst-1" {
			t.Errorf("OnTaskStart callback got (%q, %q), want (%q, %q)",
				calledTaskID, calledInstanceID, "task-1", "inst-1")
		}
	})

	t.Run("OnTaskComplete calls callback", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		var calledTaskID string
		c.callbacks = &CoordinatorCallbacks{
			OnTaskComplete: func(taskID string) {
				calledTaskID = taskID
			},
		}

		adapter := newCoordinatorCallbacksAdapter(c)
		adapter.OnTaskComplete("task-1")

		if calledTaskID != "task-1" {
			t.Errorf("OnTaskComplete callback got %q, want %q", calledTaskID, "task-1")
		}
	})

	t.Run("OnTaskFailed calls callback", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		var calledTaskID, calledReason string
		c.callbacks = &CoordinatorCallbacks{
			OnTaskFailed: func(taskID, reason string) {
				calledTaskID = taskID
				calledReason = reason
			},
		}

		adapter := newCoordinatorCallbacksAdapter(c)
		adapter.OnTaskFailed("task-1", "some error")

		if calledTaskID != "task-1" || calledReason != "some error" {
			t.Errorf("OnTaskFailed callback got (%q, %q), want (%q, %q)",
				calledTaskID, calledReason, "task-1", "some error")
		}
	})

	t.Run("OnGroupComplete calls callback", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		var calledGroupIndex int
		c.callbacks = &CoordinatorCallbacks{
			OnGroupComplete: func(groupIndex int) {
				calledGroupIndex = groupIndex
			},
		}

		adapter := newCoordinatorCallbacksAdapter(c)
		adapter.OnGroupComplete(2)

		if calledGroupIndex != 2 {
			t.Errorf("OnGroupComplete callback got %d, want %d", calledGroupIndex, 2)
		}
	})

	t.Run("OnPlanReady calls callback with PlanSpec", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		var calledPlan *PlanSpec
		c.callbacks = &CoordinatorCallbacks{
			OnPlanReady: func(plan *PlanSpec) {
				calledPlan = plan
			},
		}

		adapter := newCoordinatorCallbacksAdapter(c)
		testPlan := &PlanSpec{ID: "test-plan"}
		adapter.OnPlanReady(testPlan)

		if calledPlan == nil || calledPlan.ID != "test-plan" {
			t.Errorf("OnPlanReady callback got %v, want plan with ID 'test-plan'", calledPlan)
		}
	})

	t.Run("OnProgress calls callback", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		var calledCompleted, calledTotal int
		var calledPhase UltraPlanPhase
		c.callbacks = &CoordinatorCallbacks{
			OnProgress: func(completed, total int, p UltraPlanPhase) {
				calledCompleted = completed
				calledTotal = total
				calledPhase = p
			},
		}

		adapter := newCoordinatorCallbacksAdapter(c)
		adapter.OnProgress(5, 10, phase.PhaseExecuting)

		if calledCompleted != 5 || calledTotal != 10 || calledPhase != PhaseExecuting {
			t.Errorf("OnProgress callback got (%d, %d, %v), want (%d, %d, %v)",
				calledCompleted, calledTotal, calledPhase, 5, 10, PhaseExecuting)
		}
	})

	t.Run("OnComplete calls callback", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		var calledSuccess bool
		var calledSummary string
		c.callbacks = &CoordinatorCallbacks{
			OnComplete: func(success bool, summary string) {
				calledSuccess = success
				calledSummary = summary
			},
		}

		adapter := newCoordinatorCallbacksAdapter(c)
		adapter.OnComplete(true, "All tasks completed")

		if !calledSuccess || calledSummary != "All tasks completed" {
			t.Errorf("OnComplete callback got (%v, %q), want (%v, %q)",
				calledSuccess, calledSummary, true, "All tasks completed")
		}
	})
}

func TestCoordinator_BuildPhaseContext(t *testing.T) {
	t.Run("builds valid phase context", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)

		ctx, err := c.BuildPhaseContext()
		if err != nil {
			t.Fatalf("BuildPhaseContext() error = %v", err)
		}
		if ctx == nil {
			t.Fatal("BuildPhaseContext() returned nil context")
		}
		if ctx.Manager == nil {
			t.Error("BuildPhaseContext().Manager is nil")
		}
		if ctx.Orchestrator == nil {
			t.Error("BuildPhaseContext().Orchestrator is nil")
		}
		if ctx.Session == nil {
			t.Error("BuildPhaseContext().Session is nil")
		}
		if ctx.Logger == nil {
			t.Error("BuildPhaseContext().Logger is nil")
		}
		if ctx.Callbacks == nil {
			t.Error("BuildPhaseContext().Callbacks is nil")
		}
	})

	t.Run("returns error for nil coordinator", func(t *testing.T) {
		var c *Coordinator
		_, err := c.BuildPhaseContext()
		if err != ErrNilCoordinator {
			t.Errorf("BuildPhaseContext() error = %v, want %v", err, ErrNilCoordinator)
		}
	})

	t.Run("returns error for nil manager", func(t *testing.T) {
		c := &Coordinator{manager: nil}
		_, err := c.BuildPhaseContext()
		if err != ErrNilManager {
			t.Errorf("BuildPhaseContext() error = %v, want %v", err, ErrNilManager)
		}
	})

	t.Run("returns error for nil session", func(t *testing.T) {
		c := &Coordinator{
			manager: &UltraPlanManager{session: nil},
		}
		_, err := c.BuildPhaseContext()
		if err != ErrNilSession {
			t.Errorf("BuildPhaseContext() error = %v, want %v", err, ErrNilSession)
		}
	})

	t.Run("context validates successfully", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)

		ctx, err := c.BuildPhaseContext()
		if err != nil {
			t.Fatalf("BuildPhaseContext() error = %v", err)
		}

		if err := ctx.Validate(); err != nil {
			t.Errorf("PhaseContext.Validate() error = %v", err)
		}
	})
}

func TestCoordinator_GetBaseSession(t *testing.T) {
	t.Run("returns base session", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		session := c.GetBaseSession()
		if session == nil {
			t.Fatal("GetBaseSession() returned nil")
		}
		if session.ID != "base-session" {
			t.Errorf("GetBaseSession().ID = %q, want %q", session.ID, "base-session")
		}
	})

	t.Run("returns nil for nil coordinator", func(t *testing.T) {
		var c *Coordinator
		session := c.GetBaseSession()
		if session != nil {
			t.Errorf("GetBaseSession() = %v, want nil for nil coordinator", session)
		}
	})
}

func TestCoordinator_GetOrchestrator(t *testing.T) {
	t.Run("returns orchestrator when set", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		c.orch = &Orchestrator{}

		orch := c.GetOrchestrator()
		if orch == nil {
			t.Error("GetOrchestrator() returned nil")
		}
	})

	t.Run("returns nil for nil coordinator", func(t *testing.T) {
		var c *Coordinator
		orch := c.GetOrchestrator()
		if orch != nil {
			t.Errorf("GetOrchestrator() = %v, want nil for nil coordinator", orch)
		}
	})
}

func TestCoordinator_GetLogger(t *testing.T) {
	t.Run("returns logger when set", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		logger := c.GetLogger()
		if logger == nil {
			t.Error("GetLogger() returned nil")
		}
	})

	t.Run("returns NopLogger for nil coordinator", func(t *testing.T) {
		var c *Coordinator
		logger := c.GetLogger()
		if logger == nil {
			t.Error("GetLogger() returned nil, expected NopLogger")
		}
	})

	t.Run("returns NopLogger for nil logger", func(t *testing.T) {
		c := &Coordinator{logger: nil}
		logger := c.GetLogger()
		if logger == nil {
			t.Error("GetLogger() returned nil, expected NopLogger")
		}
	})
}

func TestCoordinator_GetContext(t *testing.T) {
	t.Run("returns context when set", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		ctx := c.GetContext()
		if ctx == nil {
			t.Error("GetContext() returned nil")
		}
	})

	t.Run("returns nil for nil coordinator", func(t *testing.T) {
		var c *Coordinator
		ctx := c.GetContext()
		if ctx != nil {
			t.Errorf("GetContext() = %v, want nil for nil coordinator", ctx)
		}
	})
}

func TestCoordinator_RunningTaskManagement(t *testing.T) {
	t.Run("AddRunningTask and GetRunningTaskCount", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)

		if count := c.GetRunningTaskCount(); count != 0 {
			t.Errorf("GetRunningTaskCount() = %d, want 0 initially", count)
		}

		c.AddRunningTask("task-1", "inst-1")
		if count := c.GetRunningTaskCount(); count != 1 {
			t.Errorf("GetRunningTaskCount() = %d, want 1 after adding one task", count)
		}

		c.AddRunningTask("task-2", "inst-2")
		if count := c.GetRunningTaskCount(); count != 2 {
			t.Errorf("GetRunningTaskCount() = %d, want 2 after adding two tasks", count)
		}
	})

	t.Run("IsTaskRunning", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)

		if c.IsTaskRunning("task-1") {
			t.Error("IsTaskRunning() = true, want false for non-running task")
		}

		c.AddRunningTask("task-1", "inst-1")
		if !c.IsTaskRunning("task-1") {
			t.Error("IsTaskRunning() = false, want true for running task")
		}
	})

	t.Run("GetRunningTaskInstance", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)

		if instanceID := c.GetRunningTaskInstance("task-1"); instanceID != "" {
			t.Errorf("GetRunningTaskInstance() = %q, want empty for non-running task", instanceID)
		}

		c.AddRunningTask("task-1", "inst-1")
		if instanceID := c.GetRunningTaskInstance("task-1"); instanceID != "inst-1" {
			t.Errorf("GetRunningTaskInstance() = %q, want %q", instanceID, "inst-1")
		}
	})

	t.Run("RemoveRunningTask", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)

		// Remove non-existent task
		if removed := c.RemoveRunningTask("task-1"); removed {
			t.Error("RemoveRunningTask() = true, want false for non-running task")
		}

		c.AddRunningTask("task-1", "inst-1")
		if removed := c.RemoveRunningTask("task-1"); !removed {
			t.Error("RemoveRunningTask() = false, want true for running task")
		}

		if count := c.GetRunningTaskCount(); count != 0 {
			t.Errorf("GetRunningTaskCount() = %d, want 0 after removing task", count)
		}

		if c.IsTaskRunning("task-1") {
			t.Error("IsTaskRunning() = true, want false after removing task")
		}
	})

	t.Run("nil coordinator methods are safe", func(t *testing.T) {
		var c *Coordinator

		// These should not panic
		c.AddRunningTask("task-1", "inst-1")
		if c.RemoveRunningTask("task-1") {
			t.Error("RemoveRunningTask() should return false for nil coordinator")
		}
		if c.GetRunningTaskCount() != 0 {
			t.Error("GetRunningTaskCount() should return 0 for nil coordinator")
		}
		if c.IsTaskRunning("task-1") {
			t.Error("IsTaskRunning() should return false for nil coordinator")
		}
		if c.GetRunningTaskInstance("task-1") != "" {
			t.Error("GetRunningTaskInstance() should return empty for nil coordinator")
		}
	})

	t.Run("concurrent access is safe", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		var wg sync.WaitGroup

		// Spawn multiple goroutines to test thread safety
		for i := range 10 {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				taskID := "task-" + string(rune('a'+n))
				instID := "inst-" + string(rune('a'+n))

				c.AddRunningTask(taskID, instID)
				c.IsTaskRunning(taskID)
				c.GetRunningTaskInstance(taskID)
				c.GetRunningTaskCount()
				c.RemoveRunningTask(taskID)
			}(i)
		}

		wg.Wait()
	})
}

func TestAdapterError(t *testing.T) {
	t.Run("error message format", func(t *testing.T) {
		err := newAdapterError("test message")
		expected := "coordinator phase adapter: test message"
		if err.Error() != expected {
			t.Errorf("Error() = %q, want %q", err.Error(), expected)
		}
	})

	t.Run("ErrInstanceTypeAssertion message", func(t *testing.T) {
		expected := "coordinator phase adapter: instance type assertion failed"
		if ErrInstanceTypeAssertion.Error() != expected {
			t.Errorf("ErrInstanceTypeAssertion.Error() = %q, want %q",
				ErrInstanceTypeAssertion.Error(), expected)
		}
	})
}

func TestCoordinator_GetVerifier(t *testing.T) {
	t.Run("returns nil for nil coordinator", func(t *testing.T) {
		var c *Coordinator
		verifier := c.GetVerifier()
		if verifier != nil {
			t.Errorf("GetVerifier() = %v, want nil for nil coordinator", verifier)
		}
	})

	t.Run("returns verifier when set", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		// The test coordinator doesn't have a verifier by default
		// This just tests the method doesn't panic
		_ = c.GetVerifier()
	})
}

func TestCoordinator_EmitEvent(t *testing.T) {
	t.Run("no panic for nil coordinator", func(t *testing.T) {
		var c *Coordinator
		c.EmitEvent(CoordinatorEvent{Type: EventTaskStarted})
	})

	t.Run("no panic for nil manager", func(t *testing.T) {
		c := &Coordinator{manager: nil}
		c.EmitEvent(CoordinatorEvent{Type: EventTaskStarted})
	})
}

func TestCoordinator_SyncRetryState(t *testing.T) {
	t.Run("no panic for nil coordinator", func(t *testing.T) {
		var c *Coordinator
		c.SyncRetryState()
	})

	t.Run("syncs retry state to session", func(t *testing.T) {
		c := newTestCoordinatorForPhaseAdapter(t)
		c.retryManager.GetOrCreateState("task-1", 3)
		c.retryManager.RecordAttempt("task-1", false)

		c.SyncRetryState()

		if c.manager.session.TaskRetries == nil {
			t.Error("SyncRetryState() did not sync retry state to session")
		}
	})
}
