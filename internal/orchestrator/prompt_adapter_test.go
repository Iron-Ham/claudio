package orchestrator

import (
	"errors"
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator/group"
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

func TestPlanInfoWithCommitCounts(t *testing.T) {
	tests := []struct {
		name         string
		spec         *PlanSpec
		commitCounts map[string]int
		want         *prompt.PlanInfo
	}{
		{
			name:         "nil spec returns nil",
			spec:         nil,
			commitCounts: nil,
			want:         nil,
		},
		{
			name: "empty commit counts",
			spec: &PlanSpec{
				ID: "plan-1",
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Task 1"},
				},
			},
			commitCounts: nil,
			want: &prompt.PlanInfo{
				ID: "plan-1",
				Tasks: []prompt.TaskInfo{
					{ID: "task-1", Title: "Task 1", CommitCount: 0},
				},
			},
		},
		{
			name: "commit counts populated",
			spec: &PlanSpec{
				ID:      "plan-1",
				Summary: "Test plan",
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Task 1", EstComplexity: ComplexityLow},
					{ID: "task-2", Title: "Task 2", EstComplexity: ComplexityHigh},
					{ID: "task-3", Title: "Task 3", EstComplexity: ComplexityMedium},
				},
				ExecutionOrder: [][]string{{"task-1"}, {"task-2", "task-3"}},
			},
			commitCounts: map[string]int{
				"task-1": 3,
				"task-2": 1,
				// task-3 intentionally missing
			},
			want: &prompt.PlanInfo{
				ID:      "plan-1",
				Summary: "Test plan",
				Tasks: []prompt.TaskInfo{
					{ID: "task-1", Title: "Task 1", EstComplexity: "low", CommitCount: 3},
					{ID: "task-2", Title: "Task 2", EstComplexity: "high", CommitCount: 1},
					{ID: "task-3", Title: "Task 3", EstComplexity: "medium", CommitCount: 0},
				},
				ExecutionOrder: [][]string{{"task-1"}, {"task-2", "task-3"}},
			},
		},
		{
			name: "nil tasks in spec",
			spec: &PlanSpec{
				ID:      "plan-1",
				Summary: "No tasks plan",
				Tasks:   nil,
			},
			commitCounts: map[string]int{"task-1": 5},
			want: &prompt.PlanInfo{
				ID:      "plan-1",
				Summary: "No tasks plan",
				Tasks:   nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := planInfoWithCommitCounts(tt.spec, tt.commitCounts)

			if tt.want == nil {
				if got != nil {
					t.Errorf("planInfoWithCommitCounts() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Error("planInfoWithCommitCounts() = nil, want non-nil")
				return
			}

			if got.ID != tt.want.ID {
				t.Errorf("planInfoWithCommitCounts().ID = %q, want %q", got.ID, tt.want.ID)
			}
			if got.Summary != tt.want.Summary {
				t.Errorf("planInfoWithCommitCounts().Summary = %q, want %q", got.Summary, tt.want.Summary)
			}

			if len(got.Tasks) != len(tt.want.Tasks) {
				t.Errorf("planInfoWithCommitCounts().Tasks length = %d, want %d", len(got.Tasks), len(tt.want.Tasks))
				return
			}

			for i, task := range got.Tasks {
				if task.ID != tt.want.Tasks[i].ID {
					t.Errorf("Tasks[%d].ID = %q, want %q", i, task.ID, tt.want.Tasks[i].ID)
				}
				if task.CommitCount != tt.want.Tasks[i].CommitCount {
					t.Errorf("Tasks[%d].CommitCount = %d, want %d", i, task.CommitCount, tt.want.Tasks[i].CommitCount)
				}
			}
		})
	}
}

func TestBuildPreviousGroupContextStrings(t *testing.T) {
	tests := []struct {
		name     string
		contexts []*GroupConsolidationCompletionFile
		want     []string
	}{
		{
			name:     "nil contexts returns nil",
			contexts: nil,
			want:     nil,
		},
		{
			name:     "empty contexts returns nil",
			contexts: []*GroupConsolidationCompletionFile{},
			want:     nil,
		},
		{
			name: "contexts with only nil entries returns nil",
			contexts: []*GroupConsolidationCompletionFile{
				nil,
				nil,
			},
			want: nil,
		},
		{
			name: "context with notes only",
			contexts: []*GroupConsolidationCompletionFile{
				{
					GroupIndex: 0,
					Notes:      "Group 0 completed successfully",
				},
			},
			want: []string{"Group 0 completed successfully"},
		},
		{
			name: "context with issues only",
			contexts: []*GroupConsolidationCompletionFile{
				{
					GroupIndex:         0,
					IssuesForNextGroup: []string{"Watch out for API changes", "Tests may be flaky"},
				},
			},
			want: []string{"Issues: Watch out for API changes; Tests may be flaky"},
		},
		{
			name: "context with notes and issues",
			contexts: []*GroupConsolidationCompletionFile{
				{
					GroupIndex:         0,
					Notes:              "Consolidated 3 tasks",
					IssuesForNextGroup: []string{"Needs review"},
				},
			},
			want: []string{"Consolidated 3 tasks | Issues: Needs review"},
		},
		{
			name: "multiple contexts",
			contexts: []*GroupConsolidationCompletionFile{
				{
					GroupIndex: 0,
					Notes:      "First group done",
				},
				nil, // Should be skipped
				{
					GroupIndex:         2,
					Notes:              "Third group done",
					IssuesForNextGroup: []string{"Issue A", "Issue B"},
				},
			},
			want: []string{
				"First group done",
				"Third group done | Issues: Issue A; Issue B",
			},
		},
		{
			name: "context with empty notes and empty issues",
			contexts: []*GroupConsolidationCompletionFile{
				{
					GroupIndex:         0,
					Notes:              "",
					IssuesForNextGroup: []string{},
				},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPreviousGroupContextStrings(tt.contexts)

			if tt.want == nil {
				if got != nil {
					t.Errorf("buildPreviousGroupContextStrings() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Errorf("buildPreviousGroupContextStrings() = nil, want %v", tt.want)
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("buildPreviousGroupContextStrings() length = %d, want %d", len(got), len(tt.want))
				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("buildPreviousGroupContextStrings()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestBuildTaskContext(t *testing.T) {
	t.Run("nil coordinator returns error", func(t *testing.T) {
		adapter := NewPromptAdapter(nil)
		ctx, err := adapter.BuildTaskContext("task-1")
		if err != ErrNilCoordinator {
			t.Errorf("BuildTaskContext() error = %v, want %v", err, ErrNilCoordinator)
		}
		if ctx != nil {
			t.Errorf("BuildTaskContext() ctx = %v, want nil", ctx)
		}
	})

	t.Run("nil manager returns error", func(t *testing.T) {
		coordinator := &Coordinator{manager: nil}
		adapter := NewPromptAdapter(coordinator)
		ctx, err := adapter.BuildTaskContext("task-1")
		if err != ErrNilManager {
			t.Errorf("BuildTaskContext() error = %v, want %v", err, ErrNilManager)
		}
		if ctx != nil {
			t.Errorf("BuildTaskContext() ctx = %v, want nil", ctx)
		}
	})

	t.Run("nil session returns error", func(t *testing.T) {
		manager := &UltraPlanManager{session: nil}
		coordinator := &Coordinator{manager: manager}
		adapter := NewPromptAdapter(coordinator)
		ctx, err := adapter.BuildTaskContext("task-1")
		if err != ErrNilSession {
			t.Errorf("BuildTaskContext() error = %v, want %v", err, ErrNilSession)
		}
		if ctx != nil {
			t.Errorf("BuildTaskContext() ctx = %v, want nil", ctx)
		}
	})

	t.Run("nil groupTracker returns error", func(t *testing.T) {
		session := &UltraPlanSession{
			ID:        "session-1",
			Objective: "Test objective",
			Plan: &PlanSpec{
				ID:    "plan-1",
				Tasks: []PlannedTask{{ID: "task-1", Title: "Task 1"}},
			},
		}
		manager := &UltraPlanManager{session: session}
		coordinator := &Coordinator{
			manager:      manager,
			groupTracker: nil, // Explicitly nil
		}
		adapter := NewPromptAdapter(coordinator)
		ctx, err := adapter.BuildTaskContext("task-1")
		if err != ErrNilGroupTracker {
			t.Errorf("BuildTaskContext() error = %v, want %v", err, ErrNilGroupTracker)
		}
		if ctx != nil {
			t.Errorf("BuildTaskContext() ctx = %v, want nil", ctx)
		}
	})

	t.Run("task not found in plan returns error", func(t *testing.T) {
		session := &UltraPlanSession{
			ID:        "session-1",
			Objective: "Test objective",
			Plan: &PlanSpec{
				ID:             "plan-1",
				Tasks:          []PlannedTask{{ID: "task-1", Title: "Task 1"}},
				ExecutionOrder: [][]string{{"task-1"}},
			},
		}
		manager := &UltraPlanManager{session: session}
		tracker := createTestGroupTracker(session)
		coordinator := &Coordinator{
			manager:      manager,
			groupTracker: tracker,
		}
		adapter := NewPromptAdapter(coordinator)
		ctx, err := adapter.BuildTaskContext("nonexistent-task")
		if err != ErrTaskNotFoundInPlan {
			t.Errorf("BuildTaskContext() error = %v, want %v", err, ErrTaskNotFoundInPlan)
		}
		if ctx != nil {
			t.Errorf("BuildTaskContext() ctx = %v, want nil", ctx)
		}
	})

	t.Run("successful task context for group 0", func(t *testing.T) {
		session := &UltraPlanSession{
			ID:        "session-123",
			Objective: "Implement feature X",
			Plan: &PlanSpec{
				ID:      "plan-456",
				Summary: "Feature implementation plan",
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Task 1", Description: "Do task 1", EstComplexity: ComplexityLow},
					{ID: "task-2", Title: "Task 2", Description: "Do task 2", EstComplexity: ComplexityMedium, DependsOn: []string{"task-1"}},
				},
				ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
			},
			CompletedTasks: []string{},
			FailedTasks:    []string{},
		}
		manager := &UltraPlanManager{session: session}
		tracker := createTestGroupTracker(session)
		coordinator := &Coordinator{
			manager:      manager,
			groupTracker: tracker,
		}
		adapter := NewPromptAdapter(coordinator)

		ctx, err := adapter.BuildTaskContext("task-1")
		if err != nil {
			t.Fatalf("BuildTaskContext() error = %v, want nil", err)
		}

		if ctx.Phase != prompt.PhaseTask {
			t.Errorf("ctx.Phase = %v, want %v", ctx.Phase, prompt.PhaseTask)
		}
		if ctx.SessionID != "session-123" {
			t.Errorf("ctx.SessionID = %q, want %q", ctx.SessionID, "session-123")
		}
		if ctx.Objective != "Implement feature X" {
			t.Errorf("ctx.Objective = %q, want %q", ctx.Objective, "Implement feature X")
		}
		if ctx.GroupIndex != 0 {
			t.Errorf("ctx.GroupIndex = %d, want 0", ctx.GroupIndex)
		}

		// Check plan
		if ctx.Plan == nil {
			t.Fatal("ctx.Plan = nil, want non-nil")
		}
		if ctx.Plan.ID != "plan-456" {
			t.Errorf("ctx.Plan.ID = %q, want %q", ctx.Plan.ID, "plan-456")
		}

		// Check task
		if ctx.Task == nil {
			t.Fatal("ctx.Task = nil, want non-nil")
		}
		if ctx.Task.ID != "task-1" {
			t.Errorf("ctx.Task.ID = %q, want %q", ctx.Task.ID, "task-1")
		}
		if ctx.Task.Title != "Task 1" {
			t.Errorf("ctx.Task.Title = %q, want %q", ctx.Task.Title, "Task 1")
		}
		if ctx.Task.Description != "Do task 1" {
			t.Errorf("ctx.Task.Description = %q, want %q", ctx.Task.Description, "Do task 1")
		}

		// For group 0, PreviousGroup should be nil
		if ctx.PreviousGroup != nil {
			t.Errorf("ctx.PreviousGroup = %v, want nil for group 0", ctx.PreviousGroup)
		}
	})

	t.Run("successful task context for group 1 with previous group context", func(t *testing.T) {
		session := &UltraPlanSession{
			ID:        "session-123",
			Objective: "Implement feature X",
			Plan: &PlanSpec{
				ID:      "plan-456",
				Summary: "Feature implementation plan",
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Task 1", EstComplexity: ComplexityLow},
					{ID: "task-2", Title: "Task 2", EstComplexity: ComplexityMedium, DependsOn: []string{"task-1"}},
				},
				ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
			},
			CompletedTasks: []string{"task-1"},
			FailedTasks:    []string{},
			GroupConsolidationContexts: []*GroupConsolidationCompletionFile{
				{
					GroupIndex:         0,
					Notes:              "Group 0 consolidated successfully",
					IssuesForNextGroup: []string{"Watch for API changes"},
					Verification: VerificationResult{
						OverallSuccess: true,
					},
				},
			},
		}
		manager := &UltraPlanManager{session: session}
		tracker := createTestGroupTracker(session)
		coordinator := &Coordinator{
			manager:      manager,
			groupTracker: tracker,
		}
		adapter := NewPromptAdapter(coordinator)

		ctx, err := adapter.BuildTaskContext("task-2")
		if err != nil {
			t.Fatalf("BuildTaskContext() error = %v, want nil", err)
		}

		if ctx.GroupIndex != 1 {
			t.Errorf("ctx.GroupIndex = %d, want 1", ctx.GroupIndex)
		}

		// Check task
		if ctx.Task == nil {
			t.Fatal("ctx.Task = nil, want non-nil")
		}
		if ctx.Task.ID != "task-2" {
			t.Errorf("ctx.Task.ID = %q, want %q", ctx.Task.ID, "task-2")
		}

		// For group 1, PreviousGroup should be populated
		if ctx.PreviousGroup == nil {
			t.Fatal("ctx.PreviousGroup = nil, want non-nil for group 1")
		}
		if ctx.PreviousGroup.GroupIndex != 0 {
			t.Errorf("ctx.PreviousGroup.GroupIndex = %d, want 0", ctx.PreviousGroup.GroupIndex)
		}
		if ctx.PreviousGroup.Notes != "Group 0 consolidated successfully" {
			t.Errorf("ctx.PreviousGroup.Notes = %q, want %q", ctx.PreviousGroup.Notes, "Group 0 consolidated successfully")
		}
		if !ctx.PreviousGroup.VerificationPassed {
			t.Error("ctx.PreviousGroup.VerificationPassed = false, want true")
		}
		if len(ctx.PreviousGroup.IssuesForNextGroup) != 1 || ctx.PreviousGroup.IssuesForNextGroup[0] != "Watch for API changes" {
			t.Errorf("ctx.PreviousGroup.IssuesForNextGroup = %v, want [Watch for API changes]", ctx.PreviousGroup.IssuesForNextGroup)
		}

		// Check completed tasks
		if len(ctx.CompletedTasks) != 1 || ctx.CompletedTasks[0] != "task-1" {
			t.Errorf("ctx.CompletedTasks = %v, want [task-1]", ctx.CompletedTasks)
		}
	})

	t.Run("task in group 1 with no consolidation context", func(t *testing.T) {
		session := &UltraPlanSession{
			ID:        "session-123",
			Objective: "Test objective",
			Plan: &PlanSpec{
				ID: "plan-1",
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Task 1"},
					{ID: "task-2", Title: "Task 2"},
				},
				ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
			},
			CompletedTasks:             []string{"task-1"},
			GroupConsolidationContexts: nil, // No consolidation context yet
		}
		manager := &UltraPlanManager{session: session}
		tracker := createTestGroupTracker(session)
		coordinator := &Coordinator{
			manager:      manager,
			groupTracker: tracker,
		}
		adapter := NewPromptAdapter(coordinator)

		ctx, err := adapter.BuildTaskContext("task-2")
		if err != nil {
			t.Fatalf("BuildTaskContext() error = %v, want nil", err)
		}

		if ctx.GroupIndex != 1 {
			t.Errorf("ctx.GroupIndex = %d, want 1", ctx.GroupIndex)
		}
		// PreviousGroup should be nil since there's no consolidation context
		if ctx.PreviousGroup != nil {
			t.Errorf("ctx.PreviousGroup = %v, want nil when no consolidation context", ctx.PreviousGroup)
		}
	})

	t.Run("validation error for empty session ID", func(t *testing.T) {
		session := &UltraPlanSession{
			ID:        "", // Empty session ID should fail validation
			Objective: "Test objective",
			Plan: &PlanSpec{
				ID:             "plan-1",
				Tasks:          []PlannedTask{{ID: "task-1", Title: "Task 1"}},
				ExecutionOrder: [][]string{{"task-1"}},
			},
		}
		manager := &UltraPlanManager{session: session}
		tracker := createTestGroupTracker(session)
		coordinator := &Coordinator{
			manager:      manager,
			groupTracker: tracker,
		}
		adapter := NewPromptAdapter(coordinator)

		ctx, err := adapter.BuildTaskContext("task-1")
		if !errors.Is(err, prompt.ErrEmptySessionID) {
			t.Errorf("BuildTaskContext() error = %v, want %v", err, prompt.ErrEmptySessionID)
		}
		if ctx != nil {
			t.Errorf("BuildTaskContext() ctx = %v, want nil", ctx)
		}
	})

	t.Run("validation error for empty objective", func(t *testing.T) {
		session := &UltraPlanSession{
			ID:        "session-1",
			Objective: "", // Empty objective should fail validation
			Plan: &PlanSpec{
				ID:             "plan-1",
				Tasks:          []PlannedTask{{ID: "task-1", Title: "Task 1"}},
				ExecutionOrder: [][]string{{"task-1"}},
			},
		}
		manager := &UltraPlanManager{session: session}
		tracker := createTestGroupTracker(session)
		coordinator := &Coordinator{
			manager:      manager,
			groupTracker: tracker,
		}
		adapter := NewPromptAdapter(coordinator)

		ctx, err := adapter.BuildTaskContext("task-1")
		if !errors.Is(err, prompt.ErrEmptyObjective) {
			t.Errorf("BuildTaskContext() error = %v, want %v", err, prompt.ErrEmptyObjective)
		}
		if ctx != nil {
			t.Errorf("BuildTaskContext() ctx = %v, want nil", ctx)
		}
	})

	t.Run("completed and failed tasks are populated", func(t *testing.T) {
		session := &UltraPlanSession{
			ID:        "session-123",
			Objective: "Test objective",
			Plan: &PlanSpec{
				ID: "plan-1",
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Task 1"},
					{ID: "task-2", Title: "Task 2"},
					{ID: "task-3", Title: "Task 3"},
					{ID: "task-4", Title: "Task 4"},
				},
				ExecutionOrder: [][]string{{"task-1", "task-2"}, {"task-3", "task-4"}},
			},
			CompletedTasks: []string{"task-1"},
			FailedTasks:    []string{"task-2"},
		}
		manager := &UltraPlanManager{session: session}
		tracker := createTestGroupTracker(session)
		coordinator := &Coordinator{
			manager:      manager,
			groupTracker: tracker,
		}
		adapter := NewPromptAdapter(coordinator)

		ctx, err := adapter.BuildTaskContext("task-3")
		if err != nil {
			t.Fatalf("BuildTaskContext() error = %v, want nil", err)
		}

		if len(ctx.CompletedTasks) != 1 || ctx.CompletedTasks[0] != "task-1" {
			t.Errorf("ctx.CompletedTasks = %v, want [task-1]", ctx.CompletedTasks)
		}
		if len(ctx.FailedTasks) != 1 || ctx.FailedTasks[0] != "task-2" {
			t.Errorf("ctx.FailedTasks = %v, want [task-2]", ctx.FailedTasks)
		}
	})
}

// createTestGroupTracker creates a group.Tracker for testing using the session adapters.
func createTestGroupTracker(session *UltraPlanSession) *group.Tracker {
	planAdapter := group.NewPlanAdapter(
		func() [][]string {
			if session.Plan == nil {
				return nil
			}
			return session.Plan.ExecutionOrder
		},
		func(taskID string) *group.Task {
			if session.Plan == nil {
				return nil
			}
			for i := range session.Plan.Tasks {
				if session.Plan.Tasks[i].ID == taskID {
					return &group.Task{
						ID:          session.Plan.Tasks[i].ID,
						Title:       session.Plan.Tasks[i].Title,
						Description: session.Plan.Tasks[i].Description,
						Files:       session.Plan.Tasks[i].Files,
						DependsOn:   session.Plan.Tasks[i].DependsOn,
					}
				}
			}
			return nil
		},
	)

	sessionAdapter := group.NewSessionAdapter(
		func() group.PlanData { return planAdapter },
		func() []string { return session.CompletedTasks },
		func() []string { return session.FailedTasks },
		func() map[string]int { return session.TaskCommitCounts },
		func() int { return session.CurrentGroup },
	)

	return group.NewTracker(sessionAdapter)
}

func TestBuildSynthesisContext(t *testing.T) {
	t.Run("nil coordinator returns error", func(t *testing.T) {
		adapter := NewPromptAdapter(nil)
		ctx, err := adapter.BuildSynthesisContext()
		if err != ErrNilCoordinator {
			t.Errorf("BuildSynthesisContext() error = %v, want %v", err, ErrNilCoordinator)
		}
		if ctx != nil {
			t.Errorf("BuildSynthesisContext() ctx = %v, want nil", ctx)
		}
	})

	t.Run("nil manager returns error", func(t *testing.T) {
		coordinator := &Coordinator{manager: nil}
		adapter := NewPromptAdapter(coordinator)
		ctx, err := adapter.BuildSynthesisContext()
		if err != ErrNilManager {
			t.Errorf("BuildSynthesisContext() error = %v, want %v", err, ErrNilManager)
		}
		if ctx != nil {
			t.Errorf("BuildSynthesisContext() ctx = %v, want nil", ctx)
		}
	})

	t.Run("nil session returns error", func(t *testing.T) {
		manager := &UltraPlanManager{session: nil}
		coordinator := &Coordinator{manager: manager}
		adapter := NewPromptAdapter(coordinator)
		ctx, err := adapter.BuildSynthesisContext()
		if err != ErrNilSession {
			t.Errorf("BuildSynthesisContext() error = %v, want %v", err, ErrNilSession)
		}
		if ctx != nil {
			t.Errorf("BuildSynthesisContext() ctx = %v, want nil", ctx)
		}
	})

	t.Run("validation error for empty objective", func(t *testing.T) {
		session := &UltraPlanSession{
			ID:        "session-1",
			Objective: "", // Empty objective should fail validation
			Plan: &PlanSpec{
				ID: "plan-1",
			},
		}
		manager := &UltraPlanManager{session: session}
		coordinator := &Coordinator{manager: manager}
		adapter := NewPromptAdapter(coordinator)

		ctx, err := adapter.BuildSynthesisContext()
		if err == nil {
			t.Error("BuildSynthesisContext() error = nil, want validation error")
		}
		if ctx != nil {
			t.Errorf("BuildSynthesisContext() ctx = %v, want nil", ctx)
		}
	})

	t.Run("successful synthesis context creation", func(t *testing.T) {
		session := &UltraPlanSession{
			ID:        "session-123",
			Objective: "Implement feature X",
			Plan: &PlanSpec{
				ID:      "plan-456",
				Summary: "Feature implementation plan",
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Task 1", EstComplexity: ComplexityLow},
					{ID: "task-2", Title: "Task 2", EstComplexity: ComplexityMedium},
				},
				ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
			},
			CompletedTasks: []string{"task-1"},
			FailedTasks:    []string{},
			TaskCommitCounts: map[string]int{
				"task-1": 2,
			},
			GroupConsolidationContexts: []*GroupConsolidationCompletionFile{
				{
					GroupIndex: 0,
					Notes:      "Group 0 completed",
				},
			},
		}
		manager := &UltraPlanManager{session: session}
		coordinator := &Coordinator{manager: manager}
		adapter := NewPromptAdapter(coordinator)

		ctx, err := adapter.BuildSynthesisContext()
		if err != nil {
			t.Fatalf("BuildSynthesisContext() error = %v, want nil", err)
		}

		if ctx.Phase != prompt.PhaseSynthesis {
			t.Errorf("ctx.Phase = %v, want %v", ctx.Phase, prompt.PhaseSynthesis)
		}
		if ctx.SessionID != "session-123" {
			t.Errorf("ctx.SessionID = %q, want %q", ctx.SessionID, "session-123")
		}
		if ctx.Objective != "Implement feature X" {
			t.Errorf("ctx.Objective = %q, want %q", ctx.Objective, "Implement feature X")
		}

		// Check plan
		if ctx.Plan == nil {
			t.Fatal("ctx.Plan = nil, want non-nil")
		}
		if ctx.Plan.ID != "plan-456" {
			t.Errorf("ctx.Plan.ID = %q, want %q", ctx.Plan.ID, "plan-456")
		}
		if len(ctx.Plan.Tasks) != 2 {
			t.Fatalf("ctx.Plan.Tasks length = %d, want 2", len(ctx.Plan.Tasks))
		}
		// Check commit count was populated
		if ctx.Plan.Tasks[0].CommitCount != 2 {
			t.Errorf("ctx.Plan.Tasks[0].CommitCount = %d, want 2", ctx.Plan.Tasks[0].CommitCount)
		}
		if ctx.Plan.Tasks[1].CommitCount != 0 {
			t.Errorf("ctx.Plan.Tasks[1].CommitCount = %d, want 0", ctx.Plan.Tasks[1].CommitCount)
		}

		// Check completed/failed tasks
		if len(ctx.CompletedTasks) != 1 || ctx.CompletedTasks[0] != "task-1" {
			t.Errorf("ctx.CompletedTasks = %v, want [task-1]", ctx.CompletedTasks)
		}
		if len(ctx.FailedTasks) != 0 {
			t.Errorf("ctx.FailedTasks = %v, want empty", ctx.FailedTasks)
		}

		// Check previous group context
		if len(ctx.PreviousGroupContext) != 1 {
			t.Fatalf("ctx.PreviousGroupContext length = %d, want 1", len(ctx.PreviousGroupContext))
		}
		if ctx.PreviousGroupContext[0] != "Group 0 completed" {
			t.Errorf("ctx.PreviousGroupContext[0] = %q, want %q", ctx.PreviousGroupContext[0], "Group 0 completed")
		}
	})

	t.Run("synthesis context with nil plan fails validation", func(t *testing.T) {
		session := &UltraPlanSession{
			ID:        "session-1",
			Objective: "Test objective",
			Plan:      nil, // Synthesis phase requires a plan
		}
		manager := &UltraPlanManager{session: session}
		coordinator := &Coordinator{manager: manager}
		adapter := NewPromptAdapter(coordinator)

		ctx, err := adapter.BuildSynthesisContext()
		if err == nil {
			t.Error("BuildSynthesisContext() error = nil, want validation error for missing plan")
		}
		if ctx != nil {
			t.Errorf("BuildSynthesisContext() ctx = %v, want nil", ctx)
		}
	})
}

func TestFindTaskByID(t *testing.T) {
	tests := []struct {
		name   string
		plan   *PlanSpec
		taskID string
		want   *PlannedTask
	}{
		{
			name:   "nil plan returns nil",
			plan:   nil,
			taskID: "task-1",
			want:   nil,
		},
		{
			name: "empty task ID returns nil",
			plan: &PlanSpec{
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Task 1"},
				},
			},
			taskID: "",
			want:   nil,
		},
		{
			name: "task not found returns nil",
			plan: &PlanSpec{
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Task 1"},
					{ID: "task-2", Title: "Task 2"},
				},
			},
			taskID: "task-3",
			want:   nil,
		},
		{
			name: "find first task",
			plan: &PlanSpec{
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Task 1", Description: "First task"},
					{ID: "task-2", Title: "Task 2", Description: "Second task"},
				},
			},
			taskID: "task-1",
			want:   &PlannedTask{ID: "task-1", Title: "Task 1", Description: "First task"},
		},
		{
			name: "find last task",
			plan: &PlanSpec{
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Task 1"},
					{ID: "task-2", Title: "Task 2"},
					{ID: "task-3", Title: "Task 3", EstComplexity: ComplexityHigh},
				},
			},
			taskID: "task-3",
			want:   &PlannedTask{ID: "task-3", Title: "Task 3", EstComplexity: ComplexityHigh},
		},
		{
			name: "empty tasks slice returns nil",
			plan: &PlanSpec{
				Tasks: []PlannedTask{},
			},
			taskID: "task-1",
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findTaskByID(tt.plan, tt.taskID)

			if tt.want == nil {
				if got != nil {
					t.Errorf("findTaskByID() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("findTaskByID() = nil, want non-nil")
			}

			if got.ID != tt.want.ID {
				t.Errorf("findTaskByID().ID = %q, want %q", got.ID, tt.want.ID)
			}
			if got.Title != tt.want.Title {
				t.Errorf("findTaskByID().Title = %q, want %q", got.Title, tt.want.Title)
			}
		})
	}
}

