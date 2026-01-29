package prompt

import (
	"strings"
	"testing"
)

func TestConsolidationBuilder_Build(t *testing.T) {
	tests := []struct {
		name        string
		ctx         *Context
		wantErr     bool
		errContains string
		contains    []string
	}{
		{
			name: "valid context with full info",
			ctx: &Context{
				Phase:     PhaseConsolidation,
				SessionID: "test-session",
				Objective: "Build authentication",
				Plan: &PlanInfo{
					Summary: "Auth plan",
					Tasks: []TaskInfo{
						{ID: "task-1", Title: "Create model"},
						{ID: "task-2", Title: "Add endpoint"},
					},
					ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
				},
				Consolidation: &ConsolidationInfo{
					Mode:         "stacked",
					BranchPrefix: "Iron-Ham",
					MainBranch:   "main",
					TaskWorktrees: []TaskWorktreeInfo{
						{TaskID: "task-1", TaskTitle: "Create model", Branch: "branch-1", WorktreePath: "/path/1"},
						{TaskID: "task-2", TaskTitle: "Add endpoint", Branch: "branch-2", WorktreePath: "/path/2"},
					},
				},
				Synthesis: &SynthesisInfo{
					Notes:           []string{"All tasks completed"},
					Recommendations: []string{"Merge in order"},
				},
			},
			wantErr: false,
			contains: []string{
				"## Original Objective",
				"Build authentication",
				"## Branch Configuration",
				"Branch Prefix**: Iron-Ham",
				"Main Branch**: main",
				"Consolidation Mode**: stacked",
				"## Execution Groups",
				"Group 1",
				"Group 2",
				"## Task Worktrees",
				"Create model",
				"Add endpoint",
				"## Synthesis Context",
				"All tasks completed",
				"Merge in order",
				"## Completion Protocol",
				ConsolidationCompletionFileName,
			},
		},
		{
			name: "context with pre-consolidated branches",
			ctx: &Context{
				Phase:     PhaseConsolidation,
				SessionID: "test-session",
				Objective: "Test",
				Plan: &PlanInfo{
					ExecutionOrder: [][]string{{"t1"}, {"t2"}},
				},
				Consolidation: &ConsolidationInfo{
					Mode:                  "stacked",
					BranchPrefix:          "prefix",
					MainBranch:            "main",
					GroupBranches:         []string{"group-1-branch", ""},
					PreConsolidatedBranch: "pre-consolidated",
				},
			},
			wantErr: false,
			contains: []string{
				"ALREADY MERGED",
				"group-1-branch",
				"Pre-Consolidated Branches",
				"pre-consolidated",
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
				Phase:         PhaseConsolidation,
				SessionID:     "test-session",
				Consolidation: &ConsolidationInfo{},
			},
			wantErr:     true,
			errContains: "plan",
		},
		{
			name: "missing consolidation info",
			ctx: &Context{
				Phase:     PhaseConsolidation,
				SessionID: "test-session",
				Plan:      &PlanInfo{},
			},
			wantErr:     true,
			errContains: "consolidation",
		},
		{
			name: "wrong phase",
			ctx: &Context{
				Phase:         PhaseTask,
				SessionID:     "test-session",
				Plan:          &PlanInfo{},
				Consolidation: &ConsolidationInfo{},
			},
			wantErr:     true,
			errContains: "invalid",
		},
	}

	builder := NewConsolidationBuilder()

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

func TestGroupConsolidatorBuilder_Build(t *testing.T) {
	tests := []struct {
		name        string
		ctx         *Context
		wantErr     bool
		errContains string
		contains    []string
	}{
		{
			name: "valid context for group 0",
			ctx: &Context{
				Phase:      PhaseConsolidation,
				SessionID:  "abcd1234-5678",
				Objective:  "Build feature",
				GroupIndex: 0,
				Plan: &PlanInfo{
					Summary: "Feature plan",
					Tasks: []TaskInfo{
						{ID: "task-1", Title: "Task One"},
						{ID: "task-2", Title: "Task Two"},
					},
					ExecutionOrder: [][]string{{"task-1", "task-2"}},
				},
				Consolidation: &ConsolidationInfo{
					Mode:         "stacked",
					BranchPrefix: "Iron-Ham",
					MainBranch:   "main",
					TaskWorktrees: []TaskWorktreeInfo{
						{TaskID: "task-1", TaskTitle: "Task One", Branch: "branch-1", WorktreePath: "/path/1"},
						{TaskID: "task-2", TaskTitle: "Task Two", Branch: "branch-2", WorktreePath: "/path/2"},
					},
				},
			},
			wantErr: false,
			contains: []string{
				"# Group 1 Consolidation",
				"## Part of Ultra-Plan: Feature plan",
				"## Tasks Completed in This Group",
				"### task-1: Task One",
				"### task-2: Task Two",
				"## Branch Configuration",
				"Base branch**: `main`",
				"Iron-Ham/ultraplan-abcd1234-group-1",
				"Task branches to consolidate**: 2",
				"## Your Tasks",
				"git checkout -b",
				"Cherry-pick commits",
				"Run verification",
				"## Conflict Resolution Guidelines",
				"## Completion Protocol",
				GroupConsolidationCompletionFileName,
				`"group_index": 0`,
			},
		},
		{
			name: "valid context for group 1 with previous group context",
			ctx: &Context{
				Phase:      PhaseConsolidation,
				SessionID:  "test-session",
				Objective:  "Build feature",
				GroupIndex: 1,
				Plan: &PlanInfo{
					Summary:        "Feature plan",
					ExecutionOrder: [][]string{{"t1"}, {"t2"}},
				},
				Consolidation: &ConsolidationInfo{
					MainBranch:    "main",
					GroupBranches: []string{"group-1-branch"},
				},
				PreviousGroup: &GroupContext{
					GroupIndex:         0,
					Notes:              "Database setup complete",
					IssuesForNextGroup: []string{"Watch for connection limits"},
					VerificationPassed: true,
				},
			},
			wantErr: false,
			contains: []string{
				"# Group 2 Consolidation",
				"## Context from Previous Group's Consolidator",
				"Database setup complete",
				"Watch for connection limits",
				"Base branch**: `group-1-branch`",
			},
		},
		{
			name: "uses default branch prefix when not specified",
			ctx: &Context{
				Phase:      PhaseConsolidation,
				SessionID:  "test1234",
				GroupIndex: 0,
				Plan: &PlanInfo{
					ExecutionOrder: [][]string{{"t1"}},
				},
				Consolidation: &ConsolidationInfo{
					MainBranch: "main",
				},
			},
			wantErr: false,
			contains: []string{
				"Iron-Ham/ultraplan-test1234-group-1",
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
				Phase:      PhaseConsolidation,
				SessionID:  "test",
				GroupIndex: 0,
			},
			wantErr:     true,
			errContains: "plan",
		},
		{
			name: "negative group index",
			ctx: &Context{
				Phase:         PhaseConsolidation,
				SessionID:     "test",
				GroupIndex:    -1,
				Plan:          &PlanInfo{},
				Consolidation: &ConsolidationInfo{},
			},
			wantErr:     true,
			errContains: "non-negative",
		},
		{
			name: "wrong phase",
			ctx: &Context{
				Phase:      PhaseTask,
				SessionID:  "test",
				GroupIndex: 0,
				Plan:       &PlanInfo{},
			},
			wantErr:     true,
			errContains: "invalid",
		},
	}

	builder := NewGroupConsolidatorBuilder()

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

func TestGroupConsolidatorBuilder_getTaskBranchesForGroup(t *testing.T) {
	builder := NewGroupConsolidatorBuilder()

	ctx := &Context{
		GroupIndex: 0,
		Plan: &PlanInfo{
			ExecutionOrder: [][]string{{"t1", "t2"}, {"t3"}},
		},
		Consolidation: &ConsolidationInfo{
			TaskWorktrees: []TaskWorktreeInfo{
				{TaskID: "t1", TaskTitle: "Task 1"},
				{TaskID: "t2", TaskTitle: "Task 2"},
				{TaskID: "t3", TaskTitle: "Task 3"},
			},
		},
	}

	// Test group 0
	branches := builder.getTaskBranchesForGroup(ctx)
	if len(branches) != 2 {
		t.Errorf("getTaskBranchesForGroup() returned %d branches, want 2", len(branches))
	}

	// Test group 1
	ctx.GroupIndex = 1
	branches = builder.getTaskBranchesForGroup(ctx)
	if len(branches) != 1 {
		t.Errorf("getTaskBranchesForGroup() returned %d branches, want 1", len(branches))
	}
	if branches[0].TaskID != "t3" {
		t.Errorf("getTaskBranchesForGroup() returned wrong task, got %s", branches[0].TaskID)
	}
}

func TestNewConsolidationBuilder(t *testing.T) {
	builder := NewConsolidationBuilder()
	if builder == nil {
		t.Error("NewConsolidationBuilder() returned nil")
	}
}

func TestNewGroupConsolidatorBuilder(t *testing.T) {
	builder := NewGroupConsolidatorBuilder()
	if builder == nil {
		t.Error("NewGroupConsolidatorBuilder() returned nil")
	}
}

func TestConsolidationBuilder_CompletionProtocol(t *testing.T) {
	builder := NewConsolidationBuilder()

	ctx := &Context{
		Phase:     PhaseConsolidation,
		SessionID: "test-session",
		Objective: "Test consolidation",
		Plan: &PlanInfo{
			Summary:        "Test plan",
			ExecutionOrder: [][]string{{"t1"}},
		},
		Consolidation: &ConsolidationInfo{
			Mode:         "stacked",
			BranchPrefix: "test",
			MainBranch:   "main",
		},
	}

	result, err := builder.Build(ctx)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Verify emphatic completion protocol wording
	expectedParts := []string{
		"## Completion Protocol - FINAL MANDATORY STEP",
		"FINAL MANDATORY ACTION",
		"orchestrator is BLOCKED waiting",
		"DO NOT",
		"wait for user prompting",
		"Write this file AUTOMATICALLY",
		"REMEMBER",
		"Your consolidation is NOT complete until you write this file",
		// Structural elements
		ConsolidationCompletionFileName,
		`"status": "complete"`,
		`"mode":`,
		`"branches_created":`,
		`"prs_created":`,
		`"verification":`,
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("Completion protocol missing %q", part)
		}
	}
}

func TestGroupConsolidatorBuilder_CompletionProtocol(t *testing.T) {
	builder := NewGroupConsolidatorBuilder()

	ctx := &Context{
		Phase:      PhaseConsolidation,
		SessionID:  "test-session",
		GroupIndex: 0,
		Plan: &PlanInfo{
			Summary:        "Test plan",
			ExecutionOrder: [][]string{{"t1"}},
		},
		Consolidation: &ConsolidationInfo{
			MainBranch: "main",
		},
	}

	result, err := builder.Build(ctx)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Verify emphatic completion protocol wording
	expectedParts := []string{
		"## Completion Protocol - FINAL MANDATORY STEP",
		"FINAL MANDATORY ACTION",
		"orchestrator is BLOCKED waiting",
		"DO NOT",
		"wait for user prompting",
		"Write this file AUTOMATICALLY",
		"REMEMBER",
		"Your consolidation is NOT complete until you write this file",
		// Structural elements
		GroupConsolidationCompletionFileName,
		`"group_index":`,
		`"status": "complete"`,
		`"branch_name":`,
		`"tasks_consolidated":`,
		`"verification":`,
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("Completion protocol missing %q", part)
		}
	}
}

func TestConsolidationCompletionFileNames(t *testing.T) {
	if ConsolidationCompletionFileName == "" {
		t.Error("ConsolidationCompletionFileName should not be empty")
	}
	if !strings.HasSuffix(ConsolidationCompletionFileName, ".json") {
		t.Errorf("ConsolidationCompletionFileName should end with .json")
	}

	if GroupConsolidationCompletionFileName == "" {
		t.Error("GroupConsolidationCompletionFileName should not be empty")
	}
	if !strings.HasSuffix(GroupConsolidationCompletionFileName, ".json") {
		t.Errorf("GroupConsolidationCompletionFileName should end with .json")
	}
}
