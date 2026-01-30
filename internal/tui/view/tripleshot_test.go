package view

import (
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/tripleshot"
)

func TestRenderTripleShotPlanGroups(t *testing.T) {
	t.Run("returns empty string when no plan groups", func(t *testing.T) {
		ctx := TripleShotRenderContext{
			TripleShot: &TripleShotState{},
			Session:    &orchestrator.Session{},
		}

		result := renderTripleShotPlanGroups(ctx, 40)

		if result != "" {
			t.Errorf("expected empty string, got: %q", result)
		}
	})

	t.Run("returns empty string when tripleshot is nil", func(t *testing.T) {
		ctx := TripleShotRenderContext{
			TripleShot: nil,
			Session:    &orchestrator.Session{},
		}

		result := renderTripleShotPlanGroups(ctx, 40)

		if result != "" {
			t.Errorf("expected empty string, got: %q", result)
		}
	})

	t.Run("returns empty string when session groups is empty", func(t *testing.T) {
		ctx := TripleShotRenderContext{
			TripleShot: &TripleShotState{
				PlanGroupIDs: []string{"group-1"},
			},
			Session: &orchestrator.Session{
				Groups: nil,
			},
		}

		result := renderTripleShotPlanGroups(ctx, 40)

		if result != "" {
			t.Errorf("expected empty string when session has no groups, got: %q", result)
		}
	})

	t.Run("renders plan group header", func(t *testing.T) {
		session := &orchestrator.Session{
			Groups: []*orchestrator.InstanceGroup{
				{
					ID:        "plan-group-1",
					Name:      "Plan: Test objective",
					Instances: []string{},
				},
			},
		}

		ctx := TripleShotRenderContext{
			TripleShot: &TripleShotState{
				PlanGroupIDs: []string{"plan-group-1"},
			},
			Session: session,
		}

		result := renderTripleShotPlanGroups(ctx, 40)

		if !strings.Contains(result, "Plans") {
			t.Error("expected 'Plans' section header in output")
		}
		if !strings.Contains(result, "Test objective") {
			t.Error("expected group name in output")
		}
	})

	t.Run("renders instances in plan group", func(t *testing.T) {
		inst := &orchestrator.Instance{
			ID:     "inst-1",
			Task:   "Test task",
			Status: orchestrator.StatusWorking,
		}
		session := &orchestrator.Session{
			Instances: []*orchestrator.Instance{inst},
			Groups: []*orchestrator.InstanceGroup{
				{
					ID:        "plan-group-1",
					Name:      "Plan: Test",
					Instances: []string{"inst-1"},
				},
			},
		}

		ctx := TripleShotRenderContext{
			TripleShot: &TripleShotState{
				PlanGroupIDs: []string{"plan-group-1"},
			},
			Session:   session,
			ActiveTab: 0,
		}

		result := renderTripleShotPlanGroups(ctx, 40)

		if !strings.Contains(result, "Test task") {
			t.Error("expected instance task name in output")
		}
		if !strings.Contains(result, "Working") {
			t.Error("expected status label in output")
		}
	})

	t.Run("highlights active instance", func(t *testing.T) {
		inst := &orchestrator.Instance{
			ID:     "inst-1",
			Task:   "Test task",
			Status: orchestrator.StatusWorking,
		}
		session := &orchestrator.Session{
			Instances: []*orchestrator.Instance{inst},
			Groups: []*orchestrator.InstanceGroup{
				{
					ID:        "plan-group-1",
					Name:      "Plan: Test",
					Instances: []string{"inst-1"},
				},
			},
		}

		ctx := TripleShotRenderContext{
			TripleShot: &TripleShotState{
				PlanGroupIDs: []string{"plan-group-1"},
			},
			Session:   session,
			ActiveTab: 0, // First instance is active
		}

		result := renderTripleShotPlanGroups(ctx, 40)

		// The active instance should have the "▶ " prefix
		if !strings.Contains(result, "▶") {
			t.Error("expected active indicator for selected instance")
		}
	})
}

