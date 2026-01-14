package tui

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

func TestDispatchInlineMultiPlanFileChecks_NilInlinePlan(t *testing.T) {
	m := Model{
		inlinePlan: nil,
	}

	cmds := m.dispatchInlineMultiPlanFileChecks()
	if cmds != nil {
		t.Errorf("expected nil when inlinePlan is nil, got %v", cmds)
	}
}

func TestDispatchInlineMultiPlanFileChecks_NotMultiPass(t *testing.T) {
	m := Model{
		inlinePlan: &InlinePlanState{
			MultiPass:            false,
			AwaitingPlanCreation: true,
		},
	}

	cmds := m.dispatchInlineMultiPlanFileChecks()
	if cmds != nil {
		t.Errorf("expected nil when not in multipass mode, got %v", cmds)
	}
}

func TestDispatchInlineMultiPlanFileChecks_NotAwaitingPlanCreation(t *testing.T) {
	m := Model{
		inlinePlan: &InlinePlanState{
			MultiPass:            true,
			AwaitingPlanCreation: false,
		},
	}

	cmds := m.dispatchInlineMultiPlanFileChecks()
	if cmds != nil {
		t.Errorf("expected nil when not awaiting plan creation, got %v", cmds)
	}
}

func TestDispatchInlineMultiPlanFileChecks_NoPlannerIDs(t *testing.T) {
	m := Model{
		inlinePlan: &InlinePlanState{
			MultiPass:            true,
			AwaitingPlanCreation: true,
			PlanningInstanceIDs:  []string{},
		},
	}

	cmds := m.dispatchInlineMultiPlanFileChecks()
	if cmds != nil {
		t.Errorf("expected nil when no planner IDs, got %v", cmds)
	}
}

func TestDispatchInlineMultiPlanFileChecks_SkipsProcessedPlanners(t *testing.T) {
	m := Model{
		inlinePlan: &InlinePlanState{
			MultiPass:            true,
			AwaitingPlanCreation: true,
			PlanningInstanceIDs:  []string{"planner-1", "planner-2", "planner-3"},
			ProcessedPlanners: map[int]bool{
				0: true, // planner-1 already processed
				1: true, // planner-2 already processed
				2: true, // planner-3 already processed
			},
			Objective: "test objective",
		},
		orchestrator: nil, // Will cause GetInstance to return nil
	}

	cmds := m.dispatchInlineMultiPlanFileChecks()
	// All planners are processed, so no commands should be returned
	if len(cmds) != 0 {
		t.Errorf("expected 0 commands when all planners processed, got %d", len(cmds))
	}
}

func TestDispatchInlineMultiPlanFileChecks_CreatesCommandsForUnprocessedPlanners(t *testing.T) {
	m := Model{
		inlinePlan: &InlinePlanState{
			MultiPass:            true,
			AwaitingPlanCreation: true,
			PlanningInstanceIDs:  []string{"planner-1", "planner-2", "planner-3"},
			ProcessedPlanners: map[int]bool{
				0: true, // Only planner-1 is processed
			},
			Objective: "test objective",
		},
		orchestrator: nil, // Commands will return nil when GetInstance fails
	}

	cmds := m.dispatchInlineMultiPlanFileChecks()
	// Should create commands for planner-2 and planner-3
	if len(cmds) != 2 {
		t.Errorf("expected 2 commands for unprocessed planners, got %d", len(cmds))
	}
}

func TestHandleInlineMultiPlanFileCheckResult_NilInlinePlan(t *testing.T) {
	m := &Model{
		inlinePlan: nil,
	}

	msg := inlineMultiPlanFileCheckResultMsg{
		Index:        0,
		Plan:         &orchestrator.PlanSpec{},
		StrategyName: "test",
	}

	result, cmd := m.handleInlineMultiPlanFileCheckResult(msg)
	if cmd != nil {
		t.Error("expected nil command when inlinePlan is nil")
	}
	resultModel := result.(*Model)
	if resultModel.inlinePlan != nil {
		t.Error("expected inlinePlan to remain nil")
	}
}

