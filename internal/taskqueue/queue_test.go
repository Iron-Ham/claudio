package taskqueue

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

func makePlan() *ultraplan.PlanSpec {
	return &ultraplan.PlanSpec{
		ID:        "test-plan",
		Objective: "test",
		Tasks: []ultraplan.PlannedTask{
			{
				ID:            "task-1",
				Title:         "First task",
				Description:   "Do first thing",
				Files:         []string{"a.go"},
				DependsOn:     []string{},
				Priority:      0,
				EstComplexity: ultraplan.ComplexityLow,
			},
			{
				ID:            "task-2",
				Title:         "Second task",
				Description:   "Do second thing",
				Files:         []string{"b.go"},
				DependsOn:     []string{"task-1"},
				Priority:      0,
				EstComplexity: ultraplan.ComplexityMedium,
			},
			{
				ID:            "task-3",
				Title:         "Third task",
				Description:   "Do third thing",
				DependsOn:     nil,
				Priority:      1,
				EstComplexity: ultraplan.ComplexityHigh,
			},
		},
		DependencyGraph: map[string][]string{
			"task-1": {},
			"task-2": {"task-1"},
			"task-3": {},
		},
	}
}

func TestNewFromPlan(t *testing.T) {
	plan := makePlan()
	q := NewFromPlan(plan)

	if len(q.tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(q.tasks))
	}
	if len(q.order) != 3 {
		t.Fatalf("expected 3 in order, got %d", len(q.order))
	}

	// All tasks should be pending
	for _, task := range q.tasks {
		if task.Status != TaskPending {
			t.Errorf("task %q status = %s, want pending", task.ID, task.Status)
		}
	}

	// task-1 should come before task-2 in order (dependency)
	idx := make(map[string]int)
	for i, id := range q.order {
		idx[id] = i
	}
	if idx["task-1"] > idx["task-2"] {
		t.Errorf("task-1 (idx %d) should come before task-2 (idx %d)", idx["task-1"], idx["task-2"])
	}

	// Check DependsOn is never nil
	for _, task := range q.tasks {
		if task.DependsOn == nil {
			t.Errorf("task %q DependsOn should not be nil", task.ID)
		}
	}

	// Check EstComplexity is preserved via embedded PlannedTask
	if q.tasks["task-1"].EstComplexity != ultraplan.ComplexityLow {
		t.Errorf("task-1 EstComplexity = %q, want %q", q.tasks["task-1"].EstComplexity, ultraplan.ComplexityLow)
	}

	// Check that PlannedTask fields are accessible via embedding
	if q.tasks["task-1"].Title != "First task" {
		t.Errorf("task-1 Title = %q, want %q", q.tasks["task-1"].Title, "First task")
	}
	if len(q.tasks["task-1"].Files) != 1 || q.tasks["task-1"].Files[0] != "a.go" {
		t.Errorf("task-1 Files = %v, want [a.go]", q.tasks["task-1"].Files)
	}

	// Claims map should be initialized and empty
	if q.claims == nil {
		t.Error("claims map should be initialized")
	}
	if len(q.claims) != 0 {
		t.Errorf("claims should be empty, got %d entries", len(q.claims))
	}
}

func TestClaimNext(t *testing.T) {
	plan := makePlan()
	q := NewFromPlan(plan)

	// First claim should get task-1 or task-3 (both have no deps, task-1 has priority 0)
	task, err := q.ClaimNext("inst-1")
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if task == nil {
		t.Fatal("ClaimNext returned nil, expected a task")
	}
	// task-1 has priority 0, task-3 has priority 1, so task-1 first
	if task.ID != "task-1" {
		t.Errorf("first claim = %q, want task-1", task.ID)
	}
	if task.Status != TaskClaimed {
		t.Errorf("claimed task status = %s, want claimed", task.Status)
	}
	if task.ClaimedBy != "inst-1" {
		t.Errorf("claimed task ClaimedBy = %q, want inst-1", task.ClaimedBy)
	}
	if task.ClaimedAt == nil {
		t.Error("claimed task ClaimedAt should not be nil")
	}

	// Claims map should be updated
	if q.claims["task-1"] != "inst-1" {
		t.Errorf("claims[task-1] = %q, want inst-1", q.claims["task-1"])
	}

	// Second claim should get task-3 (task-2 depends on task-1 which is only claimed)
	task2, err := q.ClaimNext("inst-2")
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if task2 == nil {
		t.Fatal("ClaimNext returned nil, expected task-3")
	}
	if task2.ID != "task-3" {
		t.Errorf("second claim = %q, want task-3", task2.ID)
	}

	// Third claim should return nil (task-2 blocked, others claimed)
	task3, err := q.ClaimNext("inst-3")
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if task3 != nil {
		t.Errorf("third claim = %q, want nil", task3.ID)
	}
}

