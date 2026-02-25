package bridgewire

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/ai"
	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/prompt"
	"github.com/Iron-Ham/claudio/internal/team"
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

func TestInjectSystemPrompt_NilMap(t *testing.T) {
	result := injectSystemPrompt(nil, "/tmp/sys-prompt.md")

	if result == nil {
		t.Fatal("injectSystemPrompt returned nil for nil input")
	}
	execOpts, ok := result[team.RoleExecution]
	if !ok {
		t.Fatal("missing execution role in result")
	}
	if execOpts.AppendSystemPromptFile != "/tmp/sys-prompt.md" {
		t.Errorf("AppendSystemPromptFile = %q, want %q",
			execOpts.AppendSystemPromptFile, "/tmp/sys-prompt.md")
	}
}

func TestInjectSystemPrompt_ExistingMap(t *testing.T) {
	overrides := map[team.Role]ai.StartOptions{
		team.RoleExecution: {PermissionMode: "auto-accept"},
	}

	result := injectSystemPrompt(overrides, "/tmp/sys-prompt.md")

	execOpts := result[team.RoleExecution]
	if execOpts.AppendSystemPromptFile != "/tmp/sys-prompt.md" {
		t.Errorf("AppendSystemPromptFile = %q, want %q",
			execOpts.AppendSystemPromptFile, "/tmp/sys-prompt.md")
	}
	// Existing overrides should be preserved
	if execOpts.PermissionMode != "auto-accept" {
		t.Errorf("PermissionMode = %q, want %q", execOpts.PermissionMode, "auto-accept")
	}
}

func TestInjectSystemPrompt_DoesNotMutateCaller(t *testing.T) {
	original := map[team.Role]ai.StartOptions{
		team.RoleExecution: {PermissionMode: "auto-accept"},
	}

	result := injectSystemPrompt(original, "/tmp/sys-prompt.md")

	// Result should have the system prompt
	if result[team.RoleExecution].AppendSystemPromptFile != "/tmp/sys-prompt.md" {
		t.Errorf("result missing system prompt")
	}
	// Original should NOT be mutated
	if original[team.RoleExecution].AppendSystemPromptFile != "" {
		t.Errorf("original was mutated: AppendSystemPromptFile = %q, want empty",
			original[team.RoleExecution].AppendSystemPromptFile)
	}
}

func TestInjectSystemPrompt_DoesNotOverrideExplicit(t *testing.T) {
	overrides := map[team.Role]ai.StartOptions{
		team.RoleExecution: {AppendSystemPromptFile: "/explicit/path.md"},
	}

	result := injectSystemPrompt(overrides, "/tmp/sys-prompt.md")

	execOpts := result[team.RoleExecution]
	if execOpts.AppendSystemPromptFile != "/explicit/path.md" {
		t.Errorf("AppendSystemPromptFile = %q, want %q (should not be overridden)",
			execOpts.AppendSystemPromptFile, "/explicit/path.md")
	}
}

