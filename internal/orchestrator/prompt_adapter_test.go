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

func TestRevisionInfoFromState(t *testing.T) {
	tests := []struct {
		name  string
		state *RevisionState
		want  *prompt.RevisionInfo
	}{
		{
			name:  "nil state returns nil",
			state: nil,
			want:  nil,
		},
		{
			name: "empty state",
			state: &RevisionState{
				Issues:          nil,
				RevisionRound:   0,
				MaxRevisions:    0,
				TasksToRevise:   nil,
				RevisedTasks:    nil,
				RevisionPrompts: nil,
			},
			want: &prompt.RevisionInfo{
				Round:         0,
				MaxRounds:     0,
				Issues:        nil,
				TasksToRevise: nil,
				RevisedTasks:  nil,
			},
		},
		{
			name: "full state conversion",
			state: &RevisionState{
				Issues: []RevisionIssue{
					{
						TaskID:      "task-1",
						Description: "Tests are failing",
						Files:       []string{"src/feature.go"},
						Severity:    "critical",
						Suggestion:  "Fix the test assertions",
					},
					{
						TaskID:      "task-2",
						Description: "Missing error handling",
						Files:       []string{"api/handler.go", "api/handler_test.go"},
						Severity:    "major",
						Suggestion:  "Add proper error returns",
					},
				},
				RevisionRound: 2,
				MaxRevisions:  5,
				TasksToRevise: []string{"task-1", "task-2"},
				RevisedTasks:  []string{"task-3"},
			},
			want: &prompt.RevisionInfo{
				Round:     2,
				MaxRounds: 5,
				Issues: []prompt.RevisionIssue{
					{
						TaskID:      "task-1",
						Description: "Tests are failing",
						Files:       []string{"src/feature.go"},
						Severity:    "critical",
						Suggestion:  "Fix the test assertions",
					},
					{
						TaskID:      "task-2",
						Description: "Missing error handling",
						Files:       []string{"api/handler.go", "api/handler_test.go"},
						Severity:    "major",
						Suggestion:  "Add proper error returns",
					},
				},
				TasksToRevise: []string{"task-1", "task-2"},
				RevisedTasks:  []string{"task-3"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := revisionInfoFromState(tt.state)

			if tt.want == nil {
				if got != nil {
					t.Errorf("revisionInfoFromState() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Error("revisionInfoFromState() = nil, want non-nil")
				return
			}

			if got.Round != tt.want.Round {
				t.Errorf("revisionInfoFromState().Round = %d, want %d", got.Round, tt.want.Round)
			}
			if got.MaxRounds != tt.want.MaxRounds {
				t.Errorf("revisionInfoFromState().MaxRounds = %d, want %d", got.MaxRounds, tt.want.MaxRounds)
			}
			if len(got.Issues) != len(tt.want.Issues) {
				t.Errorf("revisionInfoFromState().Issues length = %d, want %d", len(got.Issues), len(tt.want.Issues))
			}
			if len(got.TasksToRevise) != len(tt.want.TasksToRevise) {
				t.Errorf("revisionInfoFromState().TasksToRevise length = %d, want %d", len(got.TasksToRevise), len(tt.want.TasksToRevise))
			}
			if len(got.RevisedTasks) != len(tt.want.RevisedTasks) {
				t.Errorf("revisionInfoFromState().RevisedTasks length = %d, want %d", len(got.RevisedTasks), len(tt.want.RevisedTasks))
			}
		})
	}
}

func TestRevisionIssueFromOrchestratorIssue(t *testing.T) {
	tests := []struct {
		name  string
		issue RevisionIssue
		want  prompt.RevisionIssue
	}{
		{
			name:  "empty issue",
			issue: RevisionIssue{},
			want:  prompt.RevisionIssue{},
		},
		{
			name: "full issue conversion",
			issue: RevisionIssue{
				TaskID:      "task-123",
				Description: "Integration tests are broken",
				Files:       []string{"tests/integration_test.go", "pkg/client.go"},
				Severity:    "critical",
				Suggestion:  "Update mock expectations to match new API response format",
			},
			want: prompt.RevisionIssue{
				TaskID:      "task-123",
				Description: "Integration tests are broken",
				Files:       []string{"tests/integration_test.go", "pkg/client.go"},
				Severity:    "critical",
				Suggestion:  "Update mock expectations to match new API response format",
			},
		},
		{
			name: "cross-cutting issue with empty task ID",
			issue: RevisionIssue{
				TaskID:      "",
				Description: "Inconsistent error handling across modules",
				Severity:    "major",
			},
			want: prompt.RevisionIssue{
				TaskID:      "",
				Description: "Inconsistent error handling across modules",
				Severity:    "major",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := revisionIssueFromOrchestratorIssue(tt.issue)

			if got.TaskID != tt.want.TaskID {
				t.Errorf("revisionIssueFromOrchestratorIssue().TaskID = %q, want %q", got.TaskID, tt.want.TaskID)
			}
			if got.Description != tt.want.Description {
				t.Errorf("revisionIssueFromOrchestratorIssue().Description = %q, want %q", got.Description, tt.want.Description)
			}
			if got.Severity != tt.want.Severity {
				t.Errorf("revisionIssueFromOrchestratorIssue().Severity = %q, want %q", got.Severity, tt.want.Severity)
			}
			if got.Suggestion != tt.want.Suggestion {
				t.Errorf("revisionIssueFromOrchestratorIssue().Suggestion = %q, want %q", got.Suggestion, tt.want.Suggestion)
			}
			if len(got.Files) != len(tt.want.Files) {
				t.Errorf("revisionIssueFromOrchestratorIssue().Files length = %d, want %d", len(got.Files), len(tt.want.Files))
			}
		})
	}
}

func TestRevisionIssuesFromOrchestrator(t *testing.T) {
	tests := []struct {
		name   string
		issues []RevisionIssue
		want   []prompt.RevisionIssue
	}{
		{
			name:   "nil issues returns nil",
			issues: nil,
			want:   nil,
		},
		{
			name:   "empty issues returns empty slice",
			issues: []RevisionIssue{},
			want:   []prompt.RevisionIssue{},
		},
		{
			name: "multiple issues converted",
			issues: []RevisionIssue{
				{TaskID: "task-1", Description: "Issue 1", Severity: "minor"},
				{TaskID: "task-2", Description: "Issue 2", Severity: "major"},
				{TaskID: "", Description: "Cross-cutting issue", Severity: "critical"},
			},
			want: []prompt.RevisionIssue{
				{TaskID: "task-1", Description: "Issue 1", Severity: "minor"},
				{TaskID: "task-2", Description: "Issue 2", Severity: "major"},
				{TaskID: "", Description: "Cross-cutting issue", Severity: "critical"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := revisionIssuesFromOrchestrator(tt.issues)

			if tt.want == nil {
				if got != nil {
					t.Errorf("revisionIssuesFromOrchestrator() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Errorf("revisionIssuesFromOrchestrator() = nil, want %v", tt.want)
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("revisionIssuesFromOrchestrator() length = %d, want %d", len(got), len(tt.want))
				return
			}

			for i := range got {
				if got[i].TaskID != tt.want[i].TaskID {
					t.Errorf("revisionIssuesFromOrchestrator()[%d].TaskID = %q, want %q", i, got[i].TaskID, tt.want[i].TaskID)
				}
				if got[i].Description != tt.want[i].Description {
					t.Errorf("revisionIssuesFromOrchestrator()[%d].Description = %q, want %q", i, got[i].Description, tt.want[i].Description)
				}
			}
		})
	}
}

func TestSynthesisInfoFromCompletion(t *testing.T) {
	tests := []struct {
		name       string
		completion *SynthesisCompletionFile
		want       *prompt.SynthesisInfo
	}{
		{
			name:       "nil completion returns nil",
			completion: nil,
			want:       nil,
		},
		{
			name:       "empty completion",
			completion: &SynthesisCompletionFile{},
			want: &prompt.SynthesisInfo{
				Notes:           nil,
				Recommendations: nil,
				Issues:          []string{},
			},
		},
		{
			name: "completion with integration notes",
			completion: &SynthesisCompletionFile{
				IntegrationNotes: "All components integrate well, no conflicts detected",
			},
			want: &prompt.SynthesisInfo{
				Notes:           []string{"All components integrate well, no conflicts detected"},
				Recommendations: nil,
				Issues:          []string{},
			},
		},
		{
			name: "completion with recommendations",
			completion: &SynthesisCompletionFile{
				Recommendations: []string{
					"Merge task-1 before task-2",
					"Run integration tests after consolidation",
				},
			},
			want: &prompt.SynthesisInfo{
				Notes: nil,
				Recommendations: []string{
					"Merge task-1 before task-2",
					"Run integration tests after consolidation",
				},
				Issues: []string{},
			},
		},
		{
			name: "completion with issues",
			completion: &SynthesisCompletionFile{
				IssuesFound: []RevisionIssue{
					{Description: "Duplicate function definitions", Severity: "major"},
					{Description: "Missing test coverage", Severity: "minor"},
				},
			},
			want: &prompt.SynthesisInfo{
				Notes:           nil,
				Recommendations: nil,
				Issues: []string{
					"Duplicate function definitions",
					"Missing test coverage",
				},
			},
		},
		{
			name: "full completion conversion",
			completion: &SynthesisCompletionFile{
				Status:        "needs_revision",
				RevisionRound: 1,
				IssuesFound: []RevisionIssue{
					{
						TaskID:      "task-auth",
						Description: "Authentication middleware has race condition",
						Files:       []string{"middleware/auth.go"},
						Severity:    "critical",
						Suggestion:  "Use mutex to protect shared state",
					},
					{
						TaskID:      "task-api",
						Description: "API response format inconsistent",
						Files:       []string{"api/response.go"},
						Severity:    "major",
						Suggestion:  "Standardize on JSON envelope format",
					},
				},
				TasksAffected:    []string{"task-auth", "task-api"},
				IntegrationNotes: "Tasks compiled successfully but have runtime issues",
				Recommendations: []string{
					"Fix critical auth issue first",
					"Consider adding integration tests for auth flow",
				},
			},
			want: &prompt.SynthesisInfo{
				Notes: []string{"Tasks compiled successfully but have runtime issues"},
				Recommendations: []string{
					"Fix critical auth issue first",
					"Consider adding integration tests for auth flow",
				},
				Issues: []string{
					"Authentication middleware has race condition",
					"API response format inconsistent",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := synthesisInfoFromCompletion(tt.completion)

			if tt.want == nil {
				if got != nil {
					t.Errorf("synthesisInfoFromCompletion() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Error("synthesisInfoFromCompletion() = nil, want non-nil")
				return
			}

			// Check Notes
			if len(got.Notes) != len(tt.want.Notes) {
				t.Errorf("synthesisInfoFromCompletion().Notes length = %d, want %d", len(got.Notes), len(tt.want.Notes))
			} else {
				for i := range got.Notes {
					if got.Notes[i] != tt.want.Notes[i] {
						t.Errorf("synthesisInfoFromCompletion().Notes[%d] = %q, want %q", i, got.Notes[i], tt.want.Notes[i])
					}
				}
			}

			// Check Recommendations
			if len(got.Recommendations) != len(tt.want.Recommendations) {
				t.Errorf("synthesisInfoFromCompletion().Recommendations length = %d, want %d", len(got.Recommendations), len(tt.want.Recommendations))
			} else {
				for i := range got.Recommendations {
					if got.Recommendations[i] != tt.want.Recommendations[i] {
						t.Errorf("synthesisInfoFromCompletion().Recommendations[%d] = %q, want %q", i, got.Recommendations[i], tt.want.Recommendations[i])
					}
				}
			}

			// Check Issues
			if len(got.Issues) != len(tt.want.Issues) {
				t.Errorf("synthesisInfoFromCompletion().Issues length = %d, want %d", len(got.Issues), len(tt.want.Issues))
			} else {
				for i := range got.Issues {
					if got.Issues[i] != tt.want.Issues[i] {
						t.Errorf("synthesisInfoFromCompletion().Issues[%d] = %q, want %q", i, got.Issues[i], tt.want.Issues[i])
					}
				}
			}
		})
	}
}
