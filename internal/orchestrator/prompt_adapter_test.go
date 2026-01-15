package orchestrator

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator/prompt"
)

func TestNewPromptAdapter(t *testing.T) {
	tests := []struct {
		name        string
		coordinator *Coordinator
	}{
		{
			name:        "creates adapter with coordinator",
			coordinator: &Coordinator{},
		},
		{
			name:        "creates adapter with nil coordinator",
			coordinator: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewPromptAdapter(tt.coordinator)
			if adapter == nil {
				t.Fatal("NewPromptAdapter() returned nil")
			}
			if adapter.coordinator != tt.coordinator {
				t.Errorf("NewPromptAdapter() coordinator = %v, want %v", adapter.coordinator, tt.coordinator)
			}
		})
	}
}

func TestPlanInfoFromPlanSpec(t *testing.T) {
	tests := []struct {
		name string
		spec *PlanSpec
		want *prompt.PlanInfo
	}{
		{
			name: "nil spec returns nil",
			spec: nil,
			want: nil,
		},
		{
			name: "empty spec",
			spec: &PlanSpec{},
			want: &prompt.PlanInfo{},
		},
		{
			name: "full spec conversion",
			spec: &PlanSpec{
				ID:      "plan-123",
				Summary: "Test plan summary",
				Tasks: []PlannedTask{
					{
						ID:            "task-1",
						Title:         "First task",
						Description:   "Do the first thing",
						Files:         []string{"file1.go", "file2.go"},
						DependsOn:     []string{},
						Priority:      1,
						EstComplexity: ComplexityLow,
						IssueURL:      "https://github.com/org/repo/issues/1",
					},
					{
						ID:            "task-2",
						Title:         "Second task",
						Description:   "Do the second thing",
						Files:         []string{"file3.go"},
						DependsOn:     []string{"task-1"},
						Priority:      2,
						EstComplexity: ComplexityHigh,
					},
				},
				ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
				Insights:       []string{"insight 1", "insight 2"},
				Constraints:    []string{"constraint 1"},
			},
			want: &prompt.PlanInfo{
				ID:      "plan-123",
				Summary: "Test plan summary",
				Tasks: []prompt.TaskInfo{
					{
						ID:            "task-1",
						Title:         "First task",
						Description:   "Do the first thing",
						Files:         []string{"file1.go", "file2.go"},
						DependsOn:     []string{},
						Priority:      1,
						EstComplexity: "low",
						IssueURL:      "https://github.com/org/repo/issues/1",
					},
					{
						ID:            "task-2",
						Title:         "Second task",
						Description:   "Do the second thing",
						Files:         []string{"file3.go"},
						DependsOn:     []string{"task-1"},
						Priority:      2,
						EstComplexity: "high",
					},
				},
				ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
				Insights:       []string{"insight 1", "insight 2"},
				Constraints:    []string{"constraint 1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := planInfoFromPlanSpec(tt.spec)

			if tt.want == nil {
				if got != nil {
					t.Errorf("planInfoFromPlanSpec() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Error("planInfoFromPlanSpec() = nil, want non-nil")
				return
			}

			if got.ID != tt.want.ID {
				t.Errorf("planInfoFromPlanSpec().ID = %q, want %q", got.ID, tt.want.ID)
			}
			if got.Summary != tt.want.Summary {
				t.Errorf("planInfoFromPlanSpec().Summary = %q, want %q", got.Summary, tt.want.Summary)
			}
			if len(got.Tasks) != len(tt.want.Tasks) {
				t.Errorf("planInfoFromPlanSpec().Tasks length = %d, want %d", len(got.Tasks), len(tt.want.Tasks))
			}
			if len(got.ExecutionOrder) != len(tt.want.ExecutionOrder) {
				t.Errorf("planInfoFromPlanSpec().ExecutionOrder length = %d, want %d", len(got.ExecutionOrder), len(tt.want.ExecutionOrder))
			}
			if len(got.Insights) != len(tt.want.Insights) {
				t.Errorf("planInfoFromPlanSpec().Insights length = %d, want %d", len(got.Insights), len(tt.want.Insights))
			}
			if len(got.Constraints) != len(tt.want.Constraints) {
				t.Errorf("planInfoFromPlanSpec().Constraints length = %d, want %d", len(got.Constraints), len(tt.want.Constraints))
			}
		})
	}
}

func TestTaskInfoFromPlannedTask(t *testing.T) {
	tests := []struct {
		name string
		task PlannedTask
		want prompt.TaskInfo
	}{
		{
			name: "empty task",
			task: PlannedTask{},
			want: prompt.TaskInfo{},
		},
		{
			name: "full task conversion",
			task: PlannedTask{
				ID:            "task-123",
				Title:         "Implement feature X",
				Description:   "Detailed description of feature X implementation",
				Files:         []string{"src/feature.go", "src/feature_test.go"},
				DependsOn:     []string{"task-001", "task-002"},
				Priority:      5,
				EstComplexity: ComplexityMedium,
				IssueURL:      "https://linear.app/team/issue/123",
			},
			want: prompt.TaskInfo{
				ID:            "task-123",
				Title:         "Implement feature X",
				Description:   "Detailed description of feature X implementation",
				Files:         []string{"src/feature.go", "src/feature_test.go"},
				DependsOn:     []string{"task-001", "task-002"},
				Priority:      5,
				EstComplexity: "medium",
				IssueURL:      "https://linear.app/team/issue/123",
				CommitCount:   0, // Not available from PlannedTask
			},
		},
		{
			name: "low complexity",
			task: PlannedTask{
				ID:            "task-low",
				EstComplexity: ComplexityLow,
			},
			want: prompt.TaskInfo{
				ID:            "task-low",
				EstComplexity: "low",
			},
		},
		{
			name: "high complexity",
			task: PlannedTask{
				ID:            "task-high",
				EstComplexity: ComplexityHigh,
			},
			want: prompt.TaskInfo{
				ID:            "task-high",
				EstComplexity: "high",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := taskInfoFromPlannedTask(tt.task)

			if got.ID != tt.want.ID {
				t.Errorf("taskInfoFromPlannedTask().ID = %q, want %q", got.ID, tt.want.ID)
			}
			if got.Title != tt.want.Title {
				t.Errorf("taskInfoFromPlannedTask().Title = %q, want %q", got.Title, tt.want.Title)
			}
			if got.Description != tt.want.Description {
				t.Errorf("taskInfoFromPlannedTask().Description = %q, want %q", got.Description, tt.want.Description)
			}
			if got.Priority != tt.want.Priority {
				t.Errorf("taskInfoFromPlannedTask().Priority = %d, want %d", got.Priority, tt.want.Priority)
			}
			if got.EstComplexity != tt.want.EstComplexity {
				t.Errorf("taskInfoFromPlannedTask().EstComplexity = %q, want %q", got.EstComplexity, tt.want.EstComplexity)
			}
			if got.IssueURL != tt.want.IssueURL {
				t.Errorf("taskInfoFromPlannedTask().IssueURL = %q, want %q", got.IssueURL, tt.want.IssueURL)
			}
			if got.CommitCount != tt.want.CommitCount {
				t.Errorf("taskInfoFromPlannedTask().CommitCount = %d, want %d", got.CommitCount, tt.want.CommitCount)
			}

			// Check slices
			if len(got.Files) != len(tt.want.Files) {
				t.Errorf("taskInfoFromPlannedTask().Files length = %d, want %d", len(got.Files), len(tt.want.Files))
			}
			if len(got.DependsOn) != len(tt.want.DependsOn) {
				t.Errorf("taskInfoFromPlannedTask().DependsOn length = %d, want %d", len(got.DependsOn), len(tt.want.DependsOn))
			}
		})
	}
}

func TestTasksFromPlanSpec(t *testing.T) {
	tests := []struct {
		name  string
		tasks []PlannedTask
		want  []prompt.TaskInfo
	}{
		{
			name:  "nil tasks returns nil",
			tasks: nil,
			want:  nil,
		},
		{
			name:  "empty tasks returns empty slice",
			tasks: []PlannedTask{},
			want:  []prompt.TaskInfo{},
		},
		{
			name: "single task",
			tasks: []PlannedTask{
				{
					ID:            "task-1",
					Title:         "Task one",
					EstComplexity: ComplexityLow,
				},
			},
			want: []prompt.TaskInfo{
				{
					ID:            "task-1",
					Title:         "Task one",
					EstComplexity: "low",
				},
			},
		},
		{
			name: "multiple tasks",
			tasks: []PlannedTask{
				{
					ID:            "task-1",
					Title:         "First",
					EstComplexity: ComplexityLow,
				},
				{
					ID:            "task-2",
					Title:         "Second",
					EstComplexity: ComplexityMedium,
					DependsOn:     []string{"task-1"},
				},
				{
					ID:            "task-3",
					Title:         "Third",
					EstComplexity: ComplexityHigh,
					DependsOn:     []string{"task-1", "task-2"},
				},
			},
			want: []prompt.TaskInfo{
				{
					ID:            "task-1",
					Title:         "First",
					EstComplexity: "low",
				},
				{
					ID:            "task-2",
					Title:         "Second",
					EstComplexity: "medium",
					DependsOn:     []string{"task-1"},
				},
				{
					ID:            "task-3",
					Title:         "Third",
					EstComplexity: "high",
					DependsOn:     []string{"task-1", "task-2"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tasksFromPlanSpec(tt.tasks)

			if tt.want == nil {
				if got != nil {
					t.Errorf("tasksFromPlanSpec() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Errorf("tasksFromPlanSpec() = nil, want %v", tt.want)
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("tasksFromPlanSpec() length = %d, want %d", len(got), len(tt.want))
				return
			}

			for i := range got {
				if got[i].ID != tt.want[i].ID {
					t.Errorf("tasksFromPlanSpec()[%d].ID = %q, want %q", i, got[i].ID, tt.want[i].ID)
				}
				if got[i].Title != tt.want[i].Title {
					t.Errorf("tasksFromPlanSpec()[%d].Title = %q, want %q", i, got[i].Title, tt.want[i].Title)
				}
				if got[i].EstComplexity != tt.want[i].EstComplexity {
					t.Errorf("tasksFromPlanSpec()[%d].EstComplexity = %q, want %q", i, got[i].EstComplexity, tt.want[i].EstComplexity)
				}
			}
		})
	}
}
