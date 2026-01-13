package prompt

import (
	"strings"
	"testing"
)

func TestRevisionBuilder_Build(t *testing.T) {
	tests := []struct {
		name        string
		ctx         *Context
		wantErr     bool
		errContains string
		contains    []string
	}{
		{
			name: "valid context with issues",
			ctx: &Context{
				Phase:     PhaseRevision,
				SessionID: "test-session",
				Objective: "Build authentication system",
				Plan:      &PlanInfo{Summary: "Auth plan"},
				Task: &TaskInfo{
					ID:          "task-1",
					Title:       "Implement login",
					Description: "Create login functionality",
				},
				Revision: &RevisionInfo{
					Round:     0,
					MaxRounds: 3,
					Issues: []RevisionIssue{
						{
							TaskID:      "task-1",
							Description: "Missing input validation",
							Severity:    "major",
							Files:       []string{"auth.go"},
							Suggestion:  "Add validation for email format",
						},
						{
							TaskID:      "task-1",
							Description: "No error handling",
							Severity:    "critical",
							Files:       []string{"handler.go", "service.go"},
						},
					},
				},
			},
			wantErr: false,
			contains: []string{
				"## Original Objective",
				"Build authentication system",
				"## Task Being Revised",
				"Task ID: task-1",
				"Task Title: Implement login",
				"Create login functionality",
				"Revision Round: 1",
				"## Issues to Address",
				"1. **major**: Missing input validation",
				"Files: auth.go",
				"Suggestion: Add validation for email format",
				"2. **critical**: No error handling",
				"Files: handler.go, service.go",
				"## Instructions",
				"## Completion Protocol",
				RevisionCompletionFileName,
			},
		},
		{
			name: "filters issues for specific task",
			ctx: &Context{
				Phase:     PhaseRevision,
				SessionID: "test-session",
				Objective: "Test",
				Plan:      &PlanInfo{},
				Task: &TaskInfo{
					ID:          "task-2",
					Title:       "Task 2",
					Description: "Description",
				},
				Revision: &RevisionInfo{
					Issues: []RevisionIssue{
						{TaskID: "task-1", Description: "Issue for task 1"},
						{TaskID: "task-2", Description: "Issue for task 2"},
						{TaskID: "", Description: "Global issue"},
					},
				},
			},
			wantErr: false,
			contains: []string{
				"Issue for task 2",
				"Global issue",
			},
		},
		{
			name: "includes global issues (empty task ID)",
			ctx: &Context{
				Phase:     PhaseRevision,
				SessionID: "test-session",
				Objective: "Test",
				Plan:      &PlanInfo{},
				Task: &TaskInfo{
					ID:          "task-1",
					Title:       "Task 1",
					Description: "Description",
				},
				Revision: &RevisionInfo{
					Issues: []RevisionIssue{
						{TaskID: "", Description: "Cross-cutting issue", Severity: "minor"},
					},
				},
			},
			wantErr: false,
			contains: []string{
				"Cross-cutting issue",
			},
		},
		{
			name:        "nil context",
			ctx:         nil,
			wantErr:     true,
			errContains: "nil",
		},
		{
			name: "missing plan",
			ctx: &Context{
				Phase:     PhaseRevision,
				SessionID: "test-session",
				Objective: "Test",
				Task:      &TaskInfo{ID: "t1", Title: "T"},
				Revision:  &RevisionInfo{},
			},
			wantErr:     true,
			errContains: "plan",
		},
		{
			name: "missing task",
			ctx: &Context{
				Phase:     PhaseRevision,
				SessionID: "test-session",
				Objective: "Test",
				Plan:      &PlanInfo{},
				Revision:  &RevisionInfo{},
			},
			wantErr:     true,
			errContains: "task",
		},
		{
			name: "missing revision info",
			ctx: &Context{
				Phase:     PhaseRevision,
				SessionID: "test-session",
				Objective: "Test",
				Plan:      &PlanInfo{},
				Task:      &TaskInfo{ID: "t1", Title: "T"},
			},
			wantErr:     true,
			errContains: "revision",
		},
		{
			name: "missing objective",
			ctx: &Context{
				Phase:     PhaseRevision,
				SessionID: "test-session",
				Plan:      &PlanInfo{},
				Task:      &TaskInfo{ID: "t1", Title: "T"},
				Revision:  &RevisionInfo{},
			},
			wantErr:     true,
			errContains: "objective",
		},
		{
			name: "wrong phase",
			ctx: &Context{
				Phase:     PhaseTask,
				SessionID: "test-session",
				Objective: "Test",
				Plan:      &PlanInfo{},
				Task:      &TaskInfo{ID: "t1", Title: "T"},
				Revision:  &RevisionInfo{},
			},
			wantErr:     true,
			errContains: "invalid",
		},
	}

	builder := NewRevisionBuilder()

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

func TestRevisionBuilder_CompletionProtocol(t *testing.T) {
	builder := NewRevisionBuilder()

	ctx := &Context{
		Phase:     PhaseRevision,
		SessionID: "test-session",
		Objective: "Test",
		Plan:      &PlanInfo{},
		Task: &TaskInfo{
			ID:          "unique-task",
			Title:       "Task",
			Description: "Description",
		},
		Revision: &RevisionInfo{Round: 1},
	}

	result, err := builder.Build(ctx)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	expectedParts := []string{
		"## Completion Protocol",
		RevisionCompletionFileName,
		`"task_id": "unique-task"`,
		`"revision_round": 2`,
		`"status": "complete"`,
		`"issues_addressed":`,
		`"remaining_issues":`,
		`"notes":`,
		"partial",
		"blocked",
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("Completion protocol missing %q", part)
		}
	}
}

func TestNewRevisionBuilder(t *testing.T) {
	builder := NewRevisionBuilder()
	if builder == nil {
		t.Error("NewRevisionBuilder() returned nil")
	}
}

func TestRevisionCompletionFileName(t *testing.T) {
	if RevisionCompletionFileName == "" {
		t.Error("RevisionCompletionFileName should not be empty")
	}
	if !strings.HasSuffix(RevisionCompletionFileName, ".json") {
		t.Errorf("RevisionCompletionFileName should end with .json, got %q", RevisionCompletionFileName)
	}
}
