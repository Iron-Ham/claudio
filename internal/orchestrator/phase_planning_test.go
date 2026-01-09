package orchestrator

import (
	"context"
	"testing"
	"time"
)

// ============================================================================
// Tests for PlanningPhase Initialization
// ============================================================================

func TestNewPlanningPhase(t *testing.T) {
	phase := NewPlanningPhase(nil, nil)

	if phase == nil {
		t.Fatal("NewPlanningPhase returned nil")
	}

	if phase.Name() != PhasePlanning {
		t.Errorf("Name() = %q, want %q", phase.Name(), PhasePlanning)
	}

	if phase.IsCompleted() {
		t.Error("new PlanningPhase should not be completed")
	}
}

func TestPlanningPhase_Name(t *testing.T) {
	phase := NewPlanningPhase(nil, nil)

	if got := phase.Name(); got != PhasePlanning {
		t.Errorf("Name() = %v, want %v", got, PhasePlanning)
	}
}

// ============================================================================
// Tests for CanExecute
// ============================================================================

func TestPlanningPhase_CanExecute(t *testing.T) {
	tests := []struct {
		name     string
		session  *UltraPlanSession
		expected bool
	}{
		{
			name:     "nil session",
			session:  nil,
			expected: false,
		},
		{
			name: "session with objective and no plan",
			session: &UltraPlanSession{
				Objective: "Test objective",
				Plan:      nil,
			},
			expected: true,
		},
		{
			name: "session with empty objective",
			session: &UltraPlanSession{
				Objective: "",
				Plan:      nil,
			},
			expected: false,
		},
		{
			name: "session with existing plan",
			session: &UltraPlanSession{
				Objective: "Test objective",
				Plan:      createSimpleTestPlan(),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase := NewPlanningPhase(nil, nil)
			ctx := context.Background()

			if got := phase.CanExecute(ctx, tt.session); got != tt.expected {
				t.Errorf("CanExecute() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// ============================================================================
// Tests for GetProgress
// ============================================================================

func TestPlanningPhase_GetProgress_Initial(t *testing.T) {
	phase := NewPlanningPhase(nil, nil)
	ctx := context.Background()

	progress := phase.GetProgress(ctx)

	if progress.Phase != PhasePlanning {
		t.Errorf("Phase = %v, want %v", progress.Phase, PhasePlanning)
	}

	if progress.Completed != 0 {
		t.Errorf("Completed = %v, want 0", progress.Completed)
	}

	if progress.Total != 1 {
		t.Errorf("Total = %v, want 1", progress.Total)
	}

	if progress.Message == "" {
		t.Error("Message should not be empty")
	}
}

// ============================================================================
// Tests for GetResult
// ============================================================================

func TestPlanningPhase_GetResult_BeforeCompletion(t *testing.T) {
	phase := NewPlanningPhase(nil, nil)
	ctx := context.Background()

	_, err := phase.GetResult(ctx)

	if err == nil {
		t.Error("expected error when getting result before completion")
	}
}

// ============================================================================
// Tests for SetPlan
// ============================================================================

func TestPlanningPhase_SetPlan_Valid(t *testing.T) {
	phase := NewPlanningPhase(nil, nil)
	session := createTestUltraPlanSession()
	plan := createSimpleTestPlan()

	err := phase.SetPlan(session, plan)

	if err != nil {
		t.Fatalf("SetPlan returned error: %v", err)
	}

	if session.Plan == nil {
		t.Error("session.Plan should be set")
	}

	if !phase.IsCompleted() {
		t.Error("phase should be completed after SetPlan")
	}

	ctx := context.Background()
	result, err := phase.GetResult(ctx)
	if err != nil {
		t.Fatalf("GetResult returned error: %v", err)
	}

	if !result.Success {
		t.Error("result should indicate success")
	}

	if result.Data == nil {
		t.Error("result.Data should contain the plan")
	}
}

func TestPlanningPhase_SetPlan_Invalid(t *testing.T) {
	phase := NewPlanningPhase(nil, nil)
	session := createTestUltraPlanSession()

	// nil plan
	err := phase.SetPlan(session, nil)
	if err == nil {
		t.Error("expected error for nil plan")
	}

	// plan with no tasks
	emptyPlan := &PlanSpec{
		ID:        "empty",
		Objective: "Test",
		Tasks:     []PlannedTask{},
	}
	err = phase.SetPlan(session, emptyPlan)
	if err == nil {
		t.Error("expected error for plan with no tasks")
	}
}

func TestPlanningPhase_SetPlan_WithCallbacks(t *testing.T) {
	phase := NewPlanningPhase(nil, nil)
	session := createTestUltraPlanSession()
	plan := createSimpleTestPlan()

	planReadyCalled := false
	callbacks := &CoordinatorCallbacks{
		OnPlanReady: func(p *PlanSpec) {
			planReadyCalled = true
			if p.ID != plan.ID {
				t.Errorf("callback received wrong plan ID: got %q, want %q", p.ID, plan.ID)
			}
		},
	}
	phase.SetCallbacks(callbacks)

	err := phase.SetPlan(session, plan)
	if err != nil {
		t.Fatalf("SetPlan returned error: %v", err)
	}

	if !planReadyCalled {
		t.Error("OnPlanReady callback should have been called")
	}
}

// ============================================================================
// Tests for Reset
// ============================================================================

func TestPlanningPhase_Reset(t *testing.T) {
	phase := NewPlanningPhase(nil, nil)
	session := createTestUltraPlanSession()
	plan := createSimpleTestPlan()

	// Complete the phase first
	err := phase.SetPlan(session, plan)
	if err != nil {
		t.Fatalf("SetPlan returned error: %v", err)
	}

	if !phase.IsCompleted() {
		t.Error("phase should be completed before reset")
	}

	// Reset
	phase.Reset()

	if phase.IsCompleted() {
		t.Error("phase should not be completed after reset")
	}

	ctx := context.Background()
	_, err = phase.GetResult(ctx)
	if err == nil {
		t.Error("GetResult should return error after reset")
	}

	progress := phase.GetProgress(ctx)
	if progress.Completed != 0 {
		t.Errorf("progress.Completed = %v, want 0", progress.Completed)
	}
}

// ============================================================================
// Tests for CalculateExecutionOrder
// ============================================================================

func TestCalculateExecutionOrder_SimpleChain(t *testing.T) {
	tasks := []PlannedTask{
		{ID: "task-1", DependsOn: []string{}},
		{ID: "task-2", DependsOn: []string{"task-1"}},
		{ID: "task-3", DependsOn: []string{"task-2"}},
	}
	deps := map[string][]string{
		"task-1": {},
		"task-2": {"task-1"},
		"task-3": {"task-2"},
	}

	order := CalculateExecutionOrder(tasks, deps)

	// Should have 3 groups (each task depends on the previous)
	if len(order) != 3 {
		t.Fatalf("expected 3 groups, got %d: %v", len(order), order)
	}

	// Verify order: task-1 first, then task-2, then task-3
	if len(order[0]) != 1 || order[0][0] != "task-1" {
		t.Errorf("first group should be [task-1], got %v", order[0])
	}
	if len(order[1]) != 1 || order[1][0] != "task-2" {
		t.Errorf("second group should be [task-2], got %v", order[1])
	}
	if len(order[2]) != 1 || order[2][0] != "task-3" {
		t.Errorf("third group should be [task-3], got %v", order[2])
	}
}

func TestCalculateExecutionOrder_ParallelTasks(t *testing.T) {
	tasks := []PlannedTask{
		{ID: "task-1", DependsOn: []string{}},
		{ID: "task-2", DependsOn: []string{}},
		{ID: "task-3", DependsOn: []string{}},
	}
	deps := map[string][]string{
		"task-1": {},
		"task-2": {},
		"task-3": {},
	}

	order := CalculateExecutionOrder(tasks, deps)

	// All tasks should be in one group (no dependencies)
	if len(order) != 1 {
		t.Fatalf("expected 1 group for parallel tasks, got %d: %v", len(order), order)
	}

	if len(order[0]) != 3 {
		t.Errorf("first group should have 3 tasks, got %d", len(order[0]))
	}
}

func TestCalculateExecutionOrder_DiamondDependency(t *testing.T) {
	// Diamond shape: A -> B, A -> C, B -> D, C -> D
	tasks := []PlannedTask{
		{ID: "A", DependsOn: []string{}},
		{ID: "B", DependsOn: []string{"A"}},
		{ID: "C", DependsOn: []string{"A"}},
		{ID: "D", DependsOn: []string{"B", "C"}},
	}
	deps := map[string][]string{
		"A": {},
		"B": {"A"},
		"C": {"A"},
		"D": {"B", "C"},
	}

	order := CalculateExecutionOrder(tasks, deps)

	// Should have 3 groups: [A], [B,C], [D]
	if len(order) != 3 {
		t.Fatalf("expected 3 groups for diamond, got %d: %v", len(order), order)
	}

	// First group: just A
	if len(order[0]) != 1 || order[0][0] != "A" {
		t.Errorf("first group should be [A], got %v", order[0])
	}

	// Second group: B and C (parallel)
	if len(order[1]) != 2 {
		t.Errorf("second group should have 2 tasks, got %d: %v", len(order[1]), order[1])
	}

	// Third group: D
	if len(order[2]) != 1 || order[2][0] != "D" {
		t.Errorf("third group should be [D], got %v", order[2])
	}
}

// ============================================================================
// Tests for ValidatePlanStructure
// ============================================================================

func TestValidatePlanStructure_ValidPlan(t *testing.T) {
	plan := createTestPlanWithDependencies()

	issues := ValidatePlanStructure(plan)

	// Should have no errors
	for _, issue := range issues {
		if issue.Severity == "error" {
			t.Errorf("unexpected error in valid plan: %s", issue.Message)
		}
	}
}

func TestValidatePlanStructure_NilPlan(t *testing.T) {
	issues := ValidatePlanStructure(nil)

	if len(issues) == 0 {
		t.Error("expected issues for nil plan")
	}

	hasError := false
	for _, issue := range issues {
		if issue.Severity == "error" {
			hasError = true
			break
		}
	}
	if !hasError {
		t.Error("expected at least one error for nil plan")
	}
}

func TestValidatePlanStructure_SelfDependency(t *testing.T) {
	plan := &PlanSpec{
		ID:        "test",
		Objective: "Test",
		Tasks: []PlannedTask{
			{
				ID:        "task-1",
				Title:     "Self-dependent",
				DependsOn: []string{"task-1"}, // Self-dependency!
			},
		},
		DependencyGraph: map[string][]string{"task-1": {"task-1"}},
		ExecutionOrder:  [][]string{{"task-1"}},
	}

	issues := ValidatePlanStructure(plan)

	hasSelfDepError := false
	for _, issue := range issues {
		if issue.TaskID == "task-1" && issue.Severity == "error" {
			hasSelfDepError = true
			break
		}
	}

	if !hasSelfDepError {
		t.Error("expected error for self-dependency")
	}
}

func TestValidatePlanStructure_EmptyDescription(t *testing.T) {
	plan := &PlanSpec{
		ID:        "test",
		Objective: "Test",
		Tasks: []PlannedTask{
			{
				ID:          "task-1",
				Title:       "Task with empty description",
				Description: "",
			},
		},
		DependencyGraph: map[string][]string{"task-1": {}},
		ExecutionOrder:  [][]string{{"task-1"}},
	}

	issues := ValidatePlanStructure(plan)

	hasWarning := false
	for _, issue := range issues {
		if issue.Severity == "warning" && issue.TaskID == "task-1" {
			hasWarning = true
			break
		}
	}

	if !hasWarning {
		t.Error("expected warning for empty description")
	}
}

func TestValidatePlanStructure_FileOwnershipConflict(t *testing.T) {
	plan := &PlanSpec{
		ID:        "test",
		Objective: "Test",
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Task 1", Description: "Desc", Files: []string{"shared.go"}},
			{ID: "task-2", Title: "Task 2", Description: "Desc", Files: []string{"shared.go"}},
		},
		DependencyGraph: map[string][]string{"task-1": {}, "task-2": {}},
		ExecutionOrder:  [][]string{{"task-1", "task-2"}}, // Same group = parallel
	}

	issues := ValidatePlanStructure(plan)

	hasConflictWarning := false
	for _, issue := range issues {
		if issue.Severity == "warning" {
			hasConflictWarning = true
			break
		}
	}

	if !hasConflictWarning {
		t.Error("expected warning for file ownership conflict in parallel tasks")
	}
}

// ============================================================================
// Tests for AnalyzeDependencies
// ============================================================================

func TestAnalyzeDependencies_ValidPlan(t *testing.T) {
	plan := createTestPlanWithDependencies()

	analysis := AnalyzeDependencies(plan)

	if analysis.TotalDependencies <= 0 {
		t.Error("expected dependencies to be counted")
	}

	if len(analysis.TasksWithNoDeps) == 0 {
		t.Error("expected some tasks with no dependencies")
	}

	// task-1 has no dependencies
	foundTask1 := false
	for _, taskID := range analysis.TasksWithNoDeps {
		if taskID == "task-1" {
			foundTask1 = true
			break
		}
	}
	if !foundTask1 {
		t.Error("task-1 should be in TasksWithNoDeps")
	}
}

func TestAnalyzeDependencies_NilPlan(t *testing.T) {
	analysis := AnalyzeDependencies(nil)

	if analysis.TotalDependencies != 0 {
		t.Errorf("TotalDependencies = %d, want 0 for nil plan", analysis.TotalDependencies)
	}
}

func TestAnalyzeDependencies_CriticalPath(t *testing.T) {
	plan := createTestPlanWithDependencies()

	analysis := AnalyzeDependencies(plan)

	// The critical path should exist and include some tasks
	if len(analysis.CriticalPath) == 0 {
		t.Error("expected non-empty critical path")
	}
}

// ============================================================================
// Tests for EstimateResources
// ============================================================================

func TestPlanningPhase_EstimateResources(t *testing.T) {
	phase := NewPlanningPhase(nil, nil)
	plan := createTestPlanWithDependencies()
	config := DefaultUltraPlanConfig()

	estimate := phase.EstimateResources(plan, config)

	if estimate.TotalTasks != len(plan.Tasks) {
		t.Errorf("TotalTasks = %d, want %d", estimate.TotalTasks, len(plan.Tasks))
	}

	if estimate.TotalGroups != len(plan.ExecutionOrder) {
		t.Errorf("TotalGroups = %d, want %d", estimate.TotalGroups, len(plan.ExecutionOrder))
	}

	if estimate.MaxParallel != config.MaxParallel {
		t.Errorf("MaxParallel = %d, want %d", estimate.MaxParallel, config.MaxParallel)
	}
}

func TestPlanningPhase_EstimateResources_NilPlan(t *testing.T) {
	phase := NewPlanningPhase(nil, nil)
	config := DefaultUltraPlanConfig()

	estimate := phase.EstimateResources(nil, config)

	if estimate.TotalTasks != 0 {
		t.Errorf("TotalTasks = %d, want 0 for nil plan", estimate.TotalTasks)
	}
}

// ============================================================================
// Tests for OptimizeParallelization
// ============================================================================

func TestPlanningPhase_OptimizeParallelization(t *testing.T) {
	phase := NewPlanningPhase(nil, nil)

	// Create a plan with a large parallel group
	tasks := []PlannedTask{
		{ID: "t1", DependsOn: []string{}},
		{ID: "t2", DependsOn: []string{}},
		{ID: "t3", DependsOn: []string{}},
		{ID: "t4", DependsOn: []string{}},
		{ID: "t5", DependsOn: []string{}},
	}
	plan := &PlanSpec{
		ID:              "test",
		Tasks:           tasks,
		ExecutionOrder:  [][]string{{"t1", "t2", "t3", "t4", "t5"}},
		DependencyGraph: make(map[string][]string),
	}

	// Config with MaxParallel = 2
	config := UltraPlanConfig{MaxParallel: 2}

	optimized := phase.optimizeParallelization(plan, config)

	// With 5 tasks and MaxParallel=2, we need 3 groups: [t1,t2], [t3,t4], [t5]
	if len(optimized.ExecutionOrder) != 3 {
		t.Errorf("expected 3 groups, got %d: %v", len(optimized.ExecutionOrder), optimized.ExecutionOrder)
	}

	// Verify no group exceeds MaxParallel
	for i, group := range optimized.ExecutionOrder {
		if len(group) > config.MaxParallel {
			t.Errorf("group %d has %d tasks, exceeds MaxParallel %d", i, len(group), config.MaxParallel)
		}
	}
}

func TestPlanningPhase_OptimizeParallelization_UnlimitedParallelism(t *testing.T) {
	phase := NewPlanningPhase(nil, nil)

	plan := &PlanSpec{
		ExecutionOrder: [][]string{{"t1", "t2", "t3", "t4", "t5"}},
	}

	// Config with MaxParallel = 0 (unlimited)
	config := UltraPlanConfig{MaxParallel: 0}

	optimized := phase.optimizeParallelization(plan, config)

	// Should not split the group
	if len(optimized.ExecutionOrder) != 1 {
		t.Errorf("expected 1 group for unlimited parallelism, got %d", len(optimized.ExecutionOrder))
	}
}

// ============================================================================
// Tests for tasksInSameGroup Helper
// ============================================================================

func TestTasksInSameGroup(t *testing.T) {
	executionOrder := [][]string{
		{"task-1", "task-2"},
		{"task-3"},
		{"task-4", "task-5"},
	}

	tests := []struct {
		name     string
		taskIDs  []string
		expected bool
	}{
		{
			name:     "two tasks in same group",
			taskIDs:  []string{"task-1", "task-2"},
			expected: true,
		},
		{
			name:     "tasks in different groups",
			taskIDs:  []string{"task-1", "task-3"},
			expected: false,
		},
		{
			name:     "single task",
			taskIDs:  []string{"task-3"},
			expected: false,
		},
		{
			name:     "three tasks, two in same group",
			taskIDs:  []string{"task-4", "task-5", "task-3"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tasksInSameGroup(tt.taskIDs, executionOrder)
			if result != tt.expected {
				t.Errorf("tasksInSameGroup(%v) = %v, want %v", tt.taskIDs, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// Tests for findCriticalPath Helper
// ============================================================================

func TestFindCriticalPath_LinearDependencies(t *testing.T) {
	tasks := []PlannedTask{
		{ID: "A"},
		{ID: "B", DependsOn: []string{"A"}},
		{ID: "C", DependsOn: []string{"B"}},
	}
	deps := map[string][]string{
		"A": {},
		"B": {"A"},
		"C": {"B"},
	}

	path := findCriticalPath(tasks, deps)

	// Critical path should be A -> B -> C (length 3)
	if len(path) != 3 {
		t.Errorf("expected critical path length 3, got %d: %v", len(path), path)
	}
}

func TestFindCriticalPath_ParallelTasks(t *testing.T) {
	tasks := []PlannedTask{
		{ID: "A"},
		{ID: "B"},
		{ID: "C"},
	}
	deps := map[string][]string{
		"A": {},
		"B": {},
		"C": {},
	}

	path := findCriticalPath(tasks, deps)

	// All independent, critical path is just 1 task
	if len(path) != 1 {
		t.Errorf("expected critical path length 1 for parallel tasks, got %d: %v", len(path), path)
	}
}

// ============================================================================
// Tests for PlanningPhase Thread Safety
// ============================================================================

func TestPlanningPhase_ThreadSafety(t *testing.T) {
	phase := NewPlanningPhase(nil, nil)
	ctx := context.Background()

	// Spawn multiple goroutines accessing phase state
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_ = phase.GetProgress(ctx)
			_ = phase.IsCompleted()
			_ = phase.GetPlanningInstanceID()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for goroutines")
		}
	}
}

// ============================================================================
// Tests for ValidationIssue Types
// ============================================================================

func TestValidationIssue_Severities(t *testing.T) {
	issues := []ValidationIssue{
		{Severity: "error", Message: "Critical issue"},
		{Severity: "warning", Message: "Potential issue"},
		{Severity: "info", Message: "Informational"},
	}

	// Verify all severities are distinct and meaningful
	severities := make(map[string]bool)
	for _, issue := range issues {
		if _, exists := severities[issue.Severity]; exists {
			// Multiple of same severity is fine
		}
		severities[issue.Severity] = true
	}

	if len(severities) != 3 {
		t.Errorf("expected 3 distinct severities, got %d", len(severities))
	}
}

// ============================================================================
// Tests for ResourceEstimate Types
// ============================================================================

func TestResourceEstimate_Fields(t *testing.T) {
	estimate := ResourceEstimate{
		TotalTasks:            10,
		TotalGroups:           3,
		MaxParallel:           4,
		PeakParallelism:       4,
		HighComplexityTasks:   2,
		MediumComplexityTasks: 5,
		LowComplexityTasks:    3,
		EstimatedTime:         "varies",
	}

	// Verify task counts add up
	totalByComplexity := estimate.HighComplexityTasks + estimate.MediumComplexityTasks + estimate.LowComplexityTasks
	if totalByComplexity != estimate.TotalTasks {
		t.Errorf("complexity counts (%d) don't match total tasks (%d)", totalByComplexity, estimate.TotalTasks)
	}
}

// ============================================================================
// Tests for DependencyAnalysis Types
// ============================================================================

func TestDependencyAnalysis_Fields(t *testing.T) {
	analysis := DependencyAnalysis{
		TotalDependencies:    5,
		TasksWithNoDeps:      []string{"task-1"},
		CriticalPath:         []string{"task-1", "task-2"},
		PotentialBottlenecks: []string{"task-1"},
		ParallelismRatio:     2.5,
	}

	if len(analysis.TasksWithNoDeps) == 0 {
		t.Error("TasksWithNoDeps should not be empty in this test")
	}

	if analysis.ParallelismRatio <= 0 {
		t.Error("ParallelismRatio should be positive")
	}
}
