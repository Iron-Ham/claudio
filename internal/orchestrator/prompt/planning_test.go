package prompt

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPlanningBuilder_Build(t *testing.T) {
	tests := []struct {
		name        string
		ctx         *Context
		wantErr     bool
		errContains string
		contains    []string
	}{
		{
			name: "valid context with multiple plans",
			ctx: &Context{
				Phase:     PhasePlanSelection,
				SessionID: "test-session",
				Objective: "Implement user authentication",
				CandidatePlans: []CandidatePlanInfo{
					{
						Strategy: "maximize-parallelism",
						Summary:  "Focus on parallel execution",
						Tasks: []TaskInfo{
							{ID: "task-1", Title: "Setup auth module", EstComplexity: "medium"},
							{ID: "task-2", Title: "Create login endpoint", EstComplexity: "low"},
						},
						ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
						Insights:       []string{"Can parallelize more"},
						Constraints:    []string{"Need database setup first"},
					},
					{
						Strategy: "minimize-complexity",
						Summary:  "Focus on simplicity",
						Tasks: []TaskInfo{
							{ID: "task-a", Title: "Combined auth task", EstComplexity: "high"},
						},
						ExecutionOrder: [][]string{{"task-a"}},
					},
				},
			},
			wantErr: false,
			contains: []string{
				"Implement user authentication",
				"maximize-parallelism",
				"minimize-complexity",
				"Focus on parallel execution",
				"Focus on simplicity",
				"task-1",
				"task-2",
				"task-a",
				"Evaluate each plan",
				"Parallelism potential",
			},
		},
		{
			name: "valid context with PhasePlanning",
			ctx: &Context{
				Phase:     PhasePlanning,
				SessionID: "test-session",
				Objective: "Build API",
				CandidatePlans: []CandidatePlanInfo{
					{
						Strategy: "balanced",
						Summary:  "Balanced approach",
						Tasks:    []TaskInfo{{ID: "t1", Title: "Task 1"}},
					},
				},
			},
			wantErr:  false,
			contains: []string{"Build API", "balanced", "Task 1"},
		},
		{
			name:        "nil context",
			ctx:         nil,
			wantErr:     true,
			errContains: "nil",
		},
		{
			name: "missing objective",
			ctx: &Context{
				Phase:     PhasePlanSelection,
				SessionID: "test-session",
				CandidatePlans: []CandidatePlanInfo{
					{Strategy: "test", Summary: "test"},
				},
			},
			wantErr:     true,
			errContains: "objective",
		},
		{
			name: "missing candidate plans",
			ctx: &Context{
				Phase:     PhasePlanSelection,
				SessionID: "test-session",
				Objective: "Test objective",
			},
			wantErr:     true,
			errContains: "candidate plans",
		},
		{
			name: "wrong phase",
			ctx: &Context{
				Phase:     PhaseTask,
				SessionID: "test-session",
				Objective: "Test objective",
				CandidatePlans: []CandidatePlanInfo{
					{Strategy: "test", Summary: "test"},
				},
			},
			wantErr:     true,
			errContains: "invalid",
		},
	}

	builder := NewPlanningBuilder()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := builder.Build(tt.ctx)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Build() expected error, got nil")
					return
				}
				if tt.errContains != "" && !strings.Contains(strings.ToLower(err.Error()), tt.errContains) {
					t.Errorf("Build() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("Build() unexpected error: %v", err)
				return
			}

			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("Build() result missing %q\nGot:\n%s", want, result)
				}
			}
		})
	}
}