func TestFindGroupByID(t *testing.T) {
	t.Run("finds top-level group", func(t *testing.T) {
		groups := []*orchestrator.InstanceGroup{
			{ID: "group-1", Name: "Group 1"},
			{ID: "group-2", Name: "Group 2"},
		}

		result := findGroupByID(groups, "group-2")

		if result == nil {
			t.Fatal("expected to find group")
		}
		if result.Name != "Group 2" {
			t.Errorf("expected 'Group 2', got: %q", result.Name)
		}
	})

	t.Run("finds nested subgroup", func(t *testing.T) {
		groups := []*orchestrator.InstanceGroup{
			{
				ID:   "parent",
				Name: "Parent",
				SubGroups: []*orchestrator.InstanceGroup{
					{ID: "child", Name: "Child"},
				},
			},
		}

		result := findGroupByID(groups, "child")

		if result == nil {
			t.Fatal("expected to find nested group")
		}
		if result.Name != "Child" {
			t.Errorf("expected 'Child', got: %q", result.Name)
		}
	})

	t.Run("returns nil for unknown ID", func(t *testing.T) {
		groups := []*orchestrator.InstanceGroup{
			{ID: "group-1", Name: "Group 1"},
		}

		result := findGroupByID(groups, "unknown")

		if result != nil {
			t.Error("expected nil for unknown ID")
		}
	})

	t.Run("handles nil groups slice", func(t *testing.T) {
		result := findGroupByID(nil, "group-1")

		if result != nil {
			t.Error("expected nil for nil groups")
		}
	})
}

func TestTripleShotState_HasActiveCoordinators(t *testing.T) {
	t.Run("nil state returns false", func(t *testing.T) {
		var state *TripleShotState
		if state.HasActiveCoordinators() {
			t.Error("expected false for nil state")
		}
	})

	t.Run("empty state returns false", func(t *testing.T) {
		state := &TripleShotState{}
		if state.HasActiveCoordinators() {
			t.Error("expected false for empty state")
		}
	})

	t.Run("state with empty Coordinators map returns false", func(t *testing.T) {
		state := &TripleShotState{
			Coordinators: make(map[string]*tripleshot.Coordinator),
		}
		if state.HasActiveCoordinators() {
			t.Error("expected false for empty coordinators map")
		}
	})

	t.Run("state with coordinators in map returns true", func(t *testing.T) {
		state := &TripleShotState{
			Coordinators: map[string]*tripleshot.Coordinator{
				"group-1": nil, // Even nil coordinator counts as entry
			},
		}
		if !state.HasActiveCoordinators() {
			t.Error("expected true when coordinators map has entries")
		}
	})

	t.Run("state with coordinator in map returns true", func(t *testing.T) {
		state := &TripleShotState{
			Coordinators: map[string]*tripleshot.Coordinator{
				"group-1": {},
			},
		}
		if !state.HasActiveCoordinators() {
			t.Error("expected true when coordinator exists in map")
		}
	})
}

func TestTripleShotState_GetAllCoordinators(t *testing.T) {
	t.Run("nil state returns nil", func(t *testing.T) {
		var state *TripleShotState
		if state.GetAllCoordinators() != nil {
			t.Error("expected nil for nil state")
		}
	})

	t.Run("empty state returns nil", func(t *testing.T) {
		state := &TripleShotState{}
		if state.GetAllCoordinators() != nil {
			t.Error("expected nil for empty state")
		}
	})

	t.Run("returns coordinators from map", func(t *testing.T) {
		coord1 := &tripleshot.Coordinator{}
		coord2 := &tripleshot.Coordinator{}
		state := &TripleShotState{
			Coordinators: map[string]*tripleshot.Coordinator{
				"group-1": coord1,
				"group-2": coord2,
			},
		}

		result := state.GetAllCoordinators()
		if len(result) != 2 {
			t.Errorf("expected 2 coordinators, got %d", len(result))
		}
	})

	t.Run("returns deterministic order by group ID", func(t *testing.T) {
		coord1 := &tripleshot.Coordinator{}
		coord2 := &tripleshot.Coordinator{}
		coord3 := &tripleshot.Coordinator{}
		state := &TripleShotState{
			Coordinators: map[string]*tripleshot.Coordinator{
				"z-group": coord1,
				"a-group": coord2,
				"m-group": coord3,
			},
		}

		// Call multiple times to verify order is deterministic
		for i := 0; i < 10; i++ {
			result := state.GetAllCoordinators()
			// Should be sorted alphabetically: a-group, m-group, z-group
			if result[0] != coord2 || result[1] != coord3 || result[2] != coord1 {
				t.Error("expected coordinators to be returned in sorted key order")
			}
		}
	})

	t.Run("returns nil when map empty", func(t *testing.T) {
		state := &TripleShotState{
			Coordinators: make(map[string]*tripleshot.Coordinator),
		}

		result := state.GetAllCoordinators()
		if result != nil {
			t.Errorf("expected nil for empty map, got %d coordinators", len(result))
		}
	})
}

