package prompt

import (
	"strings"
	"testing"
)

func TestTaskBuilder_Build(t *testing.T) {
	tests := []struct {
		name        string
		ctx         *Context
		wantErr     bool
		errContains string
		contains    []string
		notContains []string
	}{
		{
			name: "valid context with minimal task",
			ctx: &Context{
				Phase:     PhaseTask,
				SessionID: "test-session",
				Objective: "Build feature",
				Plan: &PlanInfo{
					Summary: "Feature implementation plan",
				},
				Task: &TaskInfo{
					ID:          "task-1",
					Title:       "Setup database",
					Description: "Create the database schema and models",
				},
			},
			wantErr: false,
			contains: []string{
				"# Task: Setup database",
				"## Part of Ultra-Plan: Feature implementation plan",
				"## Your Task",
				"Create the database schema and models",
				"## Guidelines",
				"Focus only on this specific task",
				"## Completion Protocol",
				TaskCompletionFileName,
				`"task_id": "task-1"`,
			},
			notContains: []string{
				"## Expected Files",
				"## Context from Previous Group",
			},
		},
		{
			name: "valid context with files",
			ctx: &Context{
				Phase:     PhaseTask,
				SessionID: "test-session",
				Objective: "Build feature",
				Plan:      &PlanInfo{Summary: "Plan"},
				Task: &TaskInfo{
					ID:          "task-2",
					Title:       "Implement handler",
					Description: "Create HTTP handlers",
					Files:       []string{"internal/handler.go", "internal/handler_test.go"},
				},
			},
			wantErr: false,
			contains: []string{
				"## Expected Files",
				"internal/handler.go",
				"internal/handler_test.go",
			},
		},
		{
			name: "valid context with previous group context",
			ctx: &Context{
				Phase:      PhaseTask,
				SessionID:  "test-session",
				Objective:  "Build feature",
				GroupIndex: 1,
				Plan:       &PlanInfo{Summary: "Plan"},
				Task: &TaskInfo{
					ID:          "task-3",
					Title:       "Add tests",
					Description: "Add integration tests",
				},
				PreviousGroup: &GroupContext{
					GroupIndex:         0,
					Notes:              "Database schema ready",
					IssuesForNextGroup: []string{"Watch for connection pooling", "Consider caching"},
					VerificationPassed: true,
				},
			},
			wantErr: false,
			contains: []string{
				"## Context from Previous Group",
				"builds on work consolidated from Group 1",
				"**Consolidator Notes**: Database schema ready",
				"**Important**:",
				"Watch for connection pooling",
				"Consider caching",
				"has been verified (build/lint/tests passed)",
			},
		},
		{
			name: "previous group context with failed verification",
			ctx: &Context{
				Phase:      PhaseTask,
				SessionID:  "test-session",
				Objective:  "Build feature",
				GroupIndex: 1,
				Plan:       &PlanInfo{Summary: "Plan"},
				Task: &TaskInfo{
					ID:          "task-4",
					Title:       "Task after failed verification",
					Description: "Description",
				},
				PreviousGroup: &GroupContext{
					GroupIndex:         0,
					VerificationPassed: false,
				},
			},
			wantErr: false,
			contains: []string{
				"**Warning**: The previous group's code verification may have issues",
			},
			notContains: []string{
				"has been verified",
			},
		},
		{
			name: "group 0 task ignores previous group context",
			ctx: &Context{
				Phase:      PhaseTask,
				SessionID:  "test-session",
				Objective:  "Build feature",
				GroupIndex: 0,
				Plan:       &PlanInfo{Summary: "Plan"},
				Task: &TaskInfo{
					ID:          "task-1",
					Title:       "First task",
					Description: "First group task",
				},
				PreviousGroup: &GroupContext{
					Notes: "Should not appear",
				},
			},
			wantErr: false,
			notContains: []string{
				"## Context from Previous Group",
				"Should not appear",
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
				Phase:     PhaseTask,
				SessionID: "test-session",
				Objective: "Test",
				Task:      &TaskInfo{ID: "t1", Title: "Task"},
			},
			wantErr:     true,
			errContains: "plan",
		},
		{
			name: "missing task",
			ctx: &Context{
				Phase:     PhaseTask,
				SessionID: "test-session",
				Objective: "Test",
				Plan:      &PlanInfo{Summary: "Plan"},
			},
			wantErr:     true,
			errContains: "task",
		},
		{
			name: "missing task ID",
			ctx: &Context{
				Phase:     PhaseTask,
				SessionID: "test-session",
				Objective: "Test",
				Plan:      &PlanInfo{Summary: "Plan"},
				Task:      &TaskInfo{Title: "Task"},
			},
			wantErr:     true,
			errContains: "task id",
		},
		{
			name: "missing task title",
			ctx: &Context{
				Phase:     PhaseTask,
				SessionID: "test-session",
				Objective: "Test",
				Plan:      &PlanInfo{Summary: "Plan"},
				Task:      &TaskInfo{ID: "t1"},
			},
			wantErr:     true,
			errContains: "task title",
		},
		{
			name: "wrong phase",
			ctx: &Context{
				Phase:     PhasePlanning,
				SessionID: "test-session",
				Objective: "Test",
				Plan:      &PlanInfo{Summary: "Plan"},
				Task:      &TaskInfo{ID: "t1", Title: "Task"},
			},
			wantErr:     true,
			errContains: "invalid",
		},
	}

	builder := NewTaskBuilder()

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

			for _, notWant := range tt.notContains {
				if strings.Contains(result, notWant) {
					t.Errorf("Build() result should not contain %q\nGot:\n%s", notWant, result)
				}
			}
		})
	}
}

func TestTaskBuilder_CompletionProtocol(t *testing.T) {
	builder := NewTaskBuilder()

	ctx := &Context{
		Phase:     PhaseTask,
		SessionID: "test-session",
		Objective: "Test",
		Plan:      &PlanInfo{Summary: "Plan"},
		Task: &TaskInfo{
			ID:          "unique-task-id",
			Title:       "Test Task",
			Description: "Test description",
		},
	}

	result, err := builder.Build(ctx)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Verify completion protocol structure
	expectedParts := []string{
		// Emphatic completion protocol wording
		"## Completion Protocol - FINAL MANDATORY STEP",
		"FINAL MANDATORY ACTION",
		"orchestrator is BLOCKED waiting",
		"DO NOT",
		"wait for user prompting",
		"Write this file AUTOMATICALLY",
		"REMEMBER",
		"Your task is NOT complete until you write this file",
		// Structural elements
		TaskCompletionFileName,
		`"task_id": "unique-task-id"`,
		`"status": "complete"`,
		`"summary":`,
		`"files_modified":`,
		`"notes":`,
		`"issues":`,
		`"suggestions":`,
		`"dependencies":`,
		"status \"blocked\"",
		"\"failed\"",
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("Completion protocol missing %q", part)
		}
	}
}

func TestNewTaskBuilder(t *testing.T) {
	builder := NewTaskBuilder()
	if builder == nil {
		t.Error("NewTaskBuilder() returned nil")
	}
}

func TestTaskCompletionFileName(t *testing.T) {
	if TaskCompletionFileName == "" {
		t.Error("TaskCompletionFileName should not be empty")
	}
	if !strings.HasSuffix(TaskCompletionFileName, ".json") {
		t.Errorf("TaskCompletionFileName should end with .json, got %q", TaskCompletionFileName)
	}
}
