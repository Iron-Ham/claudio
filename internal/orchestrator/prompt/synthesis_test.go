package prompt

import (
	"strings"
	"testing"
)

func TestSynthesisBuilder_Build(t *testing.T) {
	tests := []struct {
		name        string
		ctx         *Context
		wantErr     bool
		errContains string
		contains    []string
	}{
		{
			name: "valid context with completed tasks",
			ctx: &Context{
				Phase:     PhaseSynthesis,
				SessionID: "test-session",
				Objective: "Build authentication system",
				Plan: &PlanInfo{
					Summary: "Auth implementation",
					Tasks: []TaskInfo{
						{ID: "task-1", Title: "Create user model", CommitCount: 2},
						{ID: "task-2", Title: "Add login endpoint", CommitCount: 3},
					},
				},
				CompletedTasks: []string{"task-1", "task-2"},
			},
			wantErr: false,
			contains: []string{
				"## Original Objective",
				"Build authentication system",
				"## Completed Tasks",
				"[task-1] Create user model (2 commits)",
				"[task-2] Add login endpoint (3 commits)",
				"## Task Results Summary",
				"### Create user model",
				"### Add login endpoint",
				"## Instructions",
				"Review",
				"Identify",
				"## Completion Protocol",
				SynthesisCompletionFileName,
			},
		},
		{
			name: "task with no commits shows warning",
			ctx: &Context{
				Phase:     PhaseSynthesis,
				SessionID: "test-session",
				Objective: "Test objective",
				Plan: &PlanInfo{
					Tasks: []TaskInfo{
						{ID: "task-1", Title: "Task with no commits", CommitCount: 0},
					},
				},
				CompletedTasks: []string{"task-1"},
			},
			wantErr: false,
			contains: []string{
				"NO COMMITS - verify this task",
			},
		},
		{
			name: "task with files shows them",
			ctx: &Context{
				Phase:     PhaseSynthesis,
				SessionID: "test-session",
				Objective: "Test",
				Plan: &PlanInfo{
					Tasks: []TaskInfo{
						{
							ID:          "task-1",
							Title:       "File task",
							CommitCount: 1,
							Files:       []string{"file1.go", "file2.go"},
						},
					},
				},
				CompletedTasks: []string{"task-1"},
			},
			wantErr: false,
			contains: []string{
				"Files: file1.go, file2.go",
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
				Phase:     PhaseSynthesis,
				SessionID: "test-session",
				Objective: "Test",
			},
			wantErr:     true,
			errContains: "plan",
		},
		{
			name: "missing objective",
			ctx: &Context{
				Phase:     PhaseSynthesis,
				SessionID: "test-session",
				Plan:      &PlanInfo{},
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
			},
			wantErr:     true,
			errContains: "invalid",
		},
	}

	builder := NewSynthesisBuilder()

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

func TestSynthesisBuilder_CompletionProtocol(t *testing.T) {
	builder := NewSynthesisBuilder()

	ctx := &Context{
		Phase:     PhaseSynthesis,
		SessionID: "test-session",
		Objective: "Test",
		Plan: &PlanInfo{
			Tasks: []TaskInfo{{ID: "t1", Title: "Task 1"}},
		},
		CompletedTasks: []string{"t1"},
	}

	result, err := builder.Build(ctx)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	expectedParts := []string{
		// Emphatic completion protocol wording
		"## Completion Protocol - FINAL MANDATORY STEP",
		"FINAL MANDATORY ACTION",
		"orchestrator is BLOCKED waiting",
		"DO NOT",
		"wait for user prompting",
		"Write this file AUTOMATICALLY",
		"REMEMBER",
		"Your review is NOT complete until you write this file",
		// Structural elements
		SynthesisCompletionFileName,
		`"status": "complete"`,
		`"revision_round":`,
		`"issues_found":`,
		`"task_id":`,
		`"description":`,
		`"severity":`,
		`"tasks_affected":`,
		`"integration_notes":`,
		`"recommendations":`,
		"needs_revision",
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("Completion protocol missing %q", part)
		}
	}
}

func TestNewSynthesisBuilder(t *testing.T) {
	builder := NewSynthesisBuilder()
	if builder == nil {
		t.Error("NewSynthesisBuilder() returned nil")
	}
}

func TestSynthesisCompletionFileName(t *testing.T) {
	if SynthesisCompletionFileName == "" {
		t.Error("SynthesisCompletionFileName should not be empty")
	}
	if !strings.HasSuffix(SynthesisCompletionFileName, ".json") {
		t.Errorf("SynthesisCompletionFileName should end with .json, got %q", SynthesisCompletionFileName)
	}
}