func TestTripleShotState_GetCoordinatorForGroup(t *testing.T) {
	t.Run("returns coordinator from map when found", func(t *testing.T) {
		coord := &tripleshot.Coordinator{}
		state := &TripleShotState{
			Coordinators: map[string]*tripleshot.Coordinator{
				"target-group": coord,
			},
		}

		result := state.GetCoordinatorForGroup("target-group")
		if result != coord {
			t.Error("expected to find coordinator by group ID")
		}
	})

	t.Run("returns nil when not found and no fallback", func(t *testing.T) {
		state := &TripleShotState{
			Coordinators: map[string]*tripleshot.Coordinator{
				"other-group": &tripleshot.Coordinator{},
			},
		}

		result := state.GetCoordinatorForGroup("missing-group")
		if result != nil {
			t.Error("expected nil for missing group with no fallback")
		}
	})

	t.Run("returns nil when not in map", func(t *testing.T) {
		state := &TripleShotState{
			Coordinators: map[string]*tripleshot.Coordinator{},
		}

		result := state.GetCoordinatorForGroup("any-group")
		if result != nil {
			t.Error("expected nil for missing group")
		}
	})

	t.Run("returns nil when coordinators map is nil", func(t *testing.T) {
		state := &TripleShotState{}

		result := state.GetCoordinatorForGroup("any-group")
		if result != nil {
			t.Error("expected nil when coordinators map is nil")
		}
	})
}

func TestTripleShotStatePlanGroupIDs(t *testing.T) {
	t.Run("starts empty", func(t *testing.T) {
		state := &TripleShotState{}

		if len(state.PlanGroupIDs) != 0 {
			t.Error("expected PlanGroupIDs to start empty")
		}
	})

	t.Run("can append plan group IDs", func(t *testing.T) {
		state := &TripleShotState{}
		state.PlanGroupIDs = append(state.PlanGroupIDs, "group-1")
		state.PlanGroupIDs = append(state.PlanGroupIDs, "group-2")

		if len(state.PlanGroupIDs) != 2 {
			t.Errorf("expected 2 plan groups, got: %d", len(state.PlanGroupIDs))
		}
		if state.PlanGroupIDs[0] != "group-1" {
			t.Errorf("expected 'group-1', got: %q", state.PlanGroupIDs[0])
		}
		if state.PlanGroupIDs[1] != "group-2" {
			t.Errorf("expected 'group-2', got: %q", state.PlanGroupIDs[1])
		}
	})
}

func TestRenderTripleShotPlanGroupsEdgeCases(t *testing.T) {
	t.Run("renders multiple plan groups", func(t *testing.T) {
		session := &orchestrator.Session{
			Instances: []*orchestrator.Instance{
				{ID: "inst-1", Task: "Task 1", Status: orchestrator.StatusCompleted},
				{ID: "inst-2", Task: "Task 2", Status: orchestrator.StatusWorking},
			},
			Groups: []*orchestrator.InstanceGroup{
				{ID: "plan-1", Name: "Plan: First", Instances: []string{"inst-1"}},
				{ID: "plan-2", Name: "Plan: Second", Instances: []string{"inst-2"}},
			},
		}
		ctx := TripleShotRenderContext{
			TripleShot: &TripleShotState{
				PlanGroupIDs: []string{"plan-1", "plan-2"},
			},
			Session: session,
		}

		result := renderTripleShotPlanGroups(ctx, 50)

		// Verify both group names appear
		if !strings.Contains(result, "First") {
			t.Error("expected first plan group to be rendered")
		}
		if !strings.Contains(result, "Second") {
			t.Error("expected second plan group to be rendered")
		}
	})

	t.Run("handles missing group ID gracefully", func(t *testing.T) {
		session := &orchestrator.Session{
			Groups: []*orchestrator.InstanceGroup{
				{ID: "existing-group", Name: "Existing"},
			},
		}
		ctx := TripleShotRenderContext{
			TripleShot: &TripleShotState{
				PlanGroupIDs: []string{"missing-group", "existing-group"},
			},
			Session: session,
		}

		result := renderTripleShotPlanGroups(ctx, 40)

		// Should render "Existing" but not crash on missing
		if !strings.Contains(result, "Existing") {
			t.Error("expected existing group to still render")
		}
	})
}

