package bridgewire

import (
	"context"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

func TestConvertPlan(t *testing.T) {
	t.Run("converts all fields", func(t *testing.T) {
		src := &orchestrator.PlanSpec{
			ID:        "plan-1",
			Objective: "build feature X",
			Summary:   "three tasks",
			Tasks: []orchestrator.PlannedTask{
				{
					ID:            "t1",
					Title:         "task one",
					Description:   "do thing one",
					Files:         []string{"a.go", "b.go"},
					DependsOn:     nil,
					Priority:      1,
					EstComplexity: orchestrator.ComplexityLow,
					IssueURL:      "https://example.com/1",
					NoCode:        false,
				},
				{
					ID:            "t2",
					Title:         "task two",
					Description:   "do thing two",
					Files:         []string{"c.go"},
					DependsOn:     []string{"t1"},
					Priority:      2,
					EstComplexity: orchestrator.ComplexityHigh,
					NoCode:        true,
				},
			},
			DependencyGraph: map[string][]string{
				"t2": {"t1"},
			},
			ExecutionOrder: [][]string{{"t1"}, {"t2"}},
			Insights:       []string{"insight 1"},
			Constraints:    []string{"constraint 1"},
			CreatedAt:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}

		got := convertPlan(src)

		if got.ID != "plan-1" {
			t.Errorf("ID = %q, want %q", got.ID, "plan-1")
		}
		if got.Objective != "build feature X" {
			t.Errorf("Objective = %q, want %q", got.Objective, "build feature X")
		}
		if got.Summary != "three tasks" {
			t.Errorf("Summary = %q, want %q", got.Summary, "three tasks")
		}
		if len(got.Tasks) != 2 {
			t.Fatalf("Tasks len = %d, want 2", len(got.Tasks))
		}

		task1 := got.Tasks[0]
		if task1.ID != "t1" {
			t.Errorf("Tasks[0].ID = %q, want %q", task1.ID, "t1")
		}
		if task1.Title != "task one" {
			t.Errorf("Tasks[0].Title = %q, want %q", task1.Title, "task one")
		}
		if task1.EstComplexity != ultraplan.TaskComplexity("low") {
			t.Errorf("Tasks[0].EstComplexity = %q, want %q", task1.EstComplexity, "low")
		}
		if task1.IssueURL != "https://example.com/1" {
			t.Errorf("Tasks[0].IssueURL = %q, want %q", task1.IssueURL, "https://example.com/1")
		}
		if task1.NoCode {
			t.Error("Tasks[0].NoCode = true, want false")
		}
		if len(task1.Files) != 2 {
			t.Errorf("Tasks[0].Files len = %d, want 2", len(task1.Files))
		}

		task2 := got.Tasks[1]
		if task2.ID != "t2" {
			t.Errorf("Tasks[1].ID = %q, want %q", task2.ID, "t2")
		}
		if !task2.NoCode {
			t.Error("Tasks[1].NoCode = false, want true")
		}
		if len(task2.DependsOn) != 1 || task2.DependsOn[0] != "t1" {
			t.Errorf("Tasks[1].DependsOn = %v, want [t1]", task2.DependsOn)
		}

		// Dependency graph
		if deps, ok := got.DependencyGraph["t2"]; !ok || len(deps) != 1 || deps[0] != "t1" {
			t.Errorf("DependencyGraph[t2] = %v, want [t1]", got.DependencyGraph["t2"])
		}

		// Execution order
		if len(got.ExecutionOrder) != 2 {
			t.Fatalf("ExecutionOrder len = %d, want 2", len(got.ExecutionOrder))
		}
		if len(got.ExecutionOrder[0]) != 1 || got.ExecutionOrder[0][0] != "t1" {
			t.Errorf("ExecutionOrder[0] = %v, want [t1]", got.ExecutionOrder[0])
		}

		if len(got.Insights) != 1 || got.Insights[0] != "insight 1" {
			t.Errorf("Insights = %v, want [insight 1]", got.Insights)
		}
		if len(got.Constraints) != 1 || got.Constraints[0] != "constraint 1" {
			t.Errorf("Constraints = %v, want [constraint 1]", got.Constraints)
		}
		if !got.CreatedAt.Equal(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("CreatedAt = %v, want 2025-01-01", got.CreatedAt)
		}
	})

	t.Run("handles empty plan", func(t *testing.T) {
		src := &orchestrator.PlanSpec{
			ID: "empty",
		}
		got := convertPlan(src)
		if got.ID != "empty" {
			t.Errorf("ID = %q, want %q", got.ID, "empty")
		}
		if len(got.Tasks) != 0 {
			t.Errorf("Tasks len = %d, want 0", len(got.Tasks))
		}
	})

	t.Run("deep copies slices", func(t *testing.T) {
		src := &orchestrator.PlanSpec{
			ID:             "copy-test",
			ExecutionOrder: [][]string{{"t1", "t2"}},
			Tasks: []orchestrator.PlannedTask{
				{ID: "t1", Files: []string{"a.go"}},
			},
		}
		got := convertPlan(src)

		// Mutating source should not affect converted plan
		src.ExecutionOrder[0][0] = "mutated"
		if got.ExecutionOrder[0][0] != "t1" {
			t.Error("convertPlan did not deep copy ExecutionOrder")
		}

		src.Tasks[0].Files[0] = "mutated.go"
		if got.Tasks[0].Files[0] != "a.go" {
			t.Error("convertPlan did not deep copy Files")
		}
	})
}

func TestNewPipelineRunner_Validation(t *testing.T) {
	t.Run("nil plan", func(t *testing.T) {
		_, err := NewPipelineRunner(PipelineRunnerConfig{
			Orch:    &orchestrator.Orchestrator{},
			Session: &orchestrator.Session{},
			Bus:     event.NewBus(),
		})
		if err == nil {
			t.Fatal("expected error for nil Plan")
		}
	})

	t.Run("nil bus", func(t *testing.T) {
		_, err := NewPipelineRunner(PipelineRunnerConfig{
			Orch:    &orchestrator.Orchestrator{},
			Session: &orchestrator.Session{},
			Plan:    &orchestrator.PlanSpec{ID: "p1"},
		})
		if err == nil {
			t.Fatal("expected error for nil Bus")
		}
	})

	t.Run("nil orch", func(t *testing.T) {
		_, err := NewPipelineRunner(PipelineRunnerConfig{
			Session: &orchestrator.Session{},
			Plan:    &orchestrator.PlanSpec{ID: "p1"},
			Bus:     event.NewBus(),
		})
		if err == nil {
			t.Fatal("expected error for nil Orch")
		}
	})

	t.Run("nil session", func(t *testing.T) {
		_, err := NewPipelineRunner(PipelineRunnerConfig{
			Orch: &orchestrator.Orchestrator{},
			Plan: &orchestrator.PlanSpec{ID: "p1"},
			Bus:  event.NewBus(),
		})
		if err == nil {
			t.Fatal("expected error for nil Session")
		}
	})
}

func TestNewPipelineRunner_CreatesSuccessfully(t *testing.T) {
	bus := event.NewBus()
	plan := &orchestrator.PlanSpec{
		ID:        "plan-1",
		Objective: "test",
		Tasks: []orchestrator.PlannedTask{
			{ID: "t1", Title: "task one", Description: "do it"},
		},
		ExecutionOrder: [][]string{{"t1"}},
	}

	runner, err := NewPipelineRunner(PipelineRunnerConfig{
		Orch:        &orchestrator.Orchestrator{},
		Session:     &orchestrator.Session{BaseRepo: t.TempDir()},
		Plan:        plan,
		Bus:         bus,
		MaxParallel: 2,
	})
	if err != nil {
		t.Fatalf("NewPipelineRunner() error = %v", err)
	}
	if runner == nil {
		t.Fatal("NewPipelineRunner() returned nil")
	}
	if runner.pipe == nil {
		t.Fatal("runner.pipe is nil")
	}
	if runner.exec == nil {
		t.Fatal("runner.exec is nil")
	}
}

func TestPipelineRunner_StopBeforeStart(t *testing.T) {
	bus := event.NewBus()
	plan := &orchestrator.PlanSpec{
		ID:             "plan-1",
		Tasks:          []orchestrator.PlannedTask{{ID: "t1", Title: "task", Description: "d"}},
		ExecutionOrder: [][]string{{"t1"}},
	}
	runner, err := NewPipelineRunner(PipelineRunnerConfig{
		Orch:        &orchestrator.Orchestrator{},
		Session:     &orchestrator.Session{BaseRepo: t.TempDir()},
		Plan:        plan,
		Bus:         bus,
		MaxParallel: 1,
	})
	if err != nil {
		t.Fatalf("NewPipelineRunner() error = %v", err)
	}

	// Stop without Start should not panic
	runner.Stop()
}

func TestPipelineRunner_MaxParallelDefault(t *testing.T) {
	bus := event.NewBus()
	plan := &orchestrator.PlanSpec{
		ID:             "plan-1",
		Tasks:          []orchestrator.PlannedTask{{ID: "t1", Title: "task", Description: "d"}},
		ExecutionOrder: [][]string{{"t1"}},
	}

	// MaxParallel=0 should default to 3 internally
	runner, err := NewPipelineRunner(PipelineRunnerConfig{
		Orch:        &orchestrator.Orchestrator{},
		Session:     &orchestrator.Session{BaseRepo: t.TempDir()},
		Plan:        plan,
		Bus:         bus,
		MaxParallel: 0,
	})
	if err != nil {
		t.Fatalf("NewPipelineRunner() error = %v", err)
	}
	if runner == nil {
		t.Fatal("runner is nil")
	}
}

func TestPipelineRunner_ImplementsExecutionRunner(t *testing.T) {
	// Compile-time check that PipelineRunner satisfies ExecutionRunner.
	var _ orchestrator.ExecutionRunner = (*PipelineRunner)(nil)
}

func TestPipelineRunner_StartCtxCancel(t *testing.T) {
	bus := event.NewBus()
	plan := &orchestrator.PlanSpec{
		ID:             "plan-1",
		Tasks:          []orchestrator.PlannedTask{{ID: "t1", Title: "task", Description: "d"}},
		ExecutionOrder: [][]string{{"t1"}},
	}
	runner, err := NewPipelineRunner(PipelineRunnerConfig{
		Orch:        &orchestrator.Orchestrator{},
		Session:     &orchestrator.Session{BaseRepo: t.TempDir()},
		Plan:        plan,
		Bus:         bus,
		MaxParallel: 1,
	})
	if err != nil {
		t.Fatalf("NewPipelineRunner() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	if err := runner.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Cancel context and stop
	cancel()

	done := make(chan struct{})
	go func() {
		runner.Stop()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() did not return within 5s")
	}
}