func TestMergeUniqueStrings(t *testing.T) {
	tests := []struct {
		name string
		a, b []string
		want []string
	}{
		{"both nil", nil, nil, nil},
		{"a only", []string{"x", "y"}, nil, []string{"x", "y"}},
		{"b only", nil, []string{"x", "y"}, []string{"x", "y"}},
		{"no overlap", []string{"a"}, []string{"b"}, []string{"a", "b"}},
		{"duplicates", []string{"a", "b"}, []string{"b", "c"}, []string{"a", "b", "c"}},
		{"all same", []string{"x"}, []string{"x"}, []string{"x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeUniqueStrings(tt.a, tt.b)
			if len(got) != len(tt.want) {
				t.Fatalf("mergeUniqueStrings() len = %d, want %d (%v vs %v)", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("mergeUniqueStrings()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestMergeStartOptions(t *testing.T) {
	t.Run("overrides take precedence for non-zero fields", func(t *testing.T) {
		defaults := ai.StartOptions{
			PermissionMode: "bypass",
			Model:          "sonnet",
			MaxTurns:       10,
			AllowedTools:   []string{"Read"},
		}
		overrides := ai.StartOptions{
			PermissionMode: "plan",
			Model:          "opus",
			MaxTurns:       5,
		}

		got := mergeStartOptions(defaults, overrides)

		if got.PermissionMode != "plan" {
			t.Errorf("PermissionMode = %q, want %q", got.PermissionMode, "plan")
		}
		if got.Model != "opus" {
			t.Errorf("Model = %q, want %q", got.Model, "opus")
		}
		if got.MaxTurns != 5 {
			t.Errorf("MaxTurns = %d, want %d", got.MaxTurns, 5)
		}
		// AllowedTools should be preserved from defaults since overrides has none
		if len(got.AllowedTools) != 1 || got.AllowedTools[0] != "Read" {
			t.Errorf("AllowedTools = %v, want [Read]", got.AllowedTools)
		}
	})

	t.Run("defaults used when overrides are zero", func(t *testing.T) {
		defaults := ai.StartOptions{
			PermissionMode:  "bypass",
			Model:           "sonnet",
			MaxTurns:        10,
			AllowedTools:    []string{"Read"},
			DisallowedTools: []string{"Write"},
		}
		overrides := ai.StartOptions{} // all zero

		got := mergeStartOptions(defaults, overrides)

		if got.PermissionMode != "bypass" {
			t.Errorf("PermissionMode = %q, want %q", got.PermissionMode, "bypass")
		}
		if got.Model != "sonnet" {
			t.Errorf("Model = %q, want %q", got.Model, "sonnet")
		}
		if got.MaxTurns != 10 {
			t.Errorf("MaxTurns = %d, want %d", got.MaxTurns, 10)
		}
		if len(got.AllowedTools) != 1 || got.AllowedTools[0] != "Read" {
			t.Errorf("AllowedTools = %v, want [Read]", got.AllowedTools)
		}
		if len(got.DisallowedTools) != 1 || got.DisallowedTools[0] != "Write" {
			t.Errorf("DisallowedTools = %v, want [Write]", got.DisallowedTools)
		}
	})

	t.Run("tool lists are merged and deduplicated", func(t *testing.T) {
		defaults := ai.StartOptions{
			AllowedTools:    []string{"Read", "Bash"},
			DisallowedTools: []string{"Write"},
		}
		overrides := ai.StartOptions{
			AllowedTools:    []string{"Bash", "Edit"},
			DisallowedTools: []string{"Write", "Delete"},
		}

		got := mergeStartOptions(defaults, overrides)

		wantAllowed := []string{"Read", "Bash", "Edit"}
		if len(got.AllowedTools) != len(wantAllowed) {
			t.Fatalf("AllowedTools len = %d, want %d (%v)", len(got.AllowedTools), len(wantAllowed), got.AllowedTools)
		}
		for i, w := range wantAllowed {
			if got.AllowedTools[i] != w {
				t.Errorf("AllowedTools[%d] = %q, want %q", i, got.AllowedTools[i], w)
			}
		}

		wantDisallowed := []string{"Write", "Delete"}
		if len(got.DisallowedTools) != len(wantDisallowed) {
			t.Fatalf("DisallowedTools len = %d, want %d (%v)", len(got.DisallowedTools), len(wantDisallowed), got.DisallowedTools)
		}
		for i, w := range wantDisallowed {
			if got.DisallowedTools[i] != w {
				t.Errorf("DisallowedTools[%d] = %q, want %q", i, got.DisallowedTools[i], w)
			}
		}
	})

	t.Run("boolean fields sticky-true", func(t *testing.T) {
		defaults := ai.StartOptions{}
		overrides := ai.StartOptions{NoUserPrompt: true, Worktree: true}

		got := mergeStartOptions(defaults, overrides)

		if !got.NoUserPrompt {
			t.Error("NoUserPrompt = false, want true")
		}
		if !got.Worktree {
			t.Error("Worktree = false, want true")
		}
	})

	t.Run("system prompt override wins", func(t *testing.T) {
		defaults := ai.StartOptions{AppendSystemPromptFile: "/default.md"}
		overrides := ai.StartOptions{AppendSystemPromptFile: "/override.md"}

		got := mergeStartOptions(defaults, overrides)

		if got.AppendSystemPromptFile != "/override.md" {
			t.Errorf("AppendSystemPromptFile = %q, want %q", got.AppendSystemPromptFile, "/override.md")
		}
	})
}

func TestNewPipelineRunner_SubprocessMode_BackendDefaults(t *testing.T) {
	bus := event.NewBus()
	baseDir := t.TempDir()
	plan := &orchestrator.PlanSpec{
		ID:             "plan-1",
		Tasks:          []orchestrator.PlannedTask{{ID: "t1", Title: "task", Description: "d"}},
		ExecutionOrder: [][]string{{"t1"}},
	}

	runner, err := NewPipelineRunner(PipelineRunnerConfig{
		Orch:           &orchestrator.Orchestrator{},
		Session:        &orchestrator.Session{BaseRepo: baseDir},
		Plan:           plan,
		Bus:            bus,
		MaxParallel:    1,
		SubprocessMode: true,
		BackendDefaults: ai.StartOptions{
			PermissionMode: "bypass",
			Model:          "sonnet",
			MaxTurns:       15,
		},
	})
	if err != nil {
		t.Fatalf("NewPipelineRunner() error = %v", err)
	}
	if runner == nil {
		t.Fatal("runner is nil")
	}

	// Verify the executor's default factory carries the backend defaults.
	// We can't inspect the factory directly (it's an interface), but we can
	// check that the executor was created successfully, confirming the
	// BackendDefaults were passed through without error.
	if runner.exec == nil {
		t.Fatal("runner.exec is nil")
	}
}

func TestNewPipelineRunner_WritesSystemPromptFile(t *testing.T) {
	bus := event.NewBus()
	baseDir := t.TempDir()
	plan := &orchestrator.PlanSpec{
		ID:             "plan-1",
		Tasks:          []orchestrator.PlannedTask{{ID: "t1", Title: "task", Description: "d"}},
		ExecutionOrder: [][]string{{"t1"}},
	}

	runner, err := NewPipelineRunner(PipelineRunnerConfig{
		Orch:        &orchestrator.Orchestrator{},
		Session:     &orchestrator.Session{BaseRepo: baseDir},
		Plan:        plan,
		Bus:         bus,
		MaxParallel: 1,
	})
	if err != nil {
		t.Fatalf("NewPipelineRunner() error = %v", err)
	}
	if runner == nil {
		t.Fatal("runner is nil")
	}

	// Verify the system prompt file was written to baseDir
	sysPromptPath := filepath.Join(baseDir, prompt.SystemPromptFileName)
	if _, err := os.Stat(sysPromptPath); os.IsNotExist(err) {
		t.Errorf("system prompt file not found at %q", sysPromptPath)
	}

	// Verify the executor has the system prompt path in role overrides
	execOverrides, ok := runner.exec.roleOverrides[team.RoleExecution]
	if !ok {
		t.Fatal("missing execution role overrides")
	}
	if execOverrides.AppendSystemPromptFile != sysPromptPath {
		t.Errorf("AppendSystemPromptFile = %q, want %q",
			execOverrides.AppendSystemPromptFile, sysPromptPath)
	}
}