func TestFindGroupByIDDeepNesting(t *testing.T) {
	t.Run("finds deeply nested subgroup", func(t *testing.T) {
		groups := []*orchestrator.InstanceGroup{
			{
				ID: "level-0",
				SubGroups: []*orchestrator.InstanceGroup{
					{
						ID: "level-1",
						SubGroups: []*orchestrator.InstanceGroup{
							{ID: "level-2", Name: "Deep"},
						},
					},
				},
			},
		}

		result := findGroupByID(groups, "level-2")

		if result == nil {
			t.Fatal("expected to find deeply nested group")
		}
		if result.Name != "Deep" {
			t.Errorf("expected 'Deep', got: %q", result.Name)
		}
	})

	t.Run("finds group in sibling branches", func(t *testing.T) {
		groups := []*orchestrator.InstanceGroup{
			{
				ID: "branch-a",
				SubGroups: []*orchestrator.InstanceGroup{
					{ID: "a-child", Name: "A Child"},
				},
			},
			{
				ID: "branch-b",
				SubGroups: []*orchestrator.InstanceGroup{
					{ID: "b-child", Name: "B Child"},
				},
			},
		}

		result := findGroupByID(groups, "b-child")

		if result == nil {
			t.Fatal("expected to find group in second branch")
		}
		if result.Name != "B Child" {
			t.Errorf("expected 'B Child', got: %q", result.Name)
		}
	})
}

func TestRenderAdversarialPhaseInfo(t *testing.T) {
	t.Run("shows Implementing phase during working", func(t *testing.T) {
		session := &tripleshot.Session{
			Phase: tripleshot.PhaseWorking,
			Config: tripleshot.Config{
				Adversarial: true,
			},
		}

		result := renderAdversarialPhaseInfo(session)

		if len(result) == 0 {
			t.Fatal("expected at least one line of output")
		}
		joined := strings.Join(result, "\n")
		if !strings.Contains(joined, "Phase:") {
			t.Error("expected Phase label in output")
		}
		if !strings.Contains(joined, "Implementing") {
			t.Errorf("expected 'Implementing' for PhaseWorking, got: %s", joined)
		}
	})

	t.Run("shows Under Review phase during adversarial review", func(t *testing.T) {
		session := &tripleshot.Session{
			Phase: tripleshot.PhaseAdversarialReview,
			Config: tripleshot.Config{
				Adversarial: true,
			},
			Attempts: [3]tripleshot.Attempt{
				{InstanceID: "impl-1", ReviewerID: "rev-1"},
				{InstanceID: "impl-2", ReviewerID: "rev-2"},
				{InstanceID: "impl-3", ReviewerID: ""},
			},
		}

		result := renderAdversarialPhaseInfo(session)

		joined := strings.Join(result, "\n")
		if !strings.Contains(joined, "Under Review") {
			t.Errorf("expected 'Under Review' for PhaseAdversarialReview, got: %s", joined)
		}
		// Should show pair count (2 reviewers active)
		if !strings.Contains(joined, "2/3 pairs active") {
			t.Errorf("expected pair count in output, got: %s", joined)
		}
	})

	t.Run("shows Judging phase during evaluation", func(t *testing.T) {
		session := &tripleshot.Session{
			Phase: tripleshot.PhaseEvaluating,
			Config: tripleshot.Config{
				Adversarial: true,
			},
		}

		result := renderAdversarialPhaseInfo(session)

		joined := strings.Join(result, "\n")
		if !strings.Contains(joined, "Judging") {
			t.Errorf("expected 'Judging' for PhaseEvaluating, got: %s", joined)
		}
	})

	t.Run("shows Complete phase when done", func(t *testing.T) {
		session := &tripleshot.Session{
			Phase: tripleshot.PhaseComplete,
			Config: tripleshot.Config{
				Adversarial: true,
			},
		}

		result := renderAdversarialPhaseInfo(session)

		joined := strings.Join(result, "\n")
		if !strings.Contains(joined, "Complete") {
			t.Errorf("expected 'Complete' for PhaseComplete, got: %s", joined)
		}
	})
}