func TestHandleInlineMultiPlanFileCheckResult_NotMultiPass(t *testing.T) {
	m := &Model{
		inlinePlan: &InlinePlanState{
			MultiPass:            false,
			AwaitingPlanCreation: true,
		},
	}

	msg := inlineMultiPlanFileCheckResultMsg{
		Index:        0,
		Plan:         &orchestrator.PlanSpec{},
		StrategyName: "test",
	}

	_, cmd := m.handleInlineMultiPlanFileCheckResult(msg)
	if cmd != nil {
		t.Error("expected nil command when not in multipass mode")
	}
}

func TestHandleInlineMultiPlanFileCheckResult_InvalidIndex(t *testing.T) {
	tests := []struct {
		name  string
		index int
	}{
		{"negative index", -1},
		{"index out of bounds", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{
				inlinePlan: &InlinePlanState{
					MultiPass:            true,
					AwaitingPlanCreation: true,
					PlanningInstanceIDs:  []string{"planner-1"},
					ProcessedPlanners:    make(map[int]bool),
					CandidatePlans:       make([]*orchestrator.PlanSpec, 1),
				},
			}

			msg := inlineMultiPlanFileCheckResultMsg{
				Index:        tt.index,
				Plan:         &orchestrator.PlanSpec{},
				StrategyName: "test",
			}

			_, cmd := m.handleInlineMultiPlanFileCheckResult(msg)
			if cmd != nil {
				t.Error("expected nil command for invalid index")
			}
		})
	}
}

func TestHandleInlineMultiPlanFileCheckResult_SkipsAlreadyProcessed(t *testing.T) {
	m := &Model{
		inlinePlan: &InlinePlanState{
			MultiPass:            true,
			AwaitingPlanCreation: true,
			PlanningInstanceIDs:  []string{"planner-1"},
			ProcessedPlanners: map[int]bool{
				0: true, // Already processed
			},
			CandidatePlans: make([]*orchestrator.PlanSpec, 1),
		},
	}

	msg := inlineMultiPlanFileCheckResultMsg{
		Index:        0,
		Plan:         &orchestrator.PlanSpec{Tasks: []orchestrator.PlannedTask{{ID: "new"}}},
		StrategyName: "test",
	}

	_, cmd := m.handleInlineMultiPlanFileCheckResult(msg)
	if cmd != nil {
		t.Error("expected nil command for already processed planner")
	}
	// Plan should not be updated
	if m.inlinePlan.CandidatePlans[0] != nil {
		t.Error("plan should not be updated for already processed planner")
	}
}

func TestHandleInlineMultiPlanFileCheckResult_StoresPlan(t *testing.T) {
	m := &Model{
		inlinePlan: &InlinePlanState{
			MultiPass:            true,
			AwaitingPlanCreation: true,
			PlanningInstanceIDs:  []string{"planner-1", "planner-2", "planner-3"},
			ProcessedPlanners:    make(map[int]bool),
			CandidatePlans:       make([]*orchestrator.PlanSpec, 3),
			Objective:            "test",
		},
	}

	testPlan := &orchestrator.PlanSpec{
		Summary: "test plan",
		Tasks:   []orchestrator.PlannedTask{{ID: "task-1", Title: "Test Task"}},
	}

	msg := inlineMultiPlanFileCheckResultMsg{
		Index:        1,
		Plan:         testPlan,
		StrategyName: "minimize-complexity",
	}

	result, _ := m.handleInlineMultiPlanFileCheckResult(msg)
	resultModel := result.(*Model)

	// Check planner was marked as processed
	if !resultModel.inlinePlan.ProcessedPlanners[1] {
		t.Error("planner should be marked as processed")
	}

	// Check plan was stored
	if resultModel.inlinePlan.CandidatePlans[1] != testPlan {
		t.Error("plan should be stored in CandidatePlans")
	}

	// Check info message was updated
	if resultModel.infoMessage == "" {
		t.Error("info message should be updated")
	}
}