func TestBuildRevisionContext(t *testing.T) {
	t.Run("nil coordinator returns error", func(t *testing.T) {
		adapter := NewPromptAdapter(nil)
		ctx, err := adapter.BuildRevisionContext("task-1")
		if err != ErrNilCoordinator {
			t.Errorf("BuildRevisionContext() error = %v, want %v", err, ErrNilCoordinator)
		}
		if ctx != nil {
			t.Errorf("BuildRevisionContext() ctx = %v, want nil", ctx)
		}
	})

	t.Run("nil manager returns error", func(t *testing.T) {
		coordinator := &Coordinator{manager: nil}
		adapter := NewPromptAdapter(coordinator)
		ctx, err := adapter.BuildRevisionContext("task-1")
		if err != ErrNilManager {
			t.Errorf("BuildRevisionContext() error = %v, want %v", err, ErrNilManager)
		}
		if ctx != nil {
			t.Errorf("BuildRevisionContext() ctx = %v, want nil", ctx)
		}
	})

	t.Run("nil session returns error", func(t *testing.T) {
		manager := &UltraPlanManager{session: nil}
		coordinator := &Coordinator{manager: manager}
		adapter := NewPromptAdapter(coordinator)
		ctx, err := adapter.BuildRevisionContext("task-1")
		if err != ErrNilSession {
			t.Errorf("BuildRevisionContext() error = %v, want %v", err, ErrNilSession)
		}
		if ctx != nil {
			t.Errorf("BuildRevisionContext() ctx = %v, want nil", ctx)
		}
	})

	t.Run("task not found returns error", func(t *testing.T) {
		session := &UltraPlanSession{
			ID:        "session-1",
			Objective: "Test objective",
			Plan: &PlanSpec{
				ID: "plan-1",
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Task 1"},
				},
			},
			Revision: &RevisionState{
				RevisionRound: 1,
				MaxRevisions:  3,
			},
		}
		manager := &UltraPlanManager{session: session}
		coordinator := &Coordinator{manager: manager}
		adapter := NewPromptAdapter(coordinator)

		ctx, err := adapter.BuildRevisionContext("nonexistent-task")
		if _, ok := err.(ErrTaskNotFound); !ok {
			t.Errorf("BuildRevisionContext() error = %T, want ErrTaskNotFound", err)
		}
		if ctx != nil {
			t.Errorf("BuildRevisionContext() ctx = %v, want nil", ctx)
		}
	})

	t.Run("validation error for empty objective", func(t *testing.T) {
		session := &UltraPlanSession{
			ID:        "session-1",
			Objective: "", // Empty objective should fail validation
			Plan: &PlanSpec{
				ID: "plan-1",
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Task 1"},
				},
			},
			Revision: &RevisionState{
				RevisionRound: 1,
				MaxRevisions:  3,
			},
		}
		manager := &UltraPlanManager{session: session}
		coordinator := &Coordinator{manager: manager}
		adapter := NewPromptAdapter(coordinator)

		ctx, err := adapter.BuildRevisionContext("task-1")
		if err == nil {
			t.Error("BuildRevisionContext() error = nil, want validation error")
		}
		if ctx != nil {
			t.Errorf("BuildRevisionContext() ctx = %v, want nil", ctx)
		}
	})

	t.Run("validation error for missing revision state", func(t *testing.T) {
		session := &UltraPlanSession{
			ID:        "session-1",
			Objective: "Test objective",
			Plan: &PlanSpec{
				ID: "plan-1",
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Task 1"},
				},
			},
			Revision: nil, // Missing revision state
		}
		manager := &UltraPlanManager{session: session}
		coordinator := &Coordinator{manager: manager}
		adapter := NewPromptAdapter(coordinator)

		ctx, err := adapter.BuildRevisionContext("task-1")
		if err == nil {
			t.Error("BuildRevisionContext() error = nil, want validation error for missing revision")
		}
		if ctx != nil {
			t.Errorf("BuildRevisionContext() ctx = %v, want nil", ctx)
		}
	})

	t.Run("successful revision context creation", func(t *testing.T) {
		session := &UltraPlanSession{
			ID:        "session-123",
			Objective: "Implement feature X",
			Plan: &PlanSpec{
				ID:      "plan-456",
				Summary: "Feature implementation plan",
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Task 1", Description: "First task", EstComplexity: ComplexityLow},
					{ID: "task-2", Title: "Task 2", Description: "Second task", EstComplexity: ComplexityMedium},
				},
				ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
			},
			Revision: &RevisionState{
				RevisionRound: 2,
				MaxRevisions:  5,
				Issues: []RevisionIssue{
					{
						TaskID:      "task-1",
						Description: "Tests failing",
						Files:       []string{"task1.go"},
						Severity:    "critical",
						Suggestion:  "Fix the assertions",
					},
				},
				TasksToRevise: []string{"task-1"},
				RevisedTasks:  []string{},
			},
		}
		manager := &UltraPlanManager{session: session}
		coordinator := &Coordinator{manager: manager}
		adapter := NewPromptAdapter(coordinator)

		ctx, err := adapter.BuildRevisionContext("task-1")
		if err != nil {
			t.Fatalf("BuildRevisionContext() error = %v, want nil", err)
		}

		// Check phase
		if ctx.Phase != prompt.PhaseRevision {
			t.Errorf("ctx.Phase = %v, want %v", ctx.Phase, prompt.PhaseRevision)
		}

		// Check session info
		if ctx.SessionID != "session-123" {
			t.Errorf("ctx.SessionID = %q, want %q", ctx.SessionID, "session-123")
		}
		if ctx.Objective != "Implement feature X" {
			t.Errorf("ctx.Objective = %q, want %q", ctx.Objective, "Implement feature X")
		}

		// Check plan
		if ctx.Plan == nil {
			t.Fatal("ctx.Plan = nil, want non-nil")
		}
		if ctx.Plan.ID != "plan-456" {
			t.Errorf("ctx.Plan.ID = %q, want %q", ctx.Plan.ID, "plan-456")
		}

		// Check task
		if ctx.Task == nil {
			t.Fatal("ctx.Task = nil, want non-nil")
		}
		if ctx.Task.ID != "task-1" {
			t.Errorf("ctx.Task.ID = %q, want %q", ctx.Task.ID, "task-1")
		}
		if ctx.Task.Title != "Task 1" {
			t.Errorf("ctx.Task.Title = %q, want %q", ctx.Task.Title, "Task 1")
		}

		// Check revision info
		if ctx.Revision == nil {
			t.Fatal("ctx.Revision = nil, want non-nil")
		}
		if ctx.Revision.Round != 2 {
			t.Errorf("ctx.Revision.Round = %d, want 2", ctx.Revision.Round)
		}
		if ctx.Revision.MaxRounds != 5 {
			t.Errorf("ctx.Revision.MaxRounds = %d, want 5", ctx.Revision.MaxRounds)
		}
		if len(ctx.Revision.Issues) != 1 {
			t.Fatalf("ctx.Revision.Issues length = %d, want 1", len(ctx.Revision.Issues))
		}
		if ctx.Revision.Issues[0].Description != "Tests failing" {
			t.Errorf("ctx.Revision.Issues[0].Description = %q, want %q", ctx.Revision.Issues[0].Description, "Tests failing")
		}

		// Check synthesis is nil when not set
		if ctx.Synthesis != nil {
			t.Errorf("ctx.Synthesis = %v, want nil", ctx.Synthesis)
		}
	})

	t.Run("revision context with synthesis completion", func(t *testing.T) {
		session := &UltraPlanSession{
			ID:        "session-123",
			Objective: "Implement feature X",
			Plan: &PlanSpec{
				ID: "plan-456",
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Task 1"},
				},
			},
			Revision: &RevisionState{
				RevisionRound: 1,
				MaxRevisions:  3,
			},
			SynthesisCompletion: &SynthesisCompletionFile{
				Status:           "needs_revision",
				IntegrationNotes: "Tasks need fixes",
				Recommendations:  []string{"Fix tests first"},
				IssuesFound: []RevisionIssue{
					{Description: "Build failing"},
				},
			},
		}
		manager := &UltraPlanManager{session: session}
		coordinator := &Coordinator{manager: manager}
		adapter := NewPromptAdapter(coordinator)

		ctx, err := adapter.BuildRevisionContext("task-1")
		if err != nil {
			t.Fatalf("BuildRevisionContext() error = %v, want nil", err)
		}

		// Check synthesis info is populated
		if ctx.Synthesis == nil {
			t.Fatal("ctx.Synthesis = nil, want non-nil")
		}
		if len(ctx.Synthesis.Notes) != 1 || ctx.Synthesis.Notes[0] != "Tasks need fixes" {
			t.Errorf("ctx.Synthesis.Notes = %v, want [Tasks need fixes]", ctx.Synthesis.Notes)
		}
		if len(ctx.Synthesis.Recommendations) != 1 || ctx.Synthesis.Recommendations[0] != "Fix tests first" {
			t.Errorf("ctx.Synthesis.Recommendations = %v, want [Fix tests first]", ctx.Synthesis.Recommendations)
		}
		if len(ctx.Synthesis.Issues) != 1 || ctx.Synthesis.Issues[0] != "Build failing" {
			t.Errorf("ctx.Synthesis.Issues = %v, want [Build failing]", ctx.Synthesis.Issues)
		}
	})

	t.Run("finding different tasks", func(t *testing.T) {
		session := &UltraPlanSession{
			ID:        "session-123",
			Objective: "Test multiple tasks",
			Plan: &PlanSpec{
				ID: "plan-456",
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Task 1", Description: "First"},
					{ID: "task-2", Title: "Task 2", Description: "Second"},
					{ID: "task-3", Title: "Task 3", Description: "Third"},
				},
			},
			Revision: &RevisionState{
				RevisionRound: 1,
				MaxRevisions:  3,
			},
		}
		manager := &UltraPlanManager{session: session}
		coordinator := &Coordinator{manager: manager}
		adapter := NewPromptAdapter(coordinator)

		// Test finding task-2
		ctx, err := adapter.BuildRevisionContext("task-2")
		if err != nil {
			t.Fatalf("BuildRevisionContext(task-2) error = %v, want nil", err)
		}
		if ctx.Task.ID != "task-2" {
			t.Errorf("ctx.Task.ID = %q, want %q", ctx.Task.ID, "task-2")
		}
		if ctx.Task.Description != "Second" {
			t.Errorf("ctx.Task.Description = %q, want %q", ctx.Task.Description, "Second")
		}

		// Test finding task-3
		ctx, err = adapter.BuildRevisionContext("task-3")
		if err != nil {
			t.Fatalf("BuildRevisionContext(task-3) error = %v, want nil", err)
		}
		if ctx.Task.ID != "task-3" {
			t.Errorf("ctx.Task.ID = %q, want %q", ctx.Task.ID, "task-3")
		}
		if ctx.Task.Description != "Third" {
			t.Errorf("ctx.Task.Description = %q, want %q", ctx.Task.Description, "Third")
		}
	})
}