func TestRenderAdversarialAttemptPairs(t *testing.T) {
	t.Run("shows implementer/reviewer pairs header", func(t *testing.T) {
		ctx := TripleShotRenderContext{
			Session: &orchestrator.Session{},
		}
		session := &tripleshot.Session{
			Config: tripleshot.Config{Adversarial: true},
			Attempts: [3]tripleshot.Attempt{
				{InstanceID: "impl-1", Status: tripleshot.AttemptStatusWorking},
				{InstanceID: "impl-2", Status: tripleshot.AttemptStatusWorking},
				{InstanceID: "impl-3", Status: tripleshot.AttemptStatusWorking},
			},
		}

		result := renderAdversarialAttemptPairs(ctx, session)

		joined := strings.Join(result, "\n")
		if !strings.Contains(joined, "Implementer/Reviewer Pairs:") {
			t.Errorf("expected pairs header, got: %s", joined)
		}
	})

	t.Run("shows all three pairs", func(t *testing.T) {
		ctx := TripleShotRenderContext{
			Session: &orchestrator.Session{},
		}
		session := &tripleshot.Session{
			Config: tripleshot.Config{Adversarial: true},
			Attempts: [3]tripleshot.Attempt{
				{InstanceID: "impl-1", Status: tripleshot.AttemptStatusWorking},
				{InstanceID: "impl-2", Status: tripleshot.AttemptStatusWorking},
				{InstanceID: "impl-3", Status: tripleshot.AttemptStatusWorking},
			},
		}

		result := renderAdversarialAttemptPairs(ctx, session)

		joined := strings.Join(result, "\n")
		if !strings.Contains(joined, "Pair 1:") {
			t.Error("expected Pair 1")
		}
		if !strings.Contains(joined, "Pair 2:") {
			t.Error("expected Pair 2")
		}
		if !strings.Contains(joined, "Pair 3:") {
			t.Error("expected Pair 3")
		}
	})

	t.Run("shows implementer and reviewer status", func(t *testing.T) {
		ctx := TripleShotRenderContext{
			Session: &orchestrator.Session{
				Instances: []*orchestrator.Instance{
					{ID: "rev-1", Status: orchestrator.StatusWorking},
				},
			},
		}
		session := &tripleshot.Session{
			Phase:  tripleshot.PhaseAdversarialReview,
			Config: tripleshot.Config{Adversarial: true},
			Attempts: [3]tripleshot.Attempt{
				{InstanceID: "impl-1", Status: tripleshot.AttemptStatusCompleted, ReviewerID: "rev-1"},
				{InstanceID: "impl-2", Status: tripleshot.AttemptStatusWorking},
				{InstanceID: "impl-3", Status: tripleshot.AttemptStatusWorking},
			},
		}

		result := renderAdversarialAttemptPairs(ctx, session)

		joined := strings.Join(result, "\n")
		// Should show implementer status
		if !strings.Contains(joined, "Impl:") {
			t.Errorf("expected implementer indicator, got: %s", joined)
		}
		// Should show reviewer status
		if !strings.Contains(joined, "Rev:") {
			t.Errorf("expected reviewer indicator, got: %s", joined)
		}
	})

	t.Run("shows review score when available", func(t *testing.T) {
		ctx := TripleShotRenderContext{
			Session: &orchestrator.Session{
				Instances: []*orchestrator.Instance{
					{ID: "rev-1", Status: orchestrator.StatusCompleted},
				},
			},
		}
		session := &tripleshot.Session{
			Phase:  tripleshot.PhaseAdversarialReview,
			Config: tripleshot.Config{Adversarial: true},
			Attempts: [3]tripleshot.Attempt{
				{
					InstanceID:     "impl-1",
					Status:         tripleshot.AttemptStatusCompleted,
					ReviewerID:     "rev-1",
					ReviewScore:    8,
					ReviewApproved: true,
				},
				{InstanceID: "impl-2", Status: tripleshot.AttemptStatusWorking},
				{InstanceID: "impl-3", Status: tripleshot.AttemptStatusWorking},
			},
		}

		result := renderAdversarialAttemptPairs(ctx, session)

		joined := strings.Join(result, "\n")
		// Should show score
		if !strings.Contains(joined, "8/10") {
			t.Errorf("expected score 8/10 in output, got: %s", joined)
		}
		// Should show approval checkmark
		if !strings.Contains(joined, "✓") {
			t.Errorf("expected approval checkmark in output, got: %s", joined)
		}
	})

	t.Run("shows rejection when score but not approved", func(t *testing.T) {
		ctx := TripleShotRenderContext{
			Session: &orchestrator.Session{
				Instances: []*orchestrator.Instance{
					{ID: "rev-1", Status: orchestrator.StatusCompleted},
				},
			},
		}
		session := &tripleshot.Session{
			Phase:  tripleshot.PhaseAdversarialReview,
			Config: tripleshot.Config{Adversarial: true},
			Attempts: [3]tripleshot.Attempt{
				{
					InstanceID:     "impl-1",
					Status:         tripleshot.AttemptStatusUnderReview,
					ReviewerID:     "rev-1",
					ReviewScore:    5,
					ReviewApproved: false,
				},
				{InstanceID: "impl-2", Status: tripleshot.AttemptStatusWorking},
				{InstanceID: "impl-3", Status: tripleshot.AttemptStatusWorking},
			},
		}

		result := renderAdversarialAttemptPairs(ctx, session)

		joined := strings.Join(result, "\n")
		// Should show rejection X
		if !strings.Contains(joined, "✗") {
			t.Errorf("expected rejection X in output, got: %s", joined)
		}
	})
}