func TestClaimNext_EmptyInstanceID(t *testing.T) {
	plan := makePlan()
	q := NewFromPlan(plan)

	_, err := q.ClaimNext("")
	if err == nil {
		t.Error("ClaimNext with empty instanceID should return error")
	}
}

func TestClaimNext_ConcurrentClaims(t *testing.T) {
	// Create a plan with many independent tasks
	plan := &ultraplan.PlanSpec{
		ID: "concurrent-test",
		Tasks: func() []ultraplan.PlannedTask {
			var tasks []ultraplan.PlannedTask
			for i := 0; i < 20; i++ {
				tasks = append(tasks, ultraplan.PlannedTask{
					ID:            fmt.Sprintf("task-%d", i),
					Title:         fmt.Sprintf("Task %d", i),
					DependsOn:     []string{},
					Priority:      i,
					EstComplexity: ultraplan.ComplexityLow,
				})
			}
			return tasks
		}(),
	}
	q := NewFromPlan(plan)

	const numGoroutines = 10
	var wg sync.WaitGroup
	claimed := make(chan string, 20)

	wg.Add(numGoroutines)
	for g := 0; g < numGoroutines; g++ {
		go func(instanceID string) {
			defer wg.Done()
			for {
				task, err := q.ClaimNext(instanceID)
				if err != nil {
					t.Errorf("ClaimNext error: %v", err)
					return
				}
				if task == nil {
					return // no more tasks
				}
				claimed <- task.ID
				_ = q.MarkRunning(task.ID)
				_, _ = q.Complete(task.ID)
			}
		}(fmt.Sprintf("inst-%d", g))
	}

	wg.Wait()
	close(claimed)

	// Collect all claimed task IDs
	seen := make(map[string]bool)
	for id := range claimed {
		if seen[id] {
			t.Errorf("task %q was claimed more than once", id)
		}
		seen[id] = true
	}

	// All 20 tasks should have been claimed exactly once
	if len(seen) != 20 {
		t.Errorf("expected 20 unique claims, got %d", len(seen))
	}

	// Queue should be complete
	if !q.IsComplete() {
		t.Error("queue should be complete after all tasks claimed and completed")
	}
}

func TestClaimNext_ConcurrentWithDeps(t *testing.T) {
	// Diamond dependency: a -> b, a -> c, b+c -> d
	plan := &ultraplan.PlanSpec{
		ID: "concurrent-deps",
		Tasks: []ultraplan.PlannedTask{
			{ID: "a", DependsOn: []string{}, Priority: 0, EstComplexity: ultraplan.ComplexityLow},
			{ID: "b", DependsOn: []string{"a"}, Priority: 0, EstComplexity: ultraplan.ComplexityLow},
			{ID: "c", DependsOn: []string{"a"}, Priority: 1, EstComplexity: ultraplan.ComplexityLow},
			{ID: "d", DependsOn: []string{"b", "c"}, Priority: 0, EstComplexity: ultraplan.ComplexityLow},
		},
	}
	q := NewFromPlan(plan)

	const numGoroutines = 4
	var wg sync.WaitGroup
	claimed := make(chan string, 10)

	wg.Add(numGoroutines)
	for g := 0; g < numGoroutines; g++ {
		go func(instanceID string) {
			defer wg.Done()
			for {
				task, err := q.ClaimNext(instanceID)
				if err != nil {
					t.Errorf("ClaimNext error: %v", err)
					return
				}
				if task == nil {
					// Check if queue is complete
					if q.IsComplete() {
						return
					}
					// Might need to wait for deps
					time.Sleep(time.Millisecond)
					if q.IsComplete() {
						return
					}
					continue
				}
				claimed <- task.ID
				_ = q.MarkRunning(task.ID)
				_, _ = q.Complete(task.ID)
			}
		}(fmt.Sprintf("inst-%d", g))
	}

	wg.Wait()
	close(claimed)

	seen := make(map[string]bool)
	for id := range claimed {
		if seen[id] {
			t.Errorf("task %q was claimed more than once", id)
		}
		seen[id] = true
	}

	if len(seen) != 4 {
		t.Errorf("expected 4 unique claims, got %d", len(seen))
	}
	if !q.IsComplete() {
		t.Errorf("queue should be complete")
	}
}

