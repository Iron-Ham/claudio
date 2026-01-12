package prompt

import (
	"errors"
	"testing"
)

func TestContext_Validate(t *testing.T) {
	// Helper to create a minimal valid context for a given phase
	validContext := func(phase PhaseType) *Context {
		ctx := &Context{
			Phase:     phase,
			SessionID: "test-session-123",
			Objective: "Implement feature X",
			BaseDir:   "/path/to/repo",
		}

		// Add phase-specific required fields
		switch phase {
		case PhaseTask:
			ctx.Plan = &PlanInfo{
				ID:      "plan-1",
				Summary: "Test plan",
			}
			ctx.Task = &TaskInfo{
				ID:          "task-1",
				Title:       "Test task",
				Description: "Do something",
			}
		case PhaseSynthesis:
			ctx.Plan = &PlanInfo{
				ID:      "plan-1",
				Summary: "Test plan",
			}
		case PhaseRevision:
			ctx.Plan = &PlanInfo{
				ID:      "plan-1",
				Summary: "Test plan",
			}
			ctx.Task = &TaskInfo{
				ID:          "task-1",
				Title:       "Test task",
				Description: "Do something",
			}
			ctx.Revision = &RevisionInfo{
				Round:     1,
				MaxRounds: 3,
			}
		case PhaseConsolidation:
			ctx.Plan = &PlanInfo{
				ID:      "plan-1",
				Summary: "Test plan",
			}
		}

		return ctx
	}

	tests := []struct {
		name    string
		ctx     *Context
		wantErr error
	}{
		// Nil context
		{
			name:    "nil context returns error",
			ctx:     nil,
			wantErr: ErrNilContext,
		},

		// Empty/invalid phase
		{
			name: "empty phase returns error",
			ctx: &Context{
				SessionID: "test-session",
				Objective: "Test objective",
			},
			wantErr: ErrInvalidPhase,
		},
		{
			name: "invalid phase returns error",
			ctx: &Context{
				Phase:     PhaseType("invalid"),
				SessionID: "test-session",
				Objective: "Test objective",
			},
			wantErr: ErrInvalidPhase,
		},

		// Missing session ID
		{
			name: "empty session ID returns error",
			ctx: &Context{
				Phase:     PhasePlanning,
				Objective: "Test objective",
			},
			wantErr: ErrEmptySessionID,
		},

		// Missing objective
		{
			name: "empty objective returns error",
			ctx: &Context{
				Phase:     PhasePlanning,
				SessionID: "test-session",
			},
			wantErr: ErrEmptyObjective,
		},

		// Planning phase - valid with minimal fields
		{
			name:    "planning phase with minimal fields is valid",
			ctx:     validContext(PhasePlanning),
			wantErr: nil,
		},

		// Plan selection phase - valid with minimal fields
		{
			name:    "plan selection phase with minimal fields is valid",
			ctx:     validContext(PhasePlanSelection),
			wantErr: nil,
		},

		// Task phase - requires plan and task
		{
			name:    "task phase with plan and task is valid",
			ctx:     validContext(PhaseTask),
			wantErr: nil,
		},
		{
			name: "task phase without plan returns error",
			ctx: &Context{
				Phase:     PhaseTask,
				SessionID: "test-session",
				Objective: "Test objective",
				Task: &TaskInfo{
					ID:    "task-1",
					Title: "Test task",
				},
			},
			wantErr: ErrMissingPlan,
		},
		{
			name: "task phase without task returns error",
			ctx: &Context{
				Phase:     PhaseTask,
				SessionID: "test-session",
				Objective: "Test objective",
				Plan: &PlanInfo{
					ID:      "plan-1",
					Summary: "Test plan",
				},
			},
			wantErr: ErrMissingTask,
		},

		// Synthesis phase - requires plan
		{
			name:    "synthesis phase with plan is valid",
			ctx:     validContext(PhaseSynthesis),
			wantErr: nil,
		},
		{
			name: "synthesis phase without plan returns error",
			ctx: &Context{
				Phase:     PhaseSynthesis,
				SessionID: "test-session",
				Objective: "Test objective",
			},
			wantErr: ErrMissingPlan,
		},

		// Revision phase - requires plan, task, and revision info
		{
			name:    "revision phase with all required fields is valid",
			ctx:     validContext(PhaseRevision),
			wantErr: nil,
		},
		{
			name: "revision phase without plan returns error",
			ctx: &Context{
				Phase:     PhaseRevision,
				SessionID: "test-session",
				Objective: "Test objective",
				Task: &TaskInfo{
					ID:    "task-1",
					Title: "Test task",
				},
				Revision: &RevisionInfo{
					Round: 1,
				},
			},
			wantErr: ErrMissingPlan,
		},
		{
			name: "revision phase without task returns error",
			ctx: &Context{
				Phase:     PhaseRevision,
				SessionID: "test-session",
				Objective: "Test objective",
				Plan: &PlanInfo{
					ID:      "plan-1",
					Summary: "Test plan",
				},
				Revision: &RevisionInfo{
					Round: 1,
				},
			},
			wantErr: ErrMissingTask,
		},
		{
			name: "revision phase without revision info returns error",
			ctx: &Context{
				Phase:     PhaseRevision,
				SessionID: "test-session",
				Objective: "Test objective",
				Plan: &PlanInfo{
					ID:      "plan-1",
					Summary: "Test plan",
				},
				Task: &TaskInfo{
					ID:    "task-1",
					Title: "Test task",
				},
			},
			wantErr: ErrMissingRevision,
		},

		// Consolidation phase - requires plan
		{
			name:    "consolidation phase with plan is valid",
			ctx:     validContext(PhaseConsolidation),
			wantErr: nil,
		},
		{
			name: "consolidation phase without plan returns error",
			ctx: &Context{
				Phase:     PhaseConsolidation,
				SessionID: "test-session",
				Objective: "Test objective",
			},
			wantErr: ErrMissingPlan,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ctx.Validate()

			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("Validate() error = %v, want nil", err)
				}
				return
			}

			if err == nil {
				t.Errorf("Validate() error = nil, want %v", tt.wantErr)
				return
			}

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidPhases(t *testing.T) {
	phases := ValidPhases()

	expectedPhases := map[PhaseType]bool{
		PhasePlanning:      true,
		PhaseTask:          true,
		PhaseSynthesis:     true,
		PhaseRevision:      true,
		PhaseConsolidation: true,
		PhasePlanSelection: true,
	}

	if len(phases) != len(expectedPhases) {
		t.Errorf("ValidPhases() returned %d phases, want %d", len(phases), len(expectedPhases))
	}

	for _, phase := range phases {
		if !expectedPhases[phase] {
			t.Errorf("ValidPhases() returned unexpected phase: %s", phase)
		}
	}
}

func TestPhaseType_String(t *testing.T) {
	tests := []struct {
		phase PhaseType
		want  string
	}{
		{PhasePlanning, "planning"},
		{PhaseTask, "task"},
		{PhaseSynthesis, "synthesis"},
		{PhaseRevision, "revision"},
		{PhaseConsolidation, "consolidation"},
		{PhasePlanSelection, "plan_selection"},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			if got := string(tt.phase); got != tt.want {
				t.Errorf("PhaseType string = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContext_FullyPopulated(t *testing.T) {
	// Test that a fully populated context validates successfully for all phases
	ctx := &Context{
		Phase:     PhaseRevision,
		SessionID: "session-abc123",
		Objective: "Refactor the authentication module",
		Plan: &PlanInfo{
			ID:      "plan-xyz",
			Summary: "Comprehensive refactor plan",
			Tasks: []TaskInfo{
				{
					ID:            "task-1",
					Title:         "Extract auth interface",
					Description:   "Create abstraction for auth providers",
					Files:         []string{"auth.go", "auth_test.go"},
					DependsOn:     []string{},
					Priority:      1,
					EstComplexity: "medium",
					CommitCount:   3,
				},
				{
					ID:            "task-2",
					Title:         "Implement JWT provider",
					Description:   "Add JWT-based authentication",
					Files:         []string{"jwt.go", "jwt_test.go"},
					DependsOn:     []string{"task-1"},
					Priority:      2,
					EstComplexity: "high",
					CommitCount:   5,
				},
			},
			ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
			Insights:       []string{"Current auth is tightly coupled"},
			Constraints:    []string{"Must maintain backward compatibility"},
		},
		Task: &TaskInfo{
			ID:            "task-1",
			Title:         "Extract auth interface",
			Description:   "Create abstraction for auth providers",
			Files:         []string{"auth.go", "auth_test.go"},
			Priority:      1,
			EstComplexity: "medium",
		},
		GroupIndex: 0,
		BaseDir:    "/home/user/project",
		Revision: &RevisionInfo{
			Round:     2,
			MaxRounds: 3,
			Issues: []RevisionIssue{
				{
					TaskID:      "task-1",
					Description: "Missing error handling",
					Files:       []string{"auth.go"},
					Severity:    "major",
					Suggestion:  "Add error wrapping",
				},
			},
			TasksToRevise: []string{"task-1"},
			RevisedTasks:  []string{},
		},
		Synthesis: &SynthesisInfo{
			Notes:           []string{"Good progress overall"},
			Recommendations: []string{"Add more tests"},
			Issues:          []string{"Error handling needs work"},
		},
		Consolidation: &ConsolidationInfo{
			Mode:         "stacked",
			BranchPrefix: "ultraplan/auth-refactor",
			MainBranch:   "main",
			TaskWorktrees: []TaskWorktreeInfo{
				{
					TaskID:       "task-1",
					WorktreePath: "/tmp/worktree-1",
					Branch:       "task-1-extract-auth",
					CommitCount:  3,
				},
			},
			GroupBranches:         []string{"group-0-consolidated"},
			PreConsolidatedBranch: "",
		},
		PreviousGroupContext: []string{"Group 0 consolidated successfully"},
		CompletedTasks:       []string{"task-1"},
		FailedTasks:          []string{},
	}

	if err := ctx.Validate(); err != nil {
		t.Errorf("Fully populated context should validate: %v", err)
	}
}

func TestRevisionIssue_Fields(t *testing.T) {
	issue := RevisionIssue{
		TaskID:      "task-abc",
		Description: "Function missing return statement",
		Files:       []string{"service.go", "handler.go"},
		Severity:    "critical",
		Suggestion:  "Add explicit return with error",
	}

	if issue.TaskID != "task-abc" {
		t.Errorf("TaskID = %v, want task-abc", issue.TaskID)
	}
	if issue.Severity != "critical" {
		t.Errorf("Severity = %v, want critical", issue.Severity)
	}
	if len(issue.Files) != 2 {
		t.Errorf("Files count = %d, want 2", len(issue.Files))
	}
}

func TestTaskWorktreeInfo_Fields(t *testing.T) {
	info := TaskWorktreeInfo{
		TaskID:       "task-123",
		WorktreePath: "/var/worktrees/task-123",
		Branch:       "feature/task-123",
		CommitCount:  7,
	}

	if info.TaskID != "task-123" {
		t.Errorf("TaskID = %v, want task-123", info.TaskID)
	}
	if info.CommitCount != 7 {
		t.Errorf("CommitCount = %d, want 7", info.CommitCount)
	}
}