func TestRenderStandardAttempts(t *testing.T) {
	t.Run("renders attempts without reviewer info", func(t *testing.T) {
		ctx := TripleShotRenderContext{
			Session: &orchestrator.Session{},
		}
		session := &tripleshot.Session{
			Config: tripleshot.Config{Adversarial: false},
			Attempts: [3]tripleshot.Attempt{
				{InstanceID: "inst-1", Status: tripleshot.AttemptStatusCompleted},
				{InstanceID: "inst-2", Status: tripleshot.AttemptStatusWorking},
				{InstanceID: "inst-3", Status: tripleshot.AttemptStatusPending},
			},
		}

		result := renderStandardAttempts(ctx, session)

		joined := strings.Join(result, "\n")
		// Should show Attempts header
		if !strings.Contains(joined, "Attempts:") {
			t.Errorf("expected Attempts header, got: %s", joined)
		}
		// Should show numbered attempts
		if !strings.Contains(joined, "1:") {
			t.Error("expected attempt 1")
		}
		if !strings.Contains(joined, "2:") {
			t.Error("expected attempt 2")
		}
		if !strings.Contains(joined, "3:") {
			t.Error("expected attempt 3")
		}
		// Should NOT show pairs or reviewers
		if strings.Contains(joined, "Pair") || strings.Contains(joined, "Rev:") {
			t.Error("standard mode should not show pair/reviewer info")
		}
	})
}

func TestGetAttemptStatusDisplay(t *testing.T) {
	tests := []struct {
		status       tripleshot.AttemptStatus
		expectedText string
	}{
		{tripleshot.AttemptStatusWorking, "working"},
		{tripleshot.AttemptStatusUnderReview, "under review"},
		{tripleshot.AttemptStatusCompleted, "done"},
		{tripleshot.AttemptStatusFailed, "failed"},
		{tripleshot.AttemptStatusPending, "pending"},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			text, _ := getAttemptStatusDisplay(tc.status)
			if text != tc.expectedText {
				t.Errorf("expected %q, got %q", tc.expectedText, text)
			}
		})
	}
}

