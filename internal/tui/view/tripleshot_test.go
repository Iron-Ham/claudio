package view

import (
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
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
		if !strings.Contains(result, "WORK") {
			t.Error("expected status abbreviation in output")
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
			Coordinators: make(map[string]*orchestrator.TripleShotCoordinator),
		}
		if state.HasActiveCoordinators() {
			t.Error("expected false for empty coordinators map")
		}
	})

	t.Run("state with coordinators in map returns true", func(t *testing.T) {
		state := &TripleShotState{
			Coordinators: map[string]*orchestrator.TripleShotCoordinator{
				"group-1": nil, // Even nil coordinator counts as entry
			},
		}
		if !state.HasActiveCoordinators() {
			t.Error("expected true when coordinators map has entries")
		}
	})

	t.Run("state with deprecated Coordinator returns true", func(t *testing.T) {
		state := &TripleShotState{
			Coordinator: &orchestrator.TripleShotCoordinator{},
		}
		if !state.HasActiveCoordinators() {
			t.Error("expected true for deprecated single coordinator")
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
		coord1 := &orchestrator.TripleShotCoordinator{}
		coord2 := &orchestrator.TripleShotCoordinator{}
		state := &TripleShotState{
			Coordinators: map[string]*orchestrator.TripleShotCoordinator{
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
		coord1 := &orchestrator.TripleShotCoordinator{}
		coord2 := &orchestrator.TripleShotCoordinator{}
		coord3 := &orchestrator.TripleShotCoordinator{}
		state := &TripleShotState{
			Coordinators: map[string]*orchestrator.TripleShotCoordinator{
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

	t.Run("falls back to deprecated Coordinator when map empty", func(t *testing.T) {
		coord := &orchestrator.TripleShotCoordinator{}
		state := &TripleShotState{
			Coordinator: coord,
		}

		result := state.GetAllCoordinators()
		if len(result) != 1 {
			t.Errorf("expected 1 coordinator from fallback, got %d", len(result))
		}
		if result[0] != coord {
			t.Error("expected fallback coordinator to be returned")
		}
	})
}

func TestTripleShotState_GetCoordinatorForGroup(t *testing.T) {
	t.Run("returns coordinator from map when found", func(t *testing.T) {
		coord := &orchestrator.TripleShotCoordinator{}
		state := &TripleShotState{
			Coordinators: map[string]*orchestrator.TripleShotCoordinator{
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
			Coordinators: map[string]*orchestrator.TripleShotCoordinator{
				"other-group": &orchestrator.TripleShotCoordinator{},
			},
		}

		result := state.GetCoordinatorForGroup("missing-group")
		if result != nil {
			t.Error("expected nil for missing group with no fallback")
		}
	})

	t.Run("falls back to deprecated Coordinator when not in map", func(t *testing.T) {
		fallbackCoord := &orchestrator.TripleShotCoordinator{}
		state := &TripleShotState{
			Coordinators: map[string]*orchestrator.TripleShotCoordinator{},
			Coordinator:  fallbackCoord,
		}

		result := state.GetCoordinatorForGroup("any-group")
		if result != fallbackCoord {
			t.Error("expected fallback to deprecated Coordinator")
		}
	})

	t.Run("prefers map over fallback when key exists", func(t *testing.T) {
		mapCoord := &orchestrator.TripleShotCoordinator{}
		fallbackCoord := &orchestrator.TripleShotCoordinator{}
		state := &TripleShotState{
			Coordinators: map[string]*orchestrator.TripleShotCoordinator{
				"target": mapCoord,
			},
			Coordinator: fallbackCoord,
		}

		result := state.GetCoordinatorForGroup("target")
		if result != mapCoord {
			t.Error("expected map coordinator over fallback")
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