func TestMarkRunning(t *testing.T) {
	plan := makePlan()
	q := NewFromPlan(plan)

	task, _ := q.ClaimNext("inst-1")

	if err := q.MarkRunning(task.ID); err != nil {
		t.Fatalf("MarkRunning: %v", err)
	}
	if q.tasks[task.ID].Status != TaskRunning {
		t.Errorf("status = %s, want running", q.tasks[task.ID].Status)
	}

	// Cannot mark running again
	if err := q.MarkRunning(task.ID); err == nil {
		t.Error("MarkRunning on running task should fail")
	}
}

func TestMarkRunning_NotFound(t *testing.T) {
	q := NewFromPlan(makePlan())
	if err := q.MarkRunning("nonexistent"); err == nil {
		t.Error("MarkRunning on nonexistent task should fail")
	}
}

func TestMarkRunning_PendingTask(t *testing.T) {
	q := NewFromPlan(makePlan())
	if err := q.MarkRunning("task-1"); err == nil {
		t.Error("MarkRunning on pending task should fail")
	}
}

func TestComplete(t *testing.T) {
	plan := makePlan()
	q := NewFromPlan(plan)

	// Claim and run task-1
	task, _ := q.ClaimNext("inst-1")
	_ = q.MarkRunning(task.ID)

	unblocked, err := q.Complete(task.ID)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if q.tasks[task.ID].Status != TaskCompleted {
		t.Errorf("status = %s, want completed", q.tasks[task.ID].Status)
	}
	if q.tasks[task.ID].CompletedAt == nil {
		t.Error("CompletedAt should not be nil")
	}

	// Should unblock task-2
	if len(unblocked) != 1 || unblocked[0] != "task-2" {
		t.Errorf("unblocked = %v, want [task-2]", unblocked)
	}
}

func TestComplete_FromClaimed(t *testing.T) {
	q := NewFromPlan(makePlan())
	_, _ = q.ClaimNext("inst-1") // claims task-1

	// Complete directly from claimed state (allowed)
	unblocked, err := q.Complete("task-1")
	if err != nil {
		t.Fatalf("Complete from claimed: %v", err)
	}
	if q.tasks["task-1"].Status != TaskCompleted {
		t.Errorf("status = %s, want completed", q.tasks["task-1"].Status)
	}
	if len(unblocked) != 1 || unblocked[0] != "task-2" {
		t.Errorf("unblocked = %v, want [task-2]", unblocked)
	}
}

func TestComplete_NotFound(t *testing.T) {
	q := NewFromPlan(makePlan())
	_, err := q.Complete("nonexistent")
	if err == nil {
		t.Error("Complete on nonexistent task should fail")
	}
}

func TestComplete_InvalidTransition(t *testing.T) {
	q := NewFromPlan(makePlan())
	// task-1 is pending
	_, err := q.Complete("task-1")
	if err == nil {
		t.Error("Complete on pending task should fail")
	}
}

