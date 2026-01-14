package orchestrator

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator/prompt"
)

func TestPlanInfoFromPlanSpec(t *testing.T) {
	tests := []struct {
		name     string
		spec     *PlanSpec
		wantNil  bool
		validate func(t *testing.T, result *prompt.PlanInfo)
	}{
		{
			name:    "nil spec returns nil",
			spec:    nil,
			wantNil: true,
		},
		{
			name: "empty spec converts correctly",
			spec: &PlanSpec{
				ID:             "plan-1",
				Summary:        "Test plan",
				Tasks:          []PlannedTask{},
				ExecutionOrder: [][]string{},
				Insights:       []string{},
				Constraints:    []string{},
			},
			validate: func(t *testing.T, result *prompt.PlanInfo) {
				if result.ID != "plan-1" {
					t.Errorf("ID = %q, want plan-1", result.ID)
				}
				if result.Summary != "Test plan" {
					t.Errorf("Summary = %q, want 'Test plan'", result.Summary)
				}
				if len(result.Tasks) != 0 {
					t.Errorf("Tasks length = %d, want 0", len(result.Tasks))
				}
			},
		},
		{
			name: "full spec converts all fields",
			spec: &PlanSpec{
				ID:      "plan-full",
				Summary: "Full test plan",
				Tasks: []PlannedTask{
					{
						ID:            "task-1",
						Title:         "First Task",
						Description:   "Do the first thing",
						Files:         []string{"file1.go", "file2.go"},
						DependsOn:     []string{},
						Priority:      1,
						EstComplexity: ComplexityLow,
						IssueURL:      "https://github.com/org/repo/issues/1",
					},
					{
						ID:            "task-2",
						Title:         "Second Task",
						Description:   "Do the second thing",
						Files:         []string{"file3.go"},
						DependsOn:     []string{"task-1"},
						Priority:      2,
						EstComplexity: ComplexityMedium,
					},
				},
				ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
				Insights:       []string{"Insight 1", "Insight 2"},
				Constraints:    []string{"Constraint 1"},
			},
			validate: func(t *testing.T, result *prompt.PlanInfo) {
				if result.ID != "plan-full" {
					t.Errorf("ID = %q, want plan-full", result.ID)
				}
				if result.Summary != "Full test plan" {
					t.Errorf("Summary = %q, want 'Full test plan'", result.Summary)
				}

				// Verify tasks
				if len(result.Tasks) != 2 {
					t.Fatalf("Tasks length = %d, want 2", len(result.Tasks))
				}

				// Check first task
				task1 := result.Tasks[0]
				if task1.ID != "task-1" {
					t.Errorf("Tasks[0].ID = %q, want task-1", task1.ID)
				}
				if task1.Title != "First Task" {
					t.Errorf("Tasks[0].Title = %q, want 'First Task'", task1.Title)
				}
				if task1.Description != "Do the first thing" {
					t.Errorf("Tasks[0].Description = %q, want 'Do the first thing'", task1.Description)
				}
				if len(task1.Files) != 2 {
					t.Errorf("Tasks[0].Files length = %d, want 2", len(task1.Files))
				}
				if task1.Priority != 1 {
					t.Errorf("Tasks[0].Priority = %d, want 1", task1.Priority)
				}
				if task1.EstComplexity != "low" {
					t.Errorf("Tasks[0].EstComplexity = %q, want 'low'", task1.EstComplexity)
				}
				if task1.IssueURL != "https://github.com/org/repo/issues/1" {
					t.Errorf("Tasks[0].IssueURL = %q, want URL", task1.IssueURL)
				}
				// CommitCount should be zero since PlannedTask doesn't have it
				if task1.CommitCount != 0 {
					t.Errorf("Tasks[0].CommitCount = %d, want 0", task1.CommitCount)
				}

				// Check second task dependencies
				task2 := result.Tasks[1]
				if len(task2.DependsOn) != 1 || task2.DependsOn[0] != "task-1" {
					t.Errorf("Tasks[1].DependsOn = %v, want [task-1]", task2.DependsOn)
				}
				if task2.EstComplexity != "medium" {
					t.Errorf("Tasks[1].EstComplexity = %q, want 'medium'", task2.EstComplexity)
				}

				// Verify execution order
				if len(result.ExecutionOrder) != 2 {
					t.Errorf("ExecutionOrder length = %d, want 2", len(result.ExecutionOrder))
				}

				// Verify insights
				if len(result.Insights) != 2 {
					t.Errorf("Insights length = %d, want 2", len(result.Insights))
				}

				// Verify constraints
				if len(result.Constraints) != 1 {
					t.Errorf("Constraints length = %d, want 1", len(result.Constraints))
				}
			},
		},
		{
			name: "high complexity converts correctly",
			spec: &PlanSpec{
				ID: "plan-complex",
				Tasks: []PlannedTask{
					{
						ID:            "task-complex",
						EstComplexity: ComplexityHigh,
					},
				},
			},
			validate: func(t *testing.T, result *prompt.PlanInfo) {
				if result.Tasks[0].EstComplexity != "high" {
					t.Errorf("Tasks[0].EstComplexity = %q, want 'high'", result.Tasks[0].EstComplexity)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PlanInfoFromPlanSpec(tt.spec)

			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestTaskInfoFromPlannedTask(t *testing.T) {
	tests := []struct {
		name        string
		task        *PlannedTask
		commitCount int
		wantNil     bool
		validate    func(t *testing.T, result *prompt.TaskInfo)
	}{
		{
			name:    "nil task returns nil",
			task:    nil,
			wantNil: true,
		},
		{
			name: "full task converts all fields",
			task: &PlannedTask{
				ID:            "task-1",
				Title:         "Test Task",
				Description:   "Do something",
				Files:         []string{"file1.go", "file2.go"},
				DependsOn:     []string{"task-0"},
				Priority:      5,
				EstComplexity: ComplexityMedium,
				IssueURL:      "https://github.com/org/repo/issues/42",
			},
			commitCount: 3,
			validate: func(t *testing.T, result *prompt.TaskInfo) {
				if result.ID != "task-1" {
					t.Errorf("ID = %q, want task-1", result.ID)
				}
				if result.Title != "Test Task" {
					t.Errorf("Title = %q, want 'Test Task'", result.Title)
				}
				if result.Description != "Do something" {
					t.Errorf("Description = %q, want 'Do something'", result.Description)
				}
				if len(result.Files) != 2 {
					t.Errorf("Files length = %d, want 2", len(result.Files))
				}
				if len(result.DependsOn) != 1 || result.DependsOn[0] != "task-0" {
					t.Errorf("DependsOn = %v, want [task-0]", result.DependsOn)
				}
				if result.Priority != 5 {
					t.Errorf("Priority = %d, want 5", result.Priority)
				}
				if result.EstComplexity != "medium" {
					t.Errorf("EstComplexity = %q, want 'medium'", result.EstComplexity)
				}
				if result.IssueURL != "https://github.com/org/repo/issues/42" {
					t.Errorf("IssueURL = %q, want URL", result.IssueURL)
				}
				if result.CommitCount != 3 {
					t.Errorf("CommitCount = %d, want 3", result.CommitCount)
				}
			},
		},
		{
			name: "zero commit count",
			task: &PlannedTask{
				ID: "task-zero",
			},
			commitCount: 0,
			validate: func(t *testing.T, result *prompt.TaskInfo) {
				if result.CommitCount != 0 {
					t.Errorf("CommitCount = %d, want 0", result.CommitCount)
				}
			},
		},
		{
			name: "empty dependencies",
			task: &PlannedTask{
				ID:        "task-no-deps",
				DependsOn: []string{},
			},
			validate: func(t *testing.T, result *prompt.TaskInfo) {
				if result.DependsOn == nil || len(result.DependsOn) != 0 {
					t.Errorf("DependsOn = %v, want empty slice", result.DependsOn)
				}
			},
		},
		{
			name: "nil dependencies",
			task: &PlannedTask{
				ID:        "task-nil-deps",
				DependsOn: nil,
			},
			validate: func(t *testing.T, result *prompt.TaskInfo) {
				if len(result.DependsOn) != 0 {
					t.Errorf("DependsOn = %v, want nil or empty", result.DependsOn)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TaskInfoFromPlannedTask(tt.task, tt.commitCount)

			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestTaskInfoSliceFromPlannedTasks(t *testing.T) {
	tests := []struct {
		name         string
		tasks        []PlannedTask
		commitCounts map[string]int
		validate     func(t *testing.T, result []prompt.TaskInfo)
	}{
		{
			name:         "nil tasks returns empty slice",
			tasks:        nil,
			commitCounts: nil,
			validate: func(t *testing.T, result []prompt.TaskInfo) {
				if result == nil {
					t.Error("expected non-nil empty slice, got nil")
				}
				if len(result) != 0 {
					t.Errorf("expected empty slice, got %d items", len(result))
				}
			},
		},
		{
			name:         "empty tasks returns empty slice",
			tasks:        []PlannedTask{},
			commitCounts: nil,
			validate: func(t *testing.T, result []prompt.TaskInfo) {
				if result == nil {
					t.Error("expected non-nil empty slice, got nil")
				}
				if len(result) != 0 {
					t.Errorf("expected empty slice, got %d items", len(result))
				}
			},
		},
		{
			name: "converts multiple tasks",
			tasks: []PlannedTask{
				{ID: "task-1", Title: "Task One", EstComplexity: ComplexityLow},
				{ID: "task-2", Title: "Task Two", EstComplexity: ComplexityHigh},
			},
			commitCounts: map[string]int{
				"task-1": 2,
				"task-2": 5,
			},
			validate: func(t *testing.T, result []prompt.TaskInfo) {
				if len(result) != 2 {
					t.Fatalf("expected 2 items, got %d", len(result))
				}

				if result[0].ID != "task-1" {
					t.Errorf("result[0].ID = %q, want task-1", result[0].ID)
				}
				if result[0].Title != "Task One" {
					t.Errorf("result[0].Title = %q, want 'Task One'", result[0].Title)
				}
				if result[0].CommitCount != 2 {
					t.Errorf("result[0].CommitCount = %d, want 2", result[0].CommitCount)
				}

				if result[1].ID != "task-2" {
					t.Errorf("result[1].ID = %q, want task-2", result[1].ID)
				}
				if result[1].CommitCount != 5 {
					t.Errorf("result[1].CommitCount = %d, want 5", result[1].CommitCount)
				}
			},
		},
		{
			name: "nil commit counts defaults to zero",
			tasks: []PlannedTask{
				{ID: "task-1"},
				{ID: "task-2"},
			},
			commitCounts: nil,
			validate: func(t *testing.T, result []prompt.TaskInfo) {
				if len(result) != 2 {
					t.Fatalf("expected 2 items, got %d", len(result))
				}
				if result[0].CommitCount != 0 {
					t.Errorf("result[0].CommitCount = %d, want 0", result[0].CommitCount)
				}
				if result[1].CommitCount != 0 {
					t.Errorf("result[1].CommitCount = %d, want 0", result[1].CommitCount)
				}
			},
		},
		{
			name: "missing task in commit counts defaults to zero",
			tasks: []PlannedTask{
				{ID: "task-1"},
				{ID: "task-2"},
				{ID: "task-3"},
			},
			commitCounts: map[string]int{
				"task-1": 3,
				// task-2 missing
				"task-3": 1,
			},
			validate: func(t *testing.T, result []prompt.TaskInfo) {
				if len(result) != 3 {
					t.Fatalf("expected 3 items, got %d", len(result))
				}
				if result[0].CommitCount != 3 {
					t.Errorf("result[0].CommitCount = %d, want 3", result[0].CommitCount)
				}
				if result[1].CommitCount != 0 {
					t.Errorf("result[1].CommitCount = %d, want 0 (missing from map)", result[1].CommitCount)
				}
				if result[2].CommitCount != 1 {
					t.Errorf("result[2].CommitCount = %d, want 1", result[2].CommitCount)
				}
			},
		},
		{
			name: "preserves all task fields",
			tasks: []PlannedTask{
				{
					ID:            "full-task",
					Title:         "Full Task",
					Description:   "Description here",
					Files:         []string{"a.go", "b.go"},
					DependsOn:     []string{"dep-1", "dep-2"},
					Priority:      10,
					EstComplexity: ComplexityMedium,
					IssueURL:      "https://example.com/issue",
				},
			},
			commitCounts: map[string]int{"full-task": 7},
			validate: func(t *testing.T, result []prompt.TaskInfo) {
				if len(result) != 1 {
					t.Fatalf("expected 1 item, got %d", len(result))
				}

				task := result[0]
				if task.ID != "full-task" {
					t.Errorf("ID = %q, want full-task", task.ID)
				}
				if task.Title != "Full Task" {
					t.Errorf("Title = %q, want 'Full Task'", task.Title)
				}
				if task.Description != "Description here" {
					t.Errorf("Description = %q, want 'Description here'", task.Description)
				}
				if len(task.Files) != 2 {
					t.Errorf("Files length = %d, want 2", len(task.Files))
				}
				if len(task.DependsOn) != 2 {
					t.Errorf("DependsOn length = %d, want 2", len(task.DependsOn))
				}
				if task.Priority != 10 {
					t.Errorf("Priority = %d, want 10", task.Priority)
				}
				if task.EstComplexity != "medium" {
					t.Errorf("EstComplexity = %q, want 'medium'", task.EstComplexity)
				}
				if task.IssueURL != "https://example.com/issue" {
					t.Errorf("IssueURL = %q, want URL", task.IssueURL)
				}
				if task.CommitCount != 7 {
					t.Errorf("CommitCount = %d, want 7", task.CommitCount)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TaskInfoSliceFromPlannedTasks(tt.tasks, tt.commitCounts)

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}