func TestPlanningBuilder_formatCandidatePlans(t *testing.T) {
	builder := NewPlanningBuilder()

	tests := []struct {
		name          string
		plans         []CandidatePlanInfo
		strategyNames []string
		contains      []string
	}{
		{
			name: "formats strategy names",
			plans: []CandidatePlanInfo{
				{Strategy: "maximize-parallelism", Summary: "Parallel focus"},
				{Strategy: "minimize-complexity", Summary: "Simple focus"},
			},
			contains: []string{
				"Plan 1: maximize-parallelism",
				"Plan 2: minimize-complexity",
				"Parallel focus",
				"Simple focus",
			},
		},
		{
			name: "uses default strategy name when empty",
			plans: []CandidatePlanInfo{
				{Strategy: "", Summary: "No strategy name"},
			},
			contains: []string{
				"Plan 1: strategy-1",
			},
		},
		{
			name: "uses context strategy name as fallback when plan strategy empty",
			plans: []CandidatePlanInfo{
				{Strategy: "", Summary: "No strategy name"},
			},
			strategyNames: []string{"fallback-strategy"},
			contains: []string{
				"Plan 1: fallback-strategy",
			},
		},
		{
			name: "plan strategy takes precedence over context strategy names",
			plans: []CandidatePlanInfo{
				{Strategy: "plan-strategy", Summary: "Has strategy"},
			},
			strategyNames: []string{"fallback-strategy"},
			contains: []string{
				"Plan 1: plan-strategy",
			},
		},
		{
			name: "includes task count",
			plans: []CandidatePlanInfo{
				{
					Strategy: "test",
					Summary:  "Test",
					Tasks: []TaskInfo{
						{ID: "t1", Title: "Task 1"},
						{ID: "t2", Title: "Task 2"},
						{ID: "t3", Title: "Task 3"},
					},
				},
			},
			contains: []string{
				"**Task Count**: 3 tasks",
			},
		},
		{
			name: "includes execution groups and max parallelism",
			plans: []CandidatePlanInfo{
				{
					Strategy:       "test",
					Summary:        "Test",
					ExecutionOrder: [][]string{{"t1", "t2"}, {"t3"}},
				},
			},
			contains: []string{
				"**Execution Groups**: 2 groups",
				"**Max Parallelism**: 2 concurrent tasks",
				"Group 1: t1, t2",
				"Group 2: t3",
			},
		},
		{
			name: "includes insights",
			plans: []CandidatePlanInfo{
				{
					Strategy: "test",
					Summary:  "Test",
					Insights: []string{"Insight one", "Insight two"},
				},
			},
			contains: []string{
				"**Insights**:",
				"- Insight one",
				"- Insight two",
			},
		},
		{
			name: "includes constraints",
			plans: []CandidatePlanInfo{
				{
					Strategy:    "test",
					Summary:     "Test",
					Constraints: []string{"Constraint one"},
				},
			},
			contains: []string{
				"**Constraints**:",
				"- Constraint one",
			},
		},
		{
			name: "includes JSON tasks",
			plans: []CandidatePlanInfo{
				{
					Strategy: "test",
					Summary:  "Test",
					Tasks: []TaskInfo{
						{ID: "task-1", Title: "First task"},
					},
				},
			},
			contains: []string{
				"**Tasks (JSON)**:",
				"```json",
				`"ID": "task-1"`,
				`"Title": "First task"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &Context{
				CandidatePlans: tt.plans,
				StrategyNames:  tt.strategyNames,
			}
			result := builder.formatCandidatePlans(ctx)

			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("formatCandidatePlans() missing %q\nGot:\n%s", want, result)
				}
			}
		})
	}
}

func TestPlanningBuilder_FormatCompactPlans(t *testing.T) {
	builder := NewPlanningBuilder()

	plans := []CandidatePlanInfo{
		{
			Strategy: "maximize-parallelism",
			Summary:  "Focus on parallel execution",
			Tasks: []TaskInfo{
				{ID: "task-1", Title: "Task One", EstComplexity: "medium", DependsOn: nil},
				{ID: "task-2", Title: "Task Two", EstComplexity: "low", DependsOn: []string{"task-1"}},
			},
			ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
			Insights:       []string{"Good parallelism"},
			Constraints:    []string{"Resource constraint"},
		},
	}

	result := builder.FormatCompactPlans(plans)

	expectedParts := []string{
		"Plan 1: maximize-parallelism Strategy",
		"**Summary:** Focus on parallel execution",
		"**Tasks (2 total):**",
		"[task-1] Task One (complexity: medium, depends: none)",
		"[task-2] Task Two (complexity: low, depends: task-1)",
		"**Execution Groups:** 2 parallel groups",
		"Group 1: task-1",
		"Group 2: task-2",
		"**Insights:**",
		"- Good parallelism",
		"**Constraints:**",
		"- Resource constraint",
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("FormatCompactPlans() missing %q\nGot:\n%s", part, result)
		}
	}
}

func TestPlanningBuilder_FormatCompactPlansWithContext(t *testing.T) {
	builder := NewPlanningBuilder()

	tests := []struct {
		name          string
		plans         []CandidatePlanInfo
		strategyNames []string
		contains      []string
		notContains   []string
	}{
		{
			name: "uses plan strategy when set",
			plans: []CandidatePlanInfo{
				{Strategy: "plan-strategy", Summary: "Test"},
			},
			strategyNames: []string{"fallback-strategy"},
			contains:      []string{"Plan 1: plan-strategy Strategy"},
			notContains:   []string{"fallback-strategy"},
		},
		{
			name: "uses fallback strategy from context when plan strategy empty",
			plans: []CandidatePlanInfo{
				{Strategy: "", Summary: "Test"},
			},
			strategyNames: []string{"context-fallback"},
			contains:      []string{"Plan 1: context-fallback Strategy"},
			notContains:   []string{"unknown"},
		},
		{
			name: "uses unknown when no strategy and no fallback",
			plans: []CandidatePlanInfo{
				{Strategy: "", Summary: "Test"},
			},
			strategyNames: nil,
			contains:      []string{"Plan 1: unknown Strategy"},
		},
		{
			name: "handles more plans than strategy names",
			plans: []CandidatePlanInfo{
				{Strategy: "", Summary: "First"},
				{Strategy: "", Summary: "Second"},
			},
			strategyNames: []string{"fallback-1"},
			contains: []string{
				"Plan 1: fallback-1 Strategy",
				"Plan 2: unknown Strategy",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.FormatCompactPlansWithContext(tt.plans, tt.strategyNames)

			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("FormatCompactPlansWithContext() missing %q\nGot:\n%s", want, result)
				}
			}
			for _, notWant := range tt.notContains {
				if strings.Contains(result, notWant) {
					t.Errorf("FormatCompactPlansWithContext() should not contain %q\nGot:\n%s", notWant, result)
				}
			}
		})
	}
}

func TestPlanningBuilder_JSONValidity(t *testing.T) {
	builder := NewPlanningBuilder()

	plan := CandidatePlanInfo{
		Strategy: "test",
		Summary:  "Test plan",
		Tasks: []TaskInfo{
			{
				ID:            "task-1",
				Title:         "Test Task",
				Description:   "A test task description",
				Files:         []string{"file1.go", "file2.go"},
				DependsOn:     []string{"task-0"},
				Priority:      1,
				EstComplexity: "medium",
			},
		},
		ExecutionOrder: [][]string{{"task-1"}},
	}

	ctx := &Context{
		CandidatePlans: []CandidatePlanInfo{plan},
	}
	result := builder.formatCandidatePlans(ctx)

	// Extract JSON from the result
	jsonStart := strings.Index(result, "```json\n") + len("```json\n")
	jsonEnd := strings.Index(result[jsonStart:], "\n```")
	if jsonStart < len("```json\n") || jsonEnd < 0 {
		t.Fatal("Could not find JSON block in output")
	}

	jsonStr := result[jsonStart : jsonStart+jsonEnd]

	var tasks []TaskInfo
	if err := json.Unmarshal([]byte(jsonStr), &tasks); err != nil {
		t.Errorf("JSON in output is not valid: %v\nJSON:\n%s", err, jsonStr)
	}

	if len(tasks) != 1 {
		t.Errorf("Expected 1 task in JSON, got %d", len(tasks))
	}

	if tasks[0].ID != "task-1" {
		t.Errorf("Task ID = %q, want %q", tasks[0].ID, "task-1")
	}
}

func TestPlanningBuilder_PromptTemplate(t *testing.T) {
	builder := NewPlanningBuilder()

	ctx := &Context{
		Phase:     PhasePlanSelection,
		SessionID: "test-session",
		Objective: "Build a REST API",
		CandidatePlans: []CandidatePlanInfo{
			{
				Strategy: "balanced",
				Summary:  "Balanced approach",
				Tasks:    []TaskInfo{{ID: "t1", Title: "Task 1"}},
			},
		},
	}

	result, err := builder.Build(ctx)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Check that the prompt includes the expected structure
	expectedSections := []string{
		"## Objective",
		"Build a REST API",
		"## Candidate Plans",
		"## Your Task",
		"Parallelism potential",
		"Task granularity",
		"Dependency structure",
		"File ownership",
		"Completeness",
		"Risk mitigation",
		"## Decision",
		"**Select** the best plan",
		"**Merge** the best elements",
		"## Output",
		PlanFileName,
		"<plan_decision>",
		"</plan_decision>",
	}

	for _, section := range expectedSections {
		if !strings.Contains(result, section) {
			t.Errorf("Build() result missing section %q", section)
		}
	}
}

func TestNewPlanningBuilder(t *testing.T) {
	builder := NewPlanningBuilder()
	if builder == nil {
		t.Error("NewPlanningBuilder() returned nil")
	}
}

func TestPlanningBuilder_BuildCompactPlanManagerPrompt(t *testing.T) {
	builder := NewPlanningBuilder()

	tests := []struct {
		name          string
		objective     string
		plans         []CandidatePlanInfo
		strategyNames []string
		contains      []string
	}{
		{
			name:      "builds complete prompt with single plan",
			objective: "Build a REST API",
			plans: []CandidatePlanInfo{
				{Strategy: "maximize-parallelism", Summary: "Parallel execution plan"},
			},
			strategyNames: nil,
			contains: []string{
				"senior technical lead",
				"## Objective",
				"Build a REST API",
				"## Candidate Plans",
				"Plan 1: maximize-parallelism",
				"Parallel execution plan",
				"## Your Task",
				"## Decision",
			},
		},
		{
			name:      "builds prompt with multiple plans",
			objective: "Implement authentication",
			plans: []CandidatePlanInfo{
				{Strategy: "minimize-dependencies", Summary: "Low coupling approach"},
				{Strategy: "maximize-parallelism", Summary: "High parallelism approach"},
				{Strategy: "balanced", Summary: "Balanced trade-offs"},
			},
			strategyNames: nil,
			contains: []string{
				"## Objective",
				"Implement authentication",
				"Plan 1: minimize-dependencies",
				"Plan 2: maximize-parallelism",
				"Plan 3: balanced",
				"Low coupling approach",
				"High parallelism approach",
				"Balanced trade-offs",
			},
		},
		{
			name:      "uses fallback strategy names when plan strategy is empty",
			objective: "Refactor module",
			plans: []CandidatePlanInfo{
				{Strategy: "", Summary: "First plan"},
				{Strategy: "", Summary: "Second plan"},
			},
			strategyNames: []string{"fallback-strategy-1", "fallback-strategy-2"},
			contains: []string{
				"Plan 1: fallback-strategy-1",
				"Plan 2: fallback-strategy-2",
			},
		},
		{
			name:      "uses unknown when no strategy and no fallback",
			objective: "Fix bugs",
			plans: []CandidatePlanInfo{
				{Strategy: "", Summary: "Bug fix plan"},
			},
			strategyNames: nil,
			contains: []string{
				"Plan 1: unknown",
			},
		},
		{
			name:      "includes evaluation criteria",
			objective: "Add feature",
			plans: []CandidatePlanInfo{
				{Strategy: "test", Summary: "Test plan"},
			},
			strategyNames: nil,
			contains: []string{
				"Parallelism potential",
				"Task granularity",
				"Dependency structure",
				"File ownership",
				"Completeness",
				"Risk mitigation",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.BuildCompactPlanManagerPrompt(tt.objective, tt.plans, tt.strategyNames)

			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("BuildCompactPlanManagerPrompt() missing %q\nGot:\n%s", want, result)
				}
			}
		})
	}
}