func TestFail_WithRetries(t *testing.T) {
	plan := makePlan()
	q := NewFromPlan(plan)

	task, _ := q.ClaimNext("inst-1")
	_ = q.MarkRunning(task.ID)

	// First failure, retries remain
	if err := q.Fail(task.ID, "timeout"); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if q.tasks[task.ID].Status != TaskPending {
		t.Errorf("status = %s, want pending (retry)", q.tasks[task.ID].Status)
	}
	if q.tasks[task.ID].RetryCount != 1 {
		t.Errorf("RetryCount = %d, want 1", q.tasks[task.ID].RetryCount)
	}
	if q.tasks[task.ID].FailureContext != "timeout" {
		t.Errorf("FailureContext = %q, want timeout", q.tasks[task.ID].FailureContext)
	}
	if q.tasks[task.ID].ClaimedBy != "" {
		t.Error("ClaimedBy should be cleared on retry")
	}
	if q.tasks[task.ID].ClaimedAt != nil {
		t.Error("ClaimedAt should be nil on retry")
	}
	// Claims map should be cleared
	if _, ok := q.claims[task.ID]; ok {
		t.Error("claims map should not contain failed task on retry")
	}
}

func TestFail_ExhaustedRetries(t *testing.T) {
	plan := makePlan()
	q := NewFromPlan(plan)

	// Exhaust all retries (default 2)
	for i := 0; i <= defaultMaxRetries; i++ {
		task, _ := q.ClaimNext("inst-1")
		if task == nil {
			t.Fatalf("iteration %d: expected task, got nil", i)
		}
		_ = q.MarkRunning(task.ID)
		if err := q.Fail(task.ID, "fail"); err != nil {
			t.Fatalf("iteration %d Fail: %v", i, err)
		}
	}

	// After exhausting retries, task-1 should be permanently failed
	if q.tasks["task-1"].Status != TaskFailed {
		t.Errorf("status = %s, want failed", q.tasks["task-1"].Status)
	}
	if q.tasks["task-1"].CompletedAt == nil {
		t.Error("CompletedAt should be set when permanently failed")
	}
}

func TestFail_Redistribution(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID: "redistribution",
		Tasks: []ultraplan.PlannedTask{
			{ID: "a", DependsOn: []string{}, EstComplexity: ultraplan.ComplexityLow},
		},
	}
	q := NewFromPlan(plan)

	// Instance 1 claims and fails
	task, _ := q.ClaimNext("inst-1")
	_ = q.MarkRunning(task.ID)
	_ = q.Fail(task.ID, "instance died")

	// Different instance can now claim the task
	task2, err := q.ClaimNext("inst-2")
	if err != nil {
		t.Fatalf("ClaimNext after fail: %v", err)
	}
	if task2 == nil {
		t.Fatal("expected failed task to be re-claimable")
	}
	if task2.ID != "a" {
		t.Errorf("re-claimed task = %q, want a", task2.ID)
	}
	if task2.ClaimedBy != "inst-2" {
		t.Errorf("re-claimed by = %q, want inst-2", task2.ClaimedBy)
	}
	if task2.RetryCount != 1 {
		t.Errorf("RetryCount = %d, want 1", task2.RetryCount)
	}
}

func TestFail_NotFound(t *testing.T) {
	q := NewFromPlan(makePlan())
	if err := q.Fail("nonexistent", "err"); err == nil {
		t.Error("Fail on nonexistent task should fail")
	}
}

func TestFail_InvalidTransition(t *testing.T) {
	q := NewFromPlan(makePlan())
	if err := q.Fail("task-1", "err"); err == nil {
		t.Error("Fail on pending task should fail")
	}
}

func TestRelease(t *testing.T) {
	plan := makePlan()
	q := NewFromPlan(plan)

	task, _ := q.ClaimNext("inst-1")

	if err := q.Release(task.ID); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if q.tasks[task.ID].Status != TaskPending {
		t.Errorf("status = %s, want pending", q.tasks[task.ID].Status)
	}
	if q.tasks[task.ID].ClaimedBy != "" {
		t.Error("ClaimedBy should be cleared")
	}
	if q.tasks[task.ID].ClaimedAt != nil {
		t.Error("ClaimedAt should be nil")
	}
	// Claims map should be cleared
	if _, ok := q.claims[task.ID]; ok {
		t.Error("claims map should not contain released task")
	}
}

