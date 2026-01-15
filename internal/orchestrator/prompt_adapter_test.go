package orchestrator

import (
	"errors"
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

func TestGroupContextFromCompletion(t *testing.T) {
	tests := []struct {
		name       string
		completion *GroupConsolidationCompletionFile
		want       *prompt.GroupContext
	}{
		{
			name:       "nil completion returns nil",
			completion: nil,
			want:       nil,
		},
		{
			name:       "empty completion",
			completion: &GroupConsolidationCompletionFile{},
			want: &prompt.GroupContext{
				GroupIndex:         0,
				Notes:              "",
				IssuesForNextGroup: nil,
				VerificationPassed: false,
			},
		},
		{
			name: "full completion with passing verification",
			completion: &GroupConsolidationCompletionFile{
				GroupIndex:         1,
				Status:             "complete",
				BranchName:         "Iron-Ham/ultraplan-abc123-group-2",
				TasksConsolidated:  []string{"task-1", "task-2"},
				Notes:              "Consolidated successfully with minor conflict resolution",
				IssuesForNextGroup: []string{"Watch for API changes in auth module", "Database schema pending migration"},
				Verification: VerificationResult{
					ProjectType:    "go",
					OverallSuccess: true,
					Summary:        "All checks passed",
				},
			},
			want: &prompt.GroupContext{
				GroupIndex:         1,
				Notes:              "Consolidated successfully with minor conflict resolution",
				IssuesForNextGroup: []string{"Watch for API changes in auth module", "Database schema pending migration"},
				VerificationPassed: true,
			},
		},
		{
			name: "completion with failing verification",
			completion: &GroupConsolidationCompletionFile{
				GroupIndex:        2,
				Status:            "complete",
				BranchName:        "Iron-Ham/ultraplan-xyz789-group-3",
				TasksConsolidated: []string{"task-5"},
				Notes:             "Merged but tests are failing",
				Verification: VerificationResult{
					ProjectType:    "go",
					OverallSuccess: false,
					Summary:        "Tests failed: 3 failures",
				},
			},
			want: &prompt.GroupContext{
				GroupIndex:         2,
				Notes:              "Merged but tests are failing",
				IssuesForNextGroup: nil,
				VerificationPassed: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := groupContextFromCompletion(tt.completion)

			if tt.want == nil {
				if got != nil {
					t.Errorf("groupContextFromCompletion() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("groupContextFromCompletion() = nil, want non-nil")
			}

			if got.GroupIndex != tt.want.GroupIndex {
				t.Errorf("GroupIndex = %d, want %d", got.GroupIndex, tt.want.GroupIndex)
			}
			if got.Notes != tt.want.Notes {
				t.Errorf("Notes = %q, want %q", got.Notes, tt.want.Notes)
			}
			if got.VerificationPassed != tt.want.VerificationPassed {
				t.Errorf("VerificationPassed = %v, want %v", got.VerificationPassed, tt.want.VerificationPassed)
			}
			if len(got.IssuesForNextGroup) != len(tt.want.IssuesForNextGroup) {
				t.Errorf("IssuesForNextGroup length = %d, want %d", len(got.IssuesForNextGroup), len(tt.want.IssuesForNextGroup))
			} else {
				for i, issue := range got.IssuesForNextGroup {
					if issue != tt.want.IssuesForNextGroup[i] {
						t.Errorf("IssuesForNextGroup[%d] = %q, want %q", i, issue, tt.want.IssuesForNextGroup[i])
					}
				}
			}
		})
	}
}

func TestConsolidationInfoFromSession(t *testing.T) {
	tests := []struct {
		name       string
		session    *UltraPlanSession
		mainBranch string
		want       *prompt.ConsolidationInfo
	}{
		{
			name:       "nil session returns nil",
			session:    nil,
			mainBranch: "main",
			want:       nil,
		},
		{
			name: "empty session with defaults",
			session: &UltraPlanSession{
				Config: UltraPlanConfig{},
			},
			mainBranch: "",
			want: &prompt.ConsolidationInfo{
				Mode:          "single",
				BranchPrefix:  "Iron-Ham",
				MainBranch:    "main",
				TaskWorktrees: []prompt.TaskWorktreeInfo{},
			},
		},
		{
			name: "session with custom config",
			session: &UltraPlanSession{
				Config: UltraPlanConfig{
					ConsolidationMode: ModeStackedPRs,
					BranchPrefix:      "feature",
				},
				TaskWorktrees: []TaskWorktreeInfo{
					{
						TaskID:       "task-1",
						TaskTitle:    "First task",
						WorktreePath: "/tmp/worktree-1",
						Branch:       "feature/task-1",
					},
				},
				TaskCommitCounts: map[string]int{
					"task-1": 3,
				},
				GroupConsolidatedBranches: []string{"feature/ultraplan-abc-group-1"},
			},
			mainBranch: "develop",
			want: &prompt.ConsolidationInfo{
				Mode:         "stacked",
				BranchPrefix: "feature",
				MainBranch:   "develop",
				TaskWorktrees: []prompt.TaskWorktreeInfo{
					{
						TaskID:       "task-1",
						TaskTitle:    "First task",
						WorktreePath: "/tmp/worktree-1",
						Branch:       "feature/task-1",
						CommitCount:  3,
					},
				},
				GroupBranches:         []string{"feature/ultraplan-abc-group-1"},
				PreConsolidatedBranch: "feature/ultraplan-abc-group-1",
			},
		},
		{
			name: "session with multiple task worktrees",
			session: &UltraPlanSession{
				Config: UltraPlanConfig{
					ConsolidationMode: ModeSinglePR,
					BranchPrefix:      "team",
				},
				TaskWorktrees: []TaskWorktreeInfo{
					{TaskID: "task-a", TaskTitle: "Task A", WorktreePath: "/wt/a", Branch: "team/task-a"},
					{TaskID: "task-b", TaskTitle: "Task B", WorktreePath: "/wt/b", Branch: "team/task-b"},
					{TaskID: "task-c", TaskTitle: "Task C", WorktreePath: "/wt/c", Branch: "team/task-c"},
				},
				TaskCommitCounts: map[string]int{
					"task-a": 1,
					"task-b": 5,
					// task-c has no commits recorded
				},
			},
			mainBranch: "main",
			want: &prompt.ConsolidationInfo{
				Mode:         "single",
				BranchPrefix: "team",
				MainBranch:   "main",
				TaskWorktrees: []prompt.TaskWorktreeInfo{
					{TaskID: "task-a", TaskTitle: "Task A", WorktreePath: "/wt/a", Branch: "team/task-a", CommitCount: 1},
					{TaskID: "task-b", TaskTitle: "Task B", WorktreePath: "/wt/b", Branch: "team/task-b", CommitCount: 5},
					{TaskID: "task-c", TaskTitle: "Task C", WorktreePath: "/wt/c", Branch: "team/task-c", CommitCount: 0},
				},
			},
		},
		{
			name: "session with multiple group branches",
			session: &UltraPlanSession{
				Config: UltraPlanConfig{
					ConsolidationMode: ModeStackedPRs,
				},
				GroupConsolidatedBranches: []string{
					"Iron-Ham/ultraplan-abc-group-1",
					"Iron-Ham/ultraplan-abc-group-2",
					"Iron-Ham/ultraplan-abc-group-3",
				},
			},
			mainBranch: "main",
			want: &prompt.ConsolidationInfo{
				Mode:                  "stacked",
				BranchPrefix:          "Iron-Ham",
				MainBranch:            "main",
				TaskWorktrees:         []prompt.TaskWorktreeInfo{},
				GroupBranches:         []string{"Iron-Ham/ultraplan-abc-group-1", "Iron-Ham/ultraplan-abc-group-2", "Iron-Ham/ultraplan-abc-group-3"},
				PreConsolidatedBranch: "Iron-Ham/ultraplan-abc-group-3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := consolidationInfoFromSession(tt.session, tt.mainBranch)

			if tt.want == nil {
				if got != nil {
					t.Errorf("consolidationInfoFromSession() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("consolidationInfoFromSession() = nil, want non-nil")
			}

			if got.Mode != tt.want.Mode {
				t.Errorf("Mode = %q, want %q", got.Mode, tt.want.Mode)
			}
			if got.BranchPrefix != tt.want.BranchPrefix {
				t.Errorf("BranchPrefix = %q, want %q", got.BranchPrefix, tt.want.BranchPrefix)
			}
			if got.MainBranch != tt.want.MainBranch {
				t.Errorf("MainBranch = %q, want %q", got.MainBranch, tt.want.MainBranch)
			}
			if got.PreConsolidatedBranch != tt.want.PreConsolidatedBranch {
				t.Errorf("PreConsolidatedBranch = %q, want %q", got.PreConsolidatedBranch, tt.want.PreConsolidatedBranch)
			}

			if len(got.TaskWorktrees) != len(tt.want.TaskWorktrees) {
				t.Errorf("TaskWorktrees length = %d, want %d", len(got.TaskWorktrees), len(tt.want.TaskWorktrees))
			} else {
				for i, tw := range got.TaskWorktrees {
					wantTW := tt.want.TaskWorktrees[i]
					if tw.TaskID != wantTW.TaskID {
						t.Errorf("TaskWorktrees[%d].TaskID = %q, want %q", i, tw.TaskID, wantTW.TaskID)
					}
					if tw.CommitCount != wantTW.CommitCount {
						t.Errorf("TaskWorktrees[%d].CommitCount = %d, want %d", i, tw.CommitCount, wantTW.CommitCount)
					}
				}
			}

			if len(got.GroupBranches) != len(tt.want.GroupBranches) {
				t.Errorf("GroupBranches length = %d, want %d", len(got.GroupBranches), len(tt.want.GroupBranches))
			}
		})
	}
}

func TestTaskWorktreeInfoFromOrchestrator(t *testing.T) {
	tests := []struct {
		name         string
		tw           TaskWorktreeInfo
		commitCounts map[string]int
		want         prompt.TaskWorktreeInfo
	}{
		{
			name:         "empty worktree info with nil commit counts",
			tw:           TaskWorktreeInfo{},
			commitCounts: nil,
			want: prompt.TaskWorktreeInfo{
				CommitCount: 0,
			},
		},
		{
			name: "full worktree info with commit count",
			tw: TaskWorktreeInfo{
				TaskID:       "task-123",
				TaskTitle:    "Implement feature X",
				WorktreePath: "/Users/dev/.claudio/worktrees/abc123",
				Branch:       "Iron-Ham/task-123-feature-x",
			},
			commitCounts: map[string]int{
				"task-123": 7,
				"task-456": 3,
			},
			want: prompt.TaskWorktreeInfo{
				TaskID:       "task-123",
				TaskTitle:    "Implement feature X",
				WorktreePath: "/Users/dev/.claudio/worktrees/abc123",
				Branch:       "Iron-Ham/task-123-feature-x",
				CommitCount:  7,
			},
		},
		{
			name: "worktree info with missing commit count",
			tw: TaskWorktreeInfo{
				TaskID:       "task-999",
				TaskTitle:    "New task",
				WorktreePath: "/tmp/wt-999",
				Branch:       "Iron-Ham/task-999",
			},
			commitCounts: map[string]int{
				"task-123": 5,
				// task-999 is not in the map
			},
			want: prompt.TaskWorktreeInfo{
				TaskID:       "task-999",
				TaskTitle:    "New task",
				WorktreePath: "/tmp/wt-999",
				Branch:       "Iron-Ham/task-999",
				CommitCount:  0, // Defaults to 0 when not found
			},
		},
		{
			name: "worktree info with zero commits in map",
			tw: TaskWorktreeInfo{
				TaskID:       "task-zero",
				TaskTitle:    "Zero commit task",
				WorktreePath: "/tmp/wt-zero",
				Branch:       "Iron-Ham/task-zero",
			},
			commitCounts: map[string]int{
				"task-zero": 0,
			},
			want: prompt.TaskWorktreeInfo{
				TaskID:       "task-zero",
				TaskTitle:    "Zero commit task",
				WorktreePath: "/tmp/wt-zero",
				Branch:       "Iron-Ham/task-zero",
				CommitCount:  0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := taskWorktreeInfoFromOrchestrator(tt.tw, tt.commitCounts)

			if got.TaskID != tt.want.TaskID {
				t.Errorf("TaskID = %q, want %q", got.TaskID, tt.want.TaskID)
			}
			if got.TaskTitle != tt.want.TaskTitle {
				t.Errorf("TaskTitle = %q, want %q", got.TaskTitle, tt.want.TaskTitle)
			}
			if got.WorktreePath != tt.want.WorktreePath {
				t.Errorf("WorktreePath = %q, want %q", got.WorktreePath, tt.want.WorktreePath)
			}
			if got.Branch != tt.want.Branch {
				t.Errorf("Branch = %q, want %q", got.Branch, tt.want.Branch)
			}
			if got.CommitCount != tt.want.CommitCount {
				t.Errorf("CommitCount = %d, want %d", got.CommitCount, tt.want.CommitCount)
			}
		})
	}
}

func TestCandidatePlanInfoFromPlanSpec(t *testing.T) {
	tests := []struct {
		name          string
		spec          *PlanSpec
		strategyIndex int
		want          prompt.CandidatePlanInfo
	}{
		{
			name:          "nil spec returns empty CandidatePlanInfo",
			spec:          nil,
			strategyIndex: 0,
			want:          prompt.CandidatePlanInfo{},
		},
		{
			name:          "empty spec returns CandidatePlanInfo with strategy only",
			spec:          &PlanSpec{},
			strategyIndex: 0,
			want: prompt.CandidatePlanInfo{
				Strategy: "maximize-parallelism",
			},
		},
		{
			name: "full spec with maximize-parallelism strategy (index 0)",
			spec: &PlanSpec{
				Summary: "Parallel-focused plan",
				Tasks: []PlannedTask{
					{
						ID:            "task-1",
						Title:         "Independent task A",
						Description:   "Do A",
						EstComplexity: ComplexityLow,
					},
					{
						ID:            "task-2",
						Title:         "Independent task B",
						Description:   "Do B",
						EstComplexity: ComplexityLow,
					},
				},
				ExecutionOrder: [][]string{{"task-1", "task-2"}},
				Insights:       []string{"Maximized parallelism"},
				Constraints:    []string{"No shared files"},
			},
			strategyIndex: 0,
			want: prompt.CandidatePlanInfo{
				Strategy: "maximize-parallelism",
				Summary:  "Parallel-focused plan",
				Tasks: []prompt.TaskInfo{
					{
						ID:            "task-1",
						Title:         "Independent task A",
						Description:   "Do A",
						EstComplexity: "low",
					},
					{
						ID:            "task-2",
						Title:         "Independent task B",
						Description:   "Do B",
						EstComplexity: "low",
					},
				},
				ExecutionOrder: [][]string{{"task-1", "task-2"}},
				Insights:       []string{"Maximized parallelism"},
				Constraints:    []string{"No shared files"},
			},
		},
		{
			name: "spec with minimize-complexity strategy (index 1)",
			spec: &PlanSpec{
				Summary: "Simple sequential plan",
				Tasks: []PlannedTask{
					{
						ID:            "task-1",
						Title:         "First step",
						DependsOn:     []string{},
						EstComplexity: ComplexityLow,
					},
					{
						ID:            "task-2",
						Title:         "Second step",
						DependsOn:     []string{"task-1"},
						EstComplexity: ComplexityLow,
					},
				},
				ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
				Insights:       []string{"Clear linear flow"},
				Constraints:    []string{},
			},
			strategyIndex: 1,
			want: prompt.CandidatePlanInfo{
				Strategy: "minimize-complexity",
				Summary:  "Simple sequential plan",
				Tasks: []prompt.TaskInfo{
					{
						ID:            "task-1",
						Title:         "First step",
						DependsOn:     []string{},
						EstComplexity: "low",
					},
					{
						ID:            "task-2",
						Title:         "Second step",
						DependsOn:     []string{"task-1"},
						EstComplexity: "low",
					},
				},
				ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
				Insights:       []string{"Clear linear flow"},
				Constraints:    []string{},
			},
		},
		{
			name: "spec with balanced-approach strategy (index 2)",
			spec: &PlanSpec{
				Summary: "Balanced plan",
				Tasks: []PlannedTask{
					{
						ID:            "task-1",
						Title:         "Setup",
						EstComplexity: ComplexityMedium,
					},
				},
				ExecutionOrder: [][]string{{"task-1"}},
				Insights:       []string{"Pragmatic approach"},
				Constraints:    []string{"Resource limits"},
			},
			strategyIndex: 2,
			want: prompt.CandidatePlanInfo{
				Strategy: "balanced-approach",
				Summary:  "Balanced plan",
				Tasks: []prompt.TaskInfo{
					{
						ID:            "task-1",
						Title:         "Setup",
						EstComplexity: "medium",
					},
				},
				ExecutionOrder: [][]string{{"task-1"}},
				Insights:       []string{"Pragmatic approach"},
				Constraints:    []string{"Resource limits"},
			},
		},
		{
			name: "negative strategy index results in empty strategy",
			spec: &PlanSpec{
				Summary: "Test plan",
			},
			strategyIndex: -1,
			want: prompt.CandidatePlanInfo{
				Strategy: "",
				Summary:  "Test plan",
			},
		},
		{
			name: "out of bounds strategy index results in empty strategy",
			spec: &PlanSpec{
				Summary: "Test plan",
			},
			strategyIndex: 100,
			want: prompt.CandidatePlanInfo{
				Strategy: "",
				Summary:  "Test plan",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := candidatePlanInfoFromPlanSpec(tt.spec, tt.strategyIndex)

			if got.Strategy != tt.want.Strategy {
				t.Errorf("candidatePlanInfoFromPlanSpec().Strategy = %q, want %q", got.Strategy, tt.want.Strategy)
			}
			if got.Summary != tt.want.Summary {
				t.Errorf("candidatePlanInfoFromPlanSpec().Summary = %q, want %q", got.Summary, tt.want.Summary)
			}

			// Check tasks length
			if len(got.Tasks) != len(tt.want.Tasks) {
				t.Errorf("candidatePlanInfoFromPlanSpec().Tasks length = %d, want %d", len(got.Tasks), len(tt.want.Tasks))
			} else {
				for i := range got.Tasks {
					if got.Tasks[i].ID != tt.want.Tasks[i].ID {
						t.Errorf("candidatePlanInfoFromPlanSpec().Tasks[%d].ID = %q, want %q", i, got.Tasks[i].ID, tt.want.Tasks[i].ID)
					}
					if got.Tasks[i].Title != tt.want.Tasks[i].Title {
						t.Errorf("candidatePlanInfoFromPlanSpec().Tasks[%d].Title = %q, want %q", i, got.Tasks[i].Title, tt.want.Tasks[i].Title)
					}
					if got.Tasks[i].EstComplexity != tt.want.Tasks[i].EstComplexity {
						t.Errorf("candidatePlanInfoFromPlanSpec().Tasks[%d].EstComplexity = %q, want %q", i, got.Tasks[i].EstComplexity, tt.want.Tasks[i].EstComplexity)
					}
				}
			}

			// Check execution order
			if len(got.ExecutionOrder) != len(tt.want.ExecutionOrder) {
				t.Errorf("candidatePlanInfoFromPlanSpec().ExecutionOrder length = %d, want %d", len(got.ExecutionOrder), len(tt.want.ExecutionOrder))
			}

			// Check insights
			if len(got.Insights) != len(tt.want.Insights) {
				t.Errorf("candidatePlanInfoFromPlanSpec().Insights length = %d, want %d", len(got.Insights), len(tt.want.Insights))
			}

			// Check constraints
			if len(got.Constraints) != len(tt.want.Constraints) {
				t.Errorf("candidatePlanInfoFromPlanSpec().Constraints length = %d, want %d", len(got.Constraints), len(tt.want.Constraints))
			}
		})
	}
}

func TestBuildPlanningContext(t *testing.T) {
	tests := []struct {
		name        string
		adapter     *PromptAdapter
		wantErr     error
		wantPhase   prompt.PhaseType
		wantSession string
		wantObj     string
		wantBaseDir string
	}{
		{
			name:    "nil coordinator returns error",
			adapter: NewPromptAdapter(nil),
			wantErr: ErrNilCoordinator,
		},
		{
			name: "nil manager session returns error",
			adapter: NewPromptAdapter(&Coordinator{
				manager: NewUltraPlanManager(nil, nil, nil, nil),
			}),
			wantErr: ErrNilUltraPlanSession,
		},
		{
			name: "valid coordinator with nil base session",
			adapter: NewPromptAdapter(&Coordinator{
				manager: NewUltraPlanManager(nil, nil, &UltraPlanSession{
					ID:        "session-123",
					Objective: "Implement feature X",
				}, nil),
				baseSession: nil,
			}),
			wantErr:     nil,
			wantPhase:   prompt.PhasePlanning,
			wantSession: "session-123",
			wantObj:     "Implement feature X",
			wantBaseDir: "",
		},
		{
			name: "valid coordinator with base session",
			adapter: NewPromptAdapter(&Coordinator{
				manager: NewUltraPlanManager(nil, nil, &UltraPlanSession{
					ID:        "session-456",
					Objective: "Build a REST API",
				}, nil),
				baseSession: &Session{
					BaseRepo: "/path/to/repo",
				},
			}),
			wantErr:     nil,
			wantPhase:   prompt.PhasePlanning,
			wantSession: "session-456",
			wantObj:     "Build a REST API",
			wantBaseDir: "/path/to/repo",
		},
		{
			name: "empty objective fails validation",
			adapter: NewPromptAdapter(&Coordinator{
				manager: NewUltraPlanManager(nil, nil, &UltraPlanSession{
					ID:        "session-789",
					Objective: "",
				}, nil),
				baseSession: &Session{
					BaseRepo: "/path/to/repo",
				},
			}),
			wantErr: prompt.ErrEmptyObjective,
		},
		{
			name: "empty session ID fails validation",
			adapter: NewPromptAdapter(&Coordinator{
				manager: NewUltraPlanManager(nil, nil, &UltraPlanSession{
					ID:        "",
					Objective: "Some objective",
				}, nil),
			}),
			wantErr: prompt.ErrEmptySessionID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, err := tt.adapter.BuildPlanningContext()

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("BuildPlanningContext() error = nil, want %v", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("BuildPlanningContext() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("BuildPlanningContext() unexpected error = %v", err)
			}

			if ctx == nil {
				t.Fatal("BuildPlanningContext() returned nil context")
			}

			if ctx.Phase != tt.wantPhase {
				t.Errorf("BuildPlanningContext().Phase = %v, want %v", ctx.Phase, tt.wantPhase)
			}

			if ctx.SessionID != tt.wantSession {
				t.Errorf("BuildPlanningContext().SessionID = %q, want %q", ctx.SessionID, tt.wantSession)
			}

			if ctx.Objective != tt.wantObj {
				t.Errorf("BuildPlanningContext().Objective = %q, want %q", ctx.Objective, tt.wantObj)
			}

			if ctx.BaseDir != tt.wantBaseDir {
				t.Errorf("BuildPlanningContext().BaseDir = %q, want %q", ctx.BaseDir, tt.wantBaseDir)
			}
		})
	}
}