func TestGetReviewerStatusDisplay(t *testing.T) {
	t.Run("returns approved for approved attempt", func(t *testing.T) {
		ctx := TripleShotRenderContext{Session: &orchestrator.Session{}}
		attempt := tripleshot.Attempt{ReviewApproved: true}

		text, _ := getReviewerStatusDisplay(ctx, attempt, tripleshot.PhaseAdversarialReview)

		if text != "approved" {
			t.Errorf("expected 'approved', got %q", text)
		}
	})

	t.Run("returns rejected for completed reviewer with low score", func(t *testing.T) {
		ctx := TripleShotRenderContext{
			Session: &orchestrator.Session{
				Instances: []*orchestrator.Instance{
					{ID: "rev-1", Status: orchestrator.StatusCompleted},
				},
			},
		}
		attempt := tripleshot.Attempt{
			ReviewerID:     "rev-1",
			ReviewScore:    5,
			ReviewApproved: false,
		}

		text, _ := getReviewerStatusDisplay(ctx, attempt, tripleshot.PhaseAdversarialReview)

		if text != "rejected" {
			t.Errorf("expected 'rejected', got %q", text)
		}
	})

	t.Run("returns reviewing for working reviewer", func(t *testing.T) {
		ctx := TripleShotRenderContext{
			Session: &orchestrator.Session{
				Instances: []*orchestrator.Instance{
					{ID: "rev-1", Status: orchestrator.StatusWorking},
				},
			},
		}
		attempt := tripleshot.Attempt{ReviewerID: "rev-1"}

		text, _ := getReviewerStatusDisplay(ctx, attempt, tripleshot.PhaseAdversarialReview)

		if text != "reviewing" {
			t.Errorf("expected 'reviewing', got %q", text)
		}
	})

	t.Run("returns error for stuck reviewer", func(t *testing.T) {
		ctx := TripleShotRenderContext{
			Session: &orchestrator.Session{
				Instances: []*orchestrator.Instance{
					{ID: "rev-1", Status: orchestrator.StatusStuck},
				},
			},
		}
		attempt := tripleshot.Attempt{ReviewerID: "rev-1"}

		text, _ := getReviewerStatusDisplay(ctx, attempt, tripleshot.PhaseAdversarialReview)

		if text != "error" {
			t.Errorf("expected 'error', got %q", text)
		}
	})
}

func TestGetInstancePrefix(t *testing.T) {
	t.Run("returns arrow for active instance", func(t *testing.T) {
		ctx := TripleShotRenderContext{
			Session: &orchestrator.Session{
				Instances: []*orchestrator.Instance{
					{ID: "inst-1"},
					{ID: "inst-2"},
				},
			},
			ActiveTab: 0,
		}

		prefix := getInstancePrefix(ctx, "inst-1")

		if prefix != "▶ " {
			t.Errorf("expected '▶ ' for active instance, got %q", prefix)
		}
	})

	t.Run("returns spaces for inactive instance", func(t *testing.T) {
		ctx := TripleShotRenderContext{
			Session: &orchestrator.Session{
				Instances: []*orchestrator.Instance{
					{ID: "inst-1"},
					{ID: "inst-2"},
				},
			},
			ActiveTab: 0,
		}

		prefix := getInstancePrefix(ctx, "inst-2")

		if prefix != "  " {
			t.Errorf("expected '  ' for inactive instance, got %q", prefix)
		}
	})

	t.Run("returns spaces for empty instance ID", func(t *testing.T) {
		ctx := TripleShotRenderContext{
			Session: &orchestrator.Session{
				Instances: []*orchestrator.Instance{
					{ID: "inst-1"},
				},
			},
			ActiveTab: 0,
		}

		prefix := getInstancePrefix(ctx, "")

		if prefix != "  " {
			t.Errorf("expected '  ' for empty instance ID, got %q", prefix)
		}
	})

	t.Run("handles nil session", func(t *testing.T) {
		ctx := TripleShotRenderContext{
			Session:   nil,
			ActiveTab: 0,
		}

		prefix := getInstancePrefix(ctx, "inst-1")

		if prefix != "  " {
			t.Errorf("expected '  ' for nil session, got %q", prefix)
		}
	})
}