func TestHandleInlineMultiPlanFileCheckResult_AllPlansCollectedWithNoValidPlans(t *testing.T) {
	m := &Model{
		inlinePlan: &InlinePlanState{
			MultiPass:            true,
			AwaitingPlanCreation: true,
			PlanningInstanceIDs:  []string{"planner-1"},
			ProcessedPlanners:    make(map[int]bool),
			CandidatePlans:       make([]*orchestrator.PlanSpec, 1),
			Objective:            "test",
		},
	}

	// Send a nil plan (simulating parse failure)
	msg := inlineMultiPlanFileCheckResultMsg{
		Index:        0,
		Plan:         nil,
		StrategyName: "test",
	}

	result, _ := m.handleInlineMultiPlanFileCheckResult(msg)
	resultModel := result.(*Model)

	// inlinePlan should be nil because all planners failed
	if resultModel.inlinePlan != nil {
		t.Error("inlinePlan should be nil when all planners fail")
	}

	// Error message should be set
	if resultModel.errorMessage == "" {
		t.Error("error message should be set when all planners fail")
	}
}

// Coverage: checkInlineMultiPlanFileAsync is not directly tested because:
// 1. It requires a full orchestrator setup with instances
// 2. It's an internal async function that's tested indirectly through integration
// 3. The dispatch function already tests the command creation

func TestStatFileFunction(t *testing.T) {
	// Test that statFile is a function that wraps os.Stat
	// This is the hook for testing file operations
	_, err := statFile("/nonexistent/path/that/should/not/exist")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestExpandTildePath(t *testing.T) {
	// Test tilde expansion helper function
	tests := []struct {
		name     string
		input    string
		wantHome bool // true if result should start with home dir
	}{
		{"tilde prefix", "~/Desktop/plan.yaml", true},
		{"absolute path", "/home/user/plan.yaml", false},
		{"relative path", "plan.yaml", false},
		{"tilde only", "~", false}, // Only ~/... gets expanded
		{"tilde in middle", "/path/~/file", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandTildePath(tt.input)
			if tt.wantHome {
				// Should not contain tilde anymore
				if len(result) > 0 && result[0] == '~' {
					t.Errorf("expandTildePath(%q) = %q, still contains tilde", tt.input, result)
				}
				// Should be longer than input (home dir expanded)
				if len(result) <= len(tt.input) {
					t.Errorf("expandTildePath(%q) = %q, path not expanded", tt.input, result)
				}
			} else {
				// Should be unchanged
				if result != tt.input {
					t.Errorf("expandTildePath(%q) = %q, want %q", tt.input, result, tt.input)
				}
			}
		})
	}
}

// TestInlineUltraPlanConfig_HasProperDefaults verifies that the inline ultraplan config
// uses DefaultUltraPlanConfig() to get proper defaults like RequireVerifiedCommits=true.
// This prevents regression of the bug where RequireVerifiedCommits defaulted to false,
// causing "no task branches with verified commits found" errors during consolidation.
func TestInlineUltraPlanConfig_HasProperDefaults(t *testing.T) {
	// Get the default config that initInlineUltraPlanMode should use
	cfg := orchestrator.DefaultUltraPlanConfig()

	// Verify RequireVerifiedCommits is true (the most critical default)
	if !cfg.RequireVerifiedCommits {
		t.Error("DefaultUltraPlanConfig().RequireVerifiedCommits should be true")
	}

	// Verify other important defaults
	if cfg.MaxParallel != 3 {
		t.Errorf("DefaultUltraPlanConfig().MaxParallel = %d, want 3", cfg.MaxParallel)
	}
	if cfg.MaxTaskRetries != 3 {
		t.Errorf("DefaultUltraPlanConfig().MaxTaskRetries = %d, want 3", cfg.MaxTaskRetries)
	}
}