func TestRelease_Running(t *testing.T) {
	q := NewFromPlan(makePlan())
	task, _ := q.ClaimNext("inst-1")
	_ = q.MarkRunning(task.ID)

	if err := q.Release(task.ID); err != nil {
		t.Fatalf("Release running task: %v", err)
	}
	if q.tasks[task.ID].Status != TaskPending {
		t.Errorf("status = %s, want pending", q.tasks[task.ID].Status)
	}
}

func TestRelease_NotFound(t *testing.T) {
	q := NewFromPlan(makePlan())
	if err := q.Release("nonexistent"); err == nil {
		t.Error("Release on nonexistent task should fail")
	}
}

func TestRelease_InvalidTransition(t *testing.T) {
	q := NewFromPlan(makePlan())
	// task-1 is pending
	if err := q.Release("task-1"); err == nil {
		t.Error("Release on pending task should fail")
	}
}

func TestStatus(t *testing.T) {
	plan := makePlan()
	q := NewFromPlan(plan)

	s := q.Status()
	if s.Total != 3 {
		t.Errorf("Total = %d, want 3", s.Total)
	}
	if s.Pending != 3 {
		t.Errorf("Pending = %d, want 3", s.Pending)
	}

	// Claim one
	_, _ = q.ClaimNext("inst-1")
	s = q.Status()
	if s.Claimed != 1 {
		t.Errorf("Claimed = %d, want 1", s.Claimed)
	}
	if s.Pending != 2 {
		t.Errorf("Pending = %d, want 2", s.Pending)
	}

	// Run it
	_ = q.MarkRunning("task-1")
	s = q.Status()
	if s.Running != 1 {
		t.Errorf("Running = %d, want 1", s.Running)
	}
	if s.Claimed != 0 {
		t.Errorf("Claimed = %d, want 0", s.Claimed)
	}

	// Complete it
	_, _ = q.Complete("task-1")
	s = q.Status()
	if s.Completed != 1 {
		t.Errorf("Completed = %d, want 1", s.Completed)
	}
}

func TestIsComplete(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID: "test",
		Tasks: []ultraplan.PlannedTask{
			{ID: "a", DependsOn: []string{}, EstComplexity: ultraplan.ComplexityLow},
		},
	}
	q := NewFromPlan(plan)

	if q.IsComplete() {
		t.Error("IsComplete should be false with pending tasks")
	}

	task, _ := q.ClaimNext("inst-1")
	_ = q.MarkRunning(task.ID)
	_, _ = q.Complete(task.ID)

	if !q.IsComplete() {
		t.Error("IsComplete should be true when all tasks completed")
	}
}

func TestIsComplete_Empty(t *testing.T) {
	plan := &ultraplan.PlanSpec{ID: "test"}
	q := NewFromPlan(plan)

	// Empty queue should not be considered complete
	if q.IsComplete() {
		t.Error("IsComplete should be false for empty queue")
	}
}

func TestIsComplete_WithFailed(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID: "test",
		Tasks: []ultraplan.PlannedTask{
			{ID: "a", DependsOn: []string{}, EstComplexity: ultraplan.ComplexityLow},
		},
	}
	q := NewFromPlan(plan)
	q.tasks["a"].MaxRetries = 0

	task, _ := q.ClaimNext("inst-1")
	_ = q.MarkRunning(task.ID)
	_ = q.Fail(task.ID, "error")

	if !q.IsComplete() {
		t.Error("IsComplete should be true when all tasks are terminal (failed)")
	}
}

func TestGetTask(t *testing.T) {
	q := NewFromPlan(makePlan())

	task := q.GetTask("task-1")
	if task == nil {
		t.Fatal("GetTask returned nil")
	}
	if task.ID != "task-1" {
		t.Errorf("GetTask ID = %q, want task-1", task.ID)
	}
	if task.Title != "First task" {
		t.Errorf("GetTask Title = %q, want First task", task.Title)
	}

	// Returned value is a copy
	task.Title = "modified"
	original := q.GetTask("task-1")
	if original.Title == "modified" {
		t.Error("GetTask should return a copy, not a reference")
	}
}

func TestGetTask_NotFound(t *testing.T) {
	q := NewFromPlan(makePlan())
	if q.GetTask("nonexistent") != nil {
		t.Error("GetTask for nonexistent ID should return nil")
	}
}

func TestGetInstanceTasks(t *testing.T) {
	q := NewFromPlan(makePlan())

	// Claim task-1 for inst-1
	_, _ = q.ClaimNext("inst-1")
	// Claim task-3 for inst-1 as well
	_, _ = q.ClaimNext("inst-1")

	tasks := q.GetInstanceTasks("inst-1")
	if len(tasks) != 2 {
		t.Fatalf("GetInstanceTasks = %d tasks, want 2", len(tasks))
	}

	ids := make(map[string]bool)
	for _, task := range tasks {
		ids[task.ID] = true
	}
	if !ids["task-1"] || !ids["task-3"] {
		t.Errorf("GetInstanceTasks = %v, want task-1 and task-3", ids)
	}
}

func TestGetInstanceTasks_Empty(t *testing.T) {
	q := NewFromPlan(makePlan())
	tasks := q.GetInstanceTasks("inst-1")
	if len(tasks) != 0 {
		t.Errorf("GetInstanceTasks = %d tasks, want 0", len(tasks))
	}
}

func TestGetInstanceTasks_ReturnsCopies(t *testing.T) {
	q := NewFromPlan(makePlan())
	_, _ = q.ClaimNext("inst-1")

	tasks := q.GetInstanceTasks("inst-1")
	tasks[0].Title = "modified"

	original := q.GetTask("task-1")
	if original.Title == "modified" {
		t.Error("GetInstanceTasks should return copies")
	}
}

func TestFullWorkflow(t *testing.T) {
	plan := makePlan()
	q := NewFromPlan(plan)

	// Instance 1 claims task-1, Instance 2 claims task-3
	t1, _ := q.ClaimNext("inst-1")
	t3, _ := q.ClaimNext("inst-2")

	if t1.ID != "task-1" {
		t.Fatalf("expected task-1, got %s", t1.ID)
	}
	if t3.ID != "task-3" {
		t.Fatalf("expected task-3, got %s", t3.ID)
	}

	// Both start running
	_ = q.MarkRunning(t1.ID)
	_ = q.MarkRunning(t3.ID)

	// Nothing claimable yet (task-2 blocked on task-1)
	none, _ := q.ClaimNext("inst-3")
	if none != nil {
		t.Fatalf("expected nil, got %s", none.ID)
	}

	// Complete task-1, unblocking task-2
	unblocked, _ := q.Complete(t1.ID)
	if len(unblocked) != 1 || unblocked[0] != "task-2" {
		t.Errorf("unblocked = %v, want [task-2]", unblocked)
	}

	// Now inst-1 can claim task-2
	t2, _ := q.ClaimNext("inst-1")
	if t2.ID != "task-2" {
		t.Fatalf("expected task-2, got %s", t2.ID)
	}
	_ = q.MarkRunning(t2.ID)

	// Complete remaining tasks
	_, _ = q.Complete(t3.ID)
	_, _ = q.Complete(t2.ID)

	if !q.IsComplete() {
		t.Error("queue should be complete")
	}
	s := q.Status()
	if s.Completed != 3 {
		t.Errorf("Completed = %d, want 3", s.Completed)
	}
}

func TestClaimNext_TimestampIsSet(t *testing.T) {
	q := NewFromPlan(makePlan())
	before := time.Now()
	task, _ := q.ClaimNext("inst-1")
	after := time.Now()

	if task.ClaimedAt.Before(before) || task.ClaimedAt.After(after) {
		t.Errorf("ClaimedAt %v not in expected range [%v, %v]", task.ClaimedAt, before, after)
	}
}
